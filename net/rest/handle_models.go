package rest

import (
	"encoding/json"
	"net/http"

	"github.com/activebook/gllm/data"
)

// ModelResponse is the JSON shape for a model entry.
type ModelResponse struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Active   bool   `json:"active"`
}

func handleModels(w http.ResponseWriter, r *http.Request) {
	store := data.NewConfigStore()
	if r.Method == http.MethodGet {
		modelsMap := store.GetModels()
		activeAgent := store.GetActiveAgent()

		var activeModelName string
		if activeAgent != nil {
			activeModelName = activeAgent.Model.Name
		}

		resp := make([]ModelResponse, 0, len(modelsMap))
		for name, model := range modelsMap {
			resp = append(resp, ModelResponse{
				Name:     name,
				Provider: model.Provider,
				Active:   name == activeModelName,
			})
		}
		sendJSON(w, http.StatusOK, resp)
		return
	}

	sendError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
}

func handleModelsActive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		sendError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	var payload struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.Name == "" {
		sendError(w, http.StatusBadRequest, "INVALID_PAYLOAD", "Model name is required")
		return
	}

	store := data.NewConfigStore()
	activeAgent := store.GetActiveAgent()
	if activeAgent == nil {
		sendError(w, http.StatusInternalServerError, "NO_ACTIVE_AGENT", "No active agent configured")
		return
	}

	// Check if model exists
	if store.GetModel(payload.Name) == nil {
		sendError(w, http.StatusNotFound, "NOT_FOUND", "Model not found")
		return
	}

	activeAgent.Model.Name = payload.Name
	if err := store.SetAgent(activeAgent.Name, activeAgent); err != nil {
		sendError(w, http.StatusInternalServerError, "UPDATE_ERROR", err.Error())
		return
	}

	sendJSON(w, http.StatusOK, map[string]string{"active": payload.Name})
}
