package service

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/activebook/gllm/data"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/anthropics/anthropic-sdk-go/shared/constant"
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
	ToolRespConfirmShell = "```\n%s\n```\n%s"

	// ToolRespShellOutput is the template for the response to the user after executing a command.
	ToolRespShellOutput = `shell executed: %s
Status:
%s
%s`

	ToolUserConfirmPrompt = "Do you want to proceed?"

	// ToolRespConfirmEdityFile is the template for the response to the user before modifying a file, including the diff.
	ToolRespDiscardEditFile = "Based on your request, the OPERATION is CANCELLED: " +
		"Cancel edit file: %s\n" +
		"The user has explicitly declined to apply these file edits. The file will remain unchanged. Do not proceed with any file modifications or ask for further confirmation without explicit new user instruction."
)

var (
	embeddingTools = []string{
		// shell tool
		"shell",
		// file tools
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
		// web tools
		"web_fetch",
		// memory tools
		"list_memory",
		"save_memory",
		// agent tools
		"switch_agent",
		"list_agent",
		// Sub-agent orchestration tools
		"call_agent",
		// shared state tools
		"get_state",
		"set_state",
		"list_state",
	}
	searchTools = []string{
		// web tools
		"web_search",
	}
	skillTools = []string{
		// skill tools
		"activate_skill",
	}
)

type ToolsUse struct {
	AutoApprove bool // Whether tools can be used without user confirmation
}

func GetAllEmbeddingTools() []string {
	return embeddingTools
}

func GetAllSearchTools() []string {
	return searchTools
}

