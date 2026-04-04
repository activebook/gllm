package data

import (
	"fmt"
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
		content.WriteString(fmt.Sprintf("<global_instructions path=\"%s\">\n", globalPath))
		content.WriteString(strings.TrimSpace(string(globalData)))
		content.WriteString("\n</global_instructions>\n\n")
	}

	// Load Local (Project) Instructions
	localPath := GetLocalInstructionFilePath()
	if absLocalPath, err := filepath.Abs(localPath); err == nil {
		localPath = absLocalPath
	}
	if localData, err := os.ReadFile(localPath); err == nil && len(localData) > 0 {
		content.WriteString(fmt.Sprintf("<project_instructions path=\"%s\">\n", localPath))
		content.WriteString(strings.TrimSpace(string(localData)))
		content.WriteString("\n</project_instructions>\n\n")
	}

	return strings.TrimSpace(content.String())
}

// LocalInstructionFileExists reports whether ./GLLM.md exists in the current working directory.
func LocalInstructionFileExists() bool {
	_, err := os.Stat(GetLocalInstructionFilePath())
	return err == nil
}

// GlobalInstructionFileExists reports whether the global GLLM.md exists in the config directory.
func GlobalInstructionFileExists() bool {
	_, err := os.Stat(GetGlobalInstructionFilePath())
	return err == nil
}
