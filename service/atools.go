package service

import (
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

// Anthropic tool implementations (wrapper functions)
func (op *OpenProcessor) AnthropicShellToolCall(toolCall anthropic.ToolUseBlockParam, argsMap *map[string]interface{}) (anthropic.MessageParam, error) {
	response, err := shellToolCallImpl(argsMap, op.toolsUse)
	isError := err != nil
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolResult := anthropic.NewToolResultBlock(toolCall.ID, response, isError)
	return anthropic.NewUserMessage(toolResult), nil
}

func (op *OpenProcessor) AnthropicReadFileToolCall(toolCall anthropic.ToolUseBlockParam, argsMap *map[string]interface{}) (anthropic.MessageParam, error) {
	response, err := readFileToolCallImpl(argsMap)
	isError := err != nil
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolResult := anthropic.NewToolResultBlock(toolCall.ID, response, isError)
	return anthropic.NewUserMessage(toolResult), nil
}

func (op *OpenProcessor) AnthropicWriteFileToolCall(toolCall anthropic.ToolUseBlockParam, argsMap *map[string]interface{}) (anthropic.MessageParam, error) {
	response, err := writeFileToolCallImpl(argsMap, op.toolsUse, op.showDiffConfirm, op.closeDiffConfirm)
	isError := err != nil
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolResult := anthropic.NewToolResultBlock(toolCall.ID, response, isError)
	return anthropic.NewUserMessage(toolResult), nil
}

func (op *OpenProcessor) AnthropicEditFileToolCall(toolCall anthropic.ToolUseBlockParam, argsMap *map[string]interface{}) (anthropic.MessageParam, error) {
	response, err := editFileToolCallImpl(argsMap, op.toolsUse, op.showDiffConfirm, op.closeDiffConfirm)
	isError := err != nil
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolResult := anthropic.NewToolResultBlock(toolCall.ID, response, isError)
	return anthropic.NewUserMessage(toolResult), nil
}

func (op *OpenProcessor) AnthropicCreateDirectoryToolCall(toolCall anthropic.ToolUseBlockParam, argsMap *map[string]interface{}) (anthropic.MessageParam, error) {
	response, err := createDirectoryToolCallImpl(argsMap)
	isError := err != nil
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolResult := anthropic.NewToolResultBlock(toolCall.ID, response, isError)
	return anthropic.NewUserMessage(toolResult), nil
}

func (op *OpenProcessor) AnthropicListDirectoryToolCall(toolCall anthropic.ToolUseBlockParam, argsMap *map[string]interface{}) (anthropic.MessageParam, error) {
	response, err := listDirectoryToolCallImpl(argsMap)
	isError := err != nil
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolResult := anthropic.NewToolResultBlock(toolCall.ID, response, isError)
	return anthropic.NewUserMessage(toolResult), nil
}

func (op *OpenProcessor) AnthropicDeleteFileToolCall(toolCall anthropic.ToolUseBlockParam, argsMap *map[string]interface{}) (anthropic.MessageParam, error) {
	response, err := deleteFileToolCallImpl(argsMap, op.toolsUse)
	isError := err != nil
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolResult := anthropic.NewToolResultBlock(toolCall.ID, response, isError)
	return anthropic.NewUserMessage(toolResult), nil
}

func (op *OpenProcessor) AnthropicDeleteDirectoryToolCall(toolCall anthropic.ToolUseBlockParam, argsMap *map[string]interface{}) (anthropic.MessageParam, error) {
	response, err := deleteDirectoryToolCallImpl(argsMap, op.toolsUse)
	isError := err != nil
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolResult := anthropic.NewToolResultBlock(toolCall.ID, response, isError)
	return anthropic.NewUserMessage(toolResult), nil
}

func (op *OpenProcessor) AnthropicMoveToolCall(toolCall anthropic.ToolUseBlockParam, argsMap *map[string]interface{}) (anthropic.MessageParam, error) {
	response, err := moveToolCallImpl(argsMap, op.toolsUse)
	isError := err != nil
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolResult := anthropic.NewToolResultBlock(toolCall.ID, response, isError)
	return anthropic.NewUserMessage(toolResult), nil
}

func (op *OpenProcessor) AnthropicCopyToolCall(toolCall anthropic.ToolUseBlockParam, argsMap *map[string]interface{}) (anthropic.MessageParam, error) {
	response, err := copyToolCallImpl(argsMap, op.toolsUse)
	isError := err != nil
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolResult := anthropic.NewToolResultBlock(toolCall.ID, response, isError)
	return anthropic.NewUserMessage(toolResult), nil
}

func (op *OpenProcessor) AnthropicSearchFilesToolCall(toolCall anthropic.ToolUseBlockParam, argsMap *map[string]interface{}) (anthropic.MessageParam, error) {
	response, err := searchFilesToolCallImpl(argsMap)
	isError := err != nil
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolResult := anthropic.NewToolResultBlock(toolCall.ID, response, isError)
	return anthropic.NewUserMessage(toolResult), nil
}

func (op *OpenProcessor) AnthropicSearchTextInFileToolCall(toolCall anthropic.ToolUseBlockParam, argsMap *map[string]interface{}) (anthropic.MessageParam, error) {
	response, err := searchTextInFileToolCallImpl(argsMap)
	isError := err != nil
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolResult := anthropic.NewToolResultBlock(toolCall.ID, response, isError)
	return anthropic.NewUserMessage(toolResult), nil
}

func (op *OpenProcessor) AnthropicReadMultipleFilesToolCall(toolCall anthropic.ToolUseBlockParam, argsMap *map[string]interface{}) (anthropic.MessageParam, error) {
	response, err := readMultipleFilesToolCallImpl(argsMap)
	isError := err != nil
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolResult := anthropic.NewToolResultBlock(toolCall.ID, response, isError)
	return anthropic.NewUserMessage(toolResult), nil
}

func (op *OpenProcessor) AnthropicWebFetchToolCall(toolCall anthropic.ToolUseBlockParam, argsMap *map[string]interface{}) (anthropic.MessageParam, error) {
	response, err := webFetchToolCallImpl(argsMap)
	isError := err != nil
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolResult := anthropic.NewToolResultBlock(toolCall.ID, response, isError)
	return anthropic.NewUserMessage(toolResult), nil
}

func (op *OpenProcessor) AnthropicWebSearchToolCall(toolCall anthropic.ToolUseBlockParam, argsMap *map[string]interface{}) (anthropic.MessageParam, error) {
	response, err := webSearchToolCallImpl(argsMap, &op.queries, &op.references, op.search)
	isError := err != nil
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolResult := anthropic.NewToolResultBlock(toolCall.ID, response, isError)
	return anthropic.NewUserMessage(toolResult), nil
}

func (op *OpenProcessor) AnthropicListMemoryToolCall(toolCall anthropic.ToolUseBlockParam, argsMap *map[string]interface{}) (anthropic.MessageParam, error) {
	response, err := listMemoryToolCallImpl()
	isError := err != nil
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolResult := anthropic.NewToolResultBlock(toolCall.ID, response, isError)
	return anthropic.NewUserMessage(toolResult), nil
}

func (op *OpenProcessor) AnthropicSaveMemoryToolCall(toolCall anthropic.ToolUseBlockParam, argsMap *map[string]interface{}) (anthropic.MessageParam, error) {
	response, err := saveMemoryToolCallImpl(argsMap)
	isError := err != nil
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolResult := anthropic.NewToolResultBlock(toolCall.ID, response, isError)
	return anthropic.NewUserMessage(toolResult), nil
}

func (op *OpenProcessor) AnthropicMCPToolCall(toolCall anthropic.ToolUseBlockParam, argsMap *map[string]interface{}) (anthropic.MessageParam, error) {
	if op.mcpClient == nil {
		return anthropic.MessageParam{}, fmt.Errorf("MCP client not initialized")
	}

	// Call the MCP tool
	result, err := op.mcpClient.CallTool(toolCall.Name, *argsMap)
	isError := err != nil
	var output string

	if err != nil {
		output = fmt.Sprintf("Error: %v", err)
	} else {
		// Concatenate all text and image URLs into a single string output
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
		output = mergedText.String()
	}

	toolResult := anthropic.NewToolResultBlock(toolCall.ID, output, isError)
	return anthropic.NewUserMessage(toolResult), nil
}

func (op *OpenProcessor) AnthropicSwitchAgentToolCall(toolCall anthropic.ToolUseBlockParam, argsMap *map[string]interface{}) (anthropic.MessageParam, error) {
	response, err := switchAgentToolCallImpl(argsMap)
	isError := err != nil && !IsSwitchAgentError(err)
	if err != nil && !IsSwitchAgentError(err) {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolResult := anthropic.NewToolResultBlock(toolCall.ID, response, isError)
	toolMessage := anthropic.NewUserMessage(toolResult)

	if err != nil {
		if IsSwitchAgentError(err) {
			return toolMessage, err
		}
		return toolMessage, err
	}

	return toolMessage, nil
}
