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

	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
)

// File and folder operation tools for LLM
// list all function names here:
// - read_file
// - write_file
// - create_directory
// - list_directory
// - delete_file
// - delete_directory
// - move
// - search_files
// - search_text_in_file
// - read_multiple_files
// - shell
// - web_search

// Tool definitions for file operations
func (ll *LangLogic) getOpenChatTools() []*model.Tool {
	var tools []*model.Tool

	// Shell tool
	getOpenChatShellTool := ll.getOpenChatShellTool()
	tools = append(tools, getOpenChatShellTool)

	// Read file tool
	readFileFunc := model.FunctionDefinition{
		Name:        "read_file",
		Description: "Read the contents of a file from the filesystem. Optionally include line numbers for easier referencing.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The path to the file to read.",
				},
				"line_numbers": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether to include line numbers in the output.",
					"default":     false,
				},
			},
			"required": []string{"path"},
		},
	}
	readFileTool := model.Tool{
		Type:     model.ToolTypeFunction,
		Function: &readFileFunc,
	}
	tools = append(tools, &readFileTool)

	// Write file tool
	writeFileFunc := model.FunctionDefinition{
		Name:        "write_file",
		Description: "Write content to a file in the filesystem. Creates the file if it doesn't exist, or overwrites it if it does.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The path to the file to write to.",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "The content to write to the file.",
				},
			},
			"required": []string{"path", "content"},
		},
	}
	writeFileTool := model.Tool{
		Type:     model.ToolTypeFunction,
		Function: &writeFileFunc,
	}
	tools = append(tools, &writeFileTool)

	// Create directory tool
	createDirFunc := model.FunctionDefinition{
		Name:        "create_directory",
		Description: "Create a new directory in the filesystem.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The path of the directory to create.",
				},
			},
			"required": []string{"path"},
		},
	}
	createDirTool := model.Tool{
		Type:     model.ToolTypeFunction,
		Function: &createDirFunc,
	}
	tools = append(tools, &createDirTool)

	// List directory tool
	listDirFunc := model.FunctionDefinition{
		Name:        "list_directory",
		Description: "List the contents of a directory in the filesystem.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The path of the directory to list.",
				},
			},
			"required": []string{"path"},
		},
	}
	listDirTool := model.Tool{
		Type:     model.ToolTypeFunction,
		Function: &listDirFunc,
	}
	tools = append(tools, &listDirTool)

	// Delete file tool
	deleteFileFunc := model.FunctionDefinition{
		Name:        "delete_file",
		Description: "Delete a file from the filesystem.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The path of the file to delete.",
				},
			},
			"required": []string{"path"},
		},
	}
	deleteFileTool := model.Tool{
		Type:     model.ToolTypeFunction,
		Function: &deleteFileFunc,
	}
	tools = append(tools, &deleteFileTool)

	// Delete directory tool
	deleteDirFunc := model.FunctionDefinition{
		Name:        "delete_directory",
		Description: "Delete a directory from the filesystem.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The path of the directory to delete.",
				},
			},
			"required": []string{"path"},
		},
	}
	deleteDirTool := model.Tool{
		Type:     model.ToolTypeFunction,
		Function: &deleteDirFunc,
	}
	tools = append(tools, &deleteDirTool)

	// Move file/directory tool
	moveFunc := model.FunctionDefinition{
		Name:        "move",
		Description: "Move or rename a file or directory in the filesystem.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"source": map[string]interface{}{
					"type":        "string",
					"description": "The current path of the file or directory.",
				},
				"destination": map[string]interface{}{
					"type":        "string",
					"description": "The new path for the file or directory.",
				},
			},
			"required": []string{"source", "destination"},
		},
	}
	moveTool := model.Tool{
		Type:     model.ToolTypeFunction,
		Function: &moveFunc,
	}
	tools = append(tools, &moveTool)

	// Search files tool
	searchFilesFunc := model.FunctionDefinition{
		Name:        "search_files",
		Description: "Search for files in a directory matching a pattern.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"directory": map[string]interface{}{
					"type":        "string",
					"description": "The directory to search in.",
				},
				"pattern": map[string]interface{}{
					"type":        "string",
					"description": "The pattern to match (e.g. '*.txt', 'config.*').",
				},
			},
			"required": []string{"directory", "pattern"},
		},
	}
	searchFilesTool := model.Tool{
		Type:     model.ToolTypeFunction,
		Function: &searchFilesFunc,
	}
	tools = append(tools, &searchFilesTool)

	// Search text in file tool
	searchTextFunc := model.FunctionDefinition{
		Name:        "search_text_in_file",
		Description: "Search for specific text within a file and return matching lines with line numbers.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The path to the file to search in.",
				},
				"text": map[string]interface{}{
					"type":        "string",
					"description": "The text to search for.",
				},
			},
			"required": []string{"path", "text"},
		},
	}
	searchTextTool := model.Tool{
		Type:     model.ToolTypeFunction,
		Function: &searchTextFunc,
	}
	tools = append(tools, &searchTextTool)

	// Read multiple files tool
	readMultipleFilesFunc := model.FunctionDefinition{
		Name:        "read_multiple_files",
		Description: "Read the contents of multiple files from the filesystem. Optionally include line numbers for easier referencing.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"paths": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "An array of file paths to read.",
				},
				"line_numbers": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether to include line numbers in the output.",
					"default":     false,
				},
			},
			"required": []string{"paths"},
		},
	}
	readMultipleFilesTool := model.Tool{
		Type:     model.ToolTypeFunction,
		Function: &readMultipleFilesFunc,
	}
	tools = append(tools, &readMultipleFilesTool)

	return tools
}

