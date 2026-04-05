package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/internal/event"
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

// stripDateStamp removes trailing date/version stamps from model names.
// Handles patterns like: -2024-05-13, -08-2024, -2512, -0528, -20250514
var dateStampRe = regexp.MustCompile(`(-\d{4}-\d{2}-\d{2}|-\d{2}-\d{4}|-\d{4,8})$`)

func stripDateStamp(name string) string {
	return dateStampRe.ReplaceAllString(name, "")
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

// findBestMatch searches the remote model index for the best matching entry.
// Tiered strategy:
//  1. Exact match (case-insensitive)
//  2. Date-stamp-stripped match (strips trailing date patterns like -2024-05-13, -2512)
//  3. Prefix match: input starts with index entry name — NOT the reverse, to prevent
//     short/generic index entries ("free", "auto", "router") from spuriously matching.
//     Entry name must be >= 6 chars as an additional guard.
//
// Returns nil if no confident match is found.
func findBestModelMatch(name string, index []remoteModelIndexEntry) *remoteModelIndexEntry {
	inputDateStripped := stripDateStamp(name)

	// Phase 1: Exact match
	for i, entry := range index {
		if strings.ToLower(entry.Name) == name {
			return &index[i]
		}
	}

	// Phase 2: Match after stripping trailing date stamps
	// e.g. "gpt-4o-2024-08-06" -> "gpt-4o" matches index entry "gpt-4o"
	// e.g. "command-r-08-2024" -> "command-r" matches index entry "command-r"
	for i, entry := range index {
		entryDateStripped := stripDateStamp(strings.ToLower(entry.Name))
		if entryDateStripped == inputDateStripped {
			return &index[i]
		}
	}

	// Phase 3: Input is a more specific variant of an index entry.
	// e.g. "gpt-4o-mini-2024-07-18" starts with index entry "gpt-4o-mini"
	// Guard: entry must be >= 6 chars to block generic names like "free", "auto", "router"
	for i, entry := range index {
		entryLower := strings.ToLower(entry.Name)
		if len(entryLower) >= 6 && strings.HasPrefix(inputDateStripped, entryLower) {
			return &index[i]
		}
	}

	return nil
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
		event.SendBanner(getModelFailedBanner(modelKey, err))
		return
	}
	defer resp.Body.Close()

	var index []remoteModelIndexEntry
	if err := json.NewDecoder(resp.Body).Decode(&index); err != nil {
		event.SendBanner(getModelFailedBanner(modelKey, err))
		return
	}

	// Find best match
	bestMatch := findBestModelMatch(normalizedLower, index)
	if bestMatch == nil {
		event.SendBanner(getModelFailedBanner(modelKey, fmt.Errorf("no match found in remote model index for %s", normalizedName)))
		return
	}

	// Fetch detail JSON
	detailURL := fmt.Sprintf("%s%s", RemoteModelsBaseURL, bestMatch.File)
	detailResp, err := client.Get(detailURL)
	if err != nil || detailResp.StatusCode != http.StatusOK {
		if detailResp != nil {
			detailResp.Body.Close()
		}
		event.SendBanner(getModelFailedBanner(modelKey, err))
		return
	}
	defer detailResp.Body.Close()

	var details remoteModelDetails
	if err := json.NewDecoder(detailResp.Body).Decode(&details); err != nil {
		event.SendBanner(getModelFailedBanner(modelKey, err))
		return
	}

	// Determine final limits
	contextLength := details.ContextLength
	if contextLength <= 0 {
		event.SendBanner(getModelFailedBanner(modelKey, fmt.Errorf("invalid ContextLength %d", contextLength)))
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
		event.SendBanner(getModelFailedBanner(modelKey, err))
	} else {
		// Send banner notification
		event.SendBanner(getModelUpdatedBanner(modelKey, normalizedName, contextLength, maxOutput))
	}
}

// getModelBanner returns a non-intrusive update notification.
func getModelUpdatedBanner(modelKey string, modelName string, contextLength int, maxOutput int) string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color(data.UpdateModelSuccessHex)).
		Bold(true)
	return style.Render(fmt.Sprintf("✓ Model %s updated: context_length=%d, max_completion_tokens=%d for %s", modelKey, contextLength, maxOutput, modelName))
}

func getModelFailedBanner(modelKey string, err error) string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color(data.UpdateModelFailedHex)).
		Bold(true)
	return style.Render(fmt.Sprintf("✗ Model %s update failed: %v", modelKey, err))
}