func GetAllSkillTools() []string {
	return skillTools
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

func AvailableSkillTool(toolName string) bool {
	for _, tool := range skillTools {
		if tool == toolName {
			return true
		}
	}
	return false
}

// AppendSkillTools appends skill tools to the given tools slice if they are not already present.
func AppendSkillTools(tools []string) []string {
	for _, tool := range skillTools {
		if !slices.Contains(tools, tool) {
			tools = append(tools, tool)
		}
	}
	return tools
}

// RemoveSkillTools removes skill tools from the given tools slice.
func RemoveSkillTools(tools []string) []string {
	for _, tool := range skillTools {
		tools = slices.DeleteFunc(tools, func(t string) bool {
			return t == tool
		})
	}
	return tools
}

// GetOpenEmbeddingToolsFiltered returns embedding tools filtered by the allowed list.
// If allowedTools is nil or empty, returns all embedding tools.
// Unknown tool names are gracefully ignored.
func GetOpenEmbeddingToolsFiltered(allowedTools []string) []*OpenTool {
	allTools := getOpenEmbeddingTools()

	// If no filter specified, return all tools
	if len(allowedTools) == 0 {
		return nil
	}

	// Create a set of allowed tool names for O(1) lookup
	allowedSet := make(map[string]bool)
	for _, name := range allowedTools {
		allowedSet[name] = true
	}

	// Filter tools
	var filtered []*OpenTool
	for _, tool := range allTools {
		if allowedSet[tool.Function.Name] {
			filtered = append(filtered, tool)
		}
	}

	return filtered
}

// IsValidEmbeddingTool checks if a tool name is a valid embedding tool
func IsValidEmbeddingTool(toolName string) bool {
	return AvailableEmbeddingTool(toolName)
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

// ToAnthropicTool converts a GenericTool to an anthropic.ToolUnionParam
func (ot *OpenTool) ToAnthropicTool() anthropic.ToolUnionParam {
	schema := anthropic.ToolInputSchemaParam{
		Type:       constant.Object("object"),
		Properties: ot.Function.Parameters["properties"],
		// Handle Required
		// ot.Function.Parameters might have "required" which is []interface{} usually from JSON
		// We need []string
	}

	if req, ok := ot.Function.Parameters["required"]; ok {
		if reqSlice, ok := req.([]interface{}); ok {
			var required []string
			for _, r := range reqSlice {
				if s, ok := r.(string); ok {
					required = append(required, s)
				}
			}
			schema.Required = required
		} else if reqSlice, ok := req.([]string); ok {
			schema.Required = reqSlice
		}
	}

	t := anthropic.ToolUnionParamOfTool(schema, ot.Function.Name)
	if t.OfTool != nil && ot.Function.Description != "" {
		t.OfTool.Description = param.NewOpt(ot.Function.Description)
	}
	return t
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

	// list_memory tool
	listMemoryFunc := OpenFunctionDefinition{
		Name:        "list_memory",
		Description: "List all saved user memories and preferences. Use this to check what the user has asked you to remember before making updates.",
		Parameters: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
			"required":   []string{},
		},
	}
	listMemoryTool := OpenTool{
		Type:     ToolTypeFunction,
		Function: &listMemoryFunc,
	}

	tools = append(tools, &listMemoryTool)

	// save_memory tool
	saveMemoryFunc := OpenFunctionDefinition{
		Name: "save_memory",
		Description: `Update long-term user memories.

CRITICAL: Do NOT use this tool for conversation history, trivial facts, or immediate context.
Only use this tool when the user EXPLICITLY asks to "remember" or "save" a preference/fact for FUTURE sessions.

This tool replaces ALL memories with the content you provide. You should:
1. Call list_memory to get current memories.
2. Decide what to add/update based on the user's explicit request.
3. Rephrase the request into a clear, standalone statement (e.g., "User prefers Go over Python").
4. Call this tool with the complete new memory list.

To clear all memories, pass an empty string.`,
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"memories": map[string]interface{}{
					"type":        "string",
					"description": "The complete new memory content. Each memory should be on its own line, starting with '- '. Pass empty string to clear all memories.",
				},
			},
			"required": []string{"memories"},
		},
	}
	saveMemoryTool := OpenTool{
		Type:     ToolTypeFunction,
		Function: &saveMemoryFunc,
	}

	tools = append(tools, &saveMemoryTool)

	// Switch Agent tool
	switchAgentFunc := OpenFunctionDefinition{
		Name: "switch_agent",
		Description: `Switch the active agent to another agent profile. 
Use this tool when:
1. The current agent's system prompt or capabilities are not suitable for the user's request.
2. The user explicitly asks to switch to a specific agent.
3. You need to access tools that are only available to another agent.
4. Check agents' system prompt, tools, capabilities and thinking level for better usage if possible.
(e.g., switching to a 'Coder' agent for programming tasks, to a 'Researcher' agent for research tasks, to a 'Writer' agent for writing tasks, etc.)

IMPORTANT: This function is highly powerful.
Pass 'list' as the name to see all available agents and their capabilities before switching.
When a switch occurs, if an instruction is provided, it replaces the original prompt for the next agent's execution. This essentially "briefs" the new agent on what to do next, preserving context.
`,
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "The name of the agent to switch to, or 'list' to see available agents.",
				},
				"instruction": map[string]interface{}{
					"type":        "string",
					"description": "Optional context or instruction to pass to the new agent. This helps the new agent understand the task and current state.",
				},
				"need_confirm": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether to prompt the user for confirmation before switching agents. Defaults to true.",
					"default":     true,
				},
			},
			"required": []string{"name"},
		},
	}
	switchAgentTool := OpenTool{
		Type:     ToolTypeFunction,
		Function: &switchAgentFunc,
	}

	tools = append(tools, &switchAgentTool)

	// list_agent tool - List all available agents
	listAgentFunc := OpenFunctionDefinition{
		Name: "list_agent",
		Description: `List all available agents with their capabilities, models, and configurations.
Use this tool to discover which agents are available before using call_agent or switch_agent.`,
		Parameters: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
			"required":   []string{},
		},
	}
	listAgentTool := OpenTool{
		Type:     ToolTypeFunction,
		Function: &listAgentFunc,
	}
	tools = append(tools, &listAgentTool)

	// call_agent tool - The core orchestration tool
	callAgentFunc := OpenFunctionDefinition{
		Name: "call_agent",
		Description: `Orchestrate concurrent sub-agents to execute specialized tasks in parallel.

This tool enables sophisticated Map/Reduce workflows by dispatching tasks to isolated agent instances.
Each sub-agent runs independently with auto-approved tools and returns results via SharedState.

Use this for:
- Delegating specialized tasks to agents with appropriate capabilities
- Parallel processing of independent tasks across multiple agent instances
- Complex multi-stage workflows requiring orchestration

Key operational details:
- Sub-agents run in isolated contexts (no shared conversation history)
- Provide complete context in each instruction
- CRITICAL: Assign a unique, semantic task_key to each task—this is your ONLY mechanism
  to retrieve results and correlate outputs across the workflow
- Returns progress summary; use get_state(task_key) for full detailed results

Differs from switch_agent:
- call_agent: Sub-agents return results to you; you maintain control
- switch_agent: Permanently hands off control; you won't see results`,
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"tasks": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"agent": map[string]interface{}{
								"type":        "string",
								"description": "Name of the agent to invoke. Use list_agent to see available agents.",
							},
							"instruction": map[string]interface{}{
								"type":        "string",
								"description": "The task instruction/prompt for the sub-agent. Be specific and provide all necessary context.",
							},
							"task_key": map[string]interface{}{
								"type":        "string",
								"description": "A unique, semantic string identifier for this specific task execution. This key acts as the 'Primary Key' for the task's output. It serves three critical roles: 1. ADDRESS: It is the specific key used to write the full result into SharedState memory. 2. STORAGE: It forms the unique suffix of the persistent output filename (e.g., ..._analysis_codereview.md), enabling debuggability. 3. RETRIEVAL: You MUST use this exact key with get_state to read the sub-agent's work. Example: 'code_review_auth_module', 'market_analysis_competitor_A'.",
							},
							"input_keys": map[string]interface{}{
								"type": "array",
								"items": map[string]interface{}{
									"type": "string",
								},
								"description": "Optional list of task_keys from PREVIOUS tasks. The output content of these keys will be automatically retrieved from SharedState and injected into this sub-agent's context. Use this to pass the results of 'agent A' as input to 'agent B'.",
							},
							"wait": map[string]interface{}{
								"type":        "boolean",
								"description": "If true, wait for ALL preceding tasks to complete before starting. Use for explicit barriers. Default is false (auto-waits based on input_keys dependencies).",
								"default":     false,
							},
						},
						"required": []string{"agent", "instruction", "task_key"},
					},
					"description": "Array of tasks to execute. Each task invokes a sub-agent with the given instruction.",
				},
				"timeout": map[string]interface{}{
					"type":        "integer",
					"description": "Timeout in seconds for all tasks. Default is 300 (5 minutes).",
					"default":     300,
				},
			},
			"required": []string{"tasks"},
		},
	}
	callAgentTool := OpenTool{
		Type:     ToolTypeFunction,
		Function: &callAgentFunc,
	}
	tools = append(tools, &callAgentTool)

	// get_state tool - Read from SharedState
	getStateFunc := OpenFunctionDefinition{
		Name: "get_state",
		Description: `Retrieve a value from the SharedState memory.

SharedState is a key-value store for communication between the orchestrator and sub-agents.
Sub-agents store their results in SharedState when you specify an output_key in call_agent.
Use list_state to see available keys.`,
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"key": map[string]interface{}{
					"type":        "string",
					"description": "The key to retrieve from SharedState.",
				},
			},
			"required": []string{"key"},
		},
	}
	getStateTool := OpenTool{
		Type:     ToolTypeFunction,
		Function: &getStateFunc,
	}
	tools = append(tools, &getStateTool)

	// set_state tool - Write to SharedState
	setStateFunc := OpenFunctionDefinition{
		Name: "set_state",
		Description: `Store a value in the SharedState memory.

Use this to save information that other agents or future tool calls can access.
SharedState persists for the duration of the current session.`,
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"key": map[string]interface{}{
					"type":        "string",
					"description": "The key to store the value under.",
				},
				"value": map[string]interface{}{
					"type":        "string",
					"description": "The value to store. Can be text, JSON, or any serializable content.",
				},
			},
			"required": []string{"key", "value"},
		},
	}
	setStateTool := OpenTool{
		Type:     ToolTypeFunction,
		Function: &setStateFunc,
	}
	tools = append(tools, &setStateTool)

	// list_state tool - List all SharedState keys
	listStateFunc := OpenFunctionDefinition{
		Name: "list_state",
		Description: `List all keys and their metadata in SharedState.

Shows what data is available in the shared memory, including who created each entry,
when it was created/updated, content type, and size.`,
		Parameters: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
			"required":   []string{},
		},
	}
	listStateTool := OpenTool{
		Type:     ToolTypeFunction,
		Function: &listStateFunc,
	}
	tools = append(tools, &listStateTool)

	// activate_skill tool
	activateSkillFunc := OpenFunctionDefinition{
		Name: "activate_skill",
		Description: `Activates a specialized agent skill by name and returns the skill's instructions.
The returned instructions provide specialized guidance for the current task.
Use this when you identify a task that matches a skill's description.
ONLY use names exactly as they appear in the <available_skills> section.`,
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "The exact name of the skill to activate (case-insensitive).",
				},
			},
			"required": []string{"name"},
		},
	}
	activateSkillTool := OpenTool{
		Type:     ToolTypeFunction,
		Function: &activateSkillFunc,
	}
	tools = append(tools, &activateSkillTool)

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

	// Sub-agent orchestration
	sharedState *data.SharedState // Shared state for inter-agent communication
	executor    *SubAgentExecutor // Sub-agent executor for call_agent tool
	agentName   string            // Current agent name (for set_state metadata)
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

