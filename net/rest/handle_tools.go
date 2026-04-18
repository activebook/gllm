package rest

import (
	"encoding/json"
	"net/http"
	"slices"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/service"
)

// ToolResponse represents the JSON shape for a tool entry.
type ToolResponse struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

func handleTools(w http.ResponseWriter, r *http.Request) {
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

	allToolNames := service.GetAllOpenTools()
	allTools := service.GetOpenToolsFiltered(allToolNames)
	enabledTools := activeAgent.Tools

	resp := make([]ToolResponse, 0, len(allTools))
	for _, t := range allTools {
		resp = append(resp, ToolResponse{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			Enabled:     slices.Contains(enabledTools, t.Function.Name),
		})
	}

	sendJSON(w, http.StatusOK, resp)
}

func handleToolsSwitch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		sendError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	var payload struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.Name == "" {
		sendError(w, http.StatusBadRequest, "INVALID_PAYLOAD", "Tool name and enabled status required")
		return
	}

	store := data.NewConfigStore()
	activeAgent := store.GetActiveAgent()
	if activeAgent == nil {
		sendError(w, http.StatusNotFound, "NO_ACTIVE_AGENT", "No active agent configured")
		return
	}

	// Logic to add/remove tool
	changed := false
	if payload.Enabled {
		if !slices.Contains(activeAgent.Tools, payload.Name) {
			activeAgent.Tools = append(activeAgent.Tools, payload.Name)
			changed = true
		}
	} else {
		if slices.Contains(activeAgent.Tools, payload.Name) {
			activeAgent.Tools = slices.DeleteFunc(activeAgent.Tools, func(t string) bool {
				return t == payload.Name
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
