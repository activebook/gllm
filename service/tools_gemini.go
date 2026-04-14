package service

import (
	"fmt"

	"google.golang.org/genai"
)

func (op *OpenProcessor) geminiMCPToolCall(call *genai.FunctionCall, a *map[string]interface{}) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}
	if op.mcpClient == nil {
		err := fmt.Errorf("MCP client not initialized")
		error := fmt.Sprintf("Error: MCP tool call failed: %v", err)
		resp.Response = map[string]any{
			"output": error,
			"error":  error,
		}
		return &resp, err
	}

	// Check permisson on mcp tools
	if err := CheckToolPermission(call.Name, a); err != nil {
		error := fmt.Sprintf("Error: MCP tool call failed: %v", err)
		resp.Response = map[string]any{
			"output": error,
			"error":  error,
		}
		return &resp, err
	}

	// Call the MCP tool
	result, err := op.mcpClient.CallTool(call.Name, *a)
	if err != nil {
		error := fmt.Sprintf("Error: MCP tool call failed: %v", err)
		resp.Response = map[string]any{
			"output": error,
			"error":  error,
		}
		return &resp, err
	}

	// Convert to markdown string output for Gemini
	output := ""
	for i, content := range result.Contents {
		switch result.Types[i] {
		case MCPResponseText:
			output += content + "\n"
		case MCPResponseImage:
			output += fmt.Sprintf("![Image](%s)\n", content)
		case MCPResponseAudio:
			output += fmt.Sprintf("![Audio](%s)\n", content)
		default:
			// Unknown file type, skip
		}
	}

	resp.Response = map[string]any{
		"output": output,
		"error":  "",
	}
	return &resp, nil
}

func (op *OpenProcessor) geminiSwitchAgentToolCall(call *genai.FunctionCall, a *map[string]interface{}) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	// Call shared implementation
	response, err := switchAgentToolCallImpl(a, op)
	error := ""
	if err != nil {
		if IsSwitchAgentError(err) {
			resp.Response = map[string]any{"output": err.Error(), "error": err.Error()}
			return &resp, err
		}
		error = fmt.Sprintf("Error: %v", err)
		if response == "" {
			response = error
		}
	}

	resp.Response = map[string]any{
		"output": response,
		"error":  error,
	}
	return &resp, err
}

// runGeminiTool wraps a (string, error) result into a genai.FunctionResponse.
// IMPORTANT: When only the error field is set with an empty output, the Gemini API
// model often hangs or returns empty responses. We always ensure output is set.
func runGeminiTool(call *genai.FunctionCall, fn ToolFunc) (*genai.FunctionResponse, error) {
	response, err := fn()
	errStr := ""
	if err != nil {
		errStr = fmt.Sprintf("Error: %v", err)
		if response == "" {
			response = errStr // Gemini-specific: output MUST be non-empty
		}
	}
	return &genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
		Response: map[string]any{
			"output": response,
			"error":  errStr,
		},
	}, err
}

// dispatchGeminiToolCall handles the routing of Gemini tool calls to the correct implementation.
func (op *OpenProcessor) dispatchGeminiToolCall(call *genai.FunctionCall, a *map[string]interface{}) (*genai.FunctionResponse, error) {
	switch call.Name {
	case ToolShell:
		return runGeminiTool(call, func() (string, error) { return shellToolCallImpl(a, op) })
	case ToolReadFile:
		return runGeminiTool(call, func() (string, error) { return readFileToolCallImpl(a) })
	case ToolWriteFile:
		return runGeminiTool(call, func() (string, error) { return writeFileToolCallImpl(a, op) })
	case ToolCreateDirectory:
		return runGeminiTool(call, func() (string, error) { return createDirectoryToolCallImpl(a, op) })
	case ToolListDirectory:
		return runGeminiTool(call, func() (string, error) { return listDirectoryToolCallImpl(a) })
	case ToolDeleteFile:
		return runGeminiTool(call, func() (string, error) { return deleteFileToolCallImpl(a, op) })
	case ToolDeleteDirectory:
		return runGeminiTool(call, func() (string, error) { return deleteDirectoryToolCallImpl(a, op) })
	case ToolMove:
		return runGeminiTool(call, func() (string, error) { return moveToolCallImpl(a, op) })
	case ToolCopy:
		return runGeminiTool(call, func() (string, error) { return copyToolCallImpl(a, op) })
	case ToolSearchFiles:
		return runGeminiTool(call, func() (string, error) { return searchFilesToolCallImpl(a) })
	case ToolSearchTextInFile:
		return runGeminiTool(call, func() (string, error) { return searchTextInFileToolCallImpl(a) })
	case ToolReadMultipleFiles:
		return runGeminiTool(call, func() (string, error) { return readMultipleFilesToolCallImpl(a) })
	case ToolWebFetch:
		return runGeminiTool(call, func() (string, error) { return webFetchToolCallImpl(a) })
	case ToolEditFile:
		return runGeminiTool(call, func() (string, error) { return editFileToolCallImpl(a, op) })
	case ToolListMemory:
		return runGeminiTool(call, func() (string, error) { return listMemoryToolCallImpl() })
	case ToolSaveMemory:
		return runGeminiTool(call, func() (string, error) { return saveMemoryToolCallImpl(a) })
	case ToolListAgent:
		return runGeminiTool(call, func() (string, error) { return listAgentToolCallImpl() })
	case ToolSpawnSubAgents:
		return runGeminiTool(call, func() (string, error) { return spawnSubAgentsToolCallImpl(a, op) })
	case ToolGetState:
		return runGeminiTool(call, func() (string, error) { return getStateToolCallImpl(a, op) })
	case ToolSetState:
		return runGeminiTool(call, func() (string, error) { return setStateToolCallImpl(a, op) })
	case ToolListState:
		return runGeminiTool(call, func() (string, error) { return listStateToolCallImpl(op) })
	case ToolActivateSkill:
		return runGeminiTool(call, func() (string, error) { return activateSkillToolCallImpl(a, op) })
	case ToolAskUser:
		return runGeminiTool(call, func() (string, error) { return askUserToolCallImpl(a, op) })
	case ToolExitPlanMode:
		return runGeminiTool(call, func() (string, error) { return exitPlanModeToolCallImpl(a, op) })
	case ToolEnterPlanMode:
		return runGeminiTool(call, func() (string, error) { return enterPlanModeToolCallImpl(a, op) })
	case ToolBuildAgent:
		return runGeminiTool(call, func() (string, error) { return buildAgentToolCallImpl(a, op) })
	case ToolSwitchAgent:
		return op.geminiSwitchAgentToolCall(call, a)
	default:
		if op.mcpClient != nil && op.mcpClient.FindTool(call.Name) != nil {
			return op.geminiMCPToolCall(call, a)
		}
		// Unknown function
		resp := &genai.FunctionResponse{
			ID:   call.ID,
			Name: call.Name,
			Response: map[string]any{
				"content": nil,
				"error":   fmt.Sprintf("Error: Unknown function '%s'. This function is not available. Please use one of the available functions from the tool list.", call.Name),
			},
		}
		op.status.ChangeTo(op.notify, StreamNotify{Status: StatusWarn, Data: fmt.Sprintf("Model attempted to call unknown function: %s", call.Name)}, nil)
		return resp, nil
	}
}
