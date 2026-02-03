package service

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/activebook/gllm/data"
)

// WorkflowManager handles workflow operations
type WorkflowManager struct {
	workflows        []data.WorkflowMetadata
	workflowsDir     string
	reservedCommands map[string]string // reserved commands that cannot be used as workflow names
	mu               sync.RWMutex
}

var (
	workflowManagerInstance *WorkflowManager
	workflowManagerOnce     sync.Once
)

// GetWorkflowManager returns the singleton instance of WorkflowManager
func GetWorkflowManager() *WorkflowManager {
	workflowManagerOnce.Do(func() {
		data.EnsureWorkflowsDir()
		workflowManagerInstance = &WorkflowManager{
			workflowsDir: data.GetWorkflowsDirPath(),
		}
	})
	return workflowManagerInstance
}

// LoadMetadata scans and loads workflow metadata
func (wm *WorkflowManager) LoadMetadata(reservedCommands map[string]string) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	workflows, err := data.ScanWorkflowsInDir(wm.workflowsDir)
	if err != nil {
		return err
	}
	wm.workflows = workflows
	wm.reservedCommands = reservedCommands
	return nil
}

// GetWorkflowByName retrieves a workflow by its name (case-insensitive)
// Returns content, description, and error
func (wm *WorkflowManager) GetWorkflowByName(name string) (string, string, error) {
	wm.mu.RLock()
	var selected *data.WorkflowMetadata
	lowerName := strings.ToLower(name)
	for _, w := range wm.workflows {
		if strings.ToLower(w.Name) == lowerName {
			selected = &w
			break
		}
	}
	wm.mu.RUnlock()

	if selected == nil {
		return "", "", fmt.Errorf("workflow '%s' not found", name)
	}

	content, err := data.GetWorkflowContent(selected.Location)
	if err != nil {
		return "", "", err
	}

	return content, selected.Description, nil
}

// GetWorkflowNames returns a sorted list of all available workflow names
func (wm *WorkflowManager) GetWorkflowNames() []string {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	names := make([]string, 0, len(wm.workflows))
	for _, w := range wm.workflows {
		names = append(names, w.Name)
	}
	sort.Strings(names)
	return names
}

// GetCommands returns a map of command->description for chat suggestions
func (wm *WorkflowManager) GetCommands() map[string]string {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	commands := make(map[string]string)
	for _, w := range wm.workflows {
		cmd := "/" + w.Name
		commands[cmd] = w.Description
	}
	return commands
}

// IsReservedCommand checks if a command is reserved
func (wm *WorkflowManager) IsReservedCommand(name string) bool {
	if !strings.HasPrefix(name, "/") {
		name = "/" + name
	}
	_, ok := wm.reservedCommands[name]
	return ok
}

// CreateWorkflow creates a new workflow file
func (wm *WorkflowManager) CreateWorkflow(name, description, content string) error {
	if wm.IsReservedCommand(name) {
		return fmt.Errorf("cannot create workflow '%s': conflicts with reserved command", name)
	}

	// Sanitize name for filename
	filename := strings.ToLower(name) + ".md"
	path := filepath.Join(wm.workflowsDir, filename)

	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("workflow '%s' already exists", name)
	}

	// Prepare content with frontmatter
	fullContent := fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n\n%s", name, description, content)

	if err := os.MkdirAll(wm.workflowsDir, 0750); err != nil {
		return err
	}

	if err := os.WriteFile(path, []byte(fullContent), 0644); err != nil {
		return fmt.Errorf("failed to write workflow file: %w", err)
	}

	// Reload metadata to include new workflow
	return wm.LoadMetadata(wm.reservedCommands)
}

// RemoveWorkflow removes a workflow
func (wm *WorkflowManager) RemoveWorkflow(name string) error {
	wm.mu.RLock()
	var path string
	lowerName := strings.ToLower(name)
	for _, w := range wm.workflows {
		if strings.ToLower(w.Name) == lowerName {
			path = w.Location
			break
		}
	}
	wm.mu.RUnlock()

	if path == "" {
		return fmt.Errorf("workflow '%s' not found", name)
	}

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to remove workflow file: %w", err)
	}

	// Reload metadata
	return wm.LoadMetadata(wm.reservedCommands)
}

// RenameWorkflow renames a workflow
func (wm *WorkflowManager) RenameWorkflow(oldName, newName string) error {
	if wm.IsReservedCommand(newName) {
		return fmt.Errorf("cannot rename to '%s': conflicts with reserved command", newName)
	}

	// Get existing data
	content, desc, err := wm.GetWorkflowByName(oldName)
	if err != nil {
		return err
	}

	// Create new workflow first to ensure it's valid
	// This will check if new name is reserved or already exists
	if err := wm.CreateWorkflow(newName, desc, content); err != nil {
		return err
	}

	// Remove old workflow
	// Note: CreateWorkflow reloaded metadata, so both exist now.
	if err := wm.RemoveWorkflow(oldName); err != nil {
		// Rollback? If we fail to remove old one, we have duplicates.
		// Not ideal but better than losing data.
		return fmt.Errorf("failed to remove old workflow '%s' during rename: %w", oldName, err)
	}

	return nil
}
