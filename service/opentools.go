package service

import (
	"context"
	"fmt"

	"github.com/sashabaranov/go-openai"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
	"google.golang.org/genai"
)

type ToolType string

const (
	ToolTypeFunction ToolType = "function"
)

// OpenTool is a generic tool definition that is not tied to any specific model.
type OpenTool struct {
	Type     ToolType
	Function *OpenFunctionDefinition
}

// OpenFunctionDefinition is a generic function definition that is not tied to any specific model.
type OpenFunctionDefinition struct {
	Name        string
	Description string
	Parameters  map[string]interface{}
}

const (
	// ToolRespConfirmShell is the template for the response to the user before executing a command.
	ToolRespConfirmShell = "```\n%s\n```\n\n%s"

	// ToolRespShellOutput is the template for the response to the user after executing a command.
	ToolRespShellOutput = `shell executed: %s
Status:
%s
%s`

	ToolUserConfirmPrompt = "Apply this change? (Yes/No): "

	// ToolRespConfirmEdityFile is the template for the response to the user before modifying a file, including the diff.
	ToolRespDiscardEditFile = "Based on your request, the OPERATION is CANCELLED: " +
		"Cancel edit file: %s\n" +
		"The user has explicitly declined to apply these file edits. The file will remain unchanged. Do not proceed with any file modifications or ask for further confirmation without explicit new user instruction."
)

var (
	embeddingTools = []string{
		"shell",
		"read_file",
		"write_file",
		"edit_file",
		"delete_file",
		"create_directory",
		"list_directory",
		"delete_directory",
		"move",
		"copy",
		"search_files",
		"search_text_in_file",
		"read_multiple_files",
		"web_fetch",
	}
	searchTools = []string{
		"web_search",
	}
)

type ToolsUse struct {
	Enable      bool // Whether tools can be used
	AutoApprove bool // Whether tools can be used without user confirmation
}

func GetAllEmbeddingTools() []string {
	return embeddingTools
}

func GetAllSearchTools() []string {
	return searchTools
}

func AvailableEmbeddingTool(toolName string) bool {
	for _, tool := range embeddingTools {
		if tool == toolName {
			return true
		}
	}
	return false
}

func AvailableSearchTool(toolName string) bool {
	for _, tool := range searchTools {
		if tool == toolName {
			return true
		}
	}
	return false
}

func AvailableMCPTool(toolName string, client *MCPClient) bool {
	if client == nil {
		return false
	}
	return client.FindTool(toolName) != nil
}

