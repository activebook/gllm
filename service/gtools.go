package service

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"google.golang.org/genai"
)

/*
A limitation of Gemini is that you can’t use a function call and a built-in tool at the same time. ADK,
when using Gemini as the underlying LLM, takes advantage of Gemini’s built-in ability to do Google searches,
and uses function calling to invoke your custom ADK tools.
So agent tools can come in handy, as you can have a main agent,
that delegates live searches to a search agent that has the GoogleSearchTool configured,
and another tool agent that makes use of a custom tool function.

Usually, this happens when you get a mysterious error like this one
(reported against ADK for Python):
{'error': {'code': 400, 'message': 'Tool use with function calling is unsupported',
 'status': 'INVALID_ARGUMENT'}}.
This means that you can’t use a built-in tool and function calling at the same time in the same agent.
*/

// Tool definitions for Gemini 2
func (ag *Agent) getGemini2Tools() []*genai.Tool {
	var tools []*genai.Tool

	// Add web search tool
	// tools = append(tools, ll.getGemini2WebSearchTool())

	// Add shell tool
	tools = append(tools, ag.getGemini2ShellTool())

	// Add web fetch tool
	webFetchTool := &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{{
			Name:        "web_fetch",
			Description: "Fetch content from a URL and extract the main text content.",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"url": {
						Type:        genai.TypeString,
						Description: "The URL to fetch content from.",
					},
				},
				Required: []string{"url"},
			},
		}},
	}
	tools = append(tools, webFetchTool)

	// Add read file tool
	readFileTool := &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{{
			Name:        "read_file",
			Description: "Read the contents of a file from the filesystem. Optionally include line numbers for easier referencing.",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"path": {
						Type:        genai.TypeString,
						Description: "The path to the file to read.",
					},
					"line_numbers": {
						Type:        genai.TypeBoolean,
						Description: "Whether to include line numbers in the output.",
						Default:     false,
					},
				},
				Required: []string{"path"},
			},
		}},
	}
	tools = append(tools, readFileTool)

	// Add write file tool
	writeFileTool := &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{{
			Name:        "write_file",
			Description: "Write content to a file in the filesystem. Creates the file if it doesn't exist, or overwrites it if it does.",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"path": {
						Type:        genai.TypeString,
						Description: "The path to the file to write to.",
					},
					"content": {
						Type:        genai.TypeString,
						Description: "The content to write to the file.",
					},
					"need_confirm": {
						Type: genai.TypeBoolean,
						Description: "Specifies whether to prompt the user for confirmation before writing to the file. " +
							"This should always be true for safety.",
						Default: true,
					},
				},
				Required: []string{"path", "content"},
			},
		}},
	}
	tools = append(tools, writeFileTool)

	// Add create directory tool
	createDirTool := &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{{
			Name:        "create_directory",
			Description: "Create a new directory in the filesystem.",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"path": {
						Type:        genai.TypeString,
						Description: "The path of the directory to create.",
					},
				},
				Required: []string{"path"},
			},
		}},
	}
	tools = append(tools, createDirTool)

	// Add list directory tool
	listDirTool := &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{{
			Name:        "list_directory",
			Description: "List the contents of a directory in the filesystem.",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"path": {
						Type:        genai.TypeString,
						Description: "The path of the directory to list.",
					},
				},
				Required: []string{"path"},
			},
		}},
	}
	tools = append(tools, listDirTool)

	// Add delete file tool
	deleteFileTool := &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{{
			Name:        "delete_file",
			Description: "Delete a file from the filesystem.",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"path": {
						Type:        genai.TypeString,
						Description: "The path of the file to delete.",
					},
					"need_confirm": {
						Type: genai.TypeBoolean,
						Description: "Specifies whether to prompt the user for confirmation before deleting the file. " +
							"This should always be true for safety.",
						Default: true,
					},
				},
				Required: []string{"path"},
			},
		}},
	}
	tools = append(tools, deleteFileTool)

	// Add delete directory tool
	deleteDirTool := &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{{
			Name:        "delete_directory",
			Description: "Delete a directory from the filesystem.",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"path": {
						Type:        genai.TypeString,
						Description: "The path of the directory to delete.",
					},
					"need_confirm": {
						Type: genai.TypeBoolean,
						Description: "Specifies whether to prompt the user for confirmation before deleting the directory. " +
							"This should always be true for safety.",
						Default: true,
					},
				},
				Required: []string{"path"},
			},
		}},
	}
	tools = append(tools, deleteDirTool)

	// Add move file/directory tool
	moveTool := &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{{
			Name:        "move",
			Description: "Move or rename a file or directory in the filesystem.",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"source": {
						Type:        genai.TypeString,
						Description: "The current path of the file or directory.",
					},
					"destination": {
						Type:        genai.TypeString,
						Description: "The new path for the file or directory.",
					},
					"need_confirm": {
						Type: genai.TypeBoolean,
						Description: "Specifies whether to prompt the user for confirmation before moving the file or directory. " +
							"This should always be true for safety.",
						Default: true,
					},
				},
				Required: []string{"source", "destination"},
			},
		}},
	}
	tools = append(tools, moveTool)

	// Add search files tool
	searchFilesTool := &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{{
			Name:        "search_files",
			Description: "Search for files in a directory matching a pattern.",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"directory": {
						Type:        genai.TypeString,
						Description: "The directory to search in.",
					},
					"pattern": {
						Type:        genai.TypeString,
						Description: "The pattern to match (e.g. '*.txt', 'config.*').",
					},
				},
				Required: []string{"directory", "pattern"},
			},
		}},
	}
	tools = append(tools, searchFilesTool)

	// Add search text in file tool
	searchTextTool := &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{{
			Name:        "search_text_in_file",
			Description: "Search for specific text within a file and return matching lines with line numbers.",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"path": {
						Type:        genai.TypeString,
						Description: "The path to the file to search in.",
					},
					"text": {
						Type:        genai.TypeString,
						Description: "The text to search for.",
					},
				},
				Required: []string{"path", "text"},
			},
		}},
	}
	tools = append(tools, searchTextTool)

	// Add read multiple files tool
	readMultipleFilesTool := &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{{
			Name:        "read_multiple_files",
			Description: "Read the contents of multiple files from the filesystem. Optionally include line numbers for easier referencing.",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"paths": {
						Type: genai.TypeArray,
						Items: &genai.Schema{
							Type: genai.TypeString,
						},
						Description: "An array of file paths to read.",
					},
					"line_numbers": {
						Type:        genai.TypeBoolean,
						Description: "Whether to include line numbers in the output.",
						Default:     false,
					},
				},
				Required: []string{"paths"},
			},
		}},
	}
	tools = append(tools, readMultipleFilesTool)

	// Add edit file tool
	editFileTool := &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{{
			Name:        "edit_file",
			Description: "Edit specific lines in a file. This tool allows adding, replacing, or deleting content at specific line numbers.",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"path": {
						Type:        genai.TypeString,
						Description: "The path to the file to edit.",
					},
					"edits": {
						Type: genai.TypeArray,
						Items: &genai.Schema{
							Type: genai.TypeObject,
							Properties: map[string]*genai.Schema{
								"line": {
									Type:        genai.TypeInteger,
									Description: "The line number to edit (1-indexed). For add operations, this is the position where content will be inserted.",
								},
								"content": {
									Type:        genai.TypeString,
									Description: "The new content for the line. Empty string to delete the line (unless operation is specified).",
								},
								"operation": {
									Type: genai.TypeString,
									Description: "The operation to perform on the specified line (1-indexed):\n" +
										"- 'add' or '++' to insert content at the given line position (if line is greater than the number of lines, content is appended).\n" +
										"- 'delete' or '--' to remove the line.\n" +
										"- 'replace' or '==' to replace the line content.\n" +
										"If 'operation' is omitted, 'delete' is assumed when 'content' is empty, otherwise 'replace' is used.\n" +
										"Accepted values: 'add', 'delete', 'replace'.",
									Enum: []string{"add", "delete", "replace"},
								},
							},
							Required: []string{"line"},
						},
						Description: "Array of edits to apply to the file. Each edit specifies a line number and the operation to perform.",
					},
					"need_confirm": {
						Type: genai.TypeBoolean,
						Description: "Specifies whether to prompt the user for confirmation before editing the file. " +
							"This should always be true for safety.",
						Default: true,
					},
				},
				Required: []string{"path", "edits"},
			},
		}},
	}
	tools = append(tools, editFileTool)

	// Add copy file/directory tool
	copyTool := &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{{
			Name:        "copy",
			Description: "Copy a file or directory from one location to another in the filesystem.",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"source": {
						Type:        genai.TypeString,
						Description: "The current path of the file or directory to copy.",
					},
					"destination": {
						Type:        genai.TypeString,
						Description: "The destination path for the file or directory copy.",
					},
					"need_confirm": {
						Type: genai.TypeBoolean,
						Description: "Specifies whether to prompt the user for confirmation before copying the file or directory. " +
							"This should be true for safety if it needs overwrite.",
						Default: true,
					},
				},
				Required: []string{"source", "destination"},
			},
		}},
	}
	tools = append(tools, copyTool)

	return tools
}