func (ll *LangLogic) getOpenChatShellTool() *model.Tool {
	shellFunc := model.FunctionDefinition{
		Name: "shell",
		Description: `Executes a shell command on the user's local machine.

IMPORTANT: This function is highly powerful and potentially dangerous.
Always prioritize user safety. Do not execute commands that could delete files (rm),
modify system configurations, or install software without explicit user consent.

Good use cases:
- Running simple, non-destructive commands like 'ls -l' to list files.
- Checking system status with commands like 'uname -a'.
- Performing simple file operations like 'cat file.txt' to read a file.
- Performing complex tasks using shell tricks, pipeline, or scripting, etc.

Example of a good call:
User asks: "Can you list the files in my current directory?"
LLM should call with:
{
  "command": "ls -l",
  "purpose": "To list the files and folders in your current directory.",
  "need_confirm": true
}
`,
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{
					"type": "string",
					"description": "The exact, single-line shell command to be executed. " +
						"The command could be complex for complex task, but should be non-destructive.",
				},
				"purpose": map[string]interface{}{
					"type": "string",
					"description": "A clear, user-friendly explanation of what the command does and why it's being run. " +
						"This will be shown to the user for confirmation.",
				},
				"need_confirm": map[string]interface{}{
					"type": "boolean",
					"description": "Specifies whether to prompt the user for confirmation before running the command. " +
						"This must always be true for any command that modifies or deletes data, or has any potential side effects. " +
						"It should only be false for simple, read-only commands explicitly requested by the user in the same turn, like 'ls' or 'pwd'.",
					"default": true,
				},
			},
			"required": []string{"command", "purpose"},
		},
	}

	shellTool := model.Tool{
		Type:     model.ToolTypeFunction,
		Function: &shellFunc,
	}

	return &shellTool
}

func (ll *LangLogic) getOpenChatSearchTool() *model.Tool {
	engine := GetSearchEngine()
	switch engine {
	case GoogleSearchEngine:
		// Use Google Search Engine
	case BingSearchEngine:
		// Use Bing Search Engine
	case TavilySearchEngine:
		// Use Tavily Search Engine
	case NoneSearchEngine:
		// Use None Search Engine
	default:
	}

	searchFunc := model.FunctionDefinition{
		Name:        "web_search",
		Description: "Retrieve the most relevant and up-to-date information from the web.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "The search term or question to find information about.",
				},
			},
			"required": []string{"query"},
		},
	}
	searchTool := model.Tool{
		Type:     model.ToolTypeFunction,
		Function: &searchFunc,
	}

	return &searchTool
}