func FilterToolArguments(argsMap map[string]interface{}, ignoreKeys []string) map[string]interface{} {
	// Create a lookup map for efficient key checking
	ignoreMap := make(map[string]bool)
	for _, key := range ignoreKeys {
		ignoreMap[key] = true
	}

	result := make(map[string]interface{})
	for k, v := range argsMap {
		// Skip keys that are in the ignore list
		if ignoreMap[k] {
			continue
		}
		result[k] = v
	}
	return result
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

// ToGeminiFunctions converts a GenericTool to a genai.Tool
func (ot *OpenTool) ToGeminiFunctions() *genai.FunctionDeclaration {
	// Convert parameters from map[string]interface{} to *genai.Schema
	properties := make(map[string]*genai.Schema)

	// Extract properties from the OpenTool parameters
	if props, ok := ot.Function.Parameters["properties"].(map[string]interface{}); ok {
		for key, value := range props {
			if propMap, ok := value.(map[string]interface{}); ok {
				schema := &genai.Schema{}
				if typeVal, ok := propMap["type"]; ok {
					switch typeVal {
					case "string":
						schema.Type = genai.TypeString
					case "integer":
						schema.Type = genai.TypeInteger
					case "boolean":
						schema.Type = genai.TypeBoolean
					case "array":
						schema.Type = genai.TypeArray
					case "object":
						schema.Type = genai.TypeObject
					}
				}
				if descVal, ok := propMap["description"]; ok {
					if descStr, ok := descVal.(string); ok {
						schema.Description = descStr
					}
				}
				if defaultVal, ok := propMap["default"]; ok {
					schema.Default = defaultVal
				}
				if enumVal, ok := propMap["enum"]; ok {
					if enumSlice, ok := enumVal.([]interface{}); ok {
						var enumStrs []string
						for _, e := range enumSlice {
							if str, ok := e.(string); ok {
								enumStrs = append(enumStrs, str)
							}
						}
						schema.Enum = enumStrs
					} else if enumStrSlice, ok := enumVal.([]string); ok {
						schema.Enum = enumStrSlice
					}
				}
				if itemsVal, ok := propMap["items"]; ok {
					if itemsMap, ok := itemsVal.(map[string]interface{}); ok {
						itemSchema := &genai.Schema{}
						if typeVal, ok := itemsMap["type"]; ok {
							switch typeVal {
							case "string":
								itemSchema.Type = genai.TypeString
							case "integer":
								itemSchema.Type = genai.TypeInteger
							case "boolean":
								itemSchema.Type = genai.TypeBoolean
							}
						}
						schema.Items = itemSchema
					}
				}
				properties[key] = schema
			}
		}
	}

	// Handle required fields
	var required []string
	if req, ok := ot.Function.Parameters["required"].([]string); ok {
		required = req
	} else if req, ok := ot.Function.Parameters["required"].([]interface{}); ok {
		for _, r := range req {
			if s, ok := r.(string); ok {
				required = append(required, s)
			}
		}
	}

	return &genai.FunctionDeclaration{
		Name:        ot.Function.Name,
		Description: ot.Function.Description,
		Parameters: &genai.Schema{
			Type:       genai.TypeObject,
			Properties: properties,
			Required:   required,
		},
	}
}

// getOpenEmbeddingTools returns the embedding tools for all models
func getOpenEmbeddingTools() []*OpenTool {
	var tools []*OpenTool

	// Shell tool
	shellTool := getOpenShellTool()

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
		Type:     ToolTypeFunction,
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
		Type:     ToolTypeFunction,
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
		Type:     ToolTypeFunction,
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
		Type:     ToolTypeFunction,
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
		Type:     ToolTypeFunction,
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
		Type:     ToolTypeFunction,
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
		Type:     ToolTypeFunction,
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
		Type:     ToolTypeFunction,
		Function: &moveFunc,
	}

	tools = append(tools, &moveTool)

	// Search files tool
	searchFilesFunc := OpenFunctionDefinition{
		Name:        "search_files",
		Description: "Search for files in a directory matching a pattern. Supports recursive search through subdirectories.",
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
				"recursive": map[string]interface{}{
					"type":        "boolean",
					"description": "If true, search recursively through all subdirectories. Default is false (search only in the specified directory).",
					"default":     false,
				},
			},
			"required": []string{"directory", "pattern"},
		},
	}
	searchFilesTool := OpenTool{
		Type:     ToolTypeFunction,
		Function: &searchFilesFunc,
	}

	tools = append(tools, &searchFilesTool)

	// Search text in file tool
	searchTextFunc := OpenFunctionDefinition{
		Name:        "search_text_in_file",
		Description: "Search for specific text within a file and return matching lines with line numbers. Supports case-insensitive and regex search.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The path to the file to search in.",
				},
				"text": map[string]interface{}{
					"type":        "string",
					"description": "The text or pattern to search for.",
				},
				"case_insensitive": map[string]interface{}{
					"type":        "boolean",
					"description": "If true, perform case-insensitive matching. Default is false.",
					"default":     false,
				},
				"regex": map[string]interface{}{
					"type":        "boolean",
					"description": "If true, treat the search text as a regular expression pattern. Default is false.",
					"default":     false,
				},
			},
			"required": []string{"path", "text"},
		},
	}
	searchTextTool := OpenTool{
		Type:     ToolTypeFunction,
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
		Type:     ToolTypeFunction,
		Function: &readMultipleFilesFunc,
	}

	tools = append(tools, &readMultipleFilesTool)

	// Edit file tool
	// editFileFunc := OpenFunctionDefinition{
	// 	Name:        "edit_file",
	// 	Description: "Edit specific lines in a file. This tool allows adding, replacing, or deleting content at specific line numbers.",
	// 	Parameters: map[string]interface{}{
	// 		"type": "object",
	// 		"properties": map[string]interface{}{
	// 			"path": map[string]interface{}{
	// 				"type":        "string",
	// 				"description": "The path to the file to edit.",
	// 			},
	// 			"edits": map[string]interface{}{
	// 				"type": "array",
	// 				"items": map[string]interface{}{
	// 					"type": "object",
	// 					"properties": map[string]interface{}{
	// 						"line": map[string]interface{}{
	// 							"type":        "integer",
	// 							"description": "The line number to edit (1-indexed). For add operations, this is the position where content will be inserted.",
	// 						},
	// 						"content": map[string]interface{}{
	// 							"type":        "string",
	// 							"description": "The new content for the line. Empty string to delete the line (unless operation is specified).",
	// 						},
	// 						"operation": map[string]interface{}{
	// 							"type": "string",
	// 							"description": "The operation to perform on the specified line (1-indexed):\n" +
	// 								"- 'add' or '++' to insert content at the given line position (if line is greater than the number of lines, content is appended).\n" +
	// 								"- 'delete' or '--' to remove the line.\n" +
	// 								"- 'replace' or '==' to replace the line content.\n" +
	// 								"If 'operation' is omitted, 'delete' is assumed when 'content' is empty, otherwise 'replace' is used.\n" +
	// 								"Accepted values: 'add', 'delete', 'replace'.",
	// 							"enum": []string{"add", "delete", "replace"},
	// 						},
	// 					},
	// 					"required": []string{"line"},
	// 				},
	// 				"description": "Array of edits to apply to the file. Each edit specifies a line number and the operation to perform.",
	// 			},
	// 			"need_confirm": map[string]interface{}{
	// 				"type": "boolean",
	// 				"description": "Specifies whether to prompt the user for confirmation before editing the file. " +
	// 					"This should always be true for safety.",
	// 				"default": true,
	// 			},
	// 		},
	// 		"required": []string{"path", "edits"},
	// 	},
	// }
	// editFileTool := OpenTool{
	// 	Type:     ToolTypeFunction,
	// 	Function: &editFileFunc,
	// }

	// tools = append(tools, &editFileTool)

	editFileFunc := OpenFunctionDefinition{
		Name: "edit_file",
		Description: "Apply targeted edits to a file using search-replace operations. " +
			"Use this for precise code modifications, refactoring, or content updates. " +
			"Provide search-replace blocks to specify exactly what to change. " +
			"Each edit can modify, add, or delete specific code sections safely.",
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
							"search": map[string]interface{}{
								"type":        "string",
								"description": "The exact text to find and replace. Include sufficient context for uniqueness.",
							},
							"replace": map[string]interface{}{
								"type":        "string",
								"description": "The replacement text. Use empty string to delete the search text.",
							},
						},
						"required": []string{"search", "replace"},
					},
					"description": "Array of search-replace operations to apply to the file.",
				},
				"need_confirm": map[string]interface{}{
					"type": "boolean",
					"description": "Specifies whether to show diff and prompt for confirmation before editing the file. " +
						"This should always be true for safety.",
					"default": true,
				},
			},
			"required": []string{"path", "edits"},
		},
	}
	editFileTool := OpenTool{
		Type:     ToolTypeFunction,
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
		Type:     ToolTypeFunction,
		Function: &copyFunc,
	}

	tools = append(tools, &copyTool)

	return tools
}

