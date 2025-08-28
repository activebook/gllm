package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sashabaranov/go-openai"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
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
	ToolRespConfirmShell = "Based on your request, I've prepared the following command to execute on your system:\n\n" +
		"```\n%s\n```\n\n" +
		"This command will %s\n\n" +
		`**Check:** 
- First, Proceed with caution. Check whether this command may make irreversible changes or affect critical parts of your system. 
- Second, Ensure that the command is safe and does not contain any malicious or harmful actions.
- Third, If you deem it's safe, execute the command. If not, let user to choose whether to proceed or modify the command.
Such as:
Would you like me to run this command for you? Please confirm with 'yes', 'proceed', or provide alternative instructions.
`

	// ToolRespShellOutput is the template for the response to the user after executing a command.
	ToolRespShellOutput = `shell executed: %s
Status:
%s
%s`

	// ToolRespConfirmFileOp is the template for the response to the user before performing file operations.
	ToolRespConfirmFileOp = "Based on your request, I'm about to perform the following file operation:\n\n" +
		"```\n%s\n```\n\n" +
		"This operation will %s\n\n" +
		`**Check:** 
- First, Proceed with caution. Check whether this operation may make irreversible changes or affect critical parts of your system. 
- Second, Ensure that the operation is safe and does not contain any malicious or harmful actions.
- Third, If you deem it's safe, execute the operation. If not, let user to choose whether to proceed or modify the operation.
Such as:
Would you like me to proceed with this operation? Please confirm with 'yes', 'proceed', or provide alternative instructions.
`
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

func formatToolCallArguments(argsMap map[string]interface{}) string {
	var argsList []string
	for k, v := range argsMap {
		switch val := v.(type) {
		case []interface{}, map[string]interface{}:
			jsonStr, _ := json.Marshal(val)
			argsList = append(argsList, fmt.Sprintf("%s=%s", k, string(jsonStr)))
		default:
			argsList = append(argsList, fmt.Sprintf("%s=%v", k, v))
		}
	}
	return strings.Join(argsList, ", ")
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
		Type:     ToolTypeFunction,
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
	response, err := writeFileToolCallImpl(argsMap, op.toolsUse)
	if err != nil {
		return openai.ChatCompletionMessage{}, err
	}

	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Content:    response,
	}, nil
}

func (op *OpenProcessor) OpenAIEditFileToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	response, err := editFileToolCallImpl(argsMap, op.toolsUse)
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

	response, err := writeFileToolCallImpl(argsMap, op.toolsUse)
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

func (op *OpenProcessor) OpenChatEditFileToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	response, err := editFileToolCallImpl(argsMap, op.toolsUse)
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