// Tool implementation functions

func (c *OpenChat) processReadFileToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	path, ok := (*argsMap)["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path not found in arguments")
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
		response := fmt.Sprintf("Error reading file %s: %v", path, err)
		toolMessage.Content = &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(response),
		}
		return &toolMessage, nil
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

	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(response),
	}
	return &toolMessage, nil
}

func (c *OpenChat) processWriteFileToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	path, ok := (*argsMap)["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path not found in arguments")
	}

	content, ok := (*argsMap)["content"].(string)
	if !ok {
		return nil, fmt.Errorf("content not found in arguments")
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		response := fmt.Sprintf("Error creating directory for %s: %v", path, err)
		toolMessage.Content = &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(response),
		}
		return &toolMessage, nil
	}

	// Write the file
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		response := fmt.Sprintf("Error writing file %s: %v", path, err)
		toolMessage.Content = &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(response),
		}
		return &toolMessage, nil
	}

	response := fmt.Sprintf("Successfully wrote to file %s", path)
	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(response),
	}
	return &toolMessage, nil
}

func (c *OpenChat) processCreateDirectoryToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	path, ok := (*argsMap)["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path not found in arguments")
	}

	// Create the directory
	err := os.MkdirAll(path, 0755)
	if err != nil {
		response := fmt.Sprintf("Error creating directory %s: %v", path, err)
		toolMessage.Content = &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(response),
		}
		return &toolMessage, nil
	}

	response := fmt.Sprintf("Successfully created directory %s", path)
	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(response),
	}
	return &toolMessage, nil
}

func (c *OpenChat) processListDirectoryToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	path, ok := (*argsMap)["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path not found in arguments")
	}

	// List directory contents
	entries, err := os.ReadDir(path)
	if err != nil {
		response := fmt.Sprintf("Error reading directory %s: %v", path, err)
		toolMessage.Content = &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(response),
		}
		return &toolMessage, nil
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

	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(result.String()),
	}
	return &toolMessage, nil
}

func (c *OpenChat) processDeleteFileToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	path, ok := (*argsMap)["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path not found in arguments")
	}

	// Delete the file
	err := os.Remove(path)
	if err != nil {
		response := fmt.Sprintf("Error deleting file %s: %v", path, err)
		toolMessage.Content = &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(response),
		}
		return &toolMessage, nil
	}

	response := fmt.Sprintf("Successfully deleted file %s", path)
	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(response),
	}
	return &toolMessage, nil
}

func (c *OpenChat) processDeleteDirectoryToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	path, ok := (*argsMap)["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path not found in arguments")
	}

	// Delete the directory
	err := os.RemoveAll(path)
	if err != nil {
		response := fmt.Sprintf("Error deleting directory %s: %v", path, err)
		toolMessage.Content = &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(response),
		}
		return &toolMessage, nil
	}

	response := fmt.Sprintf("Successfully deleted directory %s", path)
	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(response),
	}
	return &toolMessage, nil
}

func (c *OpenChat) processMoveToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	source, ok := (*argsMap)["source"].(string)
	if !ok {
		return nil, fmt.Errorf("source not found in arguments")
	}

	destination, ok := (*argsMap)["destination"].(string)
	if !ok {
		return nil, fmt.Errorf("destination not found in arguments")
	}

	// Move/rename the file or directory
	err := os.Rename(source, destination)
	if err != nil {
		response := fmt.Sprintf("Error moving %s to %s: %v", source, destination, err)
		toolMessage.Content = &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(response),
		}
		return &toolMessage, nil
	}

	response := fmt.Sprintf("Successfully moved %s to %s", source, destination)
	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(response),
	}
	return &toolMessage, nil
}

func (c *OpenChat) processSearchFilesToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	directory, ok := (*argsMap)["directory"].(string)
	if !ok {
		return nil, fmt.Errorf("directory not found in arguments")
	}

	pattern, ok := (*argsMap)["pattern"].(string)
	if !ok {
		return nil, fmt.Errorf("pattern not found in arguments")
	}

	// Search for files matching the pattern
	fullPattern := filepath.Join(directory, pattern)
	matches, err := filepath.Glob(fullPattern)
	if err != nil {
		response := fmt.Sprintf("Error searching for files with pattern %s: %v", fullPattern, err)
		toolMessage.Content = &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(response),
		}
		return &toolMessage, nil
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

	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(result.String()),
	}
	return &toolMessage, nil
}