func getOpenShellTool() *OpenTool {
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
				"timeout": map[string]interface{}{
					"type":        "integer",
					"description": "Optional timeout in seconds for the command execution. Default is 60 seconds. Use a higher value for long-running commands.",
					"default":     60,
				},
			},
			"required": []string{"command", "purpose"},
		},
	}

	shellTool := OpenTool{
		Type:     ToolTypeFunction,
		Function: &shellFunc,
	}

	return &shellTool
}

func getOpenWebSearchTool() *OpenTool {
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
		Type:     ToolTypeFunction,
		Function: &searchFunc,
	}

	return &searchTool
}

// MCPToolsToOpenTool converts an MCPTools struct to an OpenTool with proper JSON schema
func MCPToolsToOpenTool(mcpTool MCPTool) *OpenTool {
	properties := make(map[string]interface{})
	var required []string

	// Use the Properties field which contains the full schema information
	// instead of the Parameters field which only contains string descriptions
	for paramName, schema := range mcpTool.Properties {
		prop := make(map[string]interface{})

		// Set the type
		if schema.Type != "" {
			prop["type"] = schema.Type
		} else if len(schema.Types) > 0 {
			// If multiple types, use the first one
			prop["type"] = schema.Types[0]
		} else {
			// Default to string if no type specified
			prop["type"] = "string"
		}

		// Set the description
		if schema.Description != "" {
			prop["description"] = schema.Description
		}

		// Set default value if present
		if schema.Default != nil {
			prop["default"] = schema.Default
		}

		// Handle enum values
		if len(schema.Enum) > 0 {
			prop["enum"] = schema.Enum
		}

		// Handle array items
		if schema.Items != nil && schema.Type == "array" {
			items := make(map[string]interface{})
			if schema.Items.Type != "" {
				items["type"] = schema.Items.Type
			}
			prop["items"] = items
		}

		properties[paramName] = prop
		required = append(required, paramName)
	}

	parameters := map[string]interface{}{
		"type":       "object",
		"properties": properties,
		"required":   required,
	}

	return &OpenTool{
		Type: ToolTypeFunction,
		Function: &OpenFunctionDefinition{
			Name:        mcpTool.Name,
			Description: mcpTool.Description,
			Parameters:  parameters,
		},
	}
}

