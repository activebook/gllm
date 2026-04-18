package rest

import (
	"encoding/json"
	"net/http"
	"slices"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/service"
)

// CapabilityResponse represents the JSON shape for a capability entry.
type CapabilityResponse struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

func handleCapabilities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	store := data.NewConfigStore()
	activeAgent := store.GetActiveAgent()
	if activeAgent == nil {
		sendError(w, http.StatusNotFound, "NO_ACTIVE_AGENT", "No active agent configured")
		return
	}

	allCaps := service.GetAllEmbeddingCapabilities()
	enabledCaps := activeAgent.Capabilities

	resp := make([]CapabilityResponse, 0, len(allCaps))
	for _, c := range allCaps {
		resp = append(resp, CapabilityResponse{
			Name:        c,
			Title:       service.GetCapabilityTitle(c),
			Description: service.GetCapabilityDescription(c),
			Enabled:     slices.Contains(enabledCaps, c),
		})
	}

	sendJSON(w, http.StatusOK, resp)
}

func handleCapabilitiesSwitch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		sendError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	var payload struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.Name == "" {
		sendError(w, http.StatusBadRequest, "INVALID_PAYLOAD", "Capability name and enabled status required")
		return
	}

	store := data.NewConfigStore()
	activeAgent := store.GetActiveAgent()
	if activeAgent == nil {
		sendError(w, http.StatusNotFound, "NO_ACTIVE_AGENT", "No active agent configured")
		return
	}

	// Logic to add/remove capability
	changed := false
	if payload.Enabled {
		if !slices.Contains(activeAgent.Capabilities, payload.Name) {
			activeAgent.Capabilities = append(activeAgent.Capabilities, payload.Name)
			changed = true
		}
	} else {
		if slices.Contains(activeAgent.Capabilities, payload.Name) {
			activeAgent.Capabilities = slices.DeleteFunc(activeAgent.Capabilities, func(c string) bool {
				return c == payload.Name
			})
			changed = true
		}
	}

	if changed {
		if err := store.SetAgent(activeAgent.Name, activeAgent); err != nil {
			sendError(w, http.StatusInternalServerError, "SAVE_ERROR", err.Error())
			return
		}
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"name":    payload.Name,
		"enabled": payload.Enabled,
	})
}