/*
 * OpenAI tool call implements
 *
 */

// OpenAI tool implementations (wrapper functions)
func (op *OpenProcessor) OpenAIShellToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	response, err := shellToolCallImpl(argsMap, op.toolsUse)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Content:    response,
	}, err
}

func (op *OpenProcessor) OpenAIWebFetchToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	response, err := webFetchToolCallImpl(argsMap)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Content:    response,
	}, err
}

func (op *OpenProcessor) OpenAIWebSearchToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	response, err := webSearchToolCallImpl(argsMap, &op.queries, &op.references, op.search)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		Content:    response,
		ToolCallID: toolCall.ID,
	}, err
}

func (op *OpenProcessor) OpenAIReadFileToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	response, err := readFileToolCallImpl(argsMap)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Content:    response,
	}, err
}

func (op *OpenProcessor) OpenAIWriteFileToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	response, err := writeFileToolCallImpl(argsMap, op.toolsUse, op.showDiffConfirm, op.closeDiffConfirm)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Content:    response,
	}, err
}

func (op *OpenProcessor) OpenAIEditFileToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	response, err := editFileToolCallImpl(argsMap, op.toolsUse, op.showDiffConfirm, op.closeDiffConfirm)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Content:    response,
	}, err
}

func (op *OpenProcessor) OpenAICreateDirectoryToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	response, err := createDirectoryToolCallImpl(argsMap)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Content:    response,
	}, err
}

