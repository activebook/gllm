package rest

import (
	"encoding/json"
	"net/http"

	"github.com/activebook/gllm/data"
)

// SkillResponse represent the JSON shape for a skill entry.
type SkillResponse struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

func handleSkills(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getSkills(w, r)
	case http.MethodPut:
		updateSkills(w, r)
	default:
		sendError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
	}
}

func getSkills(w http.ResponseWriter, r *http.Request) {
	skills, err := data.ScanSkills()
	if err != nil {
		sendError(w, http.StatusInternalServerError, "SCAN_ERROR", err.Error())
		return
	}

	settingsStore := data.GetSettingsStore()
	resp := make([]SkillResponse, 0, len(skills))
	for _, s := range skills {
		resp = append(resp, SkillResponse{
			Name:        s.Name,
			Description: s.Description,
			Enabled:     !settingsStore.IsSkillDisabled(s.Name),
		})
	}

	sendJSON(w, http.StatusOK, resp)
}

func updateSkills(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Names []string `json:"names"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		sendError(w, http.StatusBadRequest, "INVALID_PAYLOAD", "List of skill names required")
		return
	}

	// Get all installed skills to validate input
	installed, err := data.ScanSkills()
	if err != nil {
		sendError(w, http.StatusInternalServerError, "SCAN_ERROR", err.Error())
		return
	}

	validSet := make(map[string]bool)
	for _, s := range installed {
		validSet[s.Name] = true
	}

	// Validate all provided names
	for _, name := range payload.Names {
		if !validSet[name] {
			sendError(w, http.StatusBadRequest, "INVALID_SKILL", "Unknown skill: "+name)
			return
		}
	}

	settingsStore := data.GetSettingsStore()
	payloadSet := make(map[string]bool)
	for _, name := range payload.Names {
		payloadSet[name] = true
	}

	// Update the global settings store
	for _, s := range installed {
		if payloadSet[s.Name] {
			_ = settingsStore.EnableSkill(s.Name)
		} else {
			_ = settingsStore.DisableSkill(s.Name)
		}
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"enabled": payload.Names,
	})
}
