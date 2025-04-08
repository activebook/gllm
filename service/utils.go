package service

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
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

func GetDefaultConvoName() string {
	// Get the default conversation name from the config
	// This is a placeholder function. Replace with actual logic to get the default name.
	return "default"
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

// Helper function to truncate strings with ellipsis
func TruncateString(s string, maxLen int) string {
	s = strings.TrimSpace(s)
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
