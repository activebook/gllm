package service

import (
	"context"
	"slices"

	"github.com/activebook/gllm/data"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/anthropics/anthropic-sdk-go/shared/constant"
	"github.com/sashabaranov/go-openai"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"google.golang.org/genai"
)

type ToolType string

const (
	ToolTypeFunction ToolType = "function"

	// Tool Names
	ToolShell             = "shell"
	ToolReadFile          = "read_file"
	ToolWriteFile         = "write_file"
	ToolEditFile          = "edit_file"
	ToolDeleteFile        = "delete_file"
	ToolCreateDirectory   = "create_directory"
	ToolListDirectory     = "list_directory"
	ToolDeleteDirectory   = "delete_directory"
	ToolMove              = "move"
	ToolCopy              = "copy"
	ToolSearchFiles       = "search_files"
	ToolSearchTextInFile  = "search_text_in_file"
	ToolReadMultipleFiles = "read_multiple_files"
	ToolWebFetch          = "web_fetch"
	ToolSwitchAgent       = "switch_agent"
	ToolWebSearch         = "web_search"
	ToolActivateSkill     = "activate_skill"
	ToolListMemory        = "list_memory"
	ToolSaveMemory        = "save_memory"
	ToolListAgent         = "list_agent"
	ToolSpawnSubAgents    = "spawn_subagents"
	ToolGetState          = "get_state"
	ToolSetState          = "set_state"
	ToolListState         = "list_state"
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
		ToolShell,
		// file tools
		ToolReadFile,
		ToolWriteFile,
		ToolEditFile,
		ToolDeleteFile,
		ToolCreateDirectory,
		ToolListDirectory,
		ToolDeleteDirectory,
		ToolMove,
		ToolCopy,
		ToolSearchFiles,
		ToolSearchTextInFile,
		ToolReadMultipleFiles,
		// web tools
		ToolWebFetch,
		// agent tools
		ToolSwitchAgent,
	}
	searchTools = []string{
		// web tools
		ToolWebSearch,
	}
	skillTools = []string{
		// skill tools
		ToolActivateSkill,
	}
	memoryTools = []string{
		// memory tools
		ToolListMemory,
		ToolSaveMemory,
	}
	subagentTools = []string{
		// Sub-agent orchestration tools
		ToolListAgent,
		ToolSpawnSubAgents,
		// shared state tools
		ToolGetState,
		ToolSetState,
		ToolListState,
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

func GetAllMemoryTools() []string {
	return memoryTools
}

func GetAllSubagentTools() []string {
	return subagentTools
}

func GetAllOpenTools() []string {
	tools := GetAllEmbeddingTools()
	tools = append(tools, GetAllSearchTools()...)
	tools = append(tools, GetAllSkillTools()...)
	tools = append(tools, GetAllMemoryTools()...)
	tools = append(tools, GetAllSubagentTools()...)
	return tools
}

// IsAvailableTool checks if a tool is available for the current agent.
// It checks if the tool is available in the
// embedding tools, search tools, skill tools, memory tools, subagent tools, or MCP tools.
func IsAvailableOpenTool(toolName string) bool {
	return AvailableEmbeddingTool(toolName) ||
		AvailableSearchTool(toolName) ||
		AvailableSkillTool(toolName) ||
		AvailableMemoryTool(toolName) ||
		AvailableSubagentTool(toolName)
}

// AvailableEmbeddingTool checks if a tool is available in the embedding tools.
func AvailableEmbeddingTool(toolName string) bool {
	for _, tool := range embeddingTools {
		if tool == toolName {
			return true
		}
	}
	return false
}

// AvailableSearchTool checks if a tool is available in the search tools.
func AvailableSearchTool(toolName string) bool {
	for _, tool := range searchTools {
		if tool == toolName {
			return true
		}
	}
	return false
}

// AvailableSkillTool checks if a tool is available in the skill tools.
func AvailableSkillTool(toolName string) bool {
	for _, tool := range skillTools {
		if tool == toolName {
			return true
		}
	}
	return false
}

// AvailableMemoryTool checks if a tool is available in the memory tools.
func AvailableMemoryTool(toolName string) bool {
	for _, tool := range memoryTools {
		if tool == toolName {
			return true
		}
	}
	return false
}

// AvailableSubagentTool checks if a tool is available in the subagent tools.
func AvailableSubagentTool(toolName string) bool {
	for _, tool := range subagentTools {
		if tool == toolName {
			return true
		}
	}
	return false
}

// AppendSubagentTools appends subagent tools to the given tools slice if they are not already present.
func AppendSubagentTools(tools []string) []string {
	for _, tool := range subagentTools {
		if !slices.Contains(tools, tool) {
			tools = append(tools, tool)
		}
	}
	return tools
}

// RemoveSubagentTools removes subagent tools from the given tools slice.
func RemoveSubagentTools(tools []string) []string {
	for _, tool := range subagentTools {
		tools = slices.DeleteFunc(tools, func(t string) bool {
			return t == tool
		})
	}
	return tools
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

// AppendMemoryTools appends memory tools to the given tools slice if they are not already present.
func AppendMemoryTools(tools []string) []string {
	for _, tool := range memoryTools {
		if !slices.Contains(tools, tool) {
			tools = append(tools, tool)
		}
	}
	return tools
}

// RemoveMemoryTools removes memory tools from the given tools slice.
func RemoveMemoryTools(tools []string) []string {
	for _, tool := range memoryTools {
		tools = slices.DeleteFunc(tools, func(t string) bool {
			return t == tool
		})
	}
	return tools
}

// AppendSearchTools appends search tools to the given tools slice if they are not already present.
func AppendSearchTools(tools []string) []string {
	for _, tool := range searchTools {
		if !slices.Contains(tools, tool) {
			tools = append(tools, tool)
		}
	}
	return tools
}

// RemoveSearchTools removes search tools from the given tools slice.
func RemoveSearchTools(tools []string) []string {
	for _, tool := range searchTools {
		tools = slices.DeleteFunc(tools, func(t string) bool {
			return t == tool
		})
	}
	return tools
}

// GetOpenToolsFiltered returns tools filtered by the allowed list.
// If allowedTools is nil or empty, returns all tools.
// Unknown tool names are gracefully ignored.
func GetOpenToolsFiltered(allowedTools []string) []*OpenTool {
	allTools := getOpenTools()

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

func FilterOpenToolArguments(argsMap map[string]interface{}, ignoreKeys []string) map[string]interface{} {
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

// getOpenTools returns the tools for all models
func getOpenTools() []*OpenTool {
	var tools []*OpenTool

	// Shell tool
	shellTool := getOpenShellTool()
	tools = append(tools, shellTool)

	// Web fetch tool
	webFetchTool := getWebFetchTool()
	tools = append(tools, webFetchTool)

	// Web Search tool
	webSearchTool := getWebSearchTool()
	tools = append(tools, webSearchTool)

	// Read file tool
	readFileTool := getReadFileTool()
	tools = append(tools, readFileTool)

	// Write file tool
	writeFileTool := getWriteFileTool()
	tools = append(tools, writeFileTool)

	// Create directory tool
	createDirectoryTool := getCreateDirectoryTool()
	tools = append(tools, createDirectoryTool)

	// List directory tool
	listDirectoryTool := getListDirectoryTool()
	tools = append(tools, listDirectoryTool)

	// Delete file tool
	deleteFileTool := getDeleteFileTool()
	tools = append(tools, deleteFileTool)

	// Delete directory tool
	deleteDirectoryTool := getDeleteDirectoryTool()
	tools = append(tools, deleteDirectoryTool)

	// Search files tool
	searchFilesTool := getSearchFilesTool()
	tools = append(tools, searchFilesTool)

	// Search text in file tool
	searchTextTool := getSearchTextInFileTool()
	tools = append(tools, searchTextTool)

	// Read multiple files tool
	readMultipleFilesTool := getReadMultipleFilesTool()
	tools = append(tools, readMultipleFilesTool)

	// Edit file tool
	editFileTool := getEditFileTool()
	tools = append(tools, editFileTool)

	// Move file/directory tool
	moveTool := getMoveTool()
	tools = append(tools, moveTool)

	// Copy file/directory tool
	copyTool := getCopyTool()
	tools = append(tools, copyTool)

	// list_memory tool
	listMemoryTool := getListMemoryTool()
	tools = append(tools, listMemoryTool)

	// save_memory tool
	saveMemoryTool := getSaveMemoryTool()
	tools = append(tools, saveMemoryTool)

	// Switch Agent tool
	switchAgentTool := getSwitchAgentTool()
	tools = append(tools, switchAgentTool)

	// list_agent tool - List all available agents
	listAgentTool := getListAgentTool()
	tools = append(tools, listAgentTool)

	// spawn_subagents tool - The core orchestration tool
	spawnSubAgentsTool := getSpawnSubAgentsTool()
	tools = append(tools, spawnSubAgentsTool)

	// get_state tool - Read from SharedState
	getStateTool := getGetStateTool()
	tools = append(tools, getStateTool)

	// set_state tool - Write to SharedState
	setStateTool := getSetStateTool()
	tools = append(tools, setStateTool)

	// list_state tool - List all SharedState keys
	listStateTool := getListStateTool()
	tools = append(tools, listStateTool)

	// activate_skill tool
	activateSkillTool := getActivateSkillTool()
	tools = append(tools, activateSkillTool)

	return tools
}

func getReadFileTool() *OpenTool {
	readFileFunc := OpenFunctionDefinition{
		Name:        ToolReadFile,
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
	return &readFileTool
}

func getWriteFileTool() *OpenTool {
	writeFileFunc := OpenFunctionDefinition{
		Name:        ToolWriteFile,
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
				"purpose": map[string]interface{}{
					"type":        "string",
					"description": "A terse explanation of why this file is being written.",
				},
				"need_confirm": map[string]interface{}{
					"type": "boolean",
					"description": "Specifies whether to prompt the user for confirmation before writing to the file. " +
						"This should always be true for safety.",
					"default": true,
				},
			},
			"required": []string{"path", "content", "purpose"},
		},
	}
	writeFileTool := OpenTool{
		Type:     ToolTypeFunction,
		Function: &writeFileFunc,
	}
	return &writeFileTool
}

func getCreateDirectoryTool() *OpenTool {
	createDirectoryFunc := OpenFunctionDefinition{
		Name:        ToolCreateDirectory,
		Description: "Create a new directory in the filesystem.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The path of the directory to create.",
				},
				"purpose": map[string]interface{}{
					"type":        "string",
					"description": "A terse explanation of why this directory is being created.",
				},
				"need_confirm": map[string]interface{}{
					"type": "boolean",
					"description": "Specifies whether to prompt the user for confirmation before creating the directory. " +
						"Default is true.",
					"default": true,
				},
			},
			"required": []string{"path", "purpose"},
		},
	}
	createDirectoryTool := OpenTool{
		Type:     ToolTypeFunction,
		Function: &createDirectoryFunc,
	}
	return &createDirectoryTool
}

