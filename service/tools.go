package service

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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

func writeFileToolCallImpl(argsMap *map[string]interface{}, toolsUse *ToolsUse) (string, error) {
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
		// Directly prompt user for confirmation
		confirm, err := NeedUserConfirm(fmt.Sprintf("write content to the file at path: %s", path), ToolUserConfirmPrompt)
		if err != nil {
			return "", err
		}
		if !confirm {
			return fmt.Sprintf("Operation cancelled by user: write to file %s", path), nil
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

func createDirectoryToolCallImpl(argsMap *map[string]interface{}) (string, error) {
	path, ok := (*argsMap)["path"].(string)
	if !ok {
		return "", fmt.Errorf("path not found in arguments")
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
		if entry.IsDir() {
			result.WriteString(fmt.Sprintf("[DIR]  %s\n", entry.Name()))
		} else {
			result.WriteString(fmt.Sprintf("[FILE] %s\n", entry.Name()))
		}
	}

	return result.String(), nil
}

func deleteFileToolCallImpl(argsMap *map[string]interface{}, toolsUse *ToolsUse) (string, error) {
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
		// Directly prompt user for confirmation
		confirm, err := NeedUserConfirm(fmt.Sprintf("delete the file at path: %s", path), ToolUserConfirmPrompt)
		if err != nil {
			return "", err
		}
		if !confirm {
			return fmt.Sprintf("Operation cancelled by user: delete file %s", path), nil
		}
	}

	// Delete the file
	err := os.Remove(path)
	if err != nil {
		return fmt.Sprintf("Error deleting file %s: %v", path, err), nil
	}

	return fmt.Sprintf("Successfully deleted file %s", path), nil
}

func deleteDirectoryToolCallImpl(argsMap *map[string]interface{}, toolsUse *ToolsUse) (string, error) {
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
		// Directly prompt user for confirmation
		confirm, err := NeedUserConfirm(fmt.Sprintf("delete the directory at path: %s and all its contents", path), ToolUserConfirmPrompt)
		if err != nil {
			return "", err
		}
		if !confirm {
			return fmt.Sprintf("Operation cancelled by user: delete directory %s", path), nil
		}
	}

	// Delete the directory
	err := os.RemoveAll(path)
	if err != nil {
		return fmt.Sprintf("Error deleting directory %s: %v", path, err), nil
	}

	return fmt.Sprintf("Successfully deleted directory %s", path), nil
}