func (ag *Agent) getGemini2ShellTool() *genai.Tool {
	tool := &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{{
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
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"command": {
						Type: genai.TypeString,
						Description: "The exact, single-line shell command to be executed. " +
							"The command could be complex for complex task, but should be non-destructive.",
					},
					"purpose": {
						Type: genai.TypeString,
						Description: "A clear, user-friendly explanation of what the command does and why it's being run. " +
							"This will be shown to the user for confirmation.",
					},
					"need_confirm": {
						Type: genai.TypeBoolean,
						Description: "Specifies whether to prompt the user for confirmation before running the command. " +
							"This must always be true for any command that modifies or deletes data, or has any potential side effects. " +
							"It should only be false for simple, read-only commands explicitly requested by the user in the same turn, like 'ls' or 'pwd'.",
						Default: true,
					},
				},
				Required: []string{"command", "purpose"},
			},
		}},
	}
	return tool
}

func (ag *Agent) getGemini2WebSearchTool() *genai.Tool {
	// return google embedding search tool
	tool := &genai.Tool{GoogleSearch: &genai.GoogleSearch{}}
	return tool
}

func (ag *Agent) getGemini2CodeExecTool() *genai.Tool {
	// return google embedding search tool
	tool := &genai.Tool{CodeExecution: &genai.ToolCodeExecution{}}
	return tool
}

