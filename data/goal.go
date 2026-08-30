package data

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// GoalStatus represents the lifecycle state of a session goal.
type GoalStatus string

const (
	GoalStatusActive    GoalStatus = "active"
	GoalStatusCompleted GoalStatus = "completed"
	GoalStatusAbandoned GoalStatus = "abandoned"
)

// MilestoneStatus represents the progress state of an individual milestone.
type MilestoneStatus string

const (
	MilestonePending    MilestoneStatus = "pending"     // [ ]
	MilestoneInProgress MilestoneStatus = "in_progress" // [/]
	MilestoneCompleted  MilestoneStatus = "completed"   // [x]
)

// Milestone is a discrete, verifiable sub-task contributing to the overall goal.
type Milestone struct {
	ID          int             `json:"id"`
	Description string          `json:"description"`
	Status      MilestoneStatus `json:"status"`
}

// SessionGoal captures the overarching invariant objective and its milestone roadmap.
type SessionGoal struct {
	Objective          string      `json:"objective"`
	AcceptanceCriteria string      `json:"acceptance_criteria,omitempty"`
	Status             GoalStatus  `json:"status"`
	Milestones         []Milestone `json:"milestones,omitempty"`
	VerificationNotes  string      `json:"verification_notes,omitempty"`
	CreatedAt          time.Time   `json:"created_at"`
	UpdatedAt          time.Time   `json:"updated_at"`
	CompletedAt        *time.Time  `json:"completed_at,omitempty"`
}

var (
	sessionGoals      = make(map[string]*SessionGoal)
	activeSessionName = "default"
	goalMu            sync.RWMutex
)

// SetActiveSessionName sets the globally focused session name for goal operations.
func SetActiveSessionName(name string) {
	goalMu.Lock()
	defer goalMu.Unlock()
	if name == "" {
		name = "default"
	}
	activeSessionName = strings.Split(name, "::")[0]
}

// GetActiveSessionName returns the globally focused session name.
func GetActiveSessionName() string {
	goalMu.RLock()
	defer goalMu.RUnlock()
	if activeSessionName == "" {
		return "default"
	}
	return activeSessionName
}

func getGoalFilePath(sessionName string) string {
	if sessionName == "" {
		sessionName = "default"
	}
	parts := strings.Split(sessionName, "::")
	return filepath.Join(GetSessionsDirPath(), parts[0], "goal.json")
}

func saveGoalToDisk(sessionName string, goal *SessionGoal) {
	path := getGoalFilePath(sessionName)
	if goal == nil {
		os.Remove(path)
		return
	}
	os.MkdirAll(filepath.Dir(path), 0755)
	if bytes, err := json.MarshalIndent(goal, "", "  "); err == nil {
		os.WriteFile(path, bytes, 0644)
	}
}

func loadGoalFromDisk(sessionName string) *SessionGoal {
	path := getGoalFilePath(sessionName)
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var goal SessionGoal
	if err := json.Unmarshal(bytes, &goal); err != nil {
		return nil
	}
	return &goal
}

