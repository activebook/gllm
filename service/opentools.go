package service

import (
	"github.com/sashabaranov/go-openai"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
)

// OpenTool is a generic tool definition that is not tied to any specific model.
type OpenTool struct {
	Type     string
	Function *OpenFunctionDefinition
}

// OpenFunctionDefinition is a generic function definition that is not tied to any specific model.
type OpenFunctionDefinition struct {
	Name        string
	Description string
	Parameters  map[string]interface{}
}

// ToOpenAITool converts a GenericTool to an openai.Tool
func (ot *OpenTool) ToOpenAITool() openai.Tool {
	return openai.Tool{
		Type: openai.ToolType(ot.Type),
		Function: &openai.FunctionDefinition{
			Name:        ot.Function.Name,
			Description: ot.Function.Description,
			Parameters:  ot.Function.Parameters,
		},
	}
}

// ToOpenChatTool converts a GenericTool to a model.Tool
func (ot *OpenTool) ToOpenChatTool() *model.Tool {
	return &model.Tool{
		Type: model.ToolType(ot.Type),
		Function: &model.FunctionDefinition{
			Name:        ot.Function.Name,
			Description: ot.Function.Description,
			Parameters:  ot.Function.Parameters,
		},
	}
}

// getGenericEmbeddingTools returns the embedding tools for all models
func getGenericEmbeddingTools() []*OpenTool {
	var tools []*OpenTool

	// Shell tool
	shellTool := getGenericShellTool()

	tools = append(tools, shellTool)

	// Web fetch tool
	webFetchFunc := OpenFunctionDefinition{
		Name:        "web_fetch",
		Description: "Fetch content from a URL and extract the main text content.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"url": map[string]interface{}{
					"type":        "string",
					"description": "The URL to fetch content from.",
				},
			},
			"required": []string{"url"},
		},
	}
	webFetchTool := OpenTool{
		Type:     "function",
		Function: &webFetchFunc,
	}

	tools = append(tools, &webFetchTool)

	// Read file tool
	readFileFunc := OpenFunctionDefinition{
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
	readFileTool := OpenTool{
		Type:     "function",
		Function: &readFileFunc,
	}

	tools = append(tools, &readFileTool)

	// Write file tool
	writeFileFunc := OpenFunctionDefinition{
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
				"need_confirm": map[string]interface{}{
					"type": "boolean",
					"description": "Specifies whether to prompt the user for confirmation before writing to the file. " +
						"This should always be true for safety.",
					"default": true,
				},
			},
			"required": []string{"path", "content"},
		},
	}
	writeFileTool := OpenTool{
		Type:     "function",
		Function: &writeFileFunc,
	}

	tools = append(tools, &writeFileTool)

	// Create directory tool
	createDirFunc := OpenFunctionDefinition{
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
	createDirTool := OpenTool{
		Type:     "function",
		Function: &createDirFunc,
	}

	tools = append(tools, &createDirTool)

	// List directory tool
	listDirFunc := OpenFunctionDefinition{
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
	listDirTool := OpenTool{
		Type:     "function",
		Function: &listDirFunc,
	}

	tools = append(tools, &listDirTool)

	// Delete file tool
	deleteFileFunc := OpenFunctionDefinition{
		Name:        "delete_file",
		Description: "Delete a file from the filesystem.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The path of the file to delete.",
				},
				"need_confirm": map[string]interface{}{
					"type": "boolean",
					"description": "Specifies whether to prompt the user for confirmation before deleting the file. " +
						"This should always be true for safety.",
					"default": true,
				},
			},
			"required": []string{"path"},
		},
	}
	deleteFileTool := OpenTool{
		Type:     "function",
		Function: &deleteFileFunc,
	}

	tools = append(tools, &deleteFileTool)

	// Delete directory tool
	deleteDirFunc := OpenFunctionDefinition{
		Name:        "delete_directory",
		Description: "Delete a directory from the filesystem.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The path of the directory to delete.",
				},
				"need_confirm": map[string]interface{}{
					"type": "boolean",
					"description": "Specifies whether to prompt the user for confirmation before deleting the directory. " +
						"This should always be true for safety.",
					"default": true,
				},
			},
			"required": []string{"path"},
		},
	}
	deleteDirTool := OpenTool{
		Type:     "function",
		Function: &deleteDirFunc,
	}

	tools = append(tools, &deleteDirTool)

	// Move file/directory tool
	moveFunc := OpenFunctionDefinition{
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
				"need_confirm": map[string]interface{}{
					"type": "boolean",
					"description": "Specifies whether to prompt the user for confirmation before moving the file or directory. " +
						"This should always be true for safety.",
					"default": true,
				},
			},
			"required": []string{"source", "destination"},
		},
	}
	moveTool := OpenTool{
		Type:     "function",
		Function: &moveFunc,
	}

	tools = append(tools, &moveTool)

	// Search files tool
	searchFilesFunc := OpenFunctionDefinition{
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
	searchFilesTool := OpenTool{
		Type:     "function",
		Function: &searchFilesFunc,
	}

	tools = append(tools, &searchFilesTool)

	// Search text in file tool
	searchTextFunc := OpenFunctionDefinition{
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
	searchTextTool := OpenTool{
		Type:     "function",
		Function: &searchTextFunc,
	}

	tools = append(tools, &searchTextTool)

	// Read multiple files tool
	readMultipleFilesFunc := OpenFunctionDefinition{
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
	readMultipleFilesTool := OpenTool{
		Type:     "function",
		Function: &readMultipleFilesFunc,
	}

	tools = append(tools, &readMultipleFilesTool)

	// Edit file tool
	editFileFunc := OpenFunctionDefinition{
		Name:        "edit_file",
		Description: "Edit specific lines in a file. This tool allows adding, replacing, or deleting content at specific line numbers.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The path to the file to edit.",
				},
				"edits": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"line": map[string]interface{}{
								"type":        "integer",
								"description": "The line number to edit (1-indexed). For add operations, this is the position where content will be inserted.",
							},
							"content": map[string]interface{}{
								"type":        "string",
								"description": "The new content for the line. Empty string to delete the line (unless operation is specified).",
							},
							"operation": map[string]interface{}{
								"type": "string",
								"description": "The operation to perform on the specified line (1-indexed):\n" +
									"- 'add' or '++' to insert content at the given line position (if line is greater than the number of lines, content is appended).\n" +
									"- 'delete' or '--' to remove the line.\n" +
									"- 'replace' or '==' to replace the line content.\n" +
									"If 'operation' is omitted, 'delete' is assumed when 'content' is empty, otherwise 'replace' is used.\n" +
									"Accepted values: 'add', 'delete', 'replace'.",
								"enum": []string{"add", "delete", "replace"},
							},
						},
						"required": []string{"line"},
					},
					"description": "Array of edits to apply to the file. Each edit specifies a line number and the operation to perform.",
				},
				"need_confirm": map[string]interface{}{
					"type": "boolean",
					"description": "Specifies whether to prompt the user for confirmation before editing the file. " +
						"This should always be true for safety.",
					"default": true,
				},
			},
			"required": []string{"path", "edits"},
		},
	}
	editFileTool := OpenTool{
		Type:     "function",
		Function: &editFileFunc,
	}

	tools = append(tools, &editFileTool)

	// Copy file/directory tool
	copyFunc := OpenFunctionDefinition{
		Name:        "copy",
		Description: "Copy a file or directory from one location to another in the filesystem.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"source": map[string]interface{}{
					"type":        "string",
					"description": "The current path of the file or directory to copy.",
				},
				"destination": map[string]interface{}{
					"type":        "string",
					"description": "The destination path for the file or directory copy.",
				},
				"need_confirm": map[string]interface{}{
					"type": "boolean",
					"description": "Specifies whether to prompt the user for confirmation before copying the file or directory. " +
						"This should be true for safety if it needs overwrite.",
					"default": true,
				},
			},
			"required": []string{"source", "destination"},
		},
	}
	copyTool := OpenTool{
		Type:     "function",
		Function: &copyFunc,
	}

	tools = append(tools, &copyTool)

	return tools
}

func getGenericShellTool() *OpenTool {
	shellFunc := OpenFunctionDefinition{
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

	shellTool := OpenTool{
		Type:     "function",
		Function: &shellFunc,
	}

	return &shellTool
}

func getGenericWebSearchTool() *OpenTool {
	searchFunc := OpenFunctionDefinition{
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
	searchTool := OpenTool{
		Type:     "function",
		Function: &searchFunc,
	}

	return &searchTool
}
