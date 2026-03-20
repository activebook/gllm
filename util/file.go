package util

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
	invalidFileChars       = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1F]`)
	validResourceNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
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

// ValidateResourceName checks if the name is filesystem-safe.
func ValidateResourceName(resourceType, name string) error {
	if !validResourceNameRegex.MatchString(name) {
		return fmt.Errorf("%s name '%s' is invalid: only alphanumeric characters, dashes, and underscores are allowed", resourceType, name)
	}
	return nil
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

func JoinFilePath(dir string, filename string) string {
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
	// Get the default session name from the config
	// This is a placeholder function. Replace with actual logic to get the default name.
	// Get the current time
	currentTime := time.Now()

	// Format the time as a string in the format "chat_YYYY-MM-DD_HH-MM-SS.json"
	filename := fmt.Sprintf("temp-%s", currentTime.Format("2006-01-02_15-04-05"))

	return filename
}
