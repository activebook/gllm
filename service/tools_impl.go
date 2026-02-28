package service

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/internal/ui"
)

// Tool robustness constants
const (
	DefaultShellTimeout = 60 * time.Second
	MaxFileSize         = 20 * 1024 * 1024 // 20MB
)

// Tool implementation functions

// Shared implementation functions that work with map[string]interface{} arguments
// These functions contain the actual logic that can be shared between OpenAI and OpenChat

func readFileToolCallImpl(argsMap *map[string]interface{}) (string, error) {
	path, ok := (*argsMap)["path"].(string)
	if !ok {
		return "", fmt.Errorf("path not found in arguments")
	}

	// Check if line numbers are requested
	includeLineNumbers := false
	if lineNumValue, exists := (*argsMap)["line_numbers"]; exists {
		if lineNumBool, ok := lineNumValue.(bool); ok {
			includeLineNumbers = lineNumBool
		}
	}

	// Check file size before reading
	fileInfo, err := os.Stat(path)
	if err != nil {
		return fmt.Sprintf("Error accessing file %s: %v", path, err), nil
	}
	if fileInfo.Size() > MaxFileSize {
		return fmt.Sprintf("Error: File %s is too large (%d bytes, max allowed: %d bytes / %.1f MB). Consider reading specific portions or using shell commands like 'head' or 'tail'.",
			path, fileInfo.Size(), MaxFileSize, float64(MaxFileSize)/(1024*1024)), nil
	}

	// Read the file
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("Error reading file %s: %v", path, err), nil
	}

	var response string
	if includeLineNumbers {
		// Add line numbers to the output
		lines := strings.Split(string(content), "\n")
		var numberedContent strings.Builder
		for i, line := range lines {
			numberedContent.WriteString(fmt.Sprintf("%4d | %s\n", i+1, line))
		}
		response = fmt.Sprintf("Content of %s (with line numbers):\n%s", path, numberedContent.String())
	} else {
		// Original format without line numbers
		response = fmt.Sprintf("Content of %s:\n%s", path, string(content))
	}

	return response, nil
}

func writeFileToolCallImpl(argsMap *map[string]interface{}, toolsUse *data.ToolsUse, showDiff func(diff string), closeDiff func()) (string, error) {
	path, ok := (*argsMap)["path"].(string)
	if !ok {
		return "", fmt.Errorf("path not found in arguments")
	}

	content, ok := (*argsMap)["content"].(string)
	if !ok {
		return "", fmt.Errorf("content not found in arguments")
	}

	// Check if confirmation is needed
	needConfirm, ok := (*argsMap)["need_confirm"].(bool)
	if !ok {
		// Default to true for safety
		needConfirm = true
	}

	if needConfirm && !toolsUse.AutoApprove {
		// Check if file exists and read current content
		var currentContent string
		if _, err := os.Stat(path); err == nil {
			// File exists, read current content for diff
			currentData, err := os.ReadFile(path)
			if err == nil {
				currentContent = string(currentData)
			}
		}

		// Show diff if we have current content
		diff := ui.Diff(currentContent, content, "", "", 3)
		showDiff(diff)

		// Get purpose if provided
		purpose, _ := (*argsMap)["purpose"].(string)
		if purpose == "" {
			purpose = fmt.Sprintf("write content to the file at path: %s", path)
		}

		// Prompt user for confirmation
		ui.NeedUserConfirmToolUse("", ToolUserConfirmPrompt, purpose, toolsUse)
		closeDiff() // Close the diff
		if toolsUse.Confirm == data.ToolConfirmCancel {
			return fmt.Sprintf("Operation cancelled by user: write to file %s", path), UserCancelError{Reason: UserCancelReasonDeny}
		}
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Sprintf("Error creating directory for %s: %v", path, err), nil
	}

	// Write the file
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return fmt.Sprintf("Error writing file %s: %v", path, err), nil
	}

	return fmt.Sprintf("Successfully wrote to file %s", path), nil
}

func createDirectoryToolCallImpl(argsMap *map[string]interface{}, toolsUse *data.ToolsUse) (string, error) {
	path, ok := (*argsMap)["path"].(string)
	if !ok {
		return "", fmt.Errorf("path not found in arguments")
	}

	// Check if confirmation is needed
	needConfirm, ok := (*argsMap)["need_confirm"].(bool)
	if !ok {
		// Default to true for safety
		needConfirm = true
	}

	if needConfirm && !toolsUse.AutoApprove {
		// Get purpose if provided
		purpose, _ := (*argsMap)["purpose"].(string)
		if purpose == "" {
			purpose = fmt.Sprintf("create the directory at path: %s", path)
		}

		// Prompt user for confirmation
		ui.NeedUserConfirmToolUse("", ToolUserConfirmPrompt, purpose, toolsUse)
		if toolsUse.Confirm == data.ToolConfirmCancel {
			return fmt.Sprintf("Operation cancelled by user: create directory %s", path), UserCancelError{Reason: UserCancelReasonDeny}
		}
	}

	// Create the directory
	err := os.MkdirAll(path, 0755)
	if err != nil {
		return fmt.Sprintf("Error creating directory %s: %v", path, err), nil
	}

	return fmt.Sprintf("Successfully created directory %s", path), nil
}

