package service

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/io"
	"github.com/charmbracelet/lipgloss"
)

type TokenUsage struct {
	InputTokens   int
	OutputTokens  int
	CachedTokens  int
	ThoughtTokens int
	TotalTokens   int
	// For providers like Anthropic, cached tokens are not included in the prompt tokens
	// OpenAI, OpenChat and Gemini all include cached tokens in the prompt tokens
	CachedTokensInPrompt bool
}

const (
	CachedTokensInPrompt    = true
	CachedTokensNotInPrompt = false
)

func NewTokenUsage() *TokenUsage {
	return &TokenUsage{
		InputTokens:          0,
		OutputTokens:         0,
		CachedTokens:         0,
		ThoughtTokens:        0,
		TotalTokens:          0,
		CachedTokensInPrompt: true,
	}
}

// TokenUsageSnapshot captures an immutable point-in-time record of token consumption.
type TokenUsageSnapshot struct {
	InputTokens          int       `json:"input_tokens"`
	OutputTokens         int       `json:"output_tokens"`
	CachedTokens         int       `json:"cached_tokens"`
	ThoughtTokens        int       `json:"thought_tokens"`
	TotalTokens          int       `json:"total_tokens"`
	CachedTokensInPrompt bool      `json:"cached_tokens_in_prompt"`
	ModelName            string    `json:"model_name"`
	ContextLength        int32     `json:"context_length"`
	MaxOutputTokens      int32     `json:"max_output_tokens"`
	RecordedAt           time.Time `json:"recorded_at"`
}

var (
	latestUsageSnapshot *TokenUsageSnapshot
	latestUsageMu       sync.RWMutex
)

// SetLatestTokenUsage persists the latest turn's token metrics in a thread-safe singleton.
func SetLatestTokenUsage(tu *TokenUsage, modelName string, contextLength, maxOutputTokens int32) {
	if tu == nil || tu.TotalTokens <= 0 {
		return
	}
	latestUsageMu.Lock()
	defer latestUsageMu.Unlock()
	latestUsageSnapshot = &TokenUsageSnapshot{
		InputTokens:          tu.InputTokens,
		OutputTokens:         tu.OutputTokens,
		CachedTokens:         tu.CachedTokens,
		ThoughtTokens:        tu.ThoughtTokens,
		TotalTokens:          tu.TotalTokens,
		CachedTokensInPrompt: tu.CachedTokensInPrompt,
		ModelName:            modelName,
		ContextLength:        contextLength,
		MaxOutputTokens:      maxOutputTokens,
		RecordedAt:           time.Now(),
	}
}

// GetLatestTokenUsage returns a copy of the latest recorded token usage snapshot.
func GetLatestTokenUsage() (*TokenUsageSnapshot, bool) {
	latestUsageMu.RLock()
	defer latestUsageMu.RUnlock()
	if latestUsageSnapshot == nil {
		return nil, false
	}
	copySnap := *latestUsageSnapshot
	return &copySnap, true
}

func (tu *TokenUsage) Render(output io.Output) {
	usage := tu.renderLipgloss()
	output.Writeln(usage)
}

