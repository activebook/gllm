package service

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/activebook/gllm/data"
)

// Tool robustness constants
const (
	MaxFileSize = 20 * 1024 * 1024 // 20MB
)

// Tool implementation functions

// Shared implementation functions that work with map[string]interface{} arguments
// These functions contain the actual logic that can be shared between OpenAI and OpenChat

func processFileContentRange(path string, content []byte, includeLineNumbers bool, offset int, limit int) string {
	lines := strings.Split(string(content), "\n")
	totalLines := len(lines)

	if totalLines == 0 || (totalLines == 1 && lines[0] == "") {
		return fmt.Sprintf("Content of %s (0 lines):\n<empty file>", path)
	}

	// Clamp offset
	if offset < 0 {
		offset = 0
	}
	if offset >= totalLines {
		return fmt.Sprintf("Error: Offset %d exceeds total lines (%d) in file %s", offset+1, totalLines, path)
	}

	// Calculate end index
	end := totalLines
	if limit > 0 && offset+limit < totalLines {
		end = offset + limit
	}

	selectedLines := lines[offset:end]

	var response string
	// Build response header
	if offset == 0 && (limit <= 0 || limit >= totalLines) {
		// Full file reading
		if includeLineNumbers {
			response = fmt.Sprintf("Content of %s (%d lines, with line numbers):\n", path, totalLines)
		} else {
			response = fmt.Sprintf("Content of %s (%d lines):\n", path, totalLines)
		}
	} else {
		if limit > 0 {
			response = fmt.Sprintf("Content of %s (lines %d-%d of %d):\n", path, offset+1, end, totalLines)
		} else {
			response = fmt.Sprintf("Content of %s (from line %d of %d):\n", path, offset+1, totalLines)
		}
	}

	if includeLineNumbers {
		var numberedContent strings.Builder
		for i, line := range selectedLines {
			numberedContent.WriteString(fmt.Sprintf("%4d | %s\n", offset+i+1, line))
		}
		response += numberedContent.String()
	} else {
		response += strings.Join(selectedLines, "\n")
	}

	return response
}

func readFileToolCallImpl(argsMap *map[string]interface{}) (string, error) {
	if err := CheckToolPermission(ToolReadFile, argsMap); err != nil {
		return "", err
	}

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

	// Parse optional offset and limit parameters
	offset := 0
	limit := -1 // -1 means read all lines

	if offsetVal, exists := (*argsMap)["offset"]; exists {
		switch v := offsetVal.(type) {
		case float64:
			offset = int(v)
		case int:
			offset = v
		}
		if offset > 0 {
			offset-- // Convert from 1-indexed to 0-indexed
		}
	}

	// Support both 'limit' and 'lines' parameter names (learned from model behavior)
	for _, paramName := range []string{"limit", "lines"} {
		if limitVal, exists := (*argsMap)[paramName]; exists {
			switch v := limitVal.(type) {
			case float64:
				limit = int(v)
			case int:
				limit = v
			}
			break // Use first found parameter
		}
	}

	response := processFileContentRange(path, content, includeLineNumbers, offset, limit)
	return response, nil
}

func writeFileToolCallImpl(argsMap *map[string]interface{}, op *OpenProcessor) (string, error) {
	if err := CheckToolPermission(ToolWriteFile, argsMap); err != nil {
		return "", err
	}

	path, ok := (*argsMap)["path"].(string)
	if !ok {
		return "", fmt.Errorf("path not found in arguments")
	}
	op.toolsUse.FilePath = path // Set the file path in op.toolsUse for potential use in confirmation prompt

	content, ok := (*argsMap)["content"].(string)
	if !ok {
		return "", fmt.Errorf("content not found in arguments")
	}

	if !op.toolsUse.AutoApprove {
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
		diff := op.interaction.RequestDiff(currentContent, content, 3)
		op.fileHooks.OpenDiff(path, content)
		op.showDiff(diff)

		// Get purpose if provided
		purpose, _ := (*argsMap)["purpose"].(string)
		if purpose == "" {
			purpose = fmt.Sprintf("write content to the file at path: %s", path)
		}

		// Prompt user for confirmation
		if op.interaction != nil {
			op.interaction.RequestConfirm(purpose, op.toolsUse)
		}
		op.closeDiff() // Close the diff
		if op.toolsUse.Confirm == data.ToolConfirmCancel {
			op.fileHooks.RejectDiff(path)
			return fmt.Sprintf("Operation cancelled by user: write to file %s", path), UserCancelError{Reason: UserCancelReasonDeny}
		}
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Sprintf("Error creating directory for %s: %v", path, err), nil
	}

	// Determine file permissions
	mode := os.FileMode(0644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode()
	}

	// Write the file
	err := os.WriteFile(path, []byte(content), mode)
	if err != nil {
		op.fileHooks.RejectDiff(path)
		return fmt.Sprintf("Error writing file %s: %v", path, err), nil
	}
	op.fileHooks.AcceptDiff(path)
	return fmt.Sprintf("Successfully wrote to file %s", path), nil
}