// Tool implementation functions for Gemini 2

func (ag *Agent) processGemini2ReadFileToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	path, ok := call.Args["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path not found in arguments")
	}

	// Check if line numbers are requested
	includeLineNumbers := false
	if lineNumValue, exists := call.Args["line_numbers"]; exists {
		if lineNumBool, ok := lineNumValue.(bool); ok {
			includeLineNumbers = lineNumBool
		}
	}

	// Read the file
	content, err := os.ReadFile(path)
	if err != nil {
		response := fmt.Sprintf("Error reading file %s: %v", path, err)
		resp.Response = map[string]any{
			"output": response,
			"error":  err.Error(),
		}
		return &resp, nil
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

	resp.Response = map[string]any{
		"output": response,
		"error":  "",
	}
	return &resp, nil
}

func (ag *Agent) processGemini2WriteFileToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	path, ok := call.Args["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path not found in arguments")
	}

	content, ok := call.Args["content"].(string)
	if !ok {
		return nil, fmt.Errorf("content not found in arguments")
	}

	// Check if confirmation is needed
	needConfirm, ok := call.Args["need_confirm"].(bool)
	if !ok {
		// Default to true for safety
		needConfirm = true
	}

	if needConfirm && !skipToolsConfirm {
		// Response with a prompt to let user confirm
		outStr := fmt.Sprintf(ToolRespConfirmFileOp, fmt.Sprintf("write to file %s", path), fmt.Sprintf("write content to the file at path: %s", path))
		resp.Response = map[string]any{
			"output": outStr,
			"error":  "",
		}
		return &resp, nil
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		response := fmt.Sprintf("Error creating directory for %s: %v", path, err)
		resp.Response = map[string]any{
			"output": response,
			"error":  err.Error(),
		}
		return &resp, nil
	}

	// Write the file
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		response := fmt.Sprintf("Error writing file %s: %v", path, err)
		resp.Response = map[string]any{
			"output": response,
			"error":  err.Error(),
		}
		return &resp, nil
	}

	response := fmt.Sprintf("Successfully wrote to file %s", path)
	resp.Response = map[string]any{
		"output": response,
		"error":  "",
	}
	return &resp, nil
}

