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
	switch r.Method {
	case http.MethodGet:
		getCapabilities(w, r)
	case http.MethodPut:
		updateCapabilities(w, r)
	default:
		sendError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
	}
}

func getCapabilities(w http.ResponseWriter, r *http.Request) {
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

func updateCapabilities(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Capabilities []string `json:"capabilities"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		sendError(w, http.StatusBadRequest, "INVALID_PAYLOAD", "List of capabilities required")
		return
	}

	store := data.NewConfigStore()
	activeAgent := store.GetActiveAgent()
	if activeAgent == nil {
		sendError(w, http.StatusNotFound, "NO_ACTIVE_AGENT", "No active agent configured")
		return
	}

	// Validation: Only allow valid capability names
	allValidCaps := service.GetAllEmbeddingCapabilities()
	validSet := make(map[string]bool)
	for _, c := range allValidCaps {
		validSet[c] = true
	}

	var validatedCaps []string
	for _, c := range payload.Capabilities {
		if validSet[c] {
			validatedCaps = append(validatedCaps, c)
		} else {
			sendError(w, http.StatusBadRequest, "INVALID_CAPABILITY", "Unknown capability: "+c)
			return
		}
	}

	activeAgent.Capabilities = validatedCaps

	if err := store.SetAgent(activeAgent.Name, activeAgent); err != nil {
		sendError(w, http.StatusInternalServerError, "SAVE_ERROR", err.Error())
		return
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"capabilities": activeAgent.Capabilities,
	})
}