func getListDirectoryTool() *OpenTool {
	listDirectoryFunc := OpenFunctionDefinition{
		Name:        ToolListDirectory,
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
	listDirectoryTool := OpenTool{
		Type:     ToolTypeFunction,
		Function: &listDirectoryFunc,
	}
	return &listDirectoryTool
}

func getDeleteFileTool() *OpenTool {
	deleteFileFunc := OpenFunctionDefinition{
		Name:        ToolDeleteFile,
		Description: "Delete a file in the filesystem.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The path to the file to delete.",
				},
				"purpose": map[string]interface{}{
					"type":        "string",
					"description": "A terse explanation of why this file is being deleted.",
				},
				"need_confirm": map[string]interface{}{
					"type": "boolean",
					"description": "Specifies whether to prompt the user for confirmation before deleting the file. " +
						"This should always be true for safety.",
					"default": true,
				},
			},
			"required": []string{"path", "purpose"},
		},
	}
	deleteFileTool := OpenTool{
		Type:     ToolTypeFunction,
		Function: &deleteFileFunc,
	}
	return &deleteFileTool
}

func getDeleteDirectoryTool() *OpenTool {
	deleteDirectoryFunc := OpenFunctionDefinition{
		Name:        ToolDeleteDirectory,
		Description: "Delete a directory in the filesystem.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The path to the directory to delete.",
				},
				"purpose": map[string]interface{}{
					"type":        "string",
					"description": "A terse explanation of why this directory is being deleted.",
				},
				"need_confirm": map[string]interface{}{
					"type": "boolean",
					"description": "Specifies whether to prompt the user for confirmation before deleting the directory. " +
						"This should always be true for safety.",
					"default": true,
				},
			},
			"required": []string{"path", "purpose"},
		},
	}
	deleteDirectoryTool := OpenTool{
		Type:     ToolTypeFunction,
		Function: &deleteDirectoryFunc,
	}
	return &deleteDirectoryTool
}