func moveToolCallImpl(argsMap *map[string]interface{}, toolsUse *ToolsUse) (string, error) {
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
		// Directly prompt user for confirmation
		confirm, err := NeedUserConfirm(fmt.Sprintf("move the file or directory from %s to %s", source, destination), ToolUserConfirmPrompt)
		if err != nil {
			return "", err
		}
		if !confirm {
			return fmt.Sprintf("Operation cancelled by user: move %s to %s", source, destination), nil
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

	// Search for files matching the pattern
	fullPattern := filepath.Join(directory, pattern)
	matches, err := filepath.Glob(fullPattern)
	if err != nil {
		return fmt.Sprintf("Error searching for files with pattern %s: %v", fullPattern, err), nil
	}

	var result strings.Builder
	if len(matches) == 0 {
		result.WriteString(fmt.Sprintf("No files found in %s matching pattern %s\n", directory, pattern))
	} else {
		result.WriteString(fmt.Sprintf("Files found in %s matching pattern %s:\n", directory, pattern))
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

	result.WriteString(fmt.Sprintf("Search results for '%s' in %s:\n", searchText, path))

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if strings.Contains(line, searchText) {
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

func shellToolCallImpl(argsMap *map[string]interface{}, toolsUse *ToolsUse) (string, error) {
	cmdStr, ok := (*argsMap)["command"].(string)
	if !ok {
		return "", fmt.Errorf("command not found in arguments")
	}
	needConfirm, ok := (*argsMap)["need_confirm"].(bool)
	if !ok {
		// there is no need_confirm parameter, so we assume it's false
		needConfirm = false
	}
	if needConfirm && !toolsUse.AutoApprove {
		// Directly prompt user for confirmation
		descStr, ok := (*argsMap)["purpose"].(string)
		if !ok {
			//return "", fmt.Errorf("purpose not found in arguments")
			descStr = ""
		}
		confirm, err := NeedUserConfirm(fmt.Sprintf(ToolRespConfirmShell, cmdStr, descStr), ToolUserConfirmPrompt)
		if err != nil {
			return "", err
		}
		if !confirm {
			return fmt.Sprintf("Operation cancelled by user: shell command '%s'", cmdStr), nil
		}
	}

	var errStr string

	// Do the real command
	var out []byte
	var err error
	if runtime.GOOS == "windows" {
		out, err = exec.Command("cmd", "/C", cmdStr).CombinedOutput()
	} else {
		out, err = exec.Command("sh", "-c", cmdStr).CombinedOutput()
	}

	// Handle command exec failed
	if err != nil {
		var exitCode int
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		}
		errStr = fmt.Sprintf("Command failed with exit code %d: %v", exitCode, err)
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
	case DummySearchEngine:
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

// func editFileToolCallImpl(argsMap *map[string]interface{}, toolsUse *ToolsUse) (string, error) {
// 	path, ok := (*argsMap)["path"].(string)
// 	if !ok {
// 		return "", fmt.Errorf("path not found in arguments")
// 	}

// 	// Check if confirmation is needed
// 	needConfirm, ok := (*argsMap)["need_confirm"].(bool)
// 	if !ok {
// 		// Default to true for safety
// 		needConfirm = true
// 	}

// 	// Get the edits to apply
// 	editsInterface, ok := (*argsMap)["edits"].([]interface{})
// 	if !ok {
// 		return "", fmt.Errorf("edits not found in arguments or not an array")
// 	}

// 	// If confirmation is needed, ask the user before proceeding
// 	if needConfirm && !toolsUse.AutoApprove {
// 		var editsDescription strings.Builder
// 		editsDescription.WriteString("The following edits will be applied to the file:\n")
// 		for _, editInterface := range editsInterface {
// 			editMap, ok := editInterface.(map[string]interface{})
// 			if !ok {
// 				continue
// 			}

// 			line, _ := editMap["line"].(float64) // JSON numbers are float64
// 			content, _ := editMap["content"].(string)
// 			operation, _ := editMap["operation"].(string)

// 			if operation == "add" {
// 				editsDescription.WriteString(fmt.Sprintf("  - Insert at line %d: %s\n", int(line), content))
// 			} else if operation == "delete" || content == "" {
// 				editsDescription.WriteString(fmt.Sprintf("  - Delete line %d\n", int(line)))
// 			} else {
// 				editsDescription.WriteString(fmt.Sprintf("  - Replace line %d with: %s\n", int(line), content))
// 			}
// 		}

// 		outStr := fmt.Sprintf(ToolRespConfirmFileOp, fmt.Sprintf("edit file %s", path), editsDescription.String())
// 		return outStr, nil
// 	}

// 	// Convert edits to a structured format
// 	type Edit struct {
// 		Line      int
// 		Content   string
// 		Operation string // "add", "delete", or "replace"
// 	}

// 	var edits []Edit
// 	for _, editInterface := range editsInterface {
// 		editMap, ok := editInterface.(map[string]interface{})
// 		if !ok {
// 			continue
// 		}

// 		line, _ := editMap["line"].(float64) // JSON numbers are float64
// 		content, _ := editMap["content"].(string)
// 		operation, hasOp := editMap["operation"].(string)

// 		// Determine operation if not explicitly provided
// 		if !hasOp {
// 			if content == "" {
// 				operation = "delete"
// 			} else {
// 				operation = "replace"
// 			}
// 		}

// 		edits = append(edits, Edit{
// 			Line:      int(line),
// 			Content:   content,
// 			Operation: operation,
// 		})
// 	}

// 	// Sort edits by line number in descending order to avoid line number shifts during editing
// 	// For add operations, we want to add at the specified line position
// 	// For delete/replace operations, we work with existing line positions
// 	sort.Slice(edits, func(i, j int) bool {
// 		// If both are add operations, sort by line descending
// 		if edits[i].Operation == "add" && edits[j].Operation == "add" {
// 			return edits[i].Line > edits[j].Line
// 		}
// 		// If one is add and one isn't, add operations should happen first (higher line numbers)
// 		if edits[i].Operation == "add" && edits[j].Operation != "add" {
// 			return true
// 		}
// 		if edits[i].Operation != "add" && edits[j].Operation == "add" {
// 			return false
// 		}
// 		// If neither is add, sort by line descending
// 		return edits[i].Line > edits[j].Line
// 	})

// 	// Read the file
// 	content, err := os.ReadFile(path)
// 	if err != nil {
// 		return fmt.Sprintf("Error reading file %s: %v", path, err), nil
// 	}

// 	// Split content into lines
// 	lines := strings.Split(string(content), "\n")

// 	// Apply edits
// 	for _, edit := range edits {
// 		lineIndex := edit.Line - 1 // Convert to 0-indexed

// 		switch edit.Operation {
// 		case "add", "++":
// 			// Insert new content at the specified line position
// 			// This works for inserting at the beginning (line 1), middle, or end(-1 or more than last line)
// 			if lineIndex < 0 || lineIndex > len(lines) {
// 				lineIndex = len(lines)
// 			}
// 			// Insert the new line at the specified position
// 			lines = append(lines[:lineIndex], append([]string{edit.Content}, lines[lineIndex:]...)...)

// 		case "delete", "--":
// 			// Delete line (only if within range)
// 			if lineIndex >= 0 && lineIndex < len(lines) {
// 				lines = append(lines[:lineIndex], lines[lineIndex+1:]...)
// 			}

// 		case "replace", "==":
// 			// Replace or append line
// 			if lineIndex >= 0 && lineIndex < len(lines) {
// 				lines[lineIndex] = edit.Content
// 			} else if lineIndex == len(lines) {
// 				// Append new line at the end
// 				lines = append(lines, edit.Content)
// 			}
// 		}
// 	}

// 	// Join lines back together
// 	newContent := strings.Join(lines, "\n")

// 	// Write the modified content back to the file
// 	err = os.WriteFile(path, []byte(newContent), 0644)
// 	if err != nil {
// 		return fmt.Sprintf("Error writing file %s: %v", path, err), nil
// 	}

// 	return fmt.Sprintf("Successfully edited file %s", path), nil
// }

func editFileToolCallImpl(argsMap *map[string]interface{}, toolsUse *ToolsUse, showDiff func(diff string), closeDiff func()) (string, error) {
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

	// Apply all search-replace operations
	modifiedContent := content
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

		// Apply the search-replace operation
		modifiedContent = strings.ReplaceAll(modifiedContent, searchText, replaceText)
	}

	// If confirmation is needed, show the diff and ask the user
	if needConfirm && !toolsUse.AutoApprove {
		// Show the diff
		diff := Diff(content, modifiedContent, "", "", 3)
		showDiff(diff)

		// Response with a prompt to let user confirm
		confirm, err := NeedUserConfirm("", ToolUserConfirmPrompt)
		closeDiff() // Close the diff
		if err != nil {
			return "", err
		}
		if !confirm {
			return fmt.Sprintf(ToolRespDiscardEditFile, path), nil
		}
	}

	// Write the modified content back to the file
	err = os.WriteFile(path, []byte(modifiedContent), 0644)
	if err != nil {
		return fmt.Sprintf("Error writing file %s: %v", path, err), nil
	}

	return fmt.Sprintf("Successfully edited file %s", path), nil
}

func copyToolCallImpl(argsMap *map[string]interface{}, toolsUse *ToolsUse) (string, error) {
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
		// Directly prompt user for confirmation
		confirm, err := NeedUserConfirm(fmt.Sprintf("copy the file or directory from %s to %s", source, destination), ToolUserConfirmPrompt)
		if err != nil {
			return "", err
		}
		if !confirm {
			return fmt.Sprintf("Operation cancelled by user: copy %s to %s", source, destination), nil
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

// Helper function to copy a single file
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

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
	_, err = sourceFile.Seek(0, 0)
	if err != nil {
		return err
	}

	_, err = destinationFile.Seek(0, 0)
	if err != nil {
		return err
	}

	_, err = destinationFile.ReadFrom(sourceFile)
	return err
}