func (ag *Agent) processGemini2CreateDirectoryToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	path, ok := call.Args["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path not found in arguments")
	}

	// Create the directory
	err := os.MkdirAll(path, 0755)
	if err != nil {
		response := fmt.Sprintf("Error creating directory %s: %v", path, err)
		resp.Response = map[string]any{
			"output": response,
			"error":  err.Error(),
		}
		return &resp, nil
	}

	response := fmt.Sprintf("Successfully created directory %s", path)
	resp.Response = map[string]any{
		"output": response,
		"error":  "",
	}
	return &resp, nil
}

func (ag *Agent) processGemini2ListDirectoryToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	path, ok := call.Args["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path not found in arguments")
	}

	// List directory contents
	entries, err := os.ReadDir(path)
	if err != nil {
		response := fmt.Sprintf("Error reading directory %s: %v", path, err)
		resp.Response = map[string]any{
			"output": response,
			"error":  err.Error(),
		}
		return &resp, nil
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

	resp.Response = map[string]any{
		"output": result.String(),
		"error":  "",
	}
	return &resp, nil
}

func (ag *Agent) processGemini2DeleteFileToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	path, ok := call.Args["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path not found in arguments")
	}

	// Check if confirmation is needed
	needConfirm, ok := call.Args["need_confirm"].(bool)
	if !ok {
		// Default to true for safety
		needConfirm = true
	}

	if needConfirm && !skipToolsConfirm {
		// Response with a prompt to let user confirm
		outStr := fmt.Sprintf(ToolRespConfirmFileOp, fmt.Sprintf("delete file %s", path), fmt.Sprintf("delete the file at path: %s", path))
		resp.Response = map[string]any{
			"output": outStr,
			"error":  "",
		}
		return &resp, nil
	}

	// Delete the file
	err := os.Remove(path)
	if err != nil {
		response := fmt.Sprintf("Error deleting file %s: %v", path, err)
		resp.Response = map[string]any{
			"output": response,
			"error":  err.Error(),
		}
		return &resp, nil
	}

	response := fmt.Sprintf("Successfully deleted file %s", path)
	resp.Response = map[string]any{
		"output": response,
		"error":  "",
	}
	return &resp, nil
}

func (ag *Agent) processGemini2DeleteDirectoryToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	path, ok := call.Args["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path not found in arguments")
	}

	// Check if confirmation is needed
	needConfirm, ok := call.Args["need_confirm"].(bool)
	if !ok {
		// Default to true for safety
		needConfirm = true
	}

	if needConfirm && !skipToolsConfirm {
		// Response with a prompt to let user confirm
		outStr := fmt.Sprintf(ToolRespConfirmFileOp, fmt.Sprintf("delete directory %s", path), fmt.Sprintf("delete the directory at path: %s and all its contents", path))
		resp.Response = map[string]any{
			"output": outStr,
			"error":  "",
		}
		return &resp, nil
	}

	// Delete the directory
	err := os.RemoveAll(path)
	if err != nil {
		response := fmt.Sprintf("Error deleting directory %s: %v", path, err)
		resp.Response = map[string]any{
			"output": response,
			"error":  err.Error(),
		}
		return &resp, nil
	}

	response := fmt.Sprintf("Successfully deleted directory %s", path)
	resp.Response = map[string]any{
		"output": response,
		"error":  "",
	}
	return &resp, nil
}

