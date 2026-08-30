package rest

import (
	"encoding/json"
	"net/http"

	"github.com/activebook/gllm/data"
)

type GoalRequest struct {
	Session            string   `json:"session,omitempty"`
	Objective          string   `json:"objective,omitempty"`
	AcceptanceCriteria string   `json:"acceptance_criteria,omitempty"`
	Milestones         []string `json:"milestones,omitempty"`
	Action             string   `json:"action,omitempty"`
	MilestoneID        int      `json:"milestone_id,omitempty"`
	MilestoneStatus    string   `json:"milestone_status,omitempty"`
	AddMilestone       string   `json:"add_milestone,omitempty"`
	VerificationNotes  string   `json:"verification_notes,omitempty"`
}

func handleGoal(w http.ResponseWriter, r *http.Request) {
	sessionParam := r.URL.Query().Get("session")

	switch r.Method {
	case http.MethodGet:
		goal, _ := data.GetActiveGoalForSession(sessionParam)
		sendJSON(w, http.StatusOK, map[string]interface{}{
			"session":   sessionParam,
			"goal":      goal,
			"is_active": data.IsGoalActiveForSession(sessionParam),
		})

	case http.MethodPost:
		var req GoalRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendError(w, http.StatusBadRequest, "INVALID_PAYLOAD", "Invalid JSON payload")
			return
		}

		targetSession := req.Session
		if targetSession == "" {
			targetSession = sessionParam
		}

		switch req.Action {
		case "complete", "done":
			if data.CompleteActiveGoalForSession(targetSession, req.VerificationNotes) {
				goal, _ := data.GetActiveGoalForSession(targetSession)
				sendJSON(w, http.StatusOK, map[string]interface{}{
					"session": targetSession,
					"status":  "completed",
					"message": "Goal marked as completed",
					"goal":    goal,
				})
			} else {
				sendError(w, http.StatusNotFound, "NO_ACTIVE_GOAL", "No active goal to complete")
			}

		case "update_milestone":
			if req.MilestoneID > 0 && req.MilestoneStatus != "" {
				var status data.MilestoneStatus
				switch req.MilestoneStatus {
				case "completed", "done":
					status = data.MilestoneCompleted
				case "in_progress", "active":
					status = data.MilestoneInProgress
				default:
					status = data.MilestonePending
				}

				if data.UpdateGoalMilestoneForSession(targetSession, req.MilestoneID, status) {
					goal, _ := data.GetActiveGoalForSession(targetSession)
					sendJSON(w, http.StatusOK, map[string]interface{}{
						"session": targetSession,
						"status":  "updated",
						"message": "Milestone updated",
						"goal":    goal,
					})
				} else {
					sendError(w, http.StatusNotFound, "MILESTONE_NOT_FOUND", "Milestone ID not found")
				}
			} else {
				sendError(w, http.StatusBadRequest, "MISSING_PARAMS", "milestone_id and milestone_status required")
			}

		case "add_milestone":
			if req.AddMilestone != "" {
				id := data.AddGoalMilestoneForSession(targetSession, req.AddMilestone)
				goal, _ := data.GetActiveGoalForSession(targetSession)
				sendJSON(w, http.StatusOK, map[string]interface{}{
					"session":      targetSession,
					"status":       "added",
					"milestone_id": id,
					"goal":         goal,
				})
			} else {
				sendError(w, http.StatusBadRequest, "MISSING_PARAMS", "add_milestone description required")
			}

		default:
			// Default: establish or replace active goal
			if req.Objective == "" {
				sendError(w, http.StatusBadRequest, "MISSING_OBJECTIVE", "objective is required to set a goal")
				return
			}

			goal := data.SetActiveGoalForSession(targetSession, req.Objective, req.AcceptanceCriteria, req.Milestones)
			sendJSON(w, http.StatusCreated, map[string]interface{}{
				"session": targetSession,
				"status":  "created",
				"message": "Active session goal created",
				"goal":    goal,
			})
		}

	case http.MethodDelete:
		data.ClearActiveGoalForSession(sessionParam)
		sendJSON(w, http.StatusOK, map[string]interface{}{
			"session": sessionParam,
			"status":  "cleared",
			"message": "Active session goal cleared",
		})

	default:
		sendError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
	}
}
