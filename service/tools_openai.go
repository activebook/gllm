package service

import (
	"fmt"

	openai "github.com/sashabaranov/go-openai"
)

/*
 * OpenAI tool call implements
 */

// MCPToolCall has it's own logic
func (op *OpenProcessor) openAIMCPToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	if op.mcpClient == nil {
		err := fmt.Errorf("MCP client not initialized")
		return openai.ChatCompletionMessage{
			Role:       openai.ChatMessageRoleTool,
			ToolCallID: toolCall.ID,
			Content:    fmt.Sprintf("Error: MCP tool call failed: %v", err),
		}, err
	}

	// Check permisson on mcp tools
	if err := CheckToolPermission(toolCall.Function.Name, argsMap); err != nil {
		return openai.ChatCompletionMessage{
			Role:       openai.ChatMessageRoleTool,
			ToolCallID: toolCall.ID,
			Content:    fmt.Sprintf("Error: MCP tool call failed: %v", err),
		}, err
	}

	// Call the MCP tool
	result, err := op.mcpClient.CallTool(toolCall.Function.Name, *argsMap)
	if err != nil {
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

// Switch agent tool call is special, it need to deal with IsSwitchAgentError
func (op *OpenProcessor) openAISwitchAgentToolCall(toolCall openai.ToolCall, argsMap *map[string]interface{}) (openai.ChatCompletionMessage, error) {
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

// runOpenAITool runs fn and wraps the (string, error) result into an OpenAI tool message.
func runOpenAITool(tc openai.ToolCall, fn ToolFunc) (openai.ChatCompletionMessage, error) {
	response, err := fn()
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}
	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		ToolCallID: tc.ID,
		Content:    response,
	}, err
}

// dispatchOpenAIToolCall handles the routing of OpenAI tool calls to the correct implementation.
func (op *OpenProcessor) dispatchOpenAIToolCall(toolCall openai.ToolCall, a *map[string]interface{}) (openai.ChatCompletionMessage, error) {
	switch toolCall.Function.Name {
	case ToolShell:
		return runOpenAITool(toolCall, func() (string, error) { return shellToolCallImpl(a, op.toolsUse) })
	case ToolWebFetch:
		return runOpenAITool(toolCall, func() (string, error) { return webFetchToolCallImpl(a) })
	case ToolWebSearch:
		return runOpenAITool(toolCall, func() (string, error) {
			return webSearchToolCallImpl(a, &op.queries, &op.references, op.search)
		})
	case ToolReadFile:
		return runOpenAITool(toolCall, func() (string, error) { return readFileToolCallImpl(a) })
	case ToolWriteFile:
		return runOpenAITool(toolCall, func() (string, error) {
			return writeFileToolCallImpl(a, op.toolsUse, op.showDiffConfirm, op.closeDiffConfirm)
		})
	case ToolEditFile:
		return runOpenAITool(toolCall, func() (string, error) {
			return editFileToolCallImpl(a, op.toolsUse, op.showDiffConfirm, op.closeDiffConfirm)
		})
	case ToolCreateDirectory:
		return runOpenAITool(toolCall, func() (string, error) { return createDirectoryToolCallImpl(a, op.toolsUse) })
	case ToolListDirectory:
		return runOpenAITool(toolCall, func() (string, error) { return listDirectoryToolCallImpl(a) })
	case ToolDeleteFile:
		return runOpenAITool(toolCall, func() (string, error) { return deleteFileToolCallImpl(a, op.toolsUse) })
	case ToolDeleteDirectory:
		return runOpenAITool(toolCall, func() (string, error) { return deleteDirectoryToolCallImpl(a, op.toolsUse) })
	case ToolMove:
		return runOpenAITool(toolCall, func() (string, error) { return moveToolCallImpl(a, op.toolsUse) })
	case ToolCopy:
		return runOpenAITool(toolCall, func() (string, error) { return copyToolCallImpl(a, op.toolsUse) })
	case ToolSearchFiles:
		return runOpenAITool(toolCall, func() (string, error) { return searchFilesToolCallImpl(a) })
	case ToolSearchTextInFile:
		return runOpenAITool(toolCall, func() (string, error) { return searchTextInFileToolCallImpl(a) })
	case ToolReadMultipleFiles:
		return runOpenAITool(toolCall, func() (string, error) { return readMultipleFilesToolCallImpl(a) })
	case ToolListMemory:
		return runOpenAITool(toolCall, func() (string, error) { return listMemoryToolCallImpl() })
	case ToolSaveMemory:
		return runOpenAITool(toolCall, func() (string, error) { return saveMemoryToolCallImpl(a) })
	case ToolListAgent:
		return runOpenAITool(toolCall, func() (string, error) { return listAgentToolCallImpl() })
	case ToolSpawnSubAgents:
		return runOpenAITool(toolCall, func() (string, error) { return spawnSubAgentsToolCallImpl(a, op.toolsUse, op.executor) })
	case ToolGetState:
		return runOpenAITool(toolCall, func() (string, error) { return getStateToolCallImpl(a, op.sharedState) })
	case ToolSetState:
		return runOpenAITool(toolCall, func() (string, error) { return setStateToolCallImpl(a, op.agentName, op.sharedState) })
	case ToolListState:
		return runOpenAITool(toolCall, func() (string, error) { return listStateToolCallImpl(op.sharedState) })
	case ToolActivateSkill:
		return runOpenAITool(toolCall, func() (string, error) { return activateSkillToolCallImpl(a, op.toolsUse) })
	case ToolAskUser:
		return runOpenAITool(toolCall, func() (string, error) { return askUserToolCallImpl(a) })
	case ToolExitPlanMode:
		return runOpenAITool(toolCall, func() (string, error) { return exitPlanModeToolCallImpl(a, op.toolsUse) })
	case ToolEnterPlanMode:
		return runOpenAITool(toolCall, func() (string, error) { return enterPlanModeToolCallImpl(a, op.toolsUse) })
	case ToolSwitchAgent:
		return op.openAISwitchAgentToolCall(toolCall, a)
	default:
		if op.mcpClient != nil && op.mcpClient.FindTool(toolCall.Function.Name) != nil {
			return op.openAIMCPToolCall(toolCall, a)
		}
		// Unknown function fallback
		errorMsg := fmt.Sprintf("Error: Unknown function '%s'. This function is not available. Please use one of the available functions from the tool list.", toolCall.Function.Name)
		msg := openai.ChatCompletionMessage{
			Role:       openai.ChatMessageRoleTool,
			Content:    errorMsg,
			ToolCallID: toolCall.ID,
		}
		op.status.ChangeTo(op.notify, StreamNotify{Status: StatusWarn, Data: fmt.Sprintf("Model attempted to call unknown function: %s", toolCall.Function.Name)}, nil)
		return msg, nil
	}
}