func (op *OpenProcessor) OpenAIListDirectoryToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	response, err := listDirectoryToolCallImpl(argsMap)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Content:    response,
	}, err
}

func (op *OpenProcessor) OpenAIDeleteFileToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	response, err := deleteFileToolCallImpl(argsMap, op.toolsUse)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Content:    response,
	}, err
}

func (op *OpenProcessor) OpenAIDeleteDirectoryToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	response, err := deleteDirectoryToolCallImpl(argsMap, op.toolsUse)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Content:    response,
	}, err
}

func (op *OpenProcessor) OpenAIMoveToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	response, err := moveToolCallImpl(argsMap, op.toolsUse)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Content:    response,
	}, err
}

func (op *OpenProcessor) OpenAICopyToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	response, err := copyToolCallImpl(argsMap, op.toolsUse)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Content:    response,
	}, err
}

func (op *OpenProcessor) OpenAISearchFilesToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	response, err := searchFilesToolCallImpl(argsMap)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Content:    response,
	}, err
}

func (op *OpenProcessor) OpenAISearchTextInFileToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	response, err := searchTextInFileToolCallImpl(argsMap)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Content:    response,
	}, err
}

func (op *OpenProcessor) OpenAIReadMultipleFilesToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	response, err := readMultipleFilesToolCallImpl(argsMap)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Content:    response,
	}, err
}

func (op *OpenProcessor) OpenAIMCPToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	if op.mcpClient == nil {
		err := fmt.Errorf("MCP client not initialized")
		return openai.ChatCompletionMessage{
			Role:       openai.ChatMessageRoleTool,
			ToolCallID: toolCall.ID,
			Content:    fmt.Sprintf("Error: MCP tool call failed: %v", err),
		}, err
	}

	// Call the MCP tool
	result, err := op.mcpClient.CallTool(toolCall.Function.Name, *argsMap)
	if err != nil {
		// Wrap error in response
		return openai.ChatCompletionMessage{
			Role:       openai.ChatMessageRoleTool,
			ToolCallID: toolCall.ID,
			Content:    fmt.Sprintf("Error: MCP tool call failed: %v", err),
		}, err
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
			output += fmt.Sprintf("![Image](%s)\n", content)
		case MCPResponseAudio:
			output += content + "\n"
			output += fmt.Sprintf("![Audio](%s)\n", content)
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

func (op *OpenProcessor) OpenAIListMemoryToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	// Call shared implementation (no args needed)
	response, err := listMemoryToolCallImpl()
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		Content:    response,
		ToolCallID: toolCall.ID,
	}, err
}