func listDirectoryToolCallImpl(argsMap *map[string]interface{}) (string, error) {
	path, ok := (*argsMap)["path"].(string)
	if !ok {
		return "", fmt.Errorf("path not found in arguments")
	}

	// List directory contents
	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Sprintf("Error reading directory %s: %v", path, err), nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Contents of directory %s:\n", path))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			// If we can't get info, just show the name
			if entry.IsDir() {
				result.WriteString(fmt.Sprintf("[DIR]  %s\n", entry.Name()))
			} else {
				result.WriteString(fmt.Sprintf("[FILE] %s\n", entry.Name()))
			}
			continue
		}

		if entry.IsDir() {
			result.WriteString(fmt.Sprintf("[DIR]  %-40s  %s\n",
				entry.Name(), info.ModTime().Format("2006-01-02 15:04")))
		} else {
			// Format file size
			size := info.Size()
			var sizeStr string
			if size < 1024 {
				sizeStr = fmt.Sprintf("%d B", size)
			} else if size < 1024*1024 {
				sizeStr = fmt.Sprintf("%.1f KB", float64(size)/1024)
			} else if size < 1024*1024*1024 {
				sizeStr = fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
			} else {
				sizeStr = fmt.Sprintf("%.1f GB", float64(size)/(1024*1024*1024))
			}
			result.WriteString(fmt.Sprintf("[FILE] %-40s  %8s  %s\n",
				entry.Name(), sizeStr, info.ModTime().Format("2006-01-02 15:04")))
		}
	}

	return result.String(), nil
}

func deleteFileToolCallImpl(argsMap *map[string]interface{}, toolsUse *data.ToolsUse) (string, error) {
	path, ok := (*argsMap)["path"].(string)
	if !ok {
		return "", fmt.Errorf("path not found in arguments")
	}

	// Check if confirmation is needed
	needConfirm, ok := (*argsMap)["need_confirm"].(bool)
	if !ok {
		// Default to true for safety
		needConfirm = true
	}

	if needConfirm && !toolsUse.AutoApprove {
		// Get purpose if provided
		purpose, _ := (*argsMap)["purpose"].(string)
		if purpose == "" {
			purpose = fmt.Sprintf("delete the file at path: %s", path)
		}

		// Prompt user for confirmation
		ui.NeedUserConfirmToolUse("", ToolUserConfirmPrompt, purpose, toolsUse)
		if toolsUse.Confirm == data.ToolConfirmCancel {
			return fmt.Sprintf("Operation cancelled by user: delete file %s", path), UserCancelError{Reason: UserCancelReasonDeny}
		}
	}

	// Delete the file
	err := os.Remove(path)
	if err != nil {
		return fmt.Sprintf("Error deleting file %s: %v", path, err), nil
	}

	return fmt.Sprintf("Successfully deleted file %s", path), nil
}

func deleteDirectoryToolCallImpl(argsMap *map[string]interface{}, toolsUse *data.ToolsUse) (string, error) {
	path, ok := (*argsMap)["path"].(string)
	if !ok {
		return "", fmt.Errorf("path not found in arguments")
	}

	// Check if confirmation is needed
	needConfirm, ok := (*argsMap)["need_confirm"].(bool)
	if !ok {
		// Default to true for safety
		needConfirm = true
	}

	if needConfirm && !toolsUse.AutoApprove {
		// Get purpose if provided
		purpose, _ := (*argsMap)["purpose"].(string)
		if purpose == "" {
			purpose = fmt.Sprintf("delete the directory at path: %s and all its contents", path)
		}

		// Prompt user for confirmation
		ui.NeedUserConfirmToolUse("", ToolUserConfirmPrompt, purpose, toolsUse)
		if toolsUse.Confirm == data.ToolConfirmCancel {
			return fmt.Sprintf("Operation cancelled by user: delete directory %s", path), UserCancelError{Reason: UserCancelReasonDeny}
		}
	}

	// Delete the directory
	err := os.RemoveAll(path)
	if err != nil {
		return fmt.Sprintf("Error deleting directory %s: %v", path, err), nil
	}

	return fmt.Sprintf("Successfully deleted directory %s", path), nil
}

func moveToolCallImpl(argsMap *map[string]interface{}, toolsUse *data.ToolsUse) (string, error) {
	source, ok := (*argsMap)["source"].(string)
	if !ok {
		return "", fmt.Errorf("source not found in arguments")
	}

	destination, ok := (*argsMap)["destination"].(string)
	if !ok {
		return "", fmt.Errorf("destination not found in arguments")
	}

	// Check if confirmation is needed
	needConfirm, ok := (*argsMap)["need_confirm"].(bool)
	if !ok {
		// Default to true for safety
		needConfirm = true
	}

	if needConfirm && !toolsUse.AutoApprove {
		// Get purpose if provided
		purpose, _ := (*argsMap)["purpose"].(string)
		if purpose == "" {
			purpose = fmt.Sprintf("move the file or directory from %s to %s", source, destination)
		}

		// Prompt user for confirmation
		ui.NeedUserConfirmToolUse("", ToolUserConfirmPrompt, purpose, toolsUse)
		if toolsUse.Confirm == data.ToolConfirmCancel {
			return fmt.Sprintf("Operation cancelled by user: move %s to %s", source, destination), UserCancelError{Reason: UserCancelReasonDeny}
		}
	}

	// Move/rename the file or directory
	err := os.Rename(source, destination)
	if err != nil {
		return fmt.Sprintf("Error moving %s to %s: %v", source, destination, err), nil
	}

	return fmt.Sprintf("Successfully moved %s to %s", source, destination), nil
}

