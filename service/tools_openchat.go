package service

import (
	"fmt"
	"strings"

	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
)

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

func (op *OpenProcessor) OpenChatSpawnSubAgentsToolCall(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	response, err := spawnSubAgentsToolCallImpl(argsMap, op.toolsUse, op.executor)
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
