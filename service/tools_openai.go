package service

import (
	"fmt"

	openai "github.com/sashabaranov/go-openai"
)

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

func (op *OpenProcessor) OpenAISpawnSubAgentsToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	response, err := spawnSubAgentsToolCallImpl(argsMap, op.toolsUse, op.executor)
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