// getMCPTools retrieves all MCP tools from the MCPClient and converts them to OpenTool format
func getMCPTools(client *MCPClient) []*OpenTool {
	var tools []*OpenTool

	servers := client.GetAllServers()
	for _, server := range servers {
		if server.Tools != nil {
			for _, mcpTool := range *server.Tools {
				openTool := MCPToolsToOpenTool(mcpTool)
				tools = append(tools, openTool)
			}
		}
	}

	return tools
}

// OpenProcessor is the main processor for OpenAI-like models
// For tools implementation
// - It manages the context, notifications, data streaming, and tool usage
// - It handles queries and references, and maintains the status stack
type OpenProcessor struct {
	ctx        context.Context
	notify     chan<- StreamNotify      // Sub Channel to send notifications
	data       chan<- StreamData        // Sub Channel to send data
	proceed    <-chan bool              // Main Channel to receive proceed signal
	search     *SearchEngine            // Search engine
	toolsUse   *ToolsUse                // Use tools
	queries    []string                 // List of queries to be sent to the AI assistant
	references []map[string]interface{} // keep track of the references
	status     *StatusStack             // Stack to manage streaming status
	mcpClient  *MCPClient               // MCP client for MCP tool calls
}

// Diff confirm func
func (op *OpenProcessor) showDiffConfirm(diff string) {
	// Function call is over
	op.status.ChangeTo(op.notify, StreamNotify{Status: StatusFunctionCallingOver}, op.proceed)
	// Show the diff confirm
	op.status.ChangeTo(op.notify, StreamNotify{Data: diff, Status: StatusDiffConfirm}, op.proceed)
}

// Diff close func
func (op *OpenProcessor) closeDiffConfirm() {
	// Confirm diff is over
	op.status.ChangeTo(op.notify, StreamNotify{Status: StatusDiffConfirmOver}, op.proceed)
}

// OpenAI tool implementations (wrapper functions)
func (op *OpenProcessor) OpenAIShellToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	response, err := shellToolCallImpl(argsMap, op.toolsUse)
	if err != nil {
		return openai.ChatCompletionMessage{}, err
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Content:    response,
	}, nil
}

func (op *OpenProcessor) OpenAIWebFetchToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	response, err := webFetchToolCallImpl(argsMap)
	if err != nil {
		return openai.ChatCompletionMessage{}, err
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Content:    response,
	}, nil
}

func (op *OpenProcessor) OpenAIWebSearchToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	response, err := webSearchToolCallImpl(argsMap, &op.queries, &op.references, op.search)
	if err != nil {
		return openai.ChatCompletionMessage{}, err
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Content:    response,
	}, nil
}

