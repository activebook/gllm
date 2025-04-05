package service

import (
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
