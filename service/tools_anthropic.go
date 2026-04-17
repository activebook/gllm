package service

import (
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

/*
 * Antrhopic tool call implements
 */

func (op *OpenProcessor) anthropicMCPToolCall(toolCall anthropic.ToolUseBlockParam, argsMap *map[string]interface{}) (anthropic.MessageParam, error) {

	if op.mcpClient == nil {
		err := fmt.Errorf("MCP client not initialized")
		errorStr := fmt.Sprintf("Error: MCP tool call failed: %v", err)
		toolResult := anthropic.NewToolResultBlock(toolCall.ID, errorStr, true)
		return anthropic.NewUserMessage(toolResult), err
	}

	// Check permisson on mcp tools
	if err := CheckToolPermission(toolCall.Name, argsMap); err != nil {
		errorStr := fmt.Sprintf("Error: MCP tool call failed: %v", err)
		toolResult := anthropic.NewToolResultBlock(toolCall.ID, errorStr, true)
		return anthropic.NewUserMessage(toolResult), err
	}

	// Call the MCP tool
	result, err := op.mcpClient.CallTool(toolCall.Name, *argsMap)
	if err != nil {
		errorStr := fmt.Sprintf("Error: MCP tool call failed: %v", err)
		toolResult := anthropic.NewToolResultBlock(toolCall.ID, errorStr, true)
		return anthropic.NewUserMessage(toolResult), err
	}

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

	toolResult := anthropic.NewToolResultBlock(toolCall.ID, mergedText.String(), false)
	return anthropic.NewUserMessage(toolResult), nil
}

func (op *OpenProcessor) anthropicSwitchAgentToolCall(toolCall anthropic.ToolUseBlockParam, argsMap *map[string]interface{}) (anthropic.MessageParam, error) {
	response, err := switchAgentToolCallImpl(argsMap, op)

	isError := err != nil && !IsSwitchAgentError(err)

	if err != nil && isError {
		response = fmt.Sprintf("Error: %v", err)
	}

	toolResult := anthropic.NewToolResultBlock(toolCall.ID, response, isError)
	return anthropic.NewUserMessage(toolResult), err
}

// runAnthropicTool runs fn and wraps the result into an Anthropic tool message.
func runAnthropicTool(toolID string, fn ToolFunc) (anthropic.MessageParam, error) {
	response, err := fn()
	isError := err != nil
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}
	toolResult := anthropic.NewToolResultBlock(toolID, response, isError)
	return anthropic.NewUserMessage(toolResult), err
}

// dispatchAnthropicToolCall handles the routing of Anthropic tool calls to the correct implementation.
func (op *OpenProcessor) dispatchAnthropicToolCall(toolCall anthropic.ToolUseBlockParam, a *map[string]interface{}) (anthropic.MessageParam, error) {
	switch toolCall.Name {
	case ToolShell:
		return runAnthropicTool(toolCall.ID, func() (string, error) { return shellToolCallImpl(a, op) })
	case ToolWebFetch:
		return runAnthropicTool(toolCall.ID, func() (string, error) { return webFetchToolCallImpl(a) })
	case ToolWebSearch:
		return runAnthropicTool(toolCall.ID, func() (string, error) { return webSearchToolCallImpl(a, op) })
	case ToolReadFile:
		return runAnthropicTool(toolCall.ID, func() (string, error) { return readFileToolCallImpl(a) })
	case ToolWriteFile:
		return runAnthropicTool(toolCall.ID, func() (string, error) { return writeFileToolCallImpl(a, op) })
	case ToolEditFile:
		return runAnthropicTool(toolCall.ID, func() (string, error) { return editFileToolCallImpl(a, op) })
	case ToolCreateDirectory:
		return runAnthropicTool(toolCall.ID, func() (string, error) { return createDirectoryToolCallImpl(a, op) })
	case ToolListDirectory:
		return runAnthropicTool(toolCall.ID, func() (string, error) { return listDirectoryToolCallImpl(a) })
	case ToolDeleteFile:
		return runAnthropicTool(toolCall.ID, func() (string, error) { return deleteFileToolCallImpl(a, op) })
	case ToolDeleteDirectory:
		return runAnthropicTool(toolCall.ID, func() (string, error) { return deleteDirectoryToolCallImpl(a, op) })
	case ToolMove:
		return runAnthropicTool(toolCall.ID, func() (string, error) { return moveToolCallImpl(a, op) })
	case ToolCopy:
		return runAnthropicTool(toolCall.ID, func() (string, error) { return copyToolCallImpl(a, op) })
	case ToolSearchFiles:
		return runAnthropicTool(toolCall.ID, func() (string, error) { return searchFilesToolCallImpl(a) })
	case ToolSearchTextInFile:
		return runAnthropicTool(toolCall.ID, func() (string, error) { return searchTextInFileToolCallImpl(a) })
	case ToolReadMultipleFiles:
		return runAnthropicTool(toolCall.ID, func() (string, error) { return readMultipleFilesToolCallImpl(a) })
	case ToolListMemory:
		return runAnthropicTool(toolCall.ID, func() (string, error) { return listMemoryToolCallImpl() })
	case ToolSaveMemory:
		return runAnthropicTool(toolCall.ID, func() (string, error) { return saveMemoryToolCallImpl(a) })
	case ToolBuildAgent:
		return runAnthropicTool(toolCall.ID, func() (string, error) { return buildAgentToolCallImpl(a, op) })
	case ToolSwitchAgent:
		return op.anthropicSwitchAgentToolCall(toolCall, a)
	case ToolListAgent:
		return runAnthropicTool(toolCall.ID, func() (string, error) { return listAgentToolCallImpl() })
	case ToolSpawnSubAgents:
		return runAnthropicTool(toolCall.ID, func() (string, error) { return spawnSubAgentsToolCallImpl(a, op) })
	case ToolGetState:
		return runAnthropicTool(toolCall.ID, func() (string, error) { return getStateToolCallImpl(a, op) })
	case ToolSetState:
		return runAnthropicTool(toolCall.ID, func() (string, error) { return setStateToolCallImpl(a, op) })
	case ToolListState:
		return runAnthropicTool(toolCall.ID, func() (string, error) { return listStateToolCallImpl(op) })
	case ToolActivateSkill:
		return runAnthropicTool(toolCall.ID, func() (string, error) { return activateSkillToolCallImpl(a, op) })
	case ToolAskUser:
		return runAnthropicTool(toolCall.ID, func() (string, error) { return askUserToolCallImpl(a, op) })
	case ToolExitPlanMode:
		return runAnthropicTool(toolCall.ID, func() (string, error) { return exitPlanModeToolCallImpl(a, op) })
	case ToolEnterPlanMode:
		return runAnthropicTool(toolCall.ID, func() (string, error) { return enterPlanModeToolCallImpl(a, op) })
	default:
		if op.mcpClient != nil && op.mcpClient.FindTool(toolCall.Name) != nil {
			return op.anthropicMCPToolCall(toolCall, a)
		}
		errorMsg := fmt.Sprintf("Error: Unknown function '%s'. This function is not available. Please use one of the available functions from the tool list.", toolCall.Name)
		toolResult := anthropic.NewToolResultBlock(toolCall.ID, errorMsg, true)
		msg := anthropic.NewUserMessage(toolResult)
		op.status.ChangeTo(op.notify, StreamNotify{Status: StatusWarn, Data: fmt.Sprintf("Model attempted to call unknown function: %s", toolCall.Name)}, nil)
		return msg, nil
	}
}