func searchFilesToolCallImpl(argsMap *map[string]interface{}) (string, error) {
	directory, ok := (*argsMap)["directory"].(string)
	if !ok {
		return "", fmt.Errorf("directory not found in arguments")
	}

	pattern, ok := (*argsMap)["pattern"].(string)
	if !ok {
		return "", fmt.Errorf("pattern not found in arguments")
	}

	// Check if recursive search is requested
	recursive := false
	if recursiveValue, exists := (*argsMap)["recursive"]; exists {
		if recursiveBool, ok := recursiveValue.(bool); ok {
			recursive = recursiveBool
		}
	}

	var matches []string
	var err error

	if recursive {
		// Recursive search using filepath.WalkDir
		err = filepath.WalkDir(directory, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil // Skip inaccessible paths
			}
			if d.IsDir() {
				return nil // Skip directories themselves
			}
			// Match the filename against the pattern
			matched, matchErr := filepath.Match(pattern, filepath.Base(path))
			if matchErr != nil {
				return nil // Skip invalid patterns for this file
			}
			if matched {
				matches = append(matches, path)
			}
			return nil
		})
		if err != nil {
			return fmt.Sprintf("Error walking directory %s: %v", directory, err), nil
		}
	} else {
		// Non-recursive search using filepath.Glob
		fullPattern := filepath.Join(directory, pattern)
		matches, err = filepath.Glob(fullPattern)
		if err != nil {
			return fmt.Sprintf("Error searching for files with pattern %s: %v", fullPattern, err), nil
		}
	}

	var result strings.Builder
	searchMode := "non-recursive"
	if recursive {
		searchMode = "recursive"
	}
	if len(matches) == 0 {
		result.WriteString(fmt.Sprintf("No files found in %s matching pattern '%s' (%s search)\n", directory, pattern, searchMode))
	} else {
		result.WriteString(fmt.Sprintf("Files found in %s matching pattern '%s' (%s search, %d results):\n", directory, pattern, searchMode, len(matches)))
		for _, match := range matches {
			result.WriteString(fmt.Sprintf("- %s\n", match))
		}
	}

	return result.String(), nil
}

func searchTextInFileToolCallImpl(argsMap *map[string]interface{}) (string, error) {
	path, ok := (*argsMap)["path"].(string)
	if !ok {
		return "", fmt.Errorf("path not found in arguments")
	}

	searchText, ok := (*argsMap)["text"].(string)
	if !ok {
		return "", fmt.Errorf("text not found in arguments")
	}

	// Check if case-insensitive search is requested
	caseInsensitive := false
	if ciValue, exists := (*argsMap)["case_insensitive"]; exists {
		if ciBool, ok := ciValue.(bool); ok {
			caseInsensitive = ciBool
		}
	}

	// Check if regex search is requested
	useRegex := false
	if regexValue, exists := (*argsMap)["regex"]; exists {
		if regexBool, ok := regexValue.(bool); ok {
			useRegex = regexBool
		}
	}

	// Prepare regex pattern if needed
	var regexPattern *regexp.Regexp
	if useRegex {
		patternStr := searchText
		if caseInsensitive {
			patternStr = "(?i)" + patternStr
		}
		var err error
		regexPattern, err = regexp.Compile(patternStr)
		if err != nil {
			return fmt.Sprintf("Invalid regex pattern '%s': %v", searchText, err), nil
		}
	}

	// Open the file
	file, err := os.Open(path)
	if err != nil {
		return fmt.Sprintf("Error opening file %s: %v", path, err), nil
	}
	defer file.Close()

	// Search for the text line by line
	var result strings.Builder
	scanner := bufio.NewScanner(file)
	lineNum := 0
	foundCount := 0

	// Build search mode description
	var searchMode string
	if useRegex && caseInsensitive {
		searchMode = "regex, case-insensitive"
	} else if useRegex {
		searchMode = "regex"
	} else if caseInsensitive {
		searchMode = "case-insensitive"
	} else {
		searchMode = "exact match"
	}

	result.WriteString(fmt.Sprintf("Search results for '%s' in %s (%s):\n", searchText, path, searchMode))

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		var matched bool
		if useRegex {
			matched = regexPattern.MatchString(line)
		} else if caseInsensitive {
			matched = strings.Contains(strings.ToLower(line), strings.ToLower(searchText))
		} else {
			matched = strings.Contains(line, searchText)
		}

		if matched {
			foundCount++
			result.WriteString(fmt.Sprintf("Line %d: %s\n", lineNum, line))
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Sprintf("Error reading file %s: %v", path, err), nil
	}

	if foundCount == 0 {
		result.WriteString("No matches found.")
	} else {
		result.WriteString(fmt.Sprintf("\nFound %d match(es).", foundCount))
	}

	return result.String(), nil
}

