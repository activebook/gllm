package service

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/activebook/gllm/data"
)

const (
	toolPermissionDenied = "Current session is in plan mode now. you MUST NOT make any edits or run any non-readonly tools."
)

// readOnlyPrefixes and readOnlySuffixes cover common MCP tool naming patterns
// e.g. mcp__github__search_issues, mcp__fs__find_files, mcp__db__get_record
var readOnlyToolsKeywords = []string{
	"search", "find", "get", "list", "read", "fetch",
	"lookup", "query", "describe", "show", "view",
	"research", "check", "inspect", "peek", "stat",
}

func looksLikeReadOnlyTool(toolName string) bool {
	lower := strings.ToLower(toolName)
	for _, kw := range readOnlyToolsKeywords {
		// Match keyword at start, end, or surrounded by separators (_ or -)
		// The boundary check (_kw_, _kw prefix/suffix) avoids false positives like
		// "research" matching "refresh" or "describe" matching "describe_and_delete".
		// It requires the keyword to sit at a word boundary.
		if lower == kw ||
			strings.HasPrefix(lower, kw+"_") ||
			strings.HasPrefix(lower, kw+"-") ||
			strings.HasSuffix(lower, "_"+kw) ||
			strings.HasSuffix(lower, "-"+kw) ||
			strings.Contains(lower, "_"+kw+"_") ||
			strings.Contains(lower, "-"+kw+"-") {
			return true
		}
	}
	return false
}

// CheckToolPermission checks if the tool is allowed to be executed in the current mode
func CheckToolPermission(toolName string, args *map[string]interface{}) error {
	planMode := data.GetPlanModeInSession()
	// If not in plan mode, all tools are allowed
	if !planMode {
		return nil
	}

	// Check if tool is allowed
	if readOnlyTools[toolName] {
		return nil
	}

	// Lossy name match for MCP tools that suggest read-only intent
	if looksLikeReadOnlyTool(toolName) {
		return nil
	}

	// Explicitly define conditional write tools
	writeTools := map[string]bool{
		ToolWriteFile:       true,
		ToolCreateDirectory: true,
	}

	if writeTools[toolName] {
		if args == nil {
			return fmt.Errorf("invalid path argument")
		}

		pathParam, ok := (*args)["path"].(string)
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
			return fmt.Errorf("%s You can only create plans under this directory: %s", toolPermissionDenied, plansDir)
		}

		return nil
	}

	// Default reject message for anything else (shell, edits, copy, move, delete)
	return fmt.Errorf(toolPermissionDenied)
}
