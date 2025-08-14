package service

import (
	"fmt"

	"github.com/spf13/viper"
)

var (
	InputTokens   int
	OutputTokens  int
	CachedTokens  int
	ThoughtTokens int
	TotalTokens   int
)

const ()

func GetTokenUsage() string {
	if TotalTokens > 0 {
		return fmt.Sprintf(
			bbColor+
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
			cyanColor, InputTokens, resetColor,
			cyanColor, OutputTokens, resetColor,
			cyanColor, CachedTokens, resetColor,
			cyanColor, ThoughtTokens, resetColor,
			cyanColor, TotalTokens, resetColor,
		)
	}
	return ""
}

func RenderTokenUsage() {
	if IncludeUsageMetainfo() {
		// Get the token usage
		usage := GetTokenUsage()
		fmt.Println(usage)
	}
}

func RecordTokenUsage(input, output, cached, thought int) {
	InputTokens = input
	OutputTokens = output
	CachedTokens = cached
	ThoughtTokens = thought
	TotalTokens = InputTokens + OutputTokens + CachedTokens + ThoughtTokens
}

func IncludeUsageMetainfo() bool {
	usage := viper.GetString("default.usage")
	switch usage {
	case "on":
		return true
	case "off":
		return false
	default:
		return false
	}
}
