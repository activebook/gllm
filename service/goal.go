package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/io"
	"github.com/charmbracelet/lipgloss"
)

// ConstructGoalPrompt builds the structured executive steering block injected into the system prompt.
func ConstructGoalPrompt(goal *data.SessionGoal) string {
	if goal == nil || goal.Status != data.GoalStatusActive {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<active-goal>\n")
	sb.WriteString(fmt.Sprintf("OBJECTIVE: %s\n", goal.Objective))
	sb.WriteString(fmt.Sprintf("STATUS: %s\n", strings.ToUpper(string(goal.Status))))

	if goal.AcceptanceCriteria != "" {
		sb.WriteString(fmt.Sprintf("ACCEPTANCE CRITERIA: %s\n", goal.AcceptanceCriteria))
	}

	if len(goal.Milestones) > 0 {
		sb.WriteString("MILESTONES:\n")
		for _, m := range goal.Milestones {
			marker := "[ ]"
			switch m.Status {
			case data.MilestoneCompleted:
				marker = "[x]"
			case data.MilestoneInProgress:
				marker = "[/]"
			}
			sb.WriteString(fmt.Sprintf("  %s %d. %s\n", marker, m.ID, m.Description))
		}
	}

	sb.WriteString(`
EXECUTIVE DIRECTIVES FOR GOAL EXECUTION:
1. This is your primary, invariant objective. Every tool invocation, code edit, and analysis must directly advance this goal.
2. Update milestone statuses as you make progress using the update_goal tool.
3. Do NOT declare task completion or stop execution until all acceptance criteria are empirically validated.
4. When finished, record your verification steps and mark the goal completed via the update_goal tool.
</active-goal>`)

	return sb.String()
}

// RenderGoalCard renders a comprehensive Lipgloss UI card showing the active goal, milestones, and progress.
func RenderGoalCard(goal *data.SessionGoal) string {
	borderColor := lipgloss.Color(data.BorderHex)
	titleColor := lipgloss.Color(data.SectionHex)
	labelColor := lipgloss.Color(data.LabelHex)
	valueColor := lipgloss.Color(data.DetailHex)
	activeColor := lipgloss.Color(data.PlanModeHex)
	completeColor := lipgloss.Color(data.HighCachedHex)
	inProgressColor := lipgloss.Color(data.MedCachedHex)

	termWidth := io.GetTerminalWidth()
	cardWidth := min(78, max(50, termWidth-4))

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(cardWidth).
		Padding(0, 1).
		Margin(0, 0)

	innerWidth := cardWidth - boxStyle.GetHorizontalFrameSize()

	titleStyle := lipgloss.NewStyle().
		Foreground(titleColor).
		Bold(true).
		Width(innerWidth).
		Align(lipgloss.Center)

	secHeaderStyle := lipgloss.NewStyle().
		Foreground(titleColor).
		Bold(true)

	labelW := 12
	labelStyle := lipgloss.NewStyle().Foreground(labelColor).Width(labelW)
	valW := max(20, innerWidth-labelW)
	valStyle := lipgloss.NewStyle().Foreground(valueColor)
	objValStyle := lipgloss.NewStyle().Foreground(valueColor).Bold(true).Width(valW)
	critValStyle := lipgloss.NewStyle().Foreground(valueColor).Width(valW)
	notesStyle := lipgloss.NewStyle().Foreground(valueColor).Width(innerWidth)

	var lines []string
	lines = append(lines, titleStyle.Render("🎯 Session Goal & Milestones"))
	divider := lipgloss.NewStyle().Foreground(borderColor).Render(strings.Repeat("─", innerWidth))
	lines = append(lines, divider)

	if goal == nil {
		lines = append(lines, lipgloss.NewStyle().Foreground(labelColor).Faint(true).Render("No active goal set for this session."))
		lines = append(lines, lipgloss.NewStyle().Foreground(labelColor).Render("Use '/goal <description>' to anchor an invariant objective."))
		return boxStyle.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	}

	// Status line with color and duration
	var statusDisplay string
	switch goal.Status {
	case data.GoalStatusActive:
		elapsed := time.Since(goal.CreatedAt).Round(time.Second)
		statusDisplay = lipgloss.NewStyle().Foreground(activeColor).Bold(true).Render(fmt.Sprintf("ACTIVE (Elapsed: %s)", elapsed))
	case data.GoalStatusCompleted:
		statusDisplay = lipgloss.NewStyle().Foreground(completeColor).Bold(true).Render("COMPLETED ✅")
	case data.GoalStatusAbandoned:
		statusDisplay = lipgloss.NewStyle().Foreground(labelColor).Faint(true).Render("ABANDONED")
	default:
		statusDisplay = valStyle.Render(string(goal.Status))
	}

	lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left, labelStyle.Render("Status:"), statusDisplay))
	lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top, labelStyle.Render("Objective:"), objValStyle.Render(goal.Objective)))

	if goal.AcceptanceCriteria != "" {
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top, labelStyle.Render("Criteria:"), critValStyle.Render(goal.AcceptanceCriteria)))
	}

	// Progress Bar & Milestones
	completed, total, pct := data.CalculateGoalProgress(goal)
	if total > 0 {
		lines = append(lines, divider)

		barWidth := 20
		filledWidth := int((pct / 100.0) * float64(barWidth))
		if filledWidth > barWidth {
			filledWidth = barWidth
		}
		if filledWidth < 0 {
			filledWidth = 0
		}

		var barColor lipgloss.Color
		if pct >= 100 {
			barColor = completeColor
		} else if pct >= 50 {
			barColor = inProgressColor
		} else {
			barColor = activeColor
		}

		filledBar := lipgloss.NewStyle().Foreground(barColor).Render(strings.Repeat("█", filledWidth))
		emptyBar := lipgloss.NewStyle().Foreground(borderColor).Render(strings.Repeat("░", barWidth-filledWidth))
		bar := fmt.Sprintf("[%s%s]", filledBar, emptyBar)

		lines = append(lines, secHeaderStyle.Render("Progress"))
		lines = append(lines, fmt.Sprintf("%s %s (%.1f%% - %d/%d milestones)", bar, valStyle.Render(fmt.Sprintf("%d done", completed)), pct, completed, total))

		lines = append(lines, "")
		lines = append(lines, secHeaderStyle.Render("Milestones"))

		mDescW := max(20, innerWidth-8)
		for _, m := range goal.Milestones {
			var markStr string
			var itemColor lipgloss.Color

			switch m.Status {
			case data.MilestoneCompleted:
				markStr = lipgloss.NewStyle().Foreground(completeColor).Bold(true).Render("[x]")
				itemColor = valueColor
			case data.MilestoneInProgress:
				markStr = lipgloss.NewStyle().Foreground(inProgressColor).Bold(true).Render("[/]")
				itemColor = titleColor
			default:
				markStr = lipgloss.NewStyle().Foreground(labelColor).Render("[ ]")
				itemColor = labelColor
			}

			itemStyle := lipgloss.NewStyle().Foreground(itemColor).Width(mDescW)
			prefix := fmt.Sprintf("  %s %d. ", markStr, m.ID)
			lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top, prefix, itemStyle.Render(m.Description)))
		}
	}

	if goal.VerificationNotes != "" {
		lines = append(lines, divider)
		lines = append(lines, secHeaderStyle.Render("Verification Notes"))
		lines = append(lines, notesStyle.Render(goal.VerificationNotes))
	}

	return boxStyle.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}