func readMultipleFilesToolCallImpl(argsMap *map[string]interface{}) (string, error) {
	pathsInterface, ok := (*argsMap)["paths"].([]interface{})
	if !ok {
		return "", fmt.Errorf("paths not found in arguments or not an array")
	}

	// Check if line numbers are requested
	includeLineNumbers := false
	if lineNumValue, exists := (*argsMap)["line_numbers"]; exists {
		if lineNumBool, ok := lineNumValue.(bool); ok {
			includeLineNumbers = lineNumBool
		}
	}

	// Convert []interface{} to []string
	paths := make([]string, len(pathsInterface))
	for i, v := range pathsInterface {
		path, ok := v.(string)
		if !ok {
			return "", fmt.Errorf("path at index %d is not a string", i)
		}
		paths[i] = path
	}

	var result strings.Builder
	result.WriteString("Contents of multiple files:\n\n")

	// Read each file
	for _, path := range paths {
		result.WriteString(fmt.Sprintf("--- File: %s ---\n", path))

		// Check file size before reading
		fileInfo, err := os.Stat(path)
		if err != nil {
			result.WriteString(fmt.Sprintf("Error accessing file %s: %v\n\n", path, err))
			continue
		}
		if fileInfo.Size() > MaxFileSize {
			result.WriteString(fmt.Sprintf("Skipped: File is too large (%d bytes, max allowed: %d bytes / %.1f MB)\n\n",
				fileInfo.Size(), MaxFileSize, float64(MaxFileSize)/(1024*1024)))
			continue
		}

		content, err := os.ReadFile(path)
		if err != nil {
			result.WriteString(fmt.Sprintf("Error reading file %s: %v\n\n", path, err))
			continue
		}

		if includeLineNumbers {
			// Add line numbers to the output
			lines := strings.Split(string(content), "\n")
			for i, line := range lines {
				result.WriteString(fmt.Sprintf("%4d | %s\n", i+1, line))
			}
		} else {
			result.WriteString(string(content))
		}
		result.WriteString("\n\n")
	}

	return result.String(), nil
}

func shellToolCallImpl(argsMap *map[string]interface{}, toolsUse *data.ToolsUse) (string, error) {
	cmdStr, ok := (*argsMap)["command"].(string)
	if !ok {
		return "", fmt.Errorf("command not found in arguments")
	}
	needConfirm, ok := (*argsMap)["need_confirm"].(bool)
	if !ok {
		// Default to true for safety
		needConfirm = true
	}

	// Get timeout from arguments, default to DefaultShellTimeout
	timeout := DefaultShellTimeout
	if timeoutValue, exists := (*argsMap)["timeout"]; exists {
		if timeoutFloat, ok := timeoutValue.(float64); ok && timeoutFloat > 0 {
			timeout = time.Duration(timeoutFloat) * time.Second
		}
	}

	if needConfirm && !toolsUse.AutoApprove {
		// Directly prompt user for confirmation
		descStr, ok := (*argsMap)["purpose"].(string)
		if !ok {
			//return "", fmt.Errorf("purpose not found in arguments")
			descStr = ""
		}
		// Use the command string as the info for confirmation
		ui.NeedUserConfirmToolUse("", ToolUserConfirmPrompt, descStr, toolsUse)
		if toolsUse.Confirm == data.ToolConfirmCancel {
			return fmt.Sprintf("Operation cancelled by user: shell command '%s'", cmdStr), UserCancelError{Reason: UserCancelReasonDeny}
		}
	}

	var errStr string

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Do the real command with timeout
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", cmdStr)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", cmdStr)
	}

	out, err := cmd.CombinedOutput()

	// Handle command exec failed
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			errStr = fmt.Sprintf("Command timed out after %v", timeout)
		} else {
			var exitCode int
			if exitError, ok := err.(*exec.ExitError); ok {
				exitCode = exitError.ExitCode()
			}
			errStr = fmt.Sprintf("Command failed with exit code %d: %v", exitCode, err)
		}
	}

	// Output the result
	outStr := string(out)
	if outStr != "" {
		outStr = outStr + "\n"
	}

	// Format error info if present
	errorInfo := ""
	if errStr != "" {
		errorInfo = fmt.Sprintf("Error: %s", errStr)
	}
	// Format output info
	outputInfo := ""
	if outStr != "" {
		outputInfo = fmt.Sprintf("Output:\n%s", outStr)
	} else {
		outputInfo = "Output: <no output>"
	}
	// Create a response that prompts the LLM to provide insightful analysis of the command output
	finalResponse := fmt.Sprintf(ToolRespShellOutput, cmdStr, errorInfo, outputInfo)

	return finalResponse, nil
}

func webFetchToolCallImpl(argsMap *map[string]interface{}) (string, error) {
	url, ok := (*argsMap)["url"].(string)
	if !ok {
		return "", fmt.Errorf("url not found in arguments")
	}

	// Call the fetch function
	results := FetchProcess([]string{url})

	// Check if FetchProcess returned any results
	if len(results) == 0 {
		// If no content was fetched or extracted, create an error message for the user.
		return fmt.Sprintf("Failed to fetch content from %s or no content was extracted. Please check the URL or try again.", url), nil
	}

	// Create and return the tool response message
	return fmt.Sprintf("Fetched content from %s:\n%s", url, results[0]), nil
}

