package data

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	InstructionFileName = "GLLM.md"
)

var ()

// GetGlobalInstructionFilePath returns the path to the global instruction file.
func GetGlobalInstructionFilePath() string {
	return filepath.Join(GetConfigDir(), InstructionFileName)
}

// GetLocalInstructionFilePath returns the path to the local instruction file in the current working directory.
func GetLocalInstructionFilePath() string {
	return filepath.Join(".", InstructionFileName)
}

// GetInstructionContent discovers and loads static instruction files (global and local).
// It formats them into an XML structure suitable for injection into the system prompt.
func GetInstructionContent() string {
	var content strings.Builder

	// Load Global Instructions
	globalPath := GetGlobalInstructionFilePath()
	if globalData, err := os.ReadFile(globalPath); err == nil && len(globalData) > 0 {
		content.WriteString("<global_instructions>\n")
		content.WriteString(strings.TrimSpace(string(globalData)))
		content.WriteString("\n</global_instructions>\n\n")
	}

	// Load Local (Project) Instructions
	localPath := GetLocalInstructionFilePath()
	if localData, err := os.ReadFile(localPath); err == nil && len(localData) > 0 {
		content.WriteString("<project_instructions>\n")
		content.WriteString(strings.TrimSpace(string(localData)))
		content.WriteString("\n</project_instructions>\n\n")
	}

	return strings.TrimSpace(content.String())
}