func (tu *TokenUsage) renderLipgloss() string {
	if tu.TotalTokens <= 0 {
		return ""
	}

	// Styles
	borderColor := lipgloss.Color(data.BorderHex) // Theme Border Color
	titleColor := lipgloss.Color(data.SectionHex) // Theme Section Color
	headerColor := lipgloss.Color(data.LabelHex)  // Theme Detail Color
	labelColor := lipgloss.Color(data.LabelHex)   // Theme Detail Color
	valueColor := lipgloss.Color(data.DetailHex)  // Theme Detail Color
	totalColor := lipgloss.Color(data.SectionHex) // Theme Section Color

	// Fallback if bright white is empty (some themes might be weird)
	if data.CurrentTheme.BrightWhite == "" {
		headerColor = lipgloss.Color(data.CurrentTheme.Foreground)
	}

	// Main Box Style
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Margin(0, 0) // No margin to fit in the flow

	// Title
	titleStyle := lipgloss.NewStyle().
		Foreground(titleColor).
		Bold(true).
		MarginBottom(0). // No margin bottom to separate from table
		Align(lipgloss.Center)

	// Column Styles
	colWidth := 12
	labelStyle := lipgloss.NewStyle().Foreground(labelColor).Width(colWidth).PaddingRight(2)
	valueStyle := lipgloss.NewStyle().Foreground(valueColor).Width(colWidth).Align(lipgloss.Right)
	headerStyle := lipgloss.NewStyle().Foreground(headerColor).Bold(true).Width(colWidth).PaddingRight(2)
	headerValStyle := lipgloss.NewStyle().Foreground(headerColor).Bold(true).Width(colWidth).Align(lipgloss.Right)

	// Data preparation
	totalInput := 0
	uncached := 0
	if tu.CachedTokensInPrompt {
		totalInput = tu.InputTokens
		uncached = tu.InputTokens - tu.CachedTokens
	} else {
		totalInput = tu.InputTokens + tu.CachedTokens
		uncached = tu.InputTokens
	}

	cachedPercentage := 0.0
	if totalInput > 0 {
		cachedPercentage = float64(tu.CachedTokens) / float64(totalInput) * 100
	}

	// Headers
	headers := lipgloss.JoinHorizontal(lipgloss.Left,
		headerStyle.Bold(true).Render("Type"),
		headerValStyle.Bold(true).Render("Count"),
	)

	// underline
	underline := lipgloss.NewStyle().Foreground(borderColor).Render(strings.Repeat("─", colWidth*2))

	// Rows
	rowInput := lipgloss.JoinHorizontal(lipgloss.Left,
		labelStyle.Render("Input"),
		valueStyle.Render(fmt.Sprintf("%d", totalInput)),
	)

	rowUncached := lipgloss.JoinHorizontal(lipgloss.Left,
		labelStyle.Render("Uncached"),
		valueStyle.Foreground(labelColor).Render(fmt.Sprintf("%d", uncached)),
	)

	rowOutput := lipgloss.JoinHorizontal(lipgloss.Left,
		labelStyle.Render("Output"),
		valueStyle.Foreground(labelColor).Render(fmt.Sprintf("%d", tu.OutputTokens)),
	)

	// Split Cached into two rows
	rowCachedVal := lipgloss.JoinHorizontal(lipgloss.Left,
		labelStyle.Render("Cached"),
		valueStyle.Render(fmt.Sprintf("%d", tu.CachedTokens)),
	)

	// Determine color based on percentage
	pctColor := GetCachedPercentageColor(cachedPercentage)

	rowCachedPct := lipgloss.JoinHorizontal(lipgloss.Left,
		labelStyle.Render(""),
		valueStyle.Foreground(pctColor).Render(fmt.Sprintf("(%.1f%%)", cachedPercentage)),
	)

	rowThought := lipgloss.JoinHorizontal(lipgloss.Left,
		labelStyle.Render("Thought"),
		valueStyle.Render(fmt.Sprintf("%d", tu.ThoughtTokens)),
	)

	rowTotal := lipgloss.JoinHorizontal(lipgloss.Left,
		labelStyle.Bold(true).Render("Total"),
		valueStyle.Bold(true).Foreground(totalColor).Render(fmt.Sprintf("%d", tu.TotalTokens)),
	)

	block := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render("Token Usage"),
		underline,
		headers,
		underline,
		rowInput,
		rowUncached,
		rowCachedVal,
		rowCachedPct,
		rowOutput,
		rowThought,
		underline,
		rowTotal,
	)

	return boxStyle.Render(block)
}

func (tu *TokenUsage) RecordTokenUsage(input, output, cached, thought, total int) {
	tu.InputTokens += input
	tu.OutputTokens += output
	tu.CachedTokens += cached
	tu.ThoughtTokens += thought
	tu.TotalTokens += total
}

// GetCachedPercentageColor determines the semantic color for cache hit percentage according to active theme tiers.
func GetCachedPercentageColor(cachedPercentage float64) lipgloss.Color {
	if cachedPercentage > 80 {
		return lipgloss.Color(data.HighCachedHex)
	} else if cachedPercentage > 50 {
		return lipgloss.Color(data.MedCachedHex)
	} else if cachedPercentage > 20 {
		return lipgloss.Color(data.LowCachedHex)
	}
	return lipgloss.Color(data.OffCachedHex)
}