func (ag *Agent) processGemini2MoveToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	source, ok := call.Args["source"].(string)
	if !ok {
		return nil, fmt.Errorf("source not found in arguments")
	}

	destination, ok := call.Args["destination"].(string)
	if !ok {
		return nil, fmt.Errorf("destination not found in arguments")
	}

	// Check if confirmation is needed
	needConfirm, ok := call.Args["need_confirm"].(bool)
	if !ok {
		// Default to true for safety
		needConfirm = true
	}

	if needConfirm && !skipToolsConfirm {
		// Response with a prompt to let user confirm
		outStr := fmt.Sprintf(ToolRespConfirmFileOp, fmt.Sprintf("move %s to %s", source, destination), fmt.Sprintf("move the file or directory from %s to %s", source, destination))
		resp.Response = map[string]any{
			"output": outStr,
			"error":  "",
		}
		return &resp, nil
	}

	// Move/rename the file or directory
	err := os.Rename(source, destination)
	if err != nil {
		response := fmt.Sprintf("Error moving %s to %s: %v", source, destination, err)
		resp.Response = map[string]any{
			"output": response,
			"error":  err.Error(),
		}
		return &resp, nil
	}

	response := fmt.Sprintf("Successfully moved %s to %s", source, destination)
	resp.Response = map[string]any{
		"output": response,
		"error":  "",
	}
	return &resp, nil
}

func (ag *Agent) processGemini2SearchFilesToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	directory, ok := call.Args["directory"].(string)
	if !ok {
		return nil, fmt.Errorf("directory not found in arguments")
	}

	pattern, ok := call.Args["pattern"].(string)
	if !ok {
		return nil, fmt.Errorf("pattern not found in arguments")
	}

	// Search for files matching the pattern
	fullPattern := filepath.Join(directory, pattern)
	matches, err := filepath.Glob(fullPattern)
	if err != nil {
		response := fmt.Sprintf("Error searching for files with pattern %s: %v", fullPattern, err)
		resp.Response = map[string]any{
			"output": response,
			"error":  err.Error(),
		}
		return &resp, nil
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

	resp.Response = map[string]any{
		"output": result.String(),
		"error":  "",
	}
	return &resp, nil
}

func (ag *Agent) processGemini2SearchTextInFileToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	path, ok := call.Args["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path not found in arguments")
	}

	searchText, ok := call.Args["text"].(string)
	if !ok {
		return nil, fmt.Errorf("text not found in arguments")
	}

	// Open the file
	file, err := os.Open(path)
	if err != nil {
		response := fmt.Sprintf("Error opening file %s: %v", path, err)
		resp.Response = map[string]any{
			"output": response,
			"error":  err.Error(),
		}
		return &resp, nil
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
		resp.Response = map[string]any{
			"output": response,
			"error":  err.Error(),
		}
		return &resp, nil
	}

	if foundCount == 0 {
		result.WriteString("No matches found.")
	} else {
		result.WriteString(fmt.Sprintf("\nFound %d match(es).", foundCount))
	}

	resp.Response = map[string]any{
		"output": result.String(),
		"error":  "",
	}
	return &resp, nil
}

func (ag *Agent) processGemini2ReadMultipleFilesToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	pathsInterface, ok := call.Args["paths"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("paths not found in arguments or not an array")
	}

	// Check if line numbers are requested
	includeLineNumbers := false
	if lineNumValue, exists := call.Args["line_numbers"]; exists {
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

	resp.Response = map[string]any{
		"output": result.String(),
		"error":  "",
	}
	return &resp, nil
}