func webSearchToolCallImpl(argsMap *map[string]interface{}, queries *[]string, references *[]map[string]interface{}, search *SearchEngine) (string, error) {
	query, ok := (*argsMap)["query"].(string)
	if !ok {
		return "", fmt.Errorf("query not found in arguments")
	}

	// Call the search function
	engine := search.Name
	var data map[string]any
	var err error
	switch engine {
	case GoogleSearchEngine:
		// Use Google Search Engine
		data, err = search.GoogleSearch(query)
	case BingSearchEngine:
		// Use Bing Search Engine
		data, err = search.BingSearch(query)
	case TavilySearchEngine:
		// Use Tavily Search Engine
		data, err = search.TavilySearch(query)
	case NoneSearchEngine:
		// Use None Search Engine
		data, err = search.NoneSearch(query)
	default:
		err = fmt.Errorf("unknown search engine: %s", engine)
	}

	if err != nil {
		return "", fmt.Errorf("error performing search for query '%s': %v", query, err)
	}
	// keep the search results for references
	*queries = append(*queries, query)
	*references = append(*references, data)

	// Convert search results to JSON string
	resultsJSON, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("error marshaling search results for query '%s': %v", query, err)
	}

	return string(resultsJSON), nil
}

func editFileToolCallImpl(argsMap *map[string]interface{}, toolsUse *data.ToolsUse, showDiff func(diff string), closeDiff func()) (string, error) {
	path, ok := (*argsMap)["path"].(string)
	if !ok {
		return "", fmt.Errorf("path not found in arguments")
	}

	// Check if confirmation is needed
	needConfirm, ok := (*argsMap)["need_confirm"].(bool)
	if !ok {
		// Default to true for safety
		needConfirm = true
	}

	// Get the edits to apply
	editsInterface, ok := (*argsMap)["edits"].([]interface{})
	if !ok {
		return "", fmt.Errorf("edits not found in arguments or not an array")
	}

	// Read the original file content
	originalContent, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("Error reading file %s: %v", path, err), nil
	}
	content := string(originalContent)

	// Apply all search-replace operations and track which ones didn't match
	modifiedContent := content
	var notFound []string
	var applied int

	for _, editInterface := range editsInterface {
		editMap, ok := editInterface.(map[string]interface{})
		if !ok {
			continue
		}

		searchText, ok := editMap["search"].(string)
		if !ok {
			continue
		}

		replaceText, ok := editMap["replace"].(string)
		if !ok {
			replaceText = "" // Default to empty string for deletions
		}

		// Check if the search text exists in the content
		if !strings.Contains(modifiedContent, searchText) {
			// Truncate long search text for display
			displayText := searchText
			if len(displayText) > 50 {
				displayText = displayText[:50] + "..."
			}
			notFound = append(notFound, displayText)
			continue
		}

		// Apply the search-replace operation
		modifiedContent = strings.ReplaceAll(modifiedContent, searchText, replaceText)
		applied++
	}

	// If no edits were applied, return a warning
	if applied == 0 && len(notFound) > 0 {
		var result strings.Builder
		result.WriteString(fmt.Sprintf("Warning: No edits were applied to %s.\n", path))
		result.WriteString("The following search patterns were not found:\n")
		for _, text := range notFound {
			result.WriteString(fmt.Sprintf("  - \"%s\"\n", text))
		}
		result.WriteString("\nPlease verify the search text matches exactly (including whitespace and line endings).")
		return result.String(), nil
	}

	// If confirmation is needed, show the diff and ask the user
	if needConfirm && !toolsUse.AutoApprove {
		// Show the diff
		diff := ui.Diff(content, modifiedContent, "", "", 3)
		showDiff(diff)

		// Get purpose if provided
		purpose, _ := (*argsMap)["purpose"].(string)
		if purpose == "" {
			purpose = fmt.Sprintf("edit the file at path: %s", path)
		}

		// Response with a prompt to let user confirm
		ui.NeedUserConfirmToolUse("", ToolUserConfirmPrompt, purpose, toolsUse)
		closeDiff() // Close the diff
		if toolsUse.Confirm == data.ToolConfirmCancel {
			return fmt.Sprintf(ToolRespDiscardEditFile, path), UserCancelError{Reason: UserCancelReasonDeny}
		}
	}

	// Write the modified content back to the file
	err = os.WriteFile(path, []byte(modifiedContent), 0644)
	if err != nil {
		return fmt.Sprintf("Error writing file %s: %v", path, err), nil
	}

	// Build success message
	var result strings.Builder
	result.WriteString(fmt.Sprintf("Successfully edited file %s (%d edit(s) applied)", path, applied))
	if len(notFound) > 0 {
		result.WriteString(fmt.Sprintf("\nWarning: %d search pattern(s) were not found:", len(notFound)))
		for _, text := range notFound {
			result.WriteString(fmt.Sprintf("\n  - \"%s\"", text))
		}
	}
	return result.String(), nil
}

