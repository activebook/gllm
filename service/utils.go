package service

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// SanitizeTitle replaces spaces with underscores in a given title.
var (
	invalidFileChars = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1F]`)
)

func GetSanitizeTitle(title string) string {
	// Replace invalid characters with underscore
	sanitized := invalidFileChars.ReplaceAllString(title, "_")
	// Trim spaces and dots from the beginning and end (Windows doesn't like those)
	sanitized = strings.Trim(sanitized, " .")
	// Fallback to "untitled" if title is empty after sanitization
	if sanitized == "" {
		sanitized = "untitled"
	}
	return sanitized
}

func GetUserConfigDir() string {
	// Prefer os.UserConfigDir()
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		Warnf("Warning: Could not find user dir, falling back to home directory.%v\n", err)
		userConfigDir, _ = os.UserHomeDir()
	}
	return userConfigDir
}

func MakeUserSubDir(subparts ...string) string {
	userConfigDir := GetUserConfigDir()
	subDir := filepath.Join(userConfigDir, filepath.Join(subparts...))
	if err := os.MkdirAll(subDir, 0750); err != nil { // 0750 permissions: user rwx, group rx, others none
		Errorf("Error creating subdirectory '%s': %v\n", subDir, err)
		return ""
	}
	return subDir
}

func GetFilePath(dir string, filename string) string {
	return filepath.Join(dir, filename)
}

func GetFileContent(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func GenerateTempFileName() string {
	// Get the default conversation name from the config
	// This is a placeholder function. Replace with actual logic to get the default name.
	// Get the current time
	currentTime := time.Now()

	// Format the time as a string in the format "chat_YYYY-MM-DD_HH-MM-SS.json"
	filename := fmt.Sprintf("temp_%s", currentTime.Format("2006-01-02_15-04-05"))

	return filename
}

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
func GetStringValue(data map[string]interface{}, key string) string {
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

func Ptr[T any](t T) *T { return &t }

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
