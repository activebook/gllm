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
	switch r.Method {
	case http.MethodGet:
		getTools(w, r)
	case http.MethodPut:
		updateTools(w, r)
	default:
		sendError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
	}
}

func getTools(w http.ResponseWriter, r *http.Request) {
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

func updateTools(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Tools []string `json:"tools"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		sendError(w, http.StatusBadRequest, "INVALID_PAYLOAD", "List of tools required")
		return
	}

	store := data.NewConfigStore()
	activeAgent := store.GetActiveAgent()
	if activeAgent == nil {
		sendError(w, http.StatusNotFound, "NO_ACTIVE_AGENT", "No active agent configured")
		return
	}

	// Validation: Only allow valid tool names
	allValidTools := service.GetAllOpenTools()
	validSet := make(map[string]bool)
	for _, t := range allValidTools {
		validSet[t] = true
	}

	var validatedTools []string
	for _, t := range payload.Tools {
		if validSet[t] {
			validatedTools = append(validatedTools, t)
		} else {
			sendError(w, http.StatusBadRequest, "INVALID_TOOL", "Unknown tool: "+t)
			return
		}
	}

	activeAgent.Tools = validatedTools

	if err := store.SetAgent(activeAgent.Name, activeAgent); err != nil {
		sendError(w, http.StatusInternalServerError, "SAVE_ERROR", err.Error())
		return
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"tools": activeAgent.Tools,
	})
}