func copyToolCallImpl(argsMap *map[string]interface{}, toolsUse *data.ToolsUse) (string, error) {
	source, ok := (*argsMap)["source"].(string)
	if !ok {
		return "", fmt.Errorf("source not found in arguments")
	}

	destination, ok := (*argsMap)["destination"].(string)
	if !ok {
		return "", fmt.Errorf("destination not found in arguments")
	}

	// Check if confirmation is needed
	needConfirm, ok := (*argsMap)["need_confirm"].(bool)
	if !ok {
		// Default to true for safety
		needConfirm = true
	}

	if needConfirm && !toolsUse.AutoApprove {
		// Get purpose if provided
		purpose, _ := (*argsMap)["purpose"].(string)
		if purpose == "" {
			purpose = fmt.Sprintf("copy the file or directory from %s to %s", source, destination)
		}

		// Prompt user for confirmation
		ui.NeedUserConfirmToolUse("", ToolUserConfirmPrompt, purpose, toolsUse)
		if toolsUse.Confirm == data.ToolConfirmCancel {
			return fmt.Sprintf("Operation cancelled by user: copy %s to %s", source, destination), UserCancelError{Reason: UserCancelReasonDeny}
		}
	}

	// Copy the file or directory
	err := copyFileOrDir(source, destination)
	if err != nil {
		return fmt.Sprintf("Error copying %s to %s: %v", source, destination, err), nil
	}

	return fmt.Sprintf("Successfully copied %s to %s", source, destination), nil
}

// Helper function to copy files or directories
func copyFileOrDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if srcInfo.IsDir() {
		return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Calculate the destination path
			relPath, err := filepath.Rel(src, path)
			if err != nil {
				return err
			}
			dstPath := filepath.Join(dst, relPath)

			if info.IsDir() {
				// Create directory
				return os.MkdirAll(dstPath, info.Mode())
			} else {
				// Copy file
				return copyFile(path, dstPath)
			}
		})
	} else {
		// Copy single file
		return copyFile(src, dst)
	}
}

// Helper function to copy a single file (preserves permissions)
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// Get source file info for permissions
	srcInfo, err := sourceFile.Stat()
	if err != nil {
		return err
	}

	// Create destination directory if it doesn't exist
	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return err
	}

	destinationFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destinationFile.Close()

	// Copy file contents
	_, err = destinationFile.ReadFrom(sourceFile)
	if err != nil {
		return err
	}

	// Preserve the source file's permissions
	return os.Chmod(dst, srcInfo.Mode())
}

// listMemoryToolCallImpl handles the list_memory tool call
func listMemoryToolCallImpl() (string, error) {
	memories, err := data.NewMemoryStore().Load()
	if err != nil {
		return fmt.Sprintf("Error loading memories: %v", err), nil
	}

	if len(memories) == 0 {
		return "No memories saved. The user has not asked you to remember anything yet.", nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Current saved memories (%d items):\n\n", len(memories)))
	for i, memory := range memories {
		result.WriteString(fmt.Sprintf("%d. %s\n", i+1, memory))
	}

	return result.String(), nil
}

// saveMemoryToolCallImpl handles the save_memory tool call
// Simplified design: takes complete memory content and replaces all memories
func saveMemoryToolCallImpl(argsMap *map[string]interface{}) (string, error) {
	memories, ok := (*argsMap)["memories"].(string)
	if !ok {
		return "", fmt.Errorf("memories parameter not found in arguments")
	}

	store := data.NewMemoryStore()

	// Empty string means clear all memories
	if strings.TrimSpace(memories) == "" {
		err := store.Clear()
		if err != nil {
			return fmt.Sprintf("Error clearing memories: %v", err), nil
		}
		return "Successfully cleared all memories", nil
	}

	// Calculate new memories from content
	lines := strings.Split(memories, "\n")
	var newMemories []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- ") {
			memory := strings.TrimPrefix(line, "- ")
			if memory != "" {
				newMemories = append(newMemories, memory)
			}
		} else if line != "" && !strings.HasPrefix(line, "#") {
			newMemories = append(newMemories, line)
		}
	}

	// Replace all memories with new content
	err := store.Save(newMemories)
	if err != nil {
		return fmt.Sprintf("Error updating memories: %v", err), nil
	}

	// Count how many memories were saved
	savedMemories, _ := store.Load()
	return fmt.Sprintf("Successfully updated memories (%d items saved)", len(savedMemories)), nil
}