// getThinkingLevelColor returns the lipgloss color corresponding to the active theme's thinking heatmap.
func getThinkingLevelColor(level ThinkingLevel) lipgloss.Color {
	switch level {
	case ThinkingLevelHigh:
		if data.CurrentTheme.Green != "" {
			return lipgloss.Color(data.CurrentTheme.Green)
		}
		return lipgloss.Color(data.HighCachedHex)
	case ThinkingLevelMedium:
		if data.CurrentTheme.Blue != "" {
			return lipgloss.Color(data.CurrentTheme.Blue)
		}
		return lipgloss.Color(data.SectionHex)
	case ThinkingLevelLow:
		if data.CurrentTheme.Yellow != "" {
			return lipgloss.Color(data.CurrentTheme.Yellow)
		}
		return lipgloss.Color(data.MedCachedHex)
	case ThinkingLevelMinimal:
		if data.CurrentTheme.Red != "" {
			return lipgloss.Color(data.CurrentTheme.Red)
		}
		return lipgloss.Color(data.LowCachedHex)
	case ThinkingLevelOff:
		fallthrough
	default:
		if data.OffCachedHex != "" {
			return lipgloss.Color(data.OffCachedHex)
		}
		return lipgloss.Color(data.DetailHex)
	}
}

// RenderStatusCard renders a comprehensive Lipgloss UI card showing status and token metrics.
func RenderStatusCard(snapshot *TokenUsageSnapshot, agent *data.AgentConfig, sessionName string) string {
	borderColor := lipgloss.Color(data.BorderHex)
	titleColor := lipgloss.Color(data.SectionHex)
	labelColor := lipgloss.Color(data.LabelHex)
	valueColor := lipgloss.Color(data.DetailHex)
	totalColor := lipgloss.Color(data.SectionHex)

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

	metaLabelW := 10
	metaLabelStyle := lipgloss.NewStyle().Foreground(labelColor).Width(metaLabelW)
	valW := max(20, innerWidth-metaLabelW)
	valStyle := lipgloss.NewStyle().Foreground(valueColor).Width(valW)

	var lines []string
	lines = append(lines, titleStyle.Render("System Status & Token Usage"))
	divider := lipgloss.NewStyle().Foreground(borderColor).Render(strings.Repeat("─", innerWidth))
	lines = append(lines, divider)

	// Agent & Model Section
	if agent != nil {
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left, metaLabelStyle.Render("Agent:"), valStyle.Bold(true).Render(agent.Name)))

		modelDisplay := agent.Model.Model
		if modelDisplay == "" {
			modelDisplay = agent.Model.Name
		}
		if agent.Model.Provider != "" {
			modelDisplay = fmt.Sprintf("%s (%s)", modelDisplay, agent.Model.Provider)
		}
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left, metaLabelStyle.Render("Model:"), valStyle.Render(modelDisplay)))

		thinkLvl := ParseThinkingLevel(agent.Think)
		thinkColor := getThinkingLevelColor(thinkLvl)
		thinkDisplay := lipgloss.NewStyle().Foreground(thinkColor).Render(thinkLvl.String())
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left, metaLabelStyle.Render("Think:"), thinkDisplay))

		isPlan := data.GetPlanModeInSession()
		var planDisplay string
		if isPlan {
			planDisplay = lipgloss.NewStyle().Foreground(lipgloss.Color(data.PlanModeHex)).Bold(true).Render("ON")
		} else {
			planDisplay = lipgloss.NewStyle().Foreground(lipgloss.Color(data.OffCachedHex)).Render("OFF")
		}
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left, metaLabelStyle.Render("Plan:"), planDisplay))

		isYolo := data.GetYoloModeInSession()
		var yoloDisplay string
		if isYolo {
			yoloDisplay = lipgloss.NewStyle().Foreground(lipgloss.Color(data.YoloModeHex)).Bold(true).Render("ON")
		} else {
			yoloDisplay = lipgloss.NewStyle().Foreground(lipgloss.Color(data.OffCachedHex)).Render("OFF")
		}
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left, metaLabelStyle.Render("YOLO:"), yoloDisplay))
	}

	if sessionName != "" {
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left, metaLabelStyle.Render("Session:"), valStyle.Render(sessionName)))
	}

	// Active Goal Section
	if goal, ok := data.GetActiveGoal(); ok && goal.Status == data.GoalStatusActive {
		completed, total, pct := data.CalculateGoalProgress(goal)
		var goalSummary string
		if total > 0 {
			goalSummary = fmt.Sprintf("%s (%d/%d · %.0f%%)", goal.Objective, completed, total, pct)
		} else {
			goalSummary = goal.Objective
		}
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left, metaLabelStyle.Render("Goal:"), valStyle.Foreground(lipgloss.Color(data.PlanModeHex)).Bold(true).Render(goalSummary)))
	}

	// Token Usage & Context Utilization
	if snapshot != nil && snapshot.TotalTokens > 0 {
		lines = append(lines, divider)

		// Context Utilization Bar
		totalInput := snapshot.InputTokens
		if !snapshot.CachedTokensInPrompt {
			totalInput += snapshot.CachedTokens
		}

		if snapshot.ContextLength > 0 {
			ctxLen := int(snapshot.ContextLength)
			ratio := float64(totalInput) / float64(ctxLen)
			pct := ratio * 100.0
			if pct > 100.0 {
				pct = 100.0
			}

			barWidth := 20
			filledWidth := int(ratio * float64(barWidth))
			if filledWidth > barWidth {
				filledWidth = barWidth
			}
			if filledWidth < 0 {
				filledWidth = 0
			}

			var barColor lipgloss.Color
			if pct >= 80 {
				barColor = lipgloss.Color(data.HighCachedHex) // alert / high
			} else if pct >= 50 {
				barColor = lipgloss.Color(data.MedCachedHex)
			} else {
				barColor = lipgloss.Color(data.LowCachedHex)
			}

			filledBar := lipgloss.NewStyle().Foreground(barColor).Render(strings.Repeat("█", filledWidth))
			emptyBar := lipgloss.NewStyle().Foreground(borderColor).Render(strings.Repeat("░", barWidth-filledWidth))
			bar := fmt.Sprintf("[%s%s]", filledBar, emptyBar)

			lines = append(lines, secHeaderStyle.Render("Context Utilization"))
			lines = append(lines, fmt.Sprintf("%s %s (%.1f%% of %dk)", bar, valStyle.Render(fmt.Sprintf("%d tokens", totalInput)), pct, ctxLen/1000))
		}

		lines = append(lines, "")
		lines = append(lines, secHeaderStyle.Render("Latest Turn Token Breakdown"))

		colW := 14
		lStyle := lipgloss.NewStyle().Foreground(labelColor).Width(colW)
		vStyle := lipgloss.NewStyle().Foreground(valueColor).Width(colW).Align(lipgloss.Right)

		uncached := snapshot.InputTokens
		if snapshot.CachedTokensInPrompt {
			uncached = snapshot.InputTokens - snapshot.CachedTokens
		}
		cachedPct := 0.0
		if totalInput > 0 {
			cachedPct = float64(snapshot.CachedTokens) / float64(totalInput) * 100.0
		}

		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left, lStyle.Render("Input:"), vStyle.Render(fmt.Sprintf("%d", totalInput))))
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left, lStyle.Render("  Uncached:"), vStyle.Foreground(labelColor).Render(fmt.Sprintf("%d", uncached))))
		if snapshot.CachedTokens > 0 {
			pctColor := GetCachedPercentageColor(cachedPct)
			pctStr := lipgloss.NewStyle().Foreground(pctColor).Render(fmt.Sprintf("(%.1f%%)", cachedPct))
			cachedDisplay := fmt.Sprintf("%d %s", snapshot.CachedTokens, pctStr)
			lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left, lStyle.Render("  Cached:"), lipgloss.NewStyle().Width(colW).Align(lipgloss.Right).Render(cachedDisplay)))
		}
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left, lStyle.Render("Output:"), vStyle.Render(fmt.Sprintf("%d", snapshot.OutputTokens))))
		if snapshot.ThoughtTokens > 0 {
			lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left, lStyle.Render("Thought:"), vStyle.Render(fmt.Sprintf("%d", snapshot.ThoughtTokens))))
		}
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left, lStyle.Bold(true).Render("Total Tokens:"), vStyle.Bold(true).Foreground(totalColor).Render(fmt.Sprintf("%d", snapshot.TotalTokens))))

		if !snapshot.RecordedAt.IsZero() {
			timeStr := snapshot.RecordedAt.Format("15:04:05")
			lines = append(lines, lipgloss.NewStyle().Foreground(labelColor).Faint(true).Render(fmt.Sprintf("Recorded at: %s", timeStr)))
		}
	} else {
		lines = append(lines, divider)
		lines = append(lines, lipgloss.NewStyle().Foreground(labelColor).Faint(true).Render("No turn token usage recorded in this session yet."))
	}

	return boxStyle.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}
