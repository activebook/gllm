package rest

import (
	"encoding/json"
	"net/http"

	"github.com/activebook/gllm/data"
)

// AgentResponse is the JSON shape for an agent entry.
type AgentResponse struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Model       string `json:"model"`
	Active      bool   `json:"active"`
}

func handleAgents(w http.ResponseWriter, r *http.Request) {
	store := data.NewConfigStore()
	if r.Method == http.MethodGet {
		agentsMap := store.GetAllAgents()
		activeAgentName := store.GetActiveAgentName()

		resp := make([]AgentResponse, 0, len(agentsMap))
		for name, agent := range agentsMap {
			resp = append(resp, AgentResponse{
				Name:        name,
				Description: agent.Description,
				Model:       agent.Model.Name,
				Active:      name == activeAgentName,
			})
		}
		sendJSON(w, http.StatusOK, resp)
		return
	}

	sendError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
}

func handleAgentsActive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		sendError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	var payload struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.Name == "" {
		sendError(w, http.StatusBadRequest, "INVALID_PAYLOAD", "Agent name is required")
		return
	}

	store := data.NewConfigStore()
	if store.GetAgent(payload.Name) == nil {
		sendError(w, http.StatusNotFound, "NOT_FOUND", "Agent not found")
		return
	}

	if err := store.SetActiveAgent(payload.Name); err != nil {
		sendError(w, http.StatusInternalServerError, "UPDATE_ERROR", err.Error())
		return
	}

	sendJSON(w, http.StatusOK, map[string]string{"active": payload.Name})
}