// switchAgentToolCallImpl handles the switch_agent tool call
func switchAgentToolCallImpl(argsMap *map[string]interface{}, toolsUse *data.ToolsUse) (string, error) {
	name, ok := (*argsMap)["name"].(string)
	if !ok {
		return "", fmt.Errorf("agent name is required")
	}

	store := data.NewConfigStore()

	// If name is "list", return available agents
	if name == "list" {
		agents := store.GetAllAgents()
		var sb strings.Builder
		sb.WriteString("Available Agents:\n")

		var names []string
		for n := range agents {
			names = append(names, n)
		}
		sort.Strings(names)

		// List all agents with details
		for _, n := range names {
			ag := agents[n]
			sb.WriteString(fmt.Sprintf("- %s: Model=%s, ThinkingLevel=%s, Tools=%v, Capabilities=%v\n",
				n, ag.Model.Model, ag.Think, ag.Tools, ag.Capabilities))
			if ag.SystemPrompt != "" {
				// Show more of the system prompt to help the model decide
				sysPrompt := strings.ReplaceAll(ag.SystemPrompt, "\n", " ")
				sb.WriteString(fmt.Sprintf("  System Prompt: %s\n", sysPrompt))
			}
			if ag.Template != "" {
				// Show more of the template to help the model decide
				template := strings.ReplaceAll(ag.Template, "\n", " ")
				sb.WriteString(fmt.Sprintf("  Template: %s\n", template))
			}
		}
		sb.WriteString("\nTo switch to an agent, use this tool with the agent's name.")
		return sb.String(), nil
	}

	// Check if agent exists
	if store.GetAgent(name) == nil {
		return fmt.Sprintf("Agent '%s' not found. Use 'list' to see available agents.", name), nil
	}

	// If already in this agent, just return message
	if store.GetActiveAgentName() == name {
		return fmt.Sprintf("You are already using agent '%s'. No need to switch.", name), nil
	}

	// Check for confirmation
	needConfirm, ok := (*argsMap)["need_confirm"].(bool)
	if !ok {
		needConfirm = true // Default to true
	}

	if needConfirm && !toolsUse.AutoApprove {
		purpose := fmt.Sprintf("switch to agent '%s'", name)
		ui.NeedUserConfirmToolUse("", ToolUserConfirmPrompt, purpose, toolsUse)
		if toolsUse.Confirm == data.ToolConfirmCancel {
			return fmt.Sprintf("Operation cancelled by user: switch to agent %s", name), UserCancelError{Reason: UserCancelReasonDeny}
		}
	}

	// Set active agent
	err := store.SetActiveAgent(name)
	if err != nil {
		return fmt.Sprintf("Failed to set active agent: %v", err), nil
	}

	// Set instruction for new agent
	var instruction string
	if v, ok := (*argsMap)["instruction"].(string); ok {
		instruction = v
	}

	// Signal to switch
	return fmt.Sprintf("Switching to agent '%s'...", name), SwitchAgentError{TargetAgent: name, Instruction: instruction}
}