func (ag *Agent) processGemini2ShellToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	cmdStr, ok := call.Args["command"].(string)
	if !ok {
		return nil, fmt.Errorf("command not found in arguments for tool call ID %s", call.ID)
	}
	needConfirm, ok := call.Args["need_confirm"].(bool)
	if !ok {
		// there is no need_confirm parameter, so we assume it's false
		needConfirm = false
	}
	if needConfirm && !skipToolsConfirm {
		// Response with a prompt to let user confirm
		descStr, ok := call.Args["purpose"].(string)
		if !ok {
			return nil, fmt.Errorf("purpose not found in arguments for tool call ID %s", call.ID)
		}
		outStr := fmt.Sprintf(ToolRespConfirmShell, cmdStr, descStr)
		resp.Response = map[string]any{
			"output": outStr,
			"error":  "",
		}
		return &resp, nil
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

	// Create and return the tool response
	resp.Response = map[string]any{
		"output": finalResponse,
		"error":  errStr,
	}
	return &resp, nil
}

func (ag *Agent) processGemini2WebFetchToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	url, ok := call.Args["url"].(string)
	if !ok {
		return nil, fmt.Errorf("url not found in arguments for tool call ID %s", call.ID)
	}

	// Call the fetch function
	results := FetchProcess([]string{url})

	// Check if FetchProcess returned any results
	if len(results) == 0 {
		// If no content was fetched or extracted, create an error message for the user.
		response := fmt.Sprintf("Failed to fetch content from %s or no content was extracted. Please check the URL or try again.", url)
		resp.Response = map[string]any{
			"output": response,
			"error":  "fetch failed",
		}
		return &resp, nil
	}

	// Create and return the tool response message
	response := fmt.Sprintf("Fetched content from %s:\n%s", url, results[0])
	resp.Response = map[string]any{
		"output": response,
		"error":  "",
	}
	return &resp, nil
}

