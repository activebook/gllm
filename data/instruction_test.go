package data

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetInstructionContent(t *testing.T) {
	// Create a temporary local instruction file
	localPath := filepath.Join(".", InstructionFileName)
	content := "test local instruction"
	err := os.WriteFile(localPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to create local instruction file: %v", err)
	}
	defer os.Remove(localPath)

	instructionContent := GetInstructionContent()

	if !strings.Contains(instructionContent, "<project_instructions path=") {
		t.Errorf("expected <project_instructions> to have path attribute, got: %s", instructionContent)
	}

	absPath, _ := filepath.Abs(localPath)
	expectedPathAttr := "path=\"" + absPath + "\""
	if !strings.Contains(instructionContent, expectedPathAttr) {
		t.Errorf("expected path attribute to be %s, got: %s", expectedPathAttr, instructionContent)
	}

	if !strings.Contains(instructionContent, content) {
		t.Errorf("expected instruction content to contain %s, got: %s", content, instructionContent)
	}
}