// listAgentToolCallImpl handles the list_agent tool call
// Returns a formatted list of all available agents with their capabilities
func listAgentToolCallImpl() (string, error) {
	store := data.NewConfigStore()
	agents := store.GetAllAgents()

	if len(agents) == 0 {
		return "No agents configured. Use 'gllm agent add' to create agents.", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Available Agents (%d total):\n\n", len(agents)))

	// Sort agent names for consistent output
	var names []string
	for n := range agents {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, name := range names {
		ag := agents[name]
		sb.WriteString(fmt.Sprintf("## %s\n", name))
		sb.WriteString(fmt.Sprintf("  Model: %s\n", ag.Model.Model))
		sb.WriteString(fmt.Sprintf("  Provider: %s\n", ag.Model.Provider))
		sb.WriteString(fmt.Sprintf("  Thinking Level: %s\n", ag.Think))

		if len(ag.Tools) > 0 {
			sb.WriteString(fmt.Sprintf("  Tools: %v\n", ag.Tools))
		} else {
			sb.WriteString("  Tools: none\n")
		}

		if len(ag.Capabilities) > 0 {
			sb.WriteString(fmt.Sprintf("  Capabilities: %v\n", ag.Capabilities))
		}

		if ag.SystemPrompt != "" {
			// Show more system prompt to help model understand the agent
			sysPrompt := strings.ReplaceAll(ag.SystemPrompt, "\n", " ")
			sb.WriteString(fmt.Sprintf("  System Prompt: %s\n", sysPrompt))
		}
		if ag.Template != "" {
			// Show more template to help model understand the agent
			template := strings.ReplaceAll(ag.Template, "\n", " ")
			sb.WriteString(fmt.Sprintf("  Template: %s\n", template))
		}

		sb.WriteString("\n")
	}

	sb.WriteString("Use spawn_subagents to invoke a sub-agent, or switch_agent to hand off to another agent.")
	return sb.String(), nil
}

// spawnSubAgentsToolCallImpl handles the spawn_subagents tool call
// Invokes one or more sub-agents and returns progress summary
func spawnSubAgentsToolCallImpl(
	argsMap *map[string]interface{},
	toolsUse *data.ToolsUse,
	executor *SubAgentExecutor,
) (string, error) {
	if executor == nil {
		return "", fmt.Errorf("sub-agent executor not initialized")
	}

	// Parse tasks array
	tasksInterface, ok := (*argsMap)["tasks"].([]interface{})
	if !ok {
		return "", fmt.Errorf("tasks parameter is required and must be an array")
	}

	if len(tasksInterface) == 0 {
		return "No tasks provided. Please specify at least one task.", nil
	}

	timeout := 300 * time.Second // Default 5 minutes
	if timeoutVal, exists := (*argsMap)["timeout"]; exists {
		if timeoutFloat, ok := timeoutVal.(float64); ok && timeoutFloat > 0 {
			timeout = time.Duration(timeoutFloat) * time.Second
		}
	}

	// Check if confirmation is needed
	needConfirm, ok := (*argsMap)["need_confirm"].(bool)
	if !ok {
		needConfirm = true // Default to true for safety
	}

	if needConfirm && !toolsUse.AutoApprove {
		// Build brief description of tasks
		var taskDesc strings.Builder
		for i, task := range tasksInterface {
			if i > 0 {
				taskDesc.WriteString("\n")
			}
			if tm, ok := task.(map[string]interface{}); ok {
				taskDesc.WriteString(fmt.Sprintf("- Task %d: %s (Agent: %s)", i+1, tm["task_key"], tm["agent"]))
			}
		}
		ui.NeedUserConfirmToolUse("", ToolUserConfirmPrompt, taskDesc.String(), toolsUse)
		if toolsUse.Confirm == data.ToolConfirmCancel {
			return "Operation cancelled by user: spawn sub-agents", UserCancelError{Reason: UserCancelReasonDeny}
		}
	}

	// Convert tasks to SubAgentTask structs
	var tasks []*SubAgentTask
	for i, taskInterface := range tasksInterface {
		taskMap, ok := taskInterface.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("task at index %d is not a valid object", i)
		}

		agentName, ok := taskMap["agent"].(string)
		if !ok || agentName == "" {
			return "", fmt.Errorf("task at index %d missing required 'agent' field", i)
		}

		instruction, ok := taskMap["instruction"].(string)
		if !ok || instruction == "" {
			return "", fmt.Errorf("task at index %d missing required 'instruction' field", i)
		}

		taskKey, ok := taskMap["task_key"].(string)
		if !ok || taskKey == "" {
			return "", fmt.Errorf("task at index %d missing required 'task_key' field", i)
		}

		// Parse optional input_keys
		var inputKeys []string
		if keysInterface, ok := taskMap["input_keys"].([]interface{}); ok {
			for _, k := range keysInterface {
				if keyStr, ok := k.(string); ok {
					inputKeys = append(inputKeys, keyStr)
				}
			}
		}

		// Parse optional wait flag (explicit barrier)
		wait := false
		if waitVal, ok := taskMap["wait"].(bool); ok {
			wait = waitVal
		}

		tasks = append(tasks, &SubAgentTask{
			AgentName:   agentName,
			Instruction: instruction,
			TaskKey:     taskKey,
			InputKeys:   inputKeys,
			Wait:        wait,
		})
	}

	// Submit and execute all tasks (always wait)
	executor.SubmitBatch(tasks)
	results := executor.Execute(timeout)

	// Return formatted summary
	return executor.FormatSummary(results), nil
}

// getStateToolCallImpl handles the get_state tool call
// Retrieves a value from SharedState
func getStateToolCallImpl(
	argsMap *map[string]interface{},
	state *data.SharedState,
) (string, error) {
	if state == nil {
		return "", fmt.Errorf("shared state not initialized")
	}

	key, ok := (*argsMap)["key"].(string)
	if !ok || key == "" {
		return "", fmt.Errorf("key parameter is required")
	}

	// Get metadata for context
	meta := state.GetMetadata(key)
	if meta == nil {
		return fmt.Sprintf("Key '%s' not found in SharedState. Use list_state to see available keys.", key), nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Key: %s\n", key))
	result.WriteString(fmt.Sprintf("Created by: %s\n", meta.CreatedBy))
	result.WriteString(fmt.Sprintf("Type: %s\n", meta.ContentType))
	result.WriteString(fmt.Sprintf("Size: %d bytes\n", meta.Size))

	value := state.GetString(key)
	result.WriteString("\nValue:\n")
	result.WriteString(value)

	return result.String(), nil
}

// setStateToolCallImpl handles the set_state tool call
// Stores a value in SharedState
func setStateToolCallImpl(
	argsMap *map[string]interface{},
	agentName string,
	state *data.SharedState,
) (string, error) {
	if state == nil {
		return "", fmt.Errorf("shared state not initialized")
	}

	key, ok := (*argsMap)["key"].(string)
	if !ok || key == "" {
		return "", fmt.Errorf("key parameter is required")
	}

	value, ok := (*argsMap)["value"]
	if !ok {
		return "", fmt.Errorf("value parameter is required")
	}

	// Check if key already exists
	existed := state.Has(key)

	err := state.Set(key, value, agentName)
	if err != nil {
		return "", fmt.Errorf("failed to set state: %v", err)
	}

	if existed {
		return fmt.Sprintf("Successfully updated key '%s' in SharedState.", key), nil
	}
	return fmt.Sprintf("Successfully created key '%s' in SharedState.", key), nil
}

// listStateToolCallImpl handles the list_state tool call
// Lists all keys and metadata in SharedState
func listStateToolCallImpl(state *data.SharedState) (string, error) {
	if state == nil {
		return "", fmt.Errorf("shared state not initialized")
	}

	return state.FormatList(), nil
}

// activateSkillToolCallImpl handles the activate_skill tool call.
func activateSkillToolCallImpl(argsMap *map[string]interface{}, toolsUse *data.ToolsUse) (string, error) {
	name, ok := (*argsMap)["name"].(string)
	if !ok {
		return "", fmt.Errorf("skill name not found in arguments")
	}

	// Get or create skill manager (singleton pattern)
	sm := GetSkillManager()
	// Activate skill: Get skill details
	skillDetails, desc, tree, err := sm.ActivateSkill(name)
	if err != nil {
		return "", err
	}

	// Check if confirmation is needed (default logic: always confirm unless AutoApprove is true)
	if !toolsUse.AutoApprove {
		description := "Activate Skill:\n" + name + "\n\nDescription:\n" + desc + "\n\nResources:\n" + tree
		ui.NeedUserConfirmToolUse("", ToolUserConfirmPrompt, description, toolsUse)
		if toolsUse.Confirm == data.ToolConfirmCancel {
			return fmt.Sprintf("Operation cancelled by user: activate skill %s", name), UserCancelError{Reason: UserCancelReasonDeny}
		}
	}

	return skillDetails, nil
}
