package service

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/activebook/gllm/data"
)

const (
	toolPermissionDenied   = "Current session is in plan mode now. you MUST NOT make any edits or run any non-readonly tools."
	toolPermissionPlanPath = "You can only create or edit plans, todos and related files under this directory"
)

// readOnlyToolsKeywords covers common MCP tool naming patterns that indicate
// a tool only reads/queries state and does not mutate anything.
// e.g. mcp__github__search_issues, mcp__fs__find_files, mcp__db__get_record
//
// Known gaps:
//   - Mixed separators (search-for_items) are not matched
//   - Very short keywords (get, put) are inherently ambiguous
var readOnlyToolsKeywords = []string{
	"search", "find", "get", "list", "read", "fetch",
	"lookup", "query", "describe", "show", "view",
	"research", "peek", "stat", "tree",
}

// blockedToolsKeywords: first match wins and returns false immediately.
// Intentionally conservative. Keywords that appear here must NOT also appear
// in readOnlyToolsKeywords or they become dead code.
//
// Removed from this list vs original:
//   - check, inspect  → reclassified as read-only (check_schema, inspect_container)
//   - log, audit, lint, scan → removed entirely; too ambiguous (get_log is read-only)
var blockedToolsKeywords = []string{
	"write", "edit", "create", "delete", "remove", "move", "copy",
	"rename", "update", "patch", "put", "post", "run", "exec", "install",
	"uninstall", "upgrade", "downgrade", "reinstall", "reboot", "restart",
	"shutdown", "stop", "start", "pause", "resume", "kill", "terminate",
	"apply", "save", "commit", "push", "pull", "deploy", "build",
}

// stripMCPPrefix removes the well-known MCP tool name prefix so keyword
// matching operates only on the tool segment.
//
//	mcp__github__search_issues → search_issues
//	mcp:github:search_issues   → search_issues   (colon style, future-proof)
//	search_issues              → search_issues   (no prefix, unchanged)
func stripMCPPrefix(name string) string {
	// Handle double-underscore style: mcp__server__tool
	// Split on "__" and take the last non-empty segment.
	if strings.Contains(name, "__") {
		parts := strings.Split(name, "__")
		for i := len(parts) - 1; i >= 0; i-- {
			if parts[i] != "" {
				return parts[i]
			}
		}
	}
	// Handle colon style: mcp:server:tool
	if strings.HasPrefix(name, "mcp:") {
		parts := strings.SplitN(name, ":", 3)
		if len(parts) == 3 && parts[2] != "" {
			return parts[2]
		}
	}
	return name
}

// splitCamelCase inserts underscores before each uppercase letter run so that
// camelCase and PascalCase tool names are normalised to snake_case before
// keyword matching.
//
//	searchFiles  → search_files
//	getRecord    → get_record
//	HTMLParser   → html_parser
func splitCamelCase(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 4)
	runes := []rune(s)
	for i, r := range runes {
		if i == 0 {
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		if unicode.IsUpper(r) {
			prev := runes[i-1]
			// Insert separator when:
			//   - transitioning from lower/digit to upper:  fooBar → foo_bar
			//   - transitioning from upper run to upper+lower: HTMLParser → html_parser
			if !unicode.IsUpper(prev) || (i+1 < len(runes) && unicode.IsLower(runes[i+1])) {
				b.WriteRune('_')
			}
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// matchesKeyword reports whether kw appears at a word boundary inside name.
// Word boundaries are _ and - separators. Exact match is also accepted.
//
// Examples that match "search":
//
//	search, search_issues, find-search, get_search_results
//
// Examples that do NOT match "search" (avoid substring false-positives):
//
//	researching, unsearchable
func matchesKeyword(name, kw string) bool {
	return name == kw ||
		strings.HasPrefix(name, kw+"_") ||
		strings.HasPrefix(name, kw+"-") ||
		strings.HasSuffix(name, "_"+kw) ||
		strings.HasSuffix(name, "-"+kw) ||
		strings.Contains(name, "_"+kw+"_") ||
		strings.Contains(name, "_"+kw+"-") ||
		strings.Contains(name, "-"+kw+"_") ||
		strings.Contains(name, "-"+kw+"-")
}

// looksLikeReadOnlyTool returns true when toolName appears to only read or
// query state, based on keyword heuristics. It returns false when any
// mutation-related keyword is detected, or when no read-only keyword is found.
//
// This is a best-effort heuristic. Tool authors can defeat it with unusual
// naming. Treat the result as a hint, not a security boundary.
func looksLikeReadOnlyTool(toolName string) bool {
	if toolName == "" {
		return false
	}

	// Normalise: strip MCP prefix, convert camelCase, then lowercase.
	stripped := stripMCPPrefix(toolName)
	normalised := splitCamelCase(stripped) // already lowercased

	// Blocked keywords take priority — one match and we're done.
	for _, kw := range blockedToolsKeywords {
		if matchesKeyword(normalised, kw) {
			return false
		}
	}

	// Require at least one explicit read-only keyword.
	for _, kw := range readOnlyToolsKeywords {
		if matchesKeyword(normalised, kw) {
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
		ToolEditFile:        true,
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
			return fmt.Errorf("%s %s: %s", toolPermissionDenied, toolPermissionPlanPath, plansDir)
		}

		return nil
	}

	// Default reject message for anything else (shell, edits, copy, move, delete)
	return fmt.Errorf(toolPermissionDenied)
}
