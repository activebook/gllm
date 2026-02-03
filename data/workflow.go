package data

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	WorkflowFileExt = ".md"
)

// WorkflowMetadata represents a workflow command definition
type WorkflowMetadata struct {
	Name        string `yaml:"name"`        // Display name
	Description string `yaml:"description"` // Brief description for /help
	Location    string // Full path to workflow file
}

// EnsureWorkflowsDir creates the workflows directory if it doesn't exist.
func EnsureWorkflowsDir() error {
	return os.MkdirAll(GetWorkflowsDirPath(), 0750)
}

// ParseWorkflowFrontmatter reads a workflow md file and extracts its metadata.
func ParseWorkflowFrontmatter(path string) (*WorkflowMetadata, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read workflow file: %w", err)
	}

	// Extract frontmatter (between --- separators)
	s := string(content)
	if !strings.HasPrefix(s, "---") {
		// Valid markdown but no frontmatter, define as simple workflow
		filename := filepath.Base(path)
		name := strings.TrimSuffix(filename, filepath.Ext(filename))
		return &WorkflowMetadata{
			Name:        name,
			Description: "Custom workflow",
			Location:    path,
		}, nil
	}

	parts := strings.SplitN(s, "---", 3)
	if len(parts) < 3 {
		// Frontmatter incomplete
		return nil, fmt.Errorf("invalid frontmatter format")
	}

	frontmatter := parts[1]
	var meta WorkflowMetadata
	if err := yaml.Unmarshal([]byte(frontmatter), &meta); err != nil {
		return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	// Fallback name if missing in frontmatter
	if meta.Name == "" {
		filename := filepath.Base(path)
		meta.Name = strings.TrimSuffix(filename, filepath.Ext(filename))
	}

	meta.Location = path
	return &meta, nil
}

// GetWorkflowContent returns the content of the workflow file after frontmatter.
func GetWorkflowContent(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read workflow file: %w", err)
	}

	s := string(content)
	if strings.HasPrefix(s, "---") {
		parts := strings.SplitN(s, "---", 3)
		if len(parts) >= 3 {
			return strings.TrimSpace(parts[2]), nil
		}
	}

	// No frontmatter, return full content
	return s, nil
}

// ScanWorkflows scans the default workflows directory for valid workflow files.
func ScanWorkflows() ([]WorkflowMetadata, error) {
	return ScanWorkflowsInDir(GetWorkflowsDirPath())
}

// ScanWorkflowsInDir scans the specified directory for valid workflow files.
func ScanWorkflowsInDir(dir string) ([]WorkflowMetadata, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []WorkflowMetadata{}, nil
		}
		return nil, fmt.Errorf("failed to read workflows directory: %w", err)
	}

	var workflows []WorkflowMetadata
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if filepath.Ext(entry.Name()) != WorkflowFileExt {
			continue
		}

		workflowPath := filepath.Join(dir, entry.Name())
		meta, err := ParseWorkflowFrontmatter(workflowPath)
		if err != nil {
			fmt.Printf("Warning: Skipping invalid workflow at %s: %v\n", workflowPath, err)
			continue
		}

		workflows = append(workflows, *meta)
	}

	return workflows, nil
}
