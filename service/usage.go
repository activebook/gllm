package service

import "fmt"

var (
	InputTokens   int
	OutputTokens  int
	CachedTokens  int
	ThoughtTokens int
	TotalTokens   int
)

func GetTokenUsage() string {
	if TotalTokens > 0 {
		return fmt.Sprintf("Input tokens: %d\nOutput tokens: %d\nCached tokens: %d\nThought tokens: %d\nTotal tokens: %d", InputTokens, OutputTokens, CachedTokens, ThoughtTokens, TotalTokens)
	}
	return ""
}

func RecordTokenUsage(input, output, cached, thought int) {
	InputTokens = input
	OutputTokens = output
	CachedTokens = cached
	ThoughtTokens = thought
	TotalTokens = InputTokens + OutputTokens + CachedTokens + ThoughtTokens
}
