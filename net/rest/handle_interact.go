package rest

import (
	"encoding/json"
	"net/http"

	"github.com/activebook/gllm/service"
)

// InteractRequest is the body of POST /v1/interact used by the frontend to
// resolve a pending interaction (tool confirm, ask-user, etc.).
type InteractRequest struct {
	ID        string `json:"id"`                  // UUID matching the SSE interaction request event
	Kind      string `json:"kind"`                // "tool_confirm" | "ask_user"
	Approve   string `json:"approve,omitempty"`   // For tool_confirm ("once", "always", "cancel")
	Answer    string `json:"answer,omitempty"`    // For ask_user
	Cancelled bool   `json:"cancelled,omitempty"` // For ask_user: user dismissed the dialog
}

// handleInteract routes the frontend response to the corresponding
// suspended goroutine via InteractionRegistry.
func handleInteract(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	var req InteractRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "INVALID_PAYLOAD", err.Error())
		return
	}

	var resolveErr error
	switch req.Kind {
	case string(service.InteractionKindConfirm):
		resolveErr = service.InteractionRegistry.ResolveConfirm(req.ID, req.Approve)
	case string(service.InteractionKindAskUser):
		resolveErr = service.InteractionRegistry.ResolveAskUser(req.ID, req.Answer, req.Cancelled)
	default:
		sendError(w, http.StatusBadRequest, "UNKNOWN_KIND", "Unknown interaction kind: "+req.Kind)
		return
	}

	if resolveErr != nil {
		sendError(w, http.StatusNotFound, "NOT_FOUND", resolveErr.Error())
		return
	}

	sendJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
