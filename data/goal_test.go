package data

import (
	"testing"
)

func TestSessionGoalLifecycle(t *testing.T) {
	SetActiveSessionName("test-lifecycle")
	defer ClearActiveGoal()

	ClearActiveGoal()

	if IsGoalActive() {
		t.Errorf("expected no active goal initially")
	}

	goal := SetActiveGoal("Implement user authentication", "Must pass all auth unit tests", []string{
		"Create token parser",
		"Add auth middleware",
	})

	if goal == nil || goal.Objective != "Implement user authentication" {
		t.Fatalf("expected goal to be created with objective 'Implement user authentication'")
	}
	if len(goal.Milestones) != 2 {
		t.Fatalf("expected 2 milestones, got %d", len(goal.Milestones))
	}

	if !IsGoalActive() {
		t.Errorf("expected goal to be active")
	}

	// Update milestone 1
	ok := UpdateGoalMilestone(1, MilestoneCompleted)
	if !ok {
		t.Errorf("failed to update milestone 1")
	}

	// Add milestone 3
	id := AddGoalMilestone("Add integration test")
	if id != 3 {
		t.Errorf("expected new milestone ID 3, got %d", id)
	}

	// Check progress
	curGoal, _ := GetActiveGoal()
	completed, total, pct := CalculateGoalProgress(curGoal)
	if completed != 1 || total != 3 || pct < 33.0 || pct > 34.0 {
		t.Errorf("expected 1/3 (33.3%%), got %d/%d (%.1f%%)", completed, total, pct)
	}

	// Complete goal
	ok = CompleteActiveGoal("All tests passed with 100% code coverage")
	if !ok {
		t.Errorf("failed to complete active goal")
	}

	completedGoal, _ := GetActiveGoal()
	if completedGoal.Status != GoalStatusCompleted {
		t.Errorf("expected goal status completed, got %s", completedGoal.Status)
	}
	if completedGoal.VerificationNotes != "All tests passed with 100% code coverage" {
		t.Errorf("unexpected verification notes: %s", completedGoal.VerificationNotes)
	}

	// Clear goal
	ClearActiveGoal()
	if IsGoalActive() {
		t.Errorf("expected goal to be cleared")
	}
}

func TestMultiSessionGoalIsolation(t *testing.T) {
	sessionA := "session-alpha"
	sessionB := "session-beta"

	defer ClearActiveGoalForSession(sessionA)
	defer ClearActiveGoalForSession(sessionB)

	// Set goal for session A
	SetActiveGoalForSession(sessionA, "Goal Alpha", "Criteria A", []string{"A1", "A2"})
	// Set goal for session B
	SetActiveGoalForSession(sessionB, "Goal Beta", "Criteria B", []string{"B1"})

	// Verify session A
	goalA, okA := GetActiveGoalForSession(sessionA)
	if !okA || goalA.Objective != "Goal Alpha" || len(goalA.Milestones) != 2 {
		t.Fatalf("Session A goal isolation failed: %+v", goalA)
	}

	// Verify session B
	goalB, okB := GetActiveGoalForSession(sessionB)
	if !okB || goalB.Objective != "Goal Beta" || len(goalB.Milestones) != 1 {
		t.Fatalf("Session B goal isolation failed: %+v", goalB)
	}

	// Update session A milestone
	UpdateGoalMilestoneForSession(sessionA, 1, MilestoneCompleted)
	goalAUpdated, _ := GetActiveGoalForSession(sessionA)
	if goalAUpdated.Milestones[0].Status != MilestoneCompleted {
		t.Errorf("expected session A milestone 1 to be completed")
	}

	// Ensure session B milestone remains pending
	goalBCheck, _ := GetActiveGoalForSession(sessionB)
	if goalBCheck.Milestones[0].Status != MilestonePending {
		t.Errorf("session B milestone was incorrectly modified by session A update")
	}

	// Clear session A
	ClearActiveGoalForSession(sessionA)
	if IsGoalActiveForSession(sessionA) {
		t.Errorf("expected session A goal to be cleared")
	}

	// Ensure session B goal is still active
	if !IsGoalActiveForSession(sessionB) {
		t.Errorf("session B goal was incorrectly cleared by session A clear")
	}
}
