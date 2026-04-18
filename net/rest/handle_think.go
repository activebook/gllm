package rest

import (
	"encoding/json"
	"net/http"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/service"
)

// ThinkResponse provides the current status and options for thinking mode.
type ThinkResponse struct {
	Level   string        `json:"level"`
	Display string        `json:"display"`
	Options []ThinkOption `json:"options"`
}

// ThinkOption represents a selectable thinking level.
type ThinkOption struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

func handleThink(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getThink(w, r)
	case http.MethodPut:
		updateThink(w, r)
	default:
		sendError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
	}
}

func getThink(w http.ResponseWriter, r *http.Request) {
	store := data.NewConfigStore()
	activeAgent := store.GetActiveAgent()
	if activeAgent == nil {
		sendError(w, http.StatusNotFound, "NO_ACTIVE_AGENT", "No active agent configured")
		return
	}

	currentLevel := service.ParseThinkingLevel(activeAgent.Think)

	// Build options with metadata
	options := []ThinkOption{
		{ID: "off", Label: "Off", Description: "Disable thinking mode"},
		{ID: "minimal", Label: "Minimal", Description: "Minimal reasoning effort"},
		{ID: "low", Label: "Low", Description: "Low reasoning effort"},
		{ID: "medium", Label: "Medium", Description: "Moderate reasoning effort"},
		{ID: "high", Label: "High", Description: "Maximum reasoning effort"},
	}

	sendJSON(w, http.StatusOK, ThinkResponse{
		Level:   currentLevel.String(),
		Display: string(currentLevel), // Simple string for initial response
		Options: options,
	})
}

func updateThink(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Level string `json:"level"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		sendError(w, http.StatusBadRequest, "INVALID_PAYLOAD", "Thinking level required")
		return
	}

	store := data.NewConfigStore()
	activeAgent := store.GetActiveAgent()
	if activeAgent == nil {
		sendError(w, http.StatusNotFound, "NO_ACTIVE_AGENT", "No active agent configured")
		return
	}

	// Validate and parse level
	level := service.ParseThinkingLevel(payload.Level)
	activeAgent.Think = level.String()

	if err := store.SetAgent(activeAgent.Name, activeAgent); err != nil {
		sendError(w, http.StatusInternalServerError, "SAVE_ERROR", err.Error())
		return
	}

	sendJSON(w, http.StatusOK, map[string]string{
		"level": level.String(),
	})
}