func (c *OpenChat) processSearchTextInFileToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	path, ok := (*argsMap)["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path not found in arguments")
	}

	searchText, ok := (*argsMap)["text"].(string)
	if !ok {
		return nil, fmt.Errorf("text not found in arguments")
	}

	// Open the file
	file, err := os.Open(path)
	if err != nil {
		response := fmt.Sprintf("Error opening file %s: %v", path, err)
		toolMessage.Content = &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(response),
		}
		return &toolMessage, nil
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
		response := fmt.Sprintf("Error reading file %s: %v", path, err)
		toolMessage.Content = &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(response),
		}
		return &toolMessage, nil
	}

	if foundCount == 0 {
		result.WriteString("No matches found.")
	} else {
		result.WriteString(fmt.Sprintf("\nFound %d match(es).", foundCount))
	}

	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(result.String()),
	}
	return &toolMessage, nil
}

func (c *OpenChat) processReadMultipleFilesToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	pathsInterface, ok := (*argsMap)["paths"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("paths not found in arguments or not an array")
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
			return nil, fmt.Errorf("path at index %d is not a string", i)
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

	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(result.String()),
	}
	return &toolMessage, nil
}

func (c *OpenChat) processShellToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	// Create a tool message
	// Tool Message's Content wouldn't be serialized in the response
	// That's not a problem, because each time, the tool result could be different!
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	cmdStr, ok := (*argsMap)["command"].(string)
	if !ok {
		return nil, fmt.Errorf("command not found in arguments for tool call ID %s", toolCall.ID)
	}
	needConfirm, ok := (*argsMap)["need_confirm"].(bool)
	if !ok {
		needConfirm = true
	}
	if needConfirm {
		// Response with a prompt to let user confirm
		descStr, ok := (*argsMap)["purpose"].(string)
		if !ok {
			return nil, fmt.Errorf("purpose not found in arguments for tool call ID %s", toolCall.ID)
		}
		outStr := fmt.Sprintf(ExecRespTmplConfirm, cmdStr, descStr)
		toolMessage.Content = &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(outStr),
		}
		return &toolMessage, nil
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
	finalResponse := fmt.Sprintf(ExecRespTmplOutput, cmdStr, errorInfo, outputInfo)

	// Create and return the tool response message
	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(finalResponse),
	}
	return &toolMessage, nil
}

func (c *OpenChat) processSearchToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	query, ok := (*argsMap)["query"].(string)
	if !ok {
		return nil, fmt.Errorf("query not found in arguments for tool call ID %s", toolCall.ID)
	}

	// Call the search function
	engine := GetSearchEngine()
	var data map[string]any
	var err error
	switch engine {
	case GoogleSearchEngine:
		// Use Google Search Engine
		data, err = GoogleSearch(query)
	case BingSearchEngine:
		// Use Bing Search Engine
		data, err = BingSearch(query)
	case TavilySearchEngine:
		// Use Tavily Search Engine
		data, err = TavilySearch(query)
	case NoneSearchEngine:
		// Use None Search Engine
		data, err = NoneSearch(query)
	default:
		err = fmt.Errorf("unknown search engine: %s", engine)
	}

	if err != nil {
		return nil, fmt.Errorf("error performing search for query '%s': %v", query, err)
	}
	// keep the search results for references
	c.queries = append(c.queries, query)
	c.references = append(c.references, &data)

	// Convert search results to JSON string
	resultsJSON, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("error marshaling search results for query '%s': %v", query, err)
	}

	// Create and return the tool response message
	return &model.ChatCompletionMessage{
		Role: model.ChatMessageRoleTool,
		Content: &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(string(resultsJSON)),
		}, Name: Ptr(""),
		ToolCallID: toolCall.ID,
	}, nil
}