func (op *OpenProcessor) OpenAISaveMemoryToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	// Call shared implementation
	response, err := saveMemoryToolCallImpl(argsMap)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		Content:    response,
		ToolCallID: toolCall.ID,
	}, err
}

func (op *OpenProcessor) OpenAISwitchAgentToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	response, err := switchAgentToolCallImpl(argsMap, op.toolsUse)

	// Create the tool message anyway
	toolMessage := openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Content:    response,
	}

	if err != nil {
		if IsSwitchAgentError(err) {
			return toolMessage, err
		}
		// Wrap other errors in response
		toolMessage.Content = fmt.Sprintf("Error: %v", err)
	}

	return toolMessage, err
}

// OpenAI wrappers for new orchestration tools

func (op *OpenProcessor) OpenAIListAgentToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	response, err := listAgentToolCallImpl()
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Content:    response,
	}, err
}

func (op *OpenProcessor) OpenAICallAgentToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	response, err := callAgentToolCallImpl(argsMap, op.executor)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Content:    response,
	}, err
}

func (op *OpenProcessor) OpenAIGetStateToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	response, err := getStateToolCallImpl(argsMap, op.sharedState)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Content:    response,
	}, err
}

func (op *OpenProcessor) OpenAISetStateToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	response, err := setStateToolCallImpl(argsMap, op.agentName, op.sharedState)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Content:    response,
	}, err
}

func (op *OpenProcessor) OpenAIListStateToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	response, err := listStateToolCallImpl(op.sharedState)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Content:    response,
	}, err
}

func (op *OpenProcessor) OpenAIActivateSkillToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	response, err := activateSkillToolCallImpl(argsMap, op.toolsUse)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Content:    response,
	}, err
}

/*
 * OpenChat tool call implements
 *
 */

func (op *OpenProcessor) OpenChatSwitchAgentToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	response, err := switchAgentToolCallImpl(argsMap, op.toolsUse)

	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
		Content: &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(response),
		},
	}

	if err != nil {
		if IsSwitchAgentError(err) {
			return &toolMessage, err
		}
		return &toolMessage, err
	}

	return &toolMessage, err
}

func (op *OpenProcessor) OpenChatMCPToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	if op.mcpClient == nil {
		err := fmt.Errorf("MCP client not initialized")
		toolMessage := model.ChatCompletionMessage{
			Role:       model.ChatMessageRoleTool,
			ToolCallID: toolCall.ID,
			Name:       Ptr(""),
			Content: &model.ChatCompletionMessageContent{
				StringValue: volcengine.String(fmt.Sprintf("Error: MCP tool call failed: %v", err)),
			},
		}
		return &toolMessage, err
	}

	// Call the MCP tool
	result, err := op.mcpClient.CallTool(toolCall.Function.Name, *argsMap)
	if err != nil {
		toolMessage := model.ChatCompletionMessage{
			Role:       model.ChatMessageRoleTool,
			ToolCallID: toolCall.ID,
			Name:       Ptr(""),
			Content: &model.ChatCompletionMessageContent{
				StringValue: volcengine.String(fmt.Sprintf("Error: MCP tool call failed: %v", err)),
			},
		}
		return &toolMessage, err
	}

	// OpenChat supports text, image, audio toolcall responses
	// But to be robust and consistent with OpenAI(which strictly requires string),
	// We convert all content to a single string value.
	var mergedText strings.Builder
	for i, content := range result.Contents {
		if i > 0 {
			mergedText.WriteString("\n")
		}
		switch result.Types[i] {
		case MCPResponseText:
			mergedText.WriteString(content)
		case MCPResponseImage:
			mergedText.WriteString(fmt.Sprintf("![Image](%s)", content))
		case MCPResponseAudio:
			mergedText.WriteString(fmt.Sprintf("![Audio](%s)", content))
		default:
			// Unknown file type, skip
		}
	}

	// Bugfix: only return as string value directly
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
		Content: &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(mergedText.String()),
		},
	}

	Debugf("OpenChatMCPToolCall Response: %s", *toolMessage.Content.StringValue)
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
		response = fmt.Sprintf("Error: %v", err)
	}

	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(response),
	}
	return &toolMessage, err
}

func (op *OpenProcessor) OpenChatWriteFileToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	response, err := writeFileToolCallImpl(argsMap, op.toolsUse, op.showDiffConfirm, op.closeDiffConfirm)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(response),
	}
	return &toolMessage, err
}

