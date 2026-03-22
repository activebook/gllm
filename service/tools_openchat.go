package service

import (
	"fmt"
	"strings"

	"github.com/activebook/gllm/util"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
)

/*
 * OpenChat tool call implements
 *
 */

func Ptr[T any](t T) *T { return &t }

func (op *OpenProcessor) openChatSwitchAgentToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
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

func (op *OpenProcessor) openChatMCPToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
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

	// Check permisson on mcp tools
	if err := CheckToolPermission(toolCall.Function.Name, argsMap); err != nil {
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

	util.Debugf("OpenChatMCPToolCall Response: %s\n", *toolMessage.Content.StringValue)
	return &toolMessage, nil
}

// runOpenChatTool runs fn and wraps the result into an OpenChat tool message.
func runOpenChatTool(tc *model.ToolCall, fn ToolFunc) (*model.ChatCompletionMessage, error) {
	response, err := fn()
	if err != nil {
		response = fmt.Sprintf("Error: %v", err)
	}
	return &model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: tc.ID,
		Name:       Ptr(""), // Required by Volcengine SDK
		Content: &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(response),
		},
	}, err
}

// dispatchOpenChatToolCall handles the routing of OpenChat tool calls to the correct implementation.
func (op *OpenProcessor) dispatchOpenChatToolCall(toolCall *model.ToolCall, a *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	switch toolCall.Function.Name {
	case ToolShell:
		return runOpenChatTool(toolCall, func() (string, error) { return shellToolCallImpl(a, op.toolsUse) })
	case ToolWebFetch:
		return runOpenChatTool(toolCall, func() (string, error) { return webFetchToolCallImpl(a) })
	case ToolWebSearch:
		return runOpenChatTool(toolCall, func() (string, error) { return webSearchToolCallImpl(a, &op.queries, &op.references, op.search) })
	case ToolReadFile:
		return runOpenChatTool(toolCall, func() (string, error) { return readFileToolCallImpl(a) })
	case ToolWriteFile:
		return runOpenChatTool(toolCall, func() (string, error) {
			return writeFileToolCallImpl(a, op.toolsUse, op.showDiffConfirm, op.closeDiffConfirm)
		})
	case ToolEditFile:
		return runOpenChatTool(toolCall, func() (string, error) {
			return editFileToolCallImpl(a, op.toolsUse, op.showDiffConfirm, op.closeDiffConfirm)
		})
	case ToolCreateDirectory:
		return runOpenChatTool(toolCall, func() (string, error) { return createDirectoryToolCallImpl(a, op.toolsUse) })
	case ToolListDirectory:
		return runOpenChatTool(toolCall, func() (string, error) { return listDirectoryToolCallImpl(a) })
	case ToolDeleteFile:
		return runOpenChatTool(toolCall, func() (string, error) { return deleteFileToolCallImpl(a, op.toolsUse) })
	case ToolDeleteDirectory:
		return runOpenChatTool(toolCall, func() (string, error) { return deleteDirectoryToolCallImpl(a, op.toolsUse) })
	case ToolMove:
		return runOpenChatTool(toolCall, func() (string, error) { return moveToolCallImpl(a, op.toolsUse) })
	case ToolCopy:
		return runOpenChatTool(toolCall, func() (string, error) { return copyToolCallImpl(a, op.toolsUse) })
	case ToolSearchFiles:
		return runOpenChatTool(toolCall, func() (string, error) { return searchFilesToolCallImpl(a) })
	case ToolSearchTextInFile:
		return runOpenChatTool(toolCall, func() (string, error) { return searchTextInFileToolCallImpl(a) })
	case ToolReadMultipleFiles:
		return runOpenChatTool(toolCall, func() (string, error) { return readMultipleFilesToolCallImpl(a) })
	case ToolListMemory:
		return runOpenChatTool(toolCall, func() (string, error) { return listMemoryToolCallImpl() })
	case ToolSaveMemory:
		return runOpenChatTool(toolCall, func() (string, error) { return saveMemoryToolCallImpl(a) })
	case ToolListAgent:
		return runOpenChatTool(toolCall, func() (string, error) { return listAgentToolCallImpl() })
	case ToolSpawnSubAgents:
		return runOpenChatTool(toolCall, func() (string, error) { return spawnSubAgentsToolCallImpl(a, op.toolsUse, op.executor) })
	case ToolGetState:
		return runOpenChatTool(toolCall, func() (string, error) { return getStateToolCallImpl(a, op.sharedState) })
	case ToolSetState:
		return runOpenChatTool(toolCall, func() (string, error) { return setStateToolCallImpl(a, op.agentName, op.sharedState) })
	case ToolListState:
		return runOpenChatTool(toolCall, func() (string, error) { return listStateToolCallImpl(op.sharedState) })
	case ToolActivateSkill:
		return runOpenChatTool(toolCall, func() (string, error) { return activateSkillToolCallImpl(a, op.toolsUse) })
	case ToolAskUser:
		return runOpenChatTool(toolCall, func() (string, error) { return askUserToolCallImpl(a) })
	case ToolExitPlanMode:
		return runOpenChatTool(toolCall, func() (string, error) { return exitPlanModeToolCallImpl(a, op.toolsUse) })
	case ToolEnterPlanMode:
		return runOpenChatTool(toolCall, func() (string, error) { return enterPlanModeToolCallImpl(a, op.toolsUse) })
	case ToolBuildAgent:
		return runOpenChatTool(toolCall, func() (string, error) { return buildAgentToolCallImpl(a, op.toolsUse) })
	case ToolSwitchAgent:
		return op.openChatSwitchAgentToolCall(toolCall, a)
	default:
		if op.mcpClient != nil && op.mcpClient.FindTool(toolCall.Function.Name) != nil {
			return op.openChatMCPToolCall(toolCall, a)
		}
		errorMsg := fmt.Sprintf("Error: Unknown function '%s'. This function is not available. Please use one of the available functions from the tool list.", toolCall.Function.Name)
		msg := &model.ChatCompletionMessage{
			Role:       "tool",
			ToolCallID: toolCall.ID,
			Content:    &model.ChatCompletionMessageContent{StringValue: volcengine.String(errorMsg)},
		}
		op.status.ChangeTo(op.notify, StreamNotify{Status: StatusWarn, Data: fmt.Sprintf("Model attempted to call unknown function: %s", toolCall.Function.Name)}, nil)
		return msg, nil
	}
}
