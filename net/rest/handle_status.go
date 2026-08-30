package rest

import (
	"net/http"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/service"
)

type StatusResponse struct {
	Agent      *data.AgentConfig           `json:"agent,omitempty"`
	PlanMode   bool                        `json:"plan_mode"`
	YoloMode   bool                        `json:"yolo_mode"`
	TokenUsage *service.TokenUsageSnapshot `json:"token_usage,omitempty"`
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	store := data.NewConfigStore()
	activeAgent := store.GetActiveAgent()
	snapshot, _ := service.GetLatestTokenUsage()

	resp := StatusResponse{
		Agent:      activeAgent,
		PlanMode:   data.GetPlanModeInSession(),
		YoloMode:   data.GetYoloModeInSession(),
		TokenUsage: snapshot,
	}

	sendJSON(w, http.StatusOK, resp)
}