func getSearchFilesTool() *OpenTool {
	searchFilesFunc := OpenFunctionDefinition{
		Name:        ToolSearchFiles,
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
	return &searchFilesTool
}

func getSearchTextInFileTool() *OpenTool {
	searchTextInFileFunc := OpenFunctionDefinition{
		Name:        ToolSearchTextInFile,
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
	searchTextInFileTool := OpenTool{
		Type:     ToolTypeFunction,
		Function: &searchTextInFileFunc,
	}
	return &searchTextInFileTool
}

func getReadMultipleFilesTool() *OpenTool {
	readMultipleFilesFunc := OpenFunctionDefinition{
		Name: ToolReadMultipleFiles,
		Description: "Read the contents of multiple files. " +
			"Use this when you need to inspect several files at once to understand the codebase or context.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"paths": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "string",
					},
					"description": "Array of file paths to read.",
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
	return &readMultipleFilesTool
}

func getEditFileTool() *OpenTool {
	editFileFunc := OpenFunctionDefinition{
		Name: ToolEditFile,
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
				"purpose": map[string]interface{}{
					"type":        "string",
					"description": "A terse explanation of why this file is being edited.",
				},
				"need_confirm": map[string]interface{}{
					"type": "boolean",
					"description": "Specifies whether to show diff and prompt for confirmation before editing the file. " +
						"This should always be true for safety.",
					"default": true,
				},
			},
			"required": []string{"path", "edits", "purpose"},
		},
	}
	editFileTool := OpenTool{
		Type:     ToolTypeFunction,
		Function: &editFileFunc,
	}
	return &editFileTool
}