func (op *OpenProcessor) OpenChatCreateDirectoryToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	response, err := createDirectoryToolCallImpl(argsMap)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(response),
	}
	return &toolMessage, err
}

func (op *OpenProcessor) OpenChatListDirectoryToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	response, err := listDirectoryToolCallImpl(argsMap)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(response),
	}
	return &toolMessage, err
}

func (op *OpenProcessor) OpenChatDeleteFileToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	response, err := deleteFileToolCallImpl(argsMap, op.toolsUse)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(response),
	}
	return &toolMessage, err
}

func (op *OpenProcessor) OpenChatDeleteDirectoryToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	response, err := deleteDirectoryToolCallImpl(argsMap, op.toolsUse)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(response),
	}
	return &toolMessage, err
}

func (op *OpenProcessor) OpenChatMoveToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	response, err := moveToolCallImpl(argsMap, op.toolsUse)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(response),
	}
	return &toolMessage, err
}

func (op *OpenProcessor) OpenChatSearchFilesToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	response, err := searchFilesToolCallImpl(argsMap)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(response),
	}
	return &toolMessage, err
}

func (op *OpenProcessor) OpenChatSearchTextInFileToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	response, err := searchTextInFileToolCallImpl(argsMap)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(response),
	}
	return &toolMessage, err
}

func (op *OpenProcessor) OpenChatReadMultipleFilesToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	response, err := readMultipleFilesToolCallImpl(argsMap)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(response),
	}
	return &toolMessage, err
}

func (op *OpenProcessor) OpenChatShellToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	response, err := shellToolCallImpl(argsMap, op.toolsUse)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(response),
	}
	return &toolMessage, err
}

func (op *OpenProcessor) OpenChatWebFetchToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	response, err := webFetchToolCallImpl(argsMap)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(response),
	}
	return &toolMessage, err
}

func (op *OpenProcessor) OpenChatWebSearchToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	response, err := webSearchToolCallImpl(argsMap, &op.queries, &op.references, op.search)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	// Create and return the tool response message
	return &model.ChatCompletionMessage{
		Role: model.ChatMessageRoleTool,
		Content: &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(response),
		}, Name: Ptr(""),
		ToolCallID: toolCall.ID,
	}, err
}

func (op *OpenProcessor) OpenChatEditFileToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	response, err := editFileToolCallImpl(argsMap, op.toolsUse, op.showDiffConfirm, op.closeDiffConfirm)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(response),
	}
	return &toolMessage, err
}

func (op *OpenProcessor) OpenChatCopyToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	response, err := copyToolCallImpl(argsMap, op.toolsUse)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(response),
	}
	return &toolMessage, err
}

func (op *OpenProcessor) OpenChatListMemoryToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	// Call shared implementation (no args needed)
	response, err := listMemoryToolCallImpl()
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(toolCall.Function.Name),
	}
	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(response),
	}
	return &toolMessage, err
}

func (op *OpenProcessor) OpenChatSaveMemoryToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	// Call shared implementation
	response, err := saveMemoryToolCallImpl(argsMap)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(toolCall.Function.Name),
	}
	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(response),
	}
	return &toolMessage, err
}

func (op *OpenProcessor) OpenChatListAgentToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	response, err := listAgentToolCallImpl()
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
		Content: &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(response),
		},
	}
	return &toolMessage, err
}

func (op *OpenProcessor) OpenChatCallAgentToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	response, err := callAgentToolCallImpl(argsMap, op.executor)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
		Content: &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(response),
		},
	}
	return &toolMessage, err
}

func (op *OpenProcessor) OpenChatGetStateToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	response, err := getStateToolCallImpl(argsMap, op.sharedState)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
		Content: &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(response),
		},
	}
	return &toolMessage, err
}

func (op *OpenProcessor) OpenChatSetStateToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	response, err := setStateToolCallImpl(argsMap, op.agentName, op.sharedState)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
		Content: &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(response),
		},
	}
	return &toolMessage, err
}

func (op *OpenProcessor) OpenChatListStateToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	response, err := listStateToolCallImpl(op.sharedState)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
		Content: &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(response),
		},
	}
	return &toolMessage, err
}

func (op *OpenProcessor) OpenChatActivateSkillToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	response, err := activateSkillToolCallImpl(argsMap, op.toolsUse)
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
		Content: &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(response),
		},
	}
	return &toolMessage, err
}
