package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/internal/ui"
	"github.com/charmbracelet/lipgloss"
)

const (
	RemoteModelsIndexURL = "https://raw.githubusercontent.com/activebook/models/main/list.json"
	RemoteModelsBaseURL  = "https://raw.githubusercontent.com/activebook/models/main/"
)

// ModelLimits contains context window configuration for a model
type ModelLimits struct {
	ContextWindow   int // Total context window in tokens
	MaxOutputTokens int // Maximum output tokens allowed
}

var DefaultModelLimits = ModelLimits{
	// Default for modern generation models
	ContextWindow:   128000,
	MaxOutputTokens: 8192,
}

// MaxInputTokens calculates the maximum input tokens with a safety buffer.
// The buffer ensures there's always room for the model's response.
func (ml ModelLimits) MaxInputTokens(bufferPercent float64) int {
	if bufferPercent <= 0 || bufferPercent > 1 {
		bufferPercent = 0.8 // Default to 80% if invalid
	}
	// Input tokens and output tokens share the same context window pool.
	return int(float64(ml.ContextWindow) * bufferPercent)
}

// IsModelGemini3 checks if the model name is a Gemini 3 model
func IsModelGemini3(modelName string) bool {
	return strings.Contains(modelName, "gemini-3")
}

// NormalizeModelName extracts the actual model name from a config string that might include a vendor prefix
// e.g. "xiaomi/mimo-v2-flash:free" -> "mimo-v2-flash:free"
func NormalizeModelName(configModelName string) string {
	parts := strings.Split(configModelName, "/")
	return parts[len(parts)-1]
}

var syncLocks sync.Map // To prevent multiple concurrent syncs for the same modelKey

type remoteModelIndexEntry struct {
	Name string `json:"name"`
	File string `json:"file"`
}

type remoteModelDetails struct {
	ContextLength int `json:"context_length"`
	TopProvider   struct {
		MaxCompletionTokens int `json:"max_completion_tokens"`
	} `json:"top_provider"`
}

// SyncModelLimits fetches the latest model constraints from the remote repository
// and updates the local config entry if valid info is found.
// This function operates asynchronously and debounces multiple calls for the same key.
func SyncModelLimits(modelKey, configModelName string) {
	if modelKey == "" || configModelName == "" {
		return
	}

	// Debounce: check if a sync is already in progress for this modelKey
	if _, loaded := syncLocks.LoadOrStore(modelKey, struct{}{}); loaded {
		// Already syncing
		return
	}

	// We don't remove the lock after sync, so it will only sync once per session.
	// Session-level cache to prevent re-triggering for failures
	// defer syncLocks.Delete(modelKey)

	// Clean up configModelName for matching
	normalizedName := NormalizeModelName(configModelName)
	normalizedLower := strings.ToLower(normalizedName)

	// Fast timeout for background tasks to avoid hanging goroutines unnecessarily
	client := &http.Client{Timeout: 10 * time.Second}

	// Fetch index
	resp, err := client.Get(RemoteModelsIndexURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		ui.SendEvent(ui.BannerMsg{Text: getModelFailedBanner(modelKey, err)})
		return
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		ui.SendEvent(ui.BannerMsg{Text: getModelFailedBanner(modelKey, err)})
		return
	}

	var index []remoteModelIndexEntry
	if err := json.Unmarshal(bodyBytes, &index); err != nil {
		ui.SendEvent(ui.BannerMsg{Text: getModelFailedBanner(modelKey, err)})
		return
	}

	var bestMatch *remoteModelIndexEntry

	// Phase 1: Exact Match
	for i, entry := range index {
		if strings.ToLower(entry.Name) == normalizedLower {
			bestMatch = &index[i]
			break
		}
	}

	// Phase 2: Lossy Match (inclusion check) if no exact match
	if bestMatch == nil {
		for i, entry := range index {
			entryLower := strings.ToLower(entry.Name)
			if strings.Contains(normalizedLower, entryLower) || strings.Contains(entryLower, normalizedLower) {
				bestMatch = &index[i]
				break
			}
		}
	}

	if bestMatch == nil {
		ui.SendEvent(ui.BannerMsg{Text: getModelFailedBanner(modelKey, fmt.Errorf("no match found in remote model index for %s", normalizedName))})
		return
	}

	// Fetch detail JSON
	detailURL := fmt.Sprintf("%s%s", RemoteModelsBaseURL, bestMatch.File)
	detailResp, err := client.Get(detailURL)
	if err != nil || detailResp.StatusCode != http.StatusOK {
		if detailResp != nil {
			detailResp.Body.Close()
		}
		ui.SendEvent(ui.BannerMsg{Text: getModelFailedBanner(modelKey, err)})
		return
	}
	defer detailResp.Body.Close()

	detailBytes, err := io.ReadAll(detailResp.Body)
	if err != nil {
		ui.SendEvent(ui.BannerMsg{Text: getModelFailedBanner(modelKey, err)})
		return
	}

	var details remoteModelDetails
	if err := json.Unmarshal(detailBytes, &details); err != nil {
		ui.SendEvent(ui.BannerMsg{Text: getModelFailedBanner(modelKey, err)})
		return
	}

	// Determine final limits
	contextLength := details.ContextLength
	if contextLength <= 0 {
		ui.SendEvent(ui.BannerMsg{Text: getModelFailedBanner(modelKey, fmt.Errorf("invalid ContextLength %d", contextLength))})
		return
	}

	maxOutput := details.TopProvider.MaxCompletionTokens
	if maxOutput <= 0 {
		maxOutput = DefaultModelLimits.MaxOutputTokens
	}

	// Save to config
	store := data.NewConfigStore()
	err = store.SetModelLimits(modelKey, contextLength, maxOutput)
	if err != nil {
		ui.SendEvent(ui.BannerMsg{Text: getModelFailedBanner(modelKey, err)})
	} else {
		// Send banner notification
		ui.SendEvent(ui.BannerMsg{Text: getModelUpdatedBanner(modelKey, contextLength, maxOutput)})
	}
}

// getModelBanner returns a non-intrusive update notification.
func getModelUpdatedBanner(modelKey string, contextLength int, maxOutput int) string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color(data.UpdateModelSuccessHex)).
		Bold(true)
	return style.Render(fmt.Sprintf("* Model %s updated: ContextLength=%d, MaxOutputTokens=%d", modelKey, contextLength, maxOutput))
}

func getModelFailedBanner(modelKey string, err error) string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color(data.UpdateModelFailedHex)).
		Bold(true)
	return style.Render(fmt.Sprintf("* Model %s update failed: %v", modelKey, err))
}