func getMoveTool() *OpenTool {
	moveFunc := OpenFunctionDefinition{
		Name:        ToolMove,
		Description: "Move a file or directory from one location to another in the filesystem.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"source": map[string]interface{}{
					"type":        "string",
					"description": "The current path of the file or directory to move.",
				},
				"destination": map[string]interface{}{
					"type":        "string",
					"description": "The destination path for the file or directory move.",
				},
				"purpose": map[string]interface{}{
					"type":        "string",
					"description": "A terse explanation of why this file/directory is being moved.",
				},
				"need_confirm": map[string]interface{}{
					"type": "boolean",
					"description": "Specifies whether to prompt the user for confirmation before moving the file or directory. " +
						"This should be true for safety if it needs overwrite.",
					"default": true,
				},
			},
			"required": []string{"source", "destination", "purpose"},
		},
	}
	moveTool := OpenTool{
		Type:     ToolTypeFunction,
		Function: &moveFunc,
	}
	return &moveTool
}

func getCopyTool() *OpenTool {
	copyFunc := OpenFunctionDefinition{
		Name:        ToolCopy,
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
				"purpose": map[string]interface{}{
					"type":        "string",
					"description": "A terse explanation of why this file/directory is being copied.",
				},
				"need_confirm": map[string]interface{}{
					"type": "boolean",
					"description": "Specifies whether to prompt the user for confirmation before copying the file or directory. " +
						"This should be true for safety if it needs overwrite.",
					"default": true,
				},
			},
			"required": []string{"source", "destination", "purpose"},
		},
	}
	copyTool := OpenTool{
		Type:     ToolTypeFunction,
		Function: &copyFunc,
	}
	return &copyTool
}

