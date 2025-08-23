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

func (tu *TokenUsage) getTokenUsageTip() string {
	if tu.TotalTokens > 0 {
		cachedPercentage := float64(tu.CachedTokens) / float64(tu.TotalTokens) * 100
		return fmt.Sprintf(
			bbColor+"\n"+
				"┌──────────────────────────────────────────────────────────────────────────────────────────┐\n"+
				"│"+resetColor+cyanColor+" Token Usage"+resetColor+bbColor+"                                                                              │\n"+
				"│"+resetColor+" Input: %s%6d%s "+bbColor+"│"+resetColor+" Output: %s%6d%s "+bbColor+"│"+resetColor+" Cached: %s%6d %s%s "+bbColor+"│"+resetColor+" Thought: %s%6d%s "+bbColor+"│"+resetColor+" Total: %s%6d%s "+bbColor+"│"+resetColor+"\n"+bbColor+
				"└──────────────────────────────────────────────────────────────────────────────────────────┘"+
				resetColor,
			cyanColor, tu.InputTokens, resetColor,
			cyanColor, tu.OutputTokens, resetColor,
			cyanColor, tu.CachedTokens, "("+fmt.Sprintf("%3.1f%%", cachedPercentage)+")", resetColor,
			cyanColor, tu.ThoughtTokens, resetColor,
			cyanColor, tu.TotalTokens, resetColor,
		)
	}
	return ""
}

func (tu *TokenUsage) getTokenUsageBox() string {
	if tu.TotalTokens > 0 {
		cachedPercentage := float64(tu.CachedTokens) / float64(tu.TotalTokens) * 100
		return fmt.Sprintf(
			bbColor+"\n"+
				"┌───────────────┬────────────┐\n"+
				"│ %sToken Type%s    │      %sCount%s │\n"+
				"├───────────────┼────────────┤\n"+
				"│ %sInput%s         │ %s%10d%s"+bbColor+" │\n"+
				"│ %sOutput%s        │ %s%10d%s"+bbColor+" │\n"+
				"│ %sCached%s        │ %s%10d%s"+bbColor+" │\n"+
				"│               │ %s%10s%s"+bbColor+" │\n"+
				"│ %sThought%s       │ %s%10d%s"+bbColor+" │\n"+
				"├───────────────┼────────────┤\n"+
				"│ %sTotal%s         │ %s%10d%s"+bbColor+" │\n"+
				"└───────────────┴────────────┘"+
				resetColor,
			resetColor, bbColor, resetColor, bbColor,
			resetColor, bbColor, cyanColor, tu.InputTokens, resetColor,
			resetColor, bbColor, cyanColor, tu.OutputTokens, resetColor,
			resetColor, bbColor, cyanColor, tu.CachedTokens, resetColor,
			cyanColor, fmt.Sprintf("%4.1f%%", cachedPercentage), resetColor,
			resetColor, bbColor, cyanColor, tu.ThoughtTokens, resetColor,
			resetColor, bbColor, cyanColor, tu.TotalTokens, resetColor,
		)
	}
	return ""
}

func (tu *TokenUsage) Render(render Render) {
	// Get the token usage
	usage := tu.getTokenUsageBox()
	render.Writeln(usage)
}

func (tu *TokenUsage) RecordTokenUsage(input, output, cached, thought, total int) {
	tu.InputTokens += input
	tu.OutputTokens += output
	tu.CachedTokens += cached
	tu.ThoughtTokens += thought
	tu.TotalTokens += total
}
