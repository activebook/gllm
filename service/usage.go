package service

import (
	"fmt"
	"strings"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/internal/ui"
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

func (tu *TokenUsage) Render(render ui.Render) {
	// Get the token usage
	// usages := tu.getTokenUsageBox()
	usage := tu.renderLipgloss()
	render.Writeln(usage)
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
	cachedPercentage := 0.0
	if tu.InputTokens > 0 {
		if tu.CachedTokensInPrompt {
			// Cached tokens are included in the input tokens, so we don't need to add them
			cachedPercentage = float64(tu.CachedTokens) / float64(tu.InputTokens) * 100
		} else {
			// Cached tokens are not included in the input tokens, so we need to add them
			cachedPercentage = float64(tu.CachedTokens) / float64(tu.InputTokens+tu.CachedTokens) * 100
		}
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
		valueStyle.Render(fmt.Sprintf("%d", tu.InputTokens)),
	)

	rowOutput := lipgloss.JoinHorizontal(lipgloss.Left,
		labelStyle.Render("Output"),
		valueStyle.Render(fmt.Sprintf("%d", tu.OutputTokens)),
	)

	// Split Cached into two rows
	rowCachedVal := lipgloss.JoinHorizontal(lipgloss.Left,
		labelStyle.Render("Cached"),
		valueStyle.Render(fmt.Sprintf("%d", tu.CachedTokens)),
	)

	// Determine color based on percentage
	var pctColor lipgloss.Color
	if cachedPercentage > 80 {
		pctColor = lipgloss.Color(data.HighCachedHex)
	} else if cachedPercentage > 50 {
		pctColor = lipgloss.Color(data.MedCachedHex) // Greenish/BrightGreenish
	} else if cachedPercentage > 20 {
		pctColor = lipgloss.Color(data.LowCachedHex)
	} else {
		pctColor = lipgloss.Color(data.OffCachedHex)
	}

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
		rowOutput,
		rowCachedVal,
		rowCachedPct,
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