func (op *OpenProcessor) OpenAIReadFileToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	response, err := readFileToolCallImpl(argsMap)
	if err != nil {
		return openai.ChatCompletionMessage{}, err
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Content:    response,
	}, nil
}

func (op *OpenProcessor) OpenAIWriteFileToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	response, err := writeFileToolCallImpl(argsMap, op.toolsUse, op.showDiffConfirm, op.closeDiffConfirm)
	if err != nil {
		return openai.ChatCompletionMessage{}, err
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Content:    response,
	}, nil
}

// func (op *OpenProcessor) OpenAIEditFileToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
// 	response, err := editFileToolCallImpl(argsMap, op.toolsUse)
// 	if err != nil {
// 		return openai.ChatCompletionMessage{}, err
// 	}

// 	return openai.ChatCompletionMessage{
// 		Role:       openai.ChatMessageRoleTool,
// 		ToolCallID: toolCall.ID,
// 		Content:    response,
// 	}, nil
// }

func (op *OpenProcessor) OpenAIEditFileToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	response, err := editFileToolCallImpl(argsMap, op.toolsUse, op.showDiffConfirm, op.closeDiffConfirm)
	if err != nil {
		return openai.ChatCompletionMessage{}, err
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Content:    response,
	}, nil
}

func (op *OpenProcessor) OpenAICreateDirectoryToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	response, err := createDirectoryToolCallImpl(argsMap)
	if err != nil {
		return openai.ChatCompletionMessage{}, err
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Content:    response,
	}, nil
}

func (op *OpenProcessor) OpenAIListDirectoryToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	response, err := listDirectoryToolCallImpl(argsMap)
	if err != nil {
		return openai.ChatCompletionMessage{}, err
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Content:    response,
	}, nil
}

func (op *OpenProcessor) OpenAIDeleteFileToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	response, err := deleteFileToolCallImpl(argsMap, op.toolsUse)
	if err != nil {
		return openai.ChatCompletionMessage{}, err
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Content:    response,
	}, nil
}

func (op *OpenProcessor) OpenAIDeleteDirectoryToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	response, err := deleteDirectoryToolCallImpl(argsMap, op.toolsUse)
	if err != nil {
		return openai.ChatCompletionMessage{}, err
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Content:    response,
	}, nil
}

func (op *OpenProcessor) OpenAIMoveToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	response, err := moveToolCallImpl(argsMap, op.toolsUse)
	if err != nil {
		return openai.ChatCompletionMessage{}, err
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Content:    response,
	}, nil
}

func (op *OpenProcessor) OpenAICopyToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	response, err := copyToolCallImpl(argsMap, op.toolsUse)
	if err != nil {
		return openai.ChatCompletionMessage{}, err
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Content:    response,
	}, nil
}

func (op *OpenProcessor) OpenAISearchFilesToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	response, err := searchFilesToolCallImpl(argsMap)
	if err != nil {
		return openai.ChatCompletionMessage{}, err
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Content:    response,
	}, nil
}

func (op *OpenProcessor) OpenAISearchTextInFileToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	response, err := searchTextInFileToolCallImpl(argsMap)
	if err != nil {
		return openai.ChatCompletionMessage{}, err
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Content:    response,
	}, nil
}

func (op *OpenProcessor) OpenAIReadMultipleFilesToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	response, err := readMultipleFilesToolCallImpl(argsMap)
	if err != nil {
		return openai.ChatCompletionMessage{}, err
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Content:    response,
	}, nil
}

// MCP tool call implementations
/*
Here is a bug:
status: 422 Unprocessable Entity, message: %!s(<nil>),
body: {"detail":[{"type":"string_type","loc":["body","messages",2,"ChatCompletionToolMessage","content"],"msg":"Input should be a valid string"
it is caused by the tool call response can be only a simple string
*/
// func (op *OpenProcessor) OpenAIMCPToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
// 	if op.mcpClient == nil {
// 		return openai.ChatCompletionMessage{}, fmt.Errorf("MCP client not initialized")
// 	}

