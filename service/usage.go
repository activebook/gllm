package service

import (
	"fmt"
)

type TokenUsage struct {
	InputTokens   int
	OutputTokens  int
	CachedTokens  int
	ThoughtTokens int
	TotalTokens   int
}

const ()

func NewTokenUsage() *TokenUsage {
	return &TokenUsage{
		InputTokens:   0,
		OutputTokens:  0,
		CachedTokens:  0,
		ThoughtTokens: 0,
		TotalTokens:   0,
	}
}

func (tu *TokenUsage) getTokenUsage() string {
	if tu.TotalTokens > 0 {
		return fmt.Sprintf(
			"\n"+bbColor+
				"┌───────────────┐\n"+
				"│"+resetColor+"  Token Usage"+resetColor+bbColor+"  │"+"\n"+
				"├───────────────┴───────────────────────────────────────────────────────────────────┐\n"+
				"│"+resetColor+" Input: %s%6d%s "+bbColor+"│"+resetColor+" Output: %s%6d%s "+bbColor+"│"+resetColor+" Cached: %s%6d%s "+bbColor+"│"+resetColor+" Thought: %s%6d%s "+bbColor+"│"+resetColor+" Total: %s%6d%s "+bbColor+"│"+resetColor+"\n"+bbColor+
				"└───────────────────────────────────────────────────────────────────────────────────┘"+
				resetColor,
			cyanColor, tu.InputTokens, resetColor,
			cyanColor, tu.OutputTokens, resetColor,
			cyanColor, tu.CachedTokens, resetColor,
			cyanColor, tu.ThoughtTokens, resetColor,
			cyanColor, tu.TotalTokens, resetColor,
		)
	}
	return ""
}

func (tu *TokenUsage) getTokenUsageTip() string {
	if tu.TotalTokens > 0 {
		return fmt.Sprintf(
			bbColor+"\n"+
				"┌───────────────────────────────────────────────────────────────────────────────────┐\n"+
				"│"+resetColor+cyanColor+" Token Usage"+resetColor+bbColor+"                                                                 │\n"+
				"│"+resetColor+" Input: %s%6d%s "+bbColor+"│"+resetColor+" Output: %s%6d%s "+bbColor+"│"+resetColor+" Cached: %s%6d%s "+bbColor+"│"+resetColor+" Thought: %s%6d%s "+bbColor+"│"+resetColor+" Total: %s%6d%s "+bbColor+"│"+resetColor+"\n"+bbColor+
				"└───────────────────────────────────────────────────────────────────────────────────┘"+
				resetColor,
			cyanColor, tu.InputTokens, resetColor,
			cyanColor, tu.OutputTokens, resetColor,
			cyanColor, tu.CachedTokens, resetColor,
			cyanColor, tu.ThoughtTokens, resetColor,
			cyanColor, tu.TotalTokens, resetColor,
		)
	}
	return ""
}

func (tu *TokenUsage) getTokenUsageBox() string {
	if tu.TotalTokens > 0 {
		return fmt.Sprintf(
			bbColor+"\n"+
				"┌───────────────┬────────────┐\n"+
				"│ Token Type    │   Count    │\n"+
				"├───────────────┼────────────┤\n"+
				"│ Input         │ %s%10d%s"+bbColor+" │\n"+
				"│ Output        │ %s%10d%s"+bbColor+" │\n"+
				"│ Cached        │ %s%10d%s"+bbColor+" │\n"+
				"│ Thought       │ %s%10d%s"+bbColor+" │\n"+
				"├───────────────┼────────────┤\n"+
				"│ Total         │ %s%10d%s"+bbColor+" │\n"+
				"└───────────────┴────────────┘"+
				resetColor,
			cyanColor, tu.InputTokens, resetColor,
			cyanColor, tu.OutputTokens, resetColor,
			cyanColor, tu.CachedTokens, resetColor,
			cyanColor, tu.ThoughtTokens, resetColor,
			cyanColor, tu.TotalTokens, resetColor,
		)
	}
	return ""
}

func (tu *TokenUsage) Render(render Render) {
	// Get the token usage
	usage := tu.getTokenUsage()
	render.Writeln(usage)
}

func (tu *TokenUsage) RecordTokenUsage(input, output, cached, thought, total int) {
	tu.InputTokens = input
	tu.OutputTokens = output
	tu.CachedTokens = cached
	tu.ThoughtTokens = thought
	tu.TotalTokens = total
}