func createDirectoryToolCallImpl(argsMap *map[string]interface{}, op *OpenProcessor) (string, error) {
	if err := CheckToolPermission(ToolCreateDirectory, argsMap); err != nil {
		return "", err
	}

	path, ok := (*argsMap)["path"].(string)
	if !ok {
		return "", fmt.Errorf("path not found in arguments")
	}

	if !op.toolsUse.AutoApprove {
		// Get purpose if provided
		purpose, _ := (*argsMap)["purpose"].(string)
		if purpose == "" {
			purpose = fmt.Sprintf("create the directory at path: %s", path)
		}

		// Prompt user for confirmation
		if op.interaction != nil {
			op.interaction.RequestConfirm(purpose, op.toolsUse)
		}
		if op.toolsUse.Confirm == data.ToolConfirmCancel {
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
	if err := CheckToolPermission(ToolListDirectory, argsMap); err != nil {
		return "", err
	}

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

func deleteFileToolCallImpl(argsMap *map[string]interface{}, op *OpenProcessor) (string, error) {
	if err := CheckToolPermission(ToolDeleteFile, argsMap); err != nil {
		return "", err
	}

	path, ok := (*argsMap)["path"].(string)
	if !ok {
		return "", fmt.Errorf("path not found in arguments")
	}
	op.toolsUse.FilePath = path

	if !op.toolsUse.AutoApprove {
		// Get purpose if provided
		purpose, _ := (*argsMap)["purpose"].(string)
		if purpose == "" {
			purpose = fmt.Sprintf("delete the file at path: %s", path)
		}

		// Prompt user for confirmation
		if op.interaction != nil {
			op.interaction.RequestConfirm(purpose, op.toolsUse)
		}
		if op.toolsUse.Confirm == data.ToolConfirmCancel {
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

func deleteDirectoryToolCallImpl(argsMap *map[string]interface{}, op *OpenProcessor) (string, error) {
	if err := CheckToolPermission(ToolDeleteDirectory, argsMap); err != nil {
		return "", err
	}

	path, ok := (*argsMap)["path"].(string)
	if !ok {
		return "", fmt.Errorf("path not found in arguments")
	}

	if !op.toolsUse.AutoApprove {
		// Get purpose if provided
		purpose, _ := (*argsMap)["purpose"].(string)
		if purpose == "" {
			purpose = fmt.Sprintf("delete the directory at path: %s and all its contents", path)
		}

		// Prompt user for confirmation
		if op.interaction != nil {
			op.interaction.RequestConfirm(purpose, op.toolsUse)
		}
		if op.toolsUse.Confirm == data.ToolConfirmCancel {
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

func moveToolCallImpl(argsMap *map[string]interface{}, op *OpenProcessor) (string, error) {
	if err := CheckToolPermission(ToolMove, argsMap); err != nil {
		return "", err
	}

	source, ok := (*argsMap)["source"].(string)
	if !ok {
		return "", fmt.Errorf("source not found in arguments")
	}

	destination, ok := (*argsMap)["destination"].(string)
	if !ok {
		return "", fmt.Errorf("destination not found in arguments")
	}

	if !op.toolsUse.AutoApprove {
		// Get purpose if provided
		purpose, _ := (*argsMap)["purpose"].(string)
		if purpose == "" {
			purpose = fmt.Sprintf("move the file or directory from %s to %s", source, destination)
		}

		// Prompt user for confirmation
		if op.interaction != nil {
			op.interaction.RequestConfirm(purpose, op.toolsUse)
		}
		if op.toolsUse.Confirm == data.ToolConfirmCancel {
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
	if err := CheckToolPermission(ToolSearchFiles, argsMap); err != nil {
		return "", err
	}

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
	if err := CheckToolPermission(ToolSearchTextInFile, argsMap); err != nil {
		return "", err
	}

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

	// Search for the text line by line using bufio.Reader to handle
	// arbitrarily long lines without hitting a scanner token-size limit.
	var result strings.Builder
	reader := bufio.NewReader(file)
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

	for {
		lineBytes, err := reader.ReadBytes('\n')
		lineNum++
		line := strings.TrimRight(string(lineBytes), "\r\n")

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

		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Sprintf("Error reading file %s: %v", path, err), nil
		}
	}

	if foundCount == 0 {
		result.WriteString("No matches found.")
	} else {
		result.WriteString(fmt.Sprintf("\nFound %d match(es).", foundCount))
	}

	return result.String(), nil
}

func readMultipleFilesToolCallImpl(argsMap *map[string]interface{}) (string, error) {
	if err := CheckToolPermission(ToolReadMultipleFiles, argsMap); err != nil {
		return "", err
	}

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

	// Parse optional offset and limit parameters
	// NOTE: Range reading deliberately omitted from read_multiple_files.
	// The tool is designed for batch reading of complete files for contextual understanding.
	// If range reading is needed on specific files, use read_file instead.

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

// replaceFirstOccurrence replaces the single unique occurrence of search in content.
// Returns (newContent, count) where count is the number of matches found:
//   - count == 0: not found
//   - count == 1: replaced successfully
//   - count > 1:  ambiguous — caller must reject and request more context
func replaceFirstOccurrence(content, search, replace string) (string, int) {
	count := strings.Count(content, search)
	if count != 1 {
		return content, count
	}
	idx := strings.Index(content, search)
	return content[:idx] + replace + content[idx+len(search):], 1
}

// normalizeLineWS normalizes per-line whitespace for fuzzy comparison:
// converts CRLF→LF, expands tabs to 4 spaces, and strips trailing whitespace.
// It does NOT alter leading indentation depth, only the character type.
func normalizeLineWS(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(strings.ReplaceAll(l, "\t", "    "), " \t")
	}
	return strings.Join(lines, "\n")
}

// applyWSNormalizedReplace finds search in content using line-level whitespace
// normalization and replaces the matched original lines with replace.
// Returns (result, true) on a unique normalized match; ("", false) otherwise.
// This handles the most common LLM hallucination: minor whitespace/tab drift.
func applyWSNormalizedReplace(content, search, replace string) (string, bool) {
	normContent := normalizeLineWS(content)
	normSearch := normalizeLineWS(search)

	if strings.Count(normContent, normSearch) != 1 {
		return "", false
	}

	// Line-level splice: find the matching line range in normalized space,
	// then replace those lines in the original to preserve authentic indentation
	// for any surrounding context that was not part of the search block.
	origLines := strings.Split(content, "\n")
	normLines := strings.Split(normContent, "\n")
	searchLines := strings.Split(normSearch, "\n")
	replaceLines := strings.Split(replace, "\n")
	sLen := len(searchLines)

	for i := 0; i <= len(normLines)-sLen; i++ {
		match := true
		for j := 0; j < sLen; j++ {
			if normLines[i+j] != searchLines[j] {
				match = false
				break
			}
		}
		if match {
			out := make([]string, 0, len(origLines)-sLen+len(replaceLines))
			out = append(out, origLines[:i]...)
			out = append(out, replaceLines...)
			out = append(out, origLines[i+sLen:]...)
			return strings.Join(out, "\n"), true
		}
	}
	return "", false
}

// editOutcome records the result of a single validated edit for the success report.
type editOutcome struct {
	displaySearch string
	normalized    bool // true if matched only via WS normalization fallback
}

// validateEditSchema enforces a strict whitelist on each edit object's keys.
// The ONLY valid keys are "search" and "replace".  Any other key — regardless
// of its name — means the model put a value somewhere it doesn't belong, which
// is a hallucination.  We report every unexpected key verbatim so the model
// knows exactly what to remove.
func validateEditSchema(editsInterface []interface{}) string {
	var errs []string

	for i, editInterface := range editsInterface {
		editMap, ok := editInterface.(map[string]interface{})
		if !ok {
			continue // type/format mismatch is handled downstream in Phase 1
		}

		var problems []string

		// Whitelist check: reject every key that isn't "search" or "replace".
		for k := range editMap {
			if k != "search" && k != "replace" {
				problems = append(problems, fmt.Sprintf(
					"    unexpected field %q — remove it (only \"search\" and \"replace\" are allowed)", k))
			}
		}

		// Required-field check: both must be present (possibly in addition to
		// the spurious keys above — e.g. model sent search+replace+expected).
		if _, ok := editMap["search"].(string); !ok {
			problems = append(problems, `    missing required field "search" (must be a string)`)
		}
		if _, ok := editMap["replace"].(string); !ok {
			problems = append(problems, `    missing required field "replace" (must be a string)`)
		}

		if len(problems) > 0 {
			sort.Strings(problems)
			errs = append(errs, fmt.Sprintf(
				"edit[%d]: schema violation — each edit must contain exactly "+
					"{\"search\": \"<old text>\", \"replace\": \"<new text>\"} and nothing else.\n"+
					"  Problems found:\n%s",
				i, strings.Join(problems, "\n")))
		}
	}
	if len(errs) > 0 {
		var msg strings.Builder
		msg.WriteString(fmt.Sprintf(
			"EDIT REJECTED — no changes written.\n"+
				"The 'edits' array contains %d edit(s) with invalid field names.\n\n"+
				"Correct schema for every edit object:\n"+
				"  { \"search\": \"<exact text to find>\", \"replace\": \"<replacement text>\" }\n\n"+
				"Violations:\n", len(errs)))
		for _, e := range errs {
			msg.WriteString(fmt.Sprintf("  • %s\n\n", e))
		}
		msg.WriteString("Fix the field names above and retry the full batch.")
		return msg.String()
	}
	return ""
}

func editFileToolCallImpl(argsMap *map[string]interface{}, op *OpenProcessor) (string, error) {
	if err := CheckToolPermission(ToolEditFile, argsMap); err != nil {
		return "", err
	}

	path, ok := (*argsMap)["path"].(string)
	if !ok {
		return "", fmt.Errorf("path not found in arguments")
	}
	op.toolsUse.FilePath = path

	editsInterface, ok := (*argsMap)["edits"].([]interface{})
	if !ok {
		return "", fmt.Errorf("edits not found in arguments or not an array")
	}

	// ── Phase 0: Schema defence ────────────────────────────────────────────────
	// Reject immediately if any edit item uses wrong field names (e.g. "new"
	// instead of "replace"). No file I/O is performed. The returned message
	// teaches the model the correct schema so it can self-correct and retry.
	if schemaErr := validateEditSchema(editsInterface); schemaErr != "" {
		return "", fmt.Errorf("%s", schemaErr)
	}

	// Read the original file content once.
	originalContent, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("Error reading file %s: %v", path, err), nil
	}
	content := string(originalContent)

	// ── Phase 1: Validate & simulate ALL edits before touching disk ────────────
	// We accumulate into simulatedContent so each edit operates on the result of
	// the previous one — mirroring real application order — while the original
	// content is preserved for the diff in Phase 3.
	simulatedContent := content
	var outcomes []editOutcome
	var failures []string

	for i, editInterface := range editsInterface {
		editMap, ok := editInterface.(map[string]interface{})
		if !ok {
			failures = append(failures, fmt.Sprintf("edit[%d]: invalid format (expected object)", i))
			continue
		}

		searchText, ok := editMap["search"].(string)
		if !ok || searchText == "" {
			failures = append(failures, fmt.Sprintf("edit[%d]: missing or empty 'search' field", i))
			continue
		}

		replaceText, _ := editMap["replace"].(string) // empty string is valid (deletion)

		display := searchText
		if len(display) > 60 {
			display = display[:60] + "..."
		}

		// Strategy 1: exact match (requires uniqueness)
		result, count := replaceFirstOccurrence(simulatedContent, searchText, replaceText)
		if count == 1 {
			simulatedContent = result
			outcomes = append(outcomes, editOutcome{displaySearch: display})
			continue
		}
		if count > 1 {
			failures = append(failures, fmt.Sprintf(
				"edit[%d]: ambiguous — search text appears %d times (must be exactly 1).\n"+
					"         Expand the search block with more surrounding context to make it unique.\n"+
					"         search: %q",
				i, count, display))
			continue
		}

		// Strategy 2: whitespace-normalised fallback (count == 0 from exact)
		if wsResult, found := applyWSNormalizedReplace(simulatedContent, searchText, replaceText); found {
			simulatedContent = wsResult
			outcomes = append(outcomes, editOutcome{displaySearch: display, normalized: true})
			continue
		}

		// All strategies exhausted
		failures = append(failures, fmt.Sprintf(
			"edit[%d]: not found — search text does not appear in the file\n"+
				"         (checked exact match and whitespace-normalised match).\n"+
				"         Use read_file with line_numbers=true to obtain exact content before retrying.\n"+
				"         search: %q",
			i, display))
	}

	// ── Phase 2: Abort ALL if any edit failed (no partial writes) ──────────────
	if len(failures) > 0 {
		var msg strings.Builder
		msg.WriteString(fmt.Sprintf(
			"EDIT ABORTED — no changes were written to %s.\n"+
				"%d of %d edit(s) failed:\n\n",
			path, len(failures), len(editsInterface)))
		for _, f := range failures {
			msg.WriteString(fmt.Sprintf("  • %s\n\n", f))
		}
		msg.WriteString("Fix all failing edits and retry the entire batch.")
		return msg.String(), nil
	}

	// ── Phase 3: Show diff and request user confirmation ──────────────────────
	if !op.toolsUse.AutoApprove {
		diff := op.interaction.RequestDiff(content, simulatedContent, 3)
		op.fileHooks.OpenDiff(path, simulatedContent)
		op.showDiff(diff)

		purpose, _ := (*argsMap)["purpose"].(string)
		if purpose == "" {
			purpose = fmt.Sprintf("edit file: %s", path)
		}
		if op.interaction != nil {
			op.interaction.RequestConfirm(purpose, op.toolsUse)
		}
		op.closeDiff()
		if op.toolsUse.Confirm == data.ToolConfirmCancel {
			op.fileHooks.RejectDiff(path)
			return fmt.Sprintf(ToolRespDiscardEditFile, path), UserCancelError{Reason: UserCancelReasonDeny}
		}
	}

	// ── Phase 4: Write (only reached when all edits validated and user approved) ─
	// Determine file permissions
	mode := os.FileMode(0644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode()
	}

	if err := os.WriteFile(path, []byte(simulatedContent), mode); err != nil {
		op.fileHooks.RejectDiff(path)
		return fmt.Sprintf("Error writing file %s: %v", path, err), nil
	}
	op.fileHooks.AcceptDiff(path)

	// Build success report
	var result strings.Builder
	result.WriteString(fmt.Sprintf("Successfully edited %s — %d edit(s) applied:\n", path, len(outcomes)))
	for i, o := range outcomes {
		note := ""
		if o.normalized {
			note = " [whitespace-normalized]"
		}
		result.WriteString(fmt.Sprintf("  [%d] %s%s\n", i+1, o.displaySearch, note))
	}
	return result.String(), nil
}

func copyToolCallImpl(argsMap *map[string]interface{}, op *OpenProcessor) (string, error) {
	if err := CheckToolPermission(ToolCopy, argsMap); err != nil {
		return "", err
	}

	source, ok := (*argsMap)["source"].(string)
	if !ok {
		return "", fmt.Errorf("source not found in arguments")
	}

	destination, ok := (*argsMap)["destination"].(string)
	if !ok {
		return "", fmt.Errorf("destination not found in arguments")
	}

	if !op.toolsUse.AutoApprove {
		// Get purpose if provided
		purpose, _ := (*argsMap)["purpose"].(string)
		if purpose == "" {
			purpose = fmt.Sprintf("copy the file or directory from %s to %s", source, destination)
		}

		// Prompt user for confirmation
		if op.interaction != nil {
			op.interaction.RequestConfirm(purpose, op.toolsUse)
		}
		if op.toolsUse.Confirm == data.ToolConfirmCancel {
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