// 	// Call the MCP tool
// 	result, err := op.mcpClient.CallTool(toolCall.Function.Name, *argsMap)
// 	if err != nil {
// 		return openai.ChatCompletionMessage{}, fmt.Errorf("MCP tool call failed: %v", err)
// 	}

// 	parts := []openai.ChatMessagePart{}
// 	for i, content := range result.Contents {
// 		switch result.Types[i] {
// 		case MCPResponseText:
// 			part := openai.ChatMessagePart{
// 				Type: openai.ChatMessagePartTypeText,
// 				Text: content,
// 			}
// 			parts = append(parts, part)
// 		case MCPResponseImage:
// 			part := openai.ChatMessagePart{
// 				Type: openai.ChatMessagePartTypeImageURL,
// 				ImageURL: &openai.ChatMessageImageURL{
// 					URL: content,
// 				},
// 			}
// 			parts = append(parts, part)
// 		default:
// 			// Unknown file type, skip
// 			// Don't deal with pdf, xls
// 			// It needs upload to OpenAI's servers first, so we can't include them directly in a message.
// 		}
// 	}

// 	return openai.ChatCompletionMessage{
// 		Role:         openai.ChatMessageRoleTool,
// 		ToolCallID:   toolCall.ID,
// 		MultiContent: parts,
// 	}, nil
// }

func (op *OpenProcessor) OpenAIMCPToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	if op.mcpClient == nil {
		return openai.ChatCompletionMessage{}, fmt.Errorf("MCP client not initialized")
	}

	// Call the MCP tool
	result, err := op.mcpClient.CallTool(toolCall.Function.Name, *argsMap)
	if err != nil {
		return openai.ChatCompletionMessage{}, fmt.Errorf("MCP tool call failed: %v", err)
	}

	// Concatenate all text and image URLs into a single string output
	// Because currently the tool call response can be only a simple string
	output := ""
	for i, content := range result.Contents {
		switch result.Types[i] {
		case MCPResponseText:
			output += content + "\n"
		case MCPResponseImage:
			output += content + "\n"
		default:
			// Unknown file type, skip
			// Don't deal with pdf, xls
			// It needs upload to OpenAI's servers first, so we can't include them directly in a message.
		}
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Content:    output,
	}, nil
}

func (op *OpenProcessor) OpenChatMCPToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	if op.mcpClient == nil {
		return nil, fmt.Errorf("MCP client not initialized")
	}

	// Call the MCP tool
	result, err := op.mcpClient.CallTool(toolCall.Function.Name, *argsMap)
	if err != nil {
		return nil, fmt.Errorf("MCP tool call failed: %v", err)
	}

	// OpenChat supports text, image, audio toolcall responses
	parts := []*model.ChatCompletionMessageContentPart{}
	for i, content := range result.Contents {
		switch result.Types[i] {
		case MCPResponseText:
			part := model.ChatCompletionMessageContentPart{
				Type: model.ChatCompletionMessageContentPartTypeText,
				Text: content,
			}
			parts = append(parts, &part)
		case MCPResponseImage:
			part := model.ChatCompletionMessageContentPart{
				Type: model.ChatCompletionMessageContentPartTypeImageURL,
				ImageURL: &model.ChatMessageImageURL{
					URL: content,
				},
			}
			parts = append(parts, &part)
		case MCPResponseAudio:
			part := model.ChatCompletionMessageContentPart{
				Type: model.ChatCompletionMessageContentPartTypeVideoURL,
				ImageURL: &model.ChatMessageImageURL{
					URL: content,
				},
			}
			parts = append(parts, &part)
		default:
			// Unknown file type, skip
			// Don't deal with pdf, xls
		}
	}

	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
		Content: &model.ChatCompletionMessageContent{
			ListValue: parts,
		},
	}
	return &toolMessage, nil
}

// OpenChat tool implementations (wrapper functions)
func (op *OpenProcessor) OpenChatReadFileToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	response, err := readFileToolCallImpl(argsMap)
	if err != nil {
		return nil, err
	}

	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(response),
	}
	return &toolMessage, nil
}

