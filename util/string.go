package util

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Helper function to truncate strings with ellipsis
func TruncateString(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if maxLen < 0 {
		return s
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// Helper function to safely extract string values
func GetStringValue(data map[string]any, key string) string {
	if value, ok := data[key].(string); ok {
		return value
	}
	return ""
}

func Contains(list []string, item string) bool {
	for _, v := range list {
		if v == item {
			return true
		}
	}
	return false
}

func HasContent(s *string) bool {
	return s != nil && *s != ""
}

func EndWithNewline(s string) bool {
	// Check if the string ends with a newline or empty line
	return strings.HasSuffix(s, "\n") || strings.HasSuffix(s, "\r\n") || strings.TrimSpace(s) == ""
}

func FormatMinutesSeconds(d time.Duration) string {
	totalSeconds := int(d.Seconds())
	minutes := totalSeconds / 60
	seconds := totalSeconds % 60
	return fmt.Sprintf("%02dm:%02ds", minutes, seconds)
}

// thinkTagRegex matches <think>...</think> tags, including multiline content
// Using (?s) flag to make . match newlines
var thinkTagRegex = regexp.MustCompile(`(?s)<think>(.*?)</think>`)

// ExtractThinkTags extracts thinking content from <think>...</think> tags.
// Some providers (like MiniMax, some Qwen endpoints) embed reasoning content
// in <think> tags within the regular content field instead of using a separate
// reasoning_content field.
//
// Returns:
//   - thinking: the extracted thinking content (empty if no tags found)
//   - cleaned: the content with <think> tags removed
func ExtractThinkTags(content string) (thinking, cleaned string) {
	matches := thinkTagRegex.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return "", content
	}

	// Collect all thinking content
	var thinkingParts []string
	for _, match := range matches {
		if len(match) > 1 && match[1] != "" {
			thinkingParts = append(thinkingParts, strings.TrimSpace(match[1]))
		}
	}

	// Remove all <think> tags from content
	cleaned = thinkTagRegex.ReplaceAllString(content, "")
	cleaned = strings.TrimSpace(cleaned)

	thinking = strings.Join(thinkingParts, "\n")
	return thinking, cleaned
}