func (ag *Agent) processGemini2EditFileToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	path, ok := call.Args["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path not found in arguments")
	}

	// Check if confirmation is needed
	needConfirm, ok := call.Args["need_confirm"].(bool)
	if !ok {
		// Default to true for safety
		needConfirm = true
	}

	// Get the edits to apply
	editsInterface, ok := call.Args["edits"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("edits not found in arguments or not an array")
	}

	// If confirmation is needed, ask the user before proceeding
	if needConfirm && !skipToolsConfirm {
		var editsDescription strings.Builder
		editsDescription.WriteString("The following edits will be applied to the file:\n")
		for _, editInterface := range editsInterface {
			editMap, ok := editInterface.(map[string]interface{})
			if !ok {
				continue
			}

			line, _ := editMap["line"].(float64) // JSON numbers are float64
			content, _ := editMap["content"].(string)
			operation, _ := editMap["operation"].(string)

			if operation == "add" {
				editsDescription.WriteString(fmt.Sprintf("  - Insert at line %d: %s\n", int(line), content))
			} else if operation == "delete" || content == "" {
				editsDescription.WriteString(fmt.Sprintf("  - Delete line %d\n", int(line)))
			} else {
				editsDescription.WriteString(fmt.Sprintf("  - Replace line %d with: %s\n", int(line), content))
			}
		}

		outStr := fmt.Sprintf(ToolRespConfirmFileOp, fmt.Sprintf("edit file %s", path), editsDescription.String())
		resp.Response = map[string]any{
			"output": outStr,
			"error":  "",
		}
		return &resp, nil
	}

	// Convert edits to a structured format
	type Edit struct {
		Line      int
		Content   string
		Operation string // "add", "delete", or "replace"
	}

	var edits []Edit
	for _, editInterface := range editsInterface {
		editMap, ok := editInterface.(map[string]interface{})
		if !ok {
			continue
		}

		line, _ := editMap["line"].(float64) // JSON numbers are float64
		content, _ := editMap["content"].(string)
		operation, hasOp := editMap["operation"].(string)

		// Determine operation if not explicitly provided
		if !hasOp {
			if content == "" {
				operation = "delete"
			} else {
				operation = "replace"
			}
		}

		edits = append(edits, Edit{
			Line:      int(line),
			Content:   content,
			Operation: operation,
		})
	}

	// Sort edits by line number in descending order to avoid line number shifts during editing
	// For add operations, we want to add at the specified line position
	// For delete/replace operations, we work with existing line positions
	sort.Slice(edits, func(i, j int) bool {
		// If both are add operations, sort by line descending
		if edits[i].Operation == "add" && edits[j].Operation == "add" {
			return edits[i].Line > edits[j].Line
		}
		// If one is add and one isn't, add operations should happen first (higher line numbers)
		if edits[i].Operation == "add" && edits[j].Operation != "add" {
			return true
		}
		if edits[i].Operation != "add" && edits[j].Operation == "add" {
			return false
		}
		// If neither is add, sort by line descending
		return edits[i].Line > edits[j].Line
	})

	// Read the file
	content, err := os.ReadFile(path)
	if err != nil {
		response := fmt.Sprintf("Error reading file %s: %v", path, err)
		resp.Response = map[string]any{
			"output": response,
			"error":  err.Error(),
		}
		return &resp, nil
	}

	// Split content into lines
	lines := strings.Split(string(content), "\n")

	// Apply edits
	for _, edit := range edits {
		lineIndex := edit.Line - 1 // Convert to 0-indexed

		switch edit.Operation {
		case "add", "++":
			// Insert new content at the specified line position
			// This works for inserting at the beginning (line 1), middle, or end(-1 or more than last line)
			if lineIndex < 0 || lineIndex > len(lines) {
				lineIndex = len(lines)
			}
			// Insert the new line at the specified position
			lines = append(lines[:lineIndex], append([]string{edit.Content}, lines[lineIndex:]...)...)

		case "delete", "--":
			// Delete line (only if within range)
			if lineIndex >= 0 && lineIndex < len(lines) {
				lines = append(lines[:lineIndex], lines[lineIndex+1:]...)
			}

		case "replace", "==":
			// Replace or append line
			if lineIndex >= 0 && lineIndex < len(lines) {
				lines[lineIndex] = edit.Content
			} else if lineIndex == len(lines) {
				// Append new line at the end
				lines = append(lines, edit.Content)
			}
		}
	}

	// Join lines back together
	newContent := strings.Join(lines, "\n")

	// Write the modified content back to the file
	err = os.WriteFile(path, []byte(newContent), 0644)
	if err != nil {
		response := fmt.Sprintf("Error writing file %s: %v", path, err)
		resp.Response = map[string]any{
			"output": response,
			"error":  err.Error(),
		}
		return &resp, nil
	}

	response := fmt.Sprintf("Successfully edited file %s", path)
	resp.Response = map[string]any{
		"output": response,
		"error":  "",
	}
	return &resp, nil
}

func (ag *Agent) processGemini2CopyToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	source, ok := call.Args["source"].(string)
	if !ok {
		return nil, fmt.Errorf("source not found in arguments")
	}

	destination, ok := call.Args["destination"].(string)
	if !ok {
		return nil, fmt.Errorf("destination not found in arguments")
	}

	// Check if confirmation is needed
	needConfirm, ok := call.Args["need_confirm"].(bool)
	if !ok {
		// Default to true for safety
		needConfirm = true
	}

	if needConfirm && !skipToolsConfirm {
		// Response with a prompt to let user confirm
		outStr := fmt.Sprintf(ToolRespConfirmFileOp, fmt.Sprintf("copy %s to %s", source, destination), fmt.Sprintf("copy the file or directory from %s to %s", source, destination))
		resp.Response = map[string]any{
			"output": outStr,
			"error":  "",
		}
		return &resp, nil
	}

	// Copy the file or directory
	err := copyFileOrDir(source, destination)
	if err != nil {
		response := fmt.Sprintf("Error copying %s to %s: %v", source, destination, err)
		resp.Response = map[string]any{
			"output": response,
			"error":  err.Error(),
		}
		return &resp, nil
	}

	response := fmt.Sprintf("Successfully copied %s to %s", source, destination)
	resp.Response = map[string]any{
		"output": response,
		"error":  "",
	}
	return &resp, nil
}
