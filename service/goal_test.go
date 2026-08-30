package service

import (
	"strings"
	"testing"

	"github.com/activebook/gllm/data"
)

func TestConstructGoalPrompt(t *testing.T) {
	data.ClearActiveGoal()

	// 1. Nil goal
	if prompt := ConstructGoalPrompt(nil); prompt != "" {
		t.Errorf("expected empty prompt for nil goal, got: %s", prompt)
	}

	// 2. Active goal
	data.SetActiveGoal("Refactor telemetry pipeline", "Must achieve 100% test pass", []string{
		"Extract interfaces",
		"Write unit tests",
	})
	data.UpdateGoalMilestone(1, data.MilestoneCompleted)

	goal, _ := data.GetActiveGoal()
	prompt := ConstructGoalPrompt(goal)
	if !strings.Contains(prompt, "<active-goal>") {
		t.Errorf("expected <active-goal> tag in prompt")
	}
	if !strings.Contains(prompt, "Refactor telemetry pipeline") {
		t.Errorf("expected objective in prompt")
	}
	if !strings.Contains(prompt, "[x] 1. Extract interfaces") {
		t.Errorf("expected completed milestone in prompt")
	}
	if !strings.Contains(prompt, "[ ] 2. Write unit tests") {
		t.Errorf("expected pending milestone in prompt")
	}
}

func TestRenderGoalCard(t *testing.T) {
	data.ClearActiveGoal()

	// 1. Empty state
	emptyCard := RenderGoalCard(nil)
	if !strings.Contains(emptyCard, "No active goal set") {
		t.Errorf("expected empty state message in card")
	}

	// 2. Populated state
	goal := data.SetActiveGoal("Implement /goal command", "All tests pass", []string{
		"Design state model",
		"Add renderer",
	})
	data.UpdateGoalMilestone(1, data.MilestoneCompleted)

	card := RenderGoalCard(goal)
	if !strings.Contains(card, "Implement /goal command") {
		t.Errorf("expected objective in card")
	}
	if !strings.Contains(card, "Progress") {
		t.Errorf("expected Progress section in card")
	}
	if !strings.Contains(card, "Milestones") {
		t.Errorf("expected Milestones section in card")
	}
}
