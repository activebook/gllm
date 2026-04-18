package rest

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/activebook/gllm/service"
)

// SessionResponse is the JSON shape for a single session.
type SessionResponse struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
	ModTime  int64  `json:"mod_time"`
	Empty    bool   `json:"empty"`
}

func handleSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// Pass detectProvider=true, includeSubAgents=true to mirror all functional needs
		sessions, err := service.ListSortedSessions(true, true)
		if err != nil {
			sendError(w, http.StatusInternalServerError, "LIST_ERROR", err.Error())
			return
		}

		resp := make([]SessionResponse, 0, len(sessions))
		for _, s := range sessions {
			resp = append(resp, SessionResponse{
				Name:     s.Name,
				Provider: s.Provider,
				ModTime:  s.ModTime,
				Empty:    s.Empty,
			})
		}

		sendJSON(w, http.StatusOK, resp)
		return
	}

	sendError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
}

func handleSessionDetail(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/v1/sessions/")
	if name == "" {
		sendError(w, http.StatusBadRequest, "INVALID_REQUEST", "Session name is required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		data, err := service.ReadSessionContent(name)
		if err != nil {
			sendError(w, http.StatusNotFound, "NOT_FOUND", "Session not found")
			return
		}
		// Data is in jsonl format, return it as raw text
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)
		w.Write(data)

	case http.MethodDelete:
		err := service.RemoveSession(name)
		if err != nil {
			sendError(w, http.StatusInternalServerError, "DELETE_ERROR", err.Error())
			return
		}
		// 204 No Content
		w.WriteHeader(http.StatusNoContent)

	case http.MethodPost, http.MethodPatch:
		// Map both PATCH /v1/sessions/{name} and POST /v1/sessions/{name}/rename to rename
		isRenameAction := strings.HasSuffix(name, "/rename")
		actualName := name
		if isRenameAction {
			actualName = strings.TrimSuffix(name, "/rename")
		}

		var payload struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.Name == "" {
			sendError(w, http.StatusBadRequest, "INVALID_PAYLOAD", "Target name is required")
			return
		}

		err := service.RenameSession(actualName, payload.Name)
		if err != nil {
			sendError(w, http.StatusInternalServerError, "RENAME_ERROR", err.Error())
			return
		}

		sendJSON(w, http.StatusOK, map[string]string{"name": payload.Name})

	default:
		sendError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
	}
}