func (op *OpenProcessor) OpenChatWriteFileToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	response, err := writeFileToolCallImpl(argsMap, op.toolsUse, op.showDiffConfirm, op.closeDiffConfirm)
	if err != nil {
		return nil, err
	}

	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(response),
	}
	return &toolMessage, nil
}

func (op *OpenProcessor) OpenChatCreateDirectoryToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	response, err := createDirectoryToolCallImpl(argsMap)
	if err != nil {
		return nil, err
	}

	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(response),
	}
	return &toolMessage, nil
}

func (op *OpenProcessor) OpenChatListDirectoryToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	response, err := listDirectoryToolCallImpl(argsMap)
	if err != nil {
		return nil, err
	}

	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(response),
	}
	return &toolMessage, nil
}

func (op *OpenProcessor) OpenChatDeleteFileToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	response, err := deleteFileToolCallImpl(argsMap, op.toolsUse)
	if err != nil {
		return nil, err
	}

	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(response),
	}
	return &toolMessage, nil
}

func (op *OpenProcessor) OpenChatDeleteDirectoryToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	response, err := deleteDirectoryToolCallImpl(argsMap, op.toolsUse)
	if err != nil {
		return nil, err
	}

	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(response),
	}
	return &toolMessage, nil
}

func (op *OpenProcessor) OpenChatMoveToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	response, err := moveToolCallImpl(argsMap, op.toolsUse)
	if err != nil {
		return nil, err
	}

	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(response),
	}
	return &toolMessage, nil
}

func (op *OpenProcessor) OpenChatSearchFilesToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	response, err := searchFilesToolCallImpl(argsMap)
	if err != nil {
		return nil, err
	}

	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(response),
	}
	return &toolMessage, nil
}

func (op *OpenProcessor) OpenChatSearchTextInFileToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	response, err := searchTextInFileToolCallImpl(argsMap)
	if err != nil {
		return nil, err
	}

	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(response),
	}
	return &toolMessage, nil
}

func (op *OpenProcessor) OpenChatReadMultipleFilesToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	response, err := readMultipleFilesToolCallImpl(argsMap)
	if err != nil {
		return nil, err
	}

	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(response),
	}
	return &toolMessage, nil
}

func (op *OpenProcessor) OpenChatShellToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	response, err := shellToolCallImpl(argsMap, op.toolsUse)
	if err != nil {
		return nil, err
	}

	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(response),
	}
	return &toolMessage, nil
}

func (op *OpenProcessor) OpenChatWebFetchToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	response, err := webFetchToolCallImpl(argsMap)
	if err != nil {
		return nil, err
	}

	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(response),
	}
	return &toolMessage, nil
}

func (op *OpenProcessor) OpenChatWebSearchToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	response, err := webSearchToolCallImpl(argsMap, &op.queries, &op.references, op.search)
	if err != nil {
		return nil, err
	}

	// Create and return the tool response message
	return &model.ChatCompletionMessage{
		Role: model.ChatMessageRoleTool,
		Content: &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(response),
		}, Name: Ptr(""),
		ToolCallID: toolCall.ID,
	}, nil
}

// func (op *OpenProcessor) OpenChatEditFileToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
// 	toolMessage := model.ChatCompletionMessage{
// 		Role:       model.ChatMessageRoleTool,
// 		ToolCallID: toolCall.ID,
// 		Name:       Ptr(""),
// 	}

// 	response, err := editFileToolCallImpl(argsMap, op.toolsUse)
// 	if err != nil {
// 		return nil, err
// 	}

// 	toolMessage.Content = &model.ChatCompletionMessageContent{
// 		StringValue: volcengine.String(response),
// 	}
// 	return &toolMessage, nil
// }

func (op *OpenProcessor) OpenChatEditFileToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	response, err := editFileToolCallImpl(argsMap, op.toolsUse, op.showDiffConfirm, op.closeDiffConfirm)
	if err != nil {
		return nil, err
	}

	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(response),
	}
	return &toolMessage, nil
}

func (op *OpenProcessor) OpenChatCopyToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	response, err := copyToolCallImpl(argsMap, op.toolsUse)
	if err != nil {
		return nil, err
	}

	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(response),
	}
	return &toolMessage, nil
}
