package data

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureWorkflowsDir(t *testing.T) {
	// Temporarily override GetConfigDir (mocking environment usually tough,
	// but here we can just ensure it doesn't error out generally.
	// For safer test, skipping actual dir creation mock unless we refactor GetConfigDir.
	// But EnsureWorkflowsDir is simple wrapper around MkdirAll.)
	if err := EnsureWorkflowsDir(); err != nil {
		t.Errorf("EnsureWorkflowsDir failed: %v", err)
	}
}

func TestWorkflowParsing(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gllm-test-workflows")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Test Case 1: Valid Frontmatter
	validContent := `---
name: debug-flow
description: A debug workflow
---
Do debugging steps.
`
	validFile := filepath.Join(tmpDir, "debug.md")
	if err := os.WriteFile(validFile, []byte(validContent), 0644); err != nil {
		t.Fatal(err)
	}

	meta, err := ParseWorkflowFrontmatter(validFile)
	if err != nil {
		t.Errorf("Failed to parse valid workflow: %v", err)
	}
	if meta.Name != "debug-flow" {
		t.Errorf("Expected name 'debug-flow', got '%s'", meta.Name)
	}
	if meta.Description != "A debug workflow" {
		t.Errorf("Expected description 'A debug workflow', got '%s'", meta.Description)
	}

	// Test Case 2: No Frontmatter
	simpleContent := `Just do this.`
	simpleFile := filepath.Join(tmpDir, "simple.md")
	if err := os.WriteFile(simpleFile, []byte(simpleContent), 0644); err != nil {
		t.Fatal(err)
	}

	meta2, err := ParseWorkflowFrontmatter(simpleFile)
	if err != nil {
		t.Errorf("Failed to parse simple workflow: %v", err)
	}
	if meta2.Name != "simple" {
		t.Errorf("Expected name 'simple', got '%s'", meta2.Name)
	}

	// Test Case 3: Get Content
	content, err := GetWorkflowContent(validFile)
	if err != nil {
		t.Errorf("Failed to get content: %v", err)
	}
	if content != "Do debugging steps." {
		t.Errorf("Expected content 'Do debugging steps.', got '%s'", content)
	}
}