// SetActiveGoalForSession establishes or updates the goal for a specific session.
func SetActiveGoalForSession(sessionName string, objective string, criteria string, milestoneDescs []string) *SessionGoal {
	goalMu.Lock()
	defer goalMu.Unlock()

	if sessionName == "" {
		sessionName = activeSessionName
	}
	sessionKey := strings.Split(sessionName, "::")[0]

	now := time.Now()
	milestones := make([]Milestone, 0, len(milestoneDescs))
	for i, desc := range milestoneDescs {
		if desc != "" {
			milestones = append(milestones, Milestone{
				ID:          i + 1,
				Description: desc,
				Status:      MilestonePending,
			})
		}
	}

	goal := &SessionGoal{
		Objective:          objective,
		AcceptanceCriteria: criteria,
		Status:             GoalStatusActive,
		Milestones:         milestones,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	sessionGoals[sessionKey] = goal
	saveGoalToDisk(sessionKey, goal)
	return copyGoal(goal)
}

// SetActiveGoal establishes or updates the active session goal.
func SetActiveGoal(objective string, criteria string, milestoneDescs []string) *SessionGoal {
	return SetActiveGoalForSession(GetActiveSessionName(), objective, criteria, milestoneDescs)
}

// GetActiveGoalForSession retrieves a snapshot of the goal for a specific session.
func GetActiveGoalForSession(sessionName string) (*SessionGoal, bool) {
	goalMu.Lock()
	defer goalMu.Unlock()

	if sessionName == "" {
		sessionName = activeSessionName
	}
	sessionKey := strings.Split(sessionName, "::")[0]

	goal, exists := sessionGoals[sessionKey]
	if !exists || goal == nil {
		goal = loadGoalFromDisk(sessionKey)
		sessionGoals[sessionKey] = goal
	}

	if goal == nil {
		return nil, false
	}

	return copyGoal(goal), true
}

// GetActiveGoal retrieves a snapshot of the current active session goal.
func GetActiveGoal() (*SessionGoal, bool) {
	return GetActiveGoalForSession(GetActiveSessionName())
}

// IsGoalActiveForSession returns true if an active goal is present in the specified session.
func IsGoalActiveForSession(sessionName string) bool {
	goal, ok := GetActiveGoalForSession(sessionName)
	return ok && goal.Status == GoalStatusActive
}

// IsGoalActive returns true if an active goal is present in the current session.
func IsGoalActive() bool {
	return IsGoalActiveForSession(GetActiveSessionName())
}

// UpdateGoalMilestoneForSession modifies the progress status of a specific milestone in a session.
func UpdateGoalMilestoneForSession(sessionName string, id int, status MilestoneStatus) bool {
	goalMu.Lock()
	defer goalMu.Unlock()

	if sessionName == "" {
		sessionName = activeSessionName
	}
	sessionKey := strings.Split(sessionName, "::")[0]

	goal, exists := sessionGoals[sessionKey]
	if !exists || goal == nil {
		goal = loadGoalFromDisk(sessionKey)
		sessionGoals[sessionKey] = goal
	}

	if goal == nil {
		return false
	}

	for i := range goal.Milestones {
		if goal.Milestones[i].ID == id {
			goal.Milestones[i].Status = status
			goal.UpdatedAt = time.Now()
			saveGoalToDisk(sessionKey, goal)
			return true
		}
	}

	return false
}

// UpdateGoalMilestone modifies the progress status of a specific milestone in the current session.
func UpdateGoalMilestone(id int, status MilestoneStatus) bool {
	return UpdateGoalMilestoneForSession(GetActiveSessionName(), id, status)
}

// AddGoalMilestoneForSession appends a new milestone to the session goal.
func AddGoalMilestoneForSession(sessionName string, description string) int {
	goalMu.Lock()
	defer goalMu.Unlock()

	if sessionName == "" {
		sessionName = activeSessionName
	}
	sessionKey := strings.Split(sessionName, "::")[0]

	goal, exists := sessionGoals[sessionKey]
	if !exists || goal == nil {
		goal = loadGoalFromDisk(sessionKey)
		sessionGoals[sessionKey] = goal
	}

	if goal == nil {
		return 0
	}

	nextID := len(goal.Milestones) + 1
	goal.Milestones = append(goal.Milestones, Milestone{
		ID:          nextID,
		Description: description,
		Status:      MilestonePending,
	})
	goal.UpdatedAt = time.Now()
	saveGoalToDisk(sessionKey, goal)

	return nextID
}

// AddGoalMilestone appends a new milestone to the active session goal.
func AddGoalMilestone(description string) int {
	return AddGoalMilestoneForSession(GetActiveSessionName(), description)
}

// CompleteActiveGoalForSession marks the session goal as completed and records verification notes.
func CompleteActiveGoalForSession(sessionName string, verificationNotes string) bool {
	goalMu.Lock()
	defer goalMu.Unlock()

	if sessionName == "" {
		sessionName = activeSessionName
	}
	sessionKey := strings.Split(sessionName, "::")[0]

	goal, exists := sessionGoals[sessionKey]
	if !exists || goal == nil {
		goal = loadGoalFromDisk(sessionKey)
		sessionGoals[sessionKey] = goal
	}

	if goal == nil {
		return false
	}

	now := time.Now()
	goal.Status = GoalStatusCompleted
	goal.VerificationNotes = verificationNotes
	goal.UpdatedAt = now
	goal.CompletedAt = &now

	for i := range goal.Milestones {
		if goal.Milestones[i].Status != MilestoneCompleted {
			goal.Milestones[i].Status = MilestoneCompleted
		}
	}

	saveGoalToDisk(sessionKey, goal)
	return true
}

// CompleteActiveGoal marks the current active goal as completed.
func CompleteActiveGoal(verificationNotes string) bool {
	return CompleteActiveGoalForSession(GetActiveSessionName(), verificationNotes)
}

// ClearActiveGoalForSession removes the goal for a specific session.
func ClearActiveGoalForSession(sessionName string) {
	goalMu.Lock()
	defer goalMu.Unlock()

	if sessionName == "" {
		sessionName = activeSessionName
	}
	sessionKey := strings.Split(sessionName, "::")[0]

	delete(sessionGoals, sessionKey)
	saveGoalToDisk(sessionKey, nil)
}

// ClearActiveGoal removes the current active session goal.
func ClearActiveGoal() {
	ClearActiveGoalForSession(GetActiveSessionName())
}

// CalculateGoalProgress computes the completed milestones, total milestones, and percentage.
func CalculateGoalProgress(goal *SessionGoal) (completed int, total int, percentage float64) {
	if goal == nil || len(goal.Milestones) == 0 {
		if goal != nil && goal.Status == GoalStatusCompleted {
			return 1, 1, 100.0
		}
		return 0, 0, 0.0
	}

	total = len(goal.Milestones)
	for _, m := range goal.Milestones {
		if m.Status == MilestoneCompleted {
			completed++
		}
	}

	percentage = (float64(completed) / float64(total)) * 100.0
	return completed, total, percentage
}

func copyGoal(src *SessionGoal) *SessionGoal {
	if src == nil {
		return nil
	}

	dst := *src
	if src.Milestones != nil {
		dst.Milestones = make([]Milestone, len(src.Milestones))
		copy(dst.Milestones, src.Milestones)
	}
	if src.CompletedAt != nil {
		ca := *src.CompletedAt
		dst.CompletedAt = &ca
	}

	return &dst
}