func getSaveMemoryTool() *OpenTool {
	saveMemoryFunc := OpenFunctionDefinition{
		Name: ToolSaveMemory,
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
	return &saveMemoryTool
}

func getListMemoryTool() *OpenTool {
	listMemoryFunc := OpenFunctionDefinition{
		Name:        ToolListMemory,
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
	return &listMemoryTool
}

func getSwitchAgentTool() *OpenTool {
	switchAgentFunc := OpenFunctionDefinition{
		Name: ToolSwitchAgent,
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
	return &switchAgentTool
}

func getListAgentTool() *OpenTool {
	listAgentFunc := OpenFunctionDefinition{
		Name: ToolListAgent,
		Description: `List all available agents with their capabilities, models, and configurations.
Use this tool to discover which agents are available before using spawn_subagents or switch_agent.`,
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
	return &listAgentTool
}

func getSpawnSubAgentsTool() *OpenTool {
	spawnSubAgentsFunc := OpenFunctionDefinition{
		Name: ToolSpawnSubAgents,
		Description: `Spawn multiple sub-agents to perform parallel or sequential tasks.

This tool allows you to delegate work to specialized agents, manage dependencies between tasks,
and collect results in a structured way. Sub-agents run in isolated contexts (no shared conversation history).

CRITICAL: Assign a unique, semantic task_key to each task—this is your ONLY mechanism to retrieve results
and correlate outputs across the workflow. Returns progress summary; use get_state(task_key) for full detailed results.

Differs from switch_agent:
- spawn_subagents: Sub-agents return results to you; you maintain control
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
				"need_confirm": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether to prompt the user for confirmation before spawning sub-agents. Defaults to true.",
					"default":     true,
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
	spawnSubAgentsTool := OpenTool{
		Type:     ToolTypeFunction,
		Function: &spawnSubAgentsFunc,
	}
	return &spawnSubAgentsTool
}

func getGetStateTool() *OpenTool {
	getStateFunc := OpenFunctionDefinition{
		Name: ToolGetState,
		Description: `Retrieve a value from the SharedState memory.

SharedState is a key-value store for communication between the orchestrator and sub-agents.
Sub-agents store their results in SharedState when you specify a task_key in spawn_subagents.
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
	return &getStateTool
}

func getSetStateTool() *OpenTool {
	setStateFunc := OpenFunctionDefinition{
		Name: ToolSetState,
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
	return &setStateTool
}

func getListStateTool() *OpenTool {
	listStateFunc := OpenFunctionDefinition{
		Name: ToolListState,
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
	return &listStateTool
}

func getWebSearchTool() *OpenTool {
	webSearchFunc := OpenFunctionDefinition{
		Name: ToolWebSearch,
		Description: `Performs a web search using the Search API.

Use this tool to find relevant information on the web.

IMPORTANT:
- The query must be a string containing the search terms.
- The tool will return a list of search results, each with a title, URL, and snippet.
- This tool is useful for tasks that require finding information on the web, such as:
  - Finding relevant web pages for analysis
  - Extracting specific information from web pages
  - Analyzing web page structure
  - Processing web page content for other tools

Example:
User: "Can you find some information about the latest news on AI?"
LLM should call:
{
  "query": "latest news on AI"
}`,
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "The search query to use.",
				},
			},
			"required": []string{"query"},
		},
	}
	webSearchTool := OpenTool{
		Type:     ToolTypeFunction,
		Function: &webSearchFunc,
	}
	return &webSearchTool
}

func getWebFetchTool() *OpenTool {
	webFetchFunc := OpenFunctionDefinition{
		Name: ToolWebFetch,
		Description: `Fetches the content of a web page using a URL.

Use this tool to retrieve the full HTML content of a web page for analysis.

IMPORTANT:
- The URL must be a valid, absolute URL (e.g., https://www.example.com).
- The tool will fetch the complete page content, which may be large.
- The content will be returned as a string, including all HTML tags and scripts.
- This tool is useful for tasks that require deep analysis of web page content, such as:
  - Extracting specific information from web pages
  - Analyzing web page structure
  - Processing web page content for other tools

Example:
User: "Can you fetch the content of https://www.example.com?"
LLM should call:
{
  "url": "https://www.example.com"
}`,
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"url": map[string]interface{}{
					"type":        "string",
					"description": "The absolute URL of the web page to fetch (e.g., https://www.example.com).",
				},
			},
			"required": []string{"url"},
		},
	}
	webFetchTool := OpenTool{
		Type:     ToolTypeFunction,
		Function: &webFetchFunc,
	}
	return &webFetchTool
}

func getActivateSkillTool() *OpenTool {
	activateSkillFunc := OpenFunctionDefinition{
		Name: ToolActivateSkill,
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
	return &activateSkillTool
}

func getOpenShellTool() *OpenTool {
	shellFunc := OpenFunctionDefinition{
		Name: ToolShell,
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
	executor    *SubAgentExecutor // Sub-agent executor for spawn_subagents tool
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
