package service

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/activebook/gllm/data"
)

// CheckToolPermission checks if the tool is allowed to be executed in the current mode
func CheckToolPermission(toolName string, args map[string]interface{}, planMode bool) error {
	// If not in plan mode, all tools are allowed
	if !planMode {
		return nil
	}

	// Check if tool is allowed
	if readOnlyTools[toolName] {
		return nil
	}

	// Explicitly define conditional write tools
	writeTools := map[string]bool{
		ToolWriteFile:       true,
		ToolCreateDirectory: true,
	}

	if writeTools[toolName] {
		pathParam, ok := args["path"].(string)
		if !ok {
			return fmt.Errorf("invalid path argument")
		}

		// Check if it's within the 'plans' directory in the gllm directory
		plansDir := data.GetPlansDirPath()

		absPath, err := filepath.Abs(pathParam)
		if err != nil {
			return fmt.Errorf("invalid path format")
		}

		// Ensure the resulting path is strongly inside the 'plans' folder
		if !strings.HasPrefix(absPath, plansDir+string(filepath.Separator)) && absPath != plansDir {
			return fmt.Errorf("you MUST NOT make any edits, run any non-readonly tools.")
		}

		return nil
	}

	// Default reject message for anything else (shell, edits, copy, move, delete)
	return fmt.Errorf("you MUST NOT make any edits, run any non-readonly tools.")
}
