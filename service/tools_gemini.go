package service

import (
	"fmt"

	"google.golang.org/genai"
)

/*
A limitation of Gemini is that you can't use a function call and a built-in tool at the same time. ADK,
when using Gemini as the underlying LLM, takes advantage of Gemini's built-in ability to do Google searches,
and uses function calling to invoke your custom ADK tools.
So agent tools can come in handy, as you can have a main agent,
that delegates live searches to a search agent that has the GoogleSearchTool configured,
and another tool agent that makes use of a custom tool function.

Usually, this happens when you get a mysterious error like this one
(reported against ADK for Python):
{'error': {'code': 400, 'message': 'Tool use with function calling is unsupported',
 'status': 'INVALID_ARGUMENT'}}.
This means that you can't use a built-in tool and function calling at the same time in the same agent.
*/

// Tool definitions for Gemini
func (ga *GeminiAgent) getGeminiTools() *genai.Tool {
	// Get filtered tools based on agent's enabled tools list
	openTools := GetOpenToolsFiltered(ga.EnabledTools)
	var funcs []*genai.FunctionDeclaration

	for _, openTool := range openTools {
		geminiTool := openTool.ToGeminiFunctions()
		funcs = append(funcs, geminiTool)
	}

	// The Gemini API expects all function declarations to be grouped together under a single Tool object.
	return &genai.Tool{
		FunctionDeclarations: funcs,
	}
}

func (ga *GeminiAgent) getGeminiWebSearchTool() *genai.Tool {
	// return google embedding search tool
	tool := &genai.Tool{GoogleSearch: &genai.GoogleSearch{}}
	return tool
}

func (ga *GeminiAgent) getGeminiCodeExecTool() *genai.Tool {
	// return google embedding search tool
	tool := &genai.Tool{CodeExecution: &genai.ToolCodeExecution{}}
	return tool
}

// Diff confirm func
func (ga *GeminiAgent) geminiShowDiffConfirm(diff string) {
	// Function call is over
	ga.Status.ChangeTo(ga.NotifyChan, StreamNotify{Status: StatusFunctionCallingOver}, ga.ProceedChan)

	// Show the diff confirm
	ga.Status.ChangeTo(ga.NotifyChan, StreamNotify{Data: diff, Status: StatusDiffConfirm}, ga.ProceedChan)
}

// Diff close func
func (ga *GeminiAgent) geminiCloseDiffConfirm() {
	// Confirm diff is over
	ga.Status.ChangeTo(ga.NotifyChan, StreamNotify{Status: StatusDiffConfirmOver}, ga.ProceedChan)
}

func (ga *GeminiAgent) geminiMCPToolCall(call *genai.FunctionCall, a *map[string]interface{}) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}
	if ga.MCPClient == nil {
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
	result, err := ga.MCPClient.CallTool(call.Name, *a)
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

func (ga *GeminiAgent) geminiSwitchAgentToolCall(call *genai.FunctionCall, a *map[string]interface{}) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	// Call shared implementation
	response, err := switchAgentToolCallImpl(a, &ga.ToolsUse)
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
func (ga *GeminiAgent) runGeminiTool(call *genai.FunctionCall, fn ToolFunc) (*genai.FunctionResponse, error) {
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
func (ga *GeminiAgent) dispatchGeminiToolCall(call *genai.FunctionCall, a *map[string]interface{}) (*genai.FunctionResponse, error) {
	switch call.Name {
	case ToolShell:
		return ga.runGeminiTool(call, func() (string, error) { return shellToolCallImpl(a, &ga.ToolsUse) })
	case ToolReadFile:
		return ga.runGeminiTool(call, func() (string, error) { return readFileToolCallImpl(a) })
	case ToolWriteFile:
		return ga.runGeminiTool(call, func() (string, error) {
			return writeFileToolCallImpl(a, &ga.ToolsUse, ga.geminiShowDiffConfirm, ga.geminiCloseDiffConfirm)
		})
	case ToolCreateDirectory:
		return ga.runGeminiTool(call, func() (string, error) { return createDirectoryToolCallImpl(a, &ga.ToolsUse) })
	case ToolListDirectory:
		return ga.runGeminiTool(call, func() (string, error) { return listDirectoryToolCallImpl(a) })
	case ToolDeleteFile:
		return ga.runGeminiTool(call, func() (string, error) { return deleteFileToolCallImpl(a, &ga.ToolsUse) })
	case ToolDeleteDirectory:
		return ga.runGeminiTool(call, func() (string, error) { return deleteDirectoryToolCallImpl(a, &ga.ToolsUse) })
	case ToolMove:
		return ga.runGeminiTool(call, func() (string, error) { return moveToolCallImpl(a, &ga.ToolsUse) })
	case ToolCopy:
		return ga.runGeminiTool(call, func() (string, error) { return copyToolCallImpl(a, &ga.ToolsUse) })
	case ToolSearchFiles:
		return ga.runGeminiTool(call, func() (string, error) { return searchFilesToolCallImpl(a) })
	case ToolSearchTextInFile:
		return ga.runGeminiTool(call, func() (string, error) { return searchTextInFileToolCallImpl(a) })
	case ToolReadMultipleFiles:
		return ga.runGeminiTool(call, func() (string, error) { return readMultipleFilesToolCallImpl(a) })
	case ToolWebFetch:
		return ga.runGeminiTool(call, func() (string, error) { return webFetchToolCallImpl(a) })
	case ToolEditFile:
		return ga.runGeminiTool(call, func() (string, error) {
			return editFileToolCallImpl(a, &ga.ToolsUse, ga.geminiShowDiffConfirm, ga.geminiCloseDiffConfirm)
		})
	case ToolListMemory:
		return ga.runGeminiTool(call, func() (string, error) { return listMemoryToolCallImpl() })
	case ToolSaveMemory:
		return ga.runGeminiTool(call, func() (string, error) { return saveMemoryToolCallImpl(a) })
	case ToolListAgent:
		return ga.runGeminiTool(call, func() (string, error) { return listAgentToolCallImpl() })
	case ToolSpawnSubAgents:
		return ga.runGeminiTool(call, func() (string, error) { return spawnSubAgentsToolCallImpl(a, &ga.ToolsUse, ga.executor) })
	case ToolGetState:
		return ga.runGeminiTool(call, func() (string, error) { return getStateToolCallImpl(a, ga.SharedState) })
	case ToolSetState:
		return ga.runGeminiTool(call, func() (string, error) { return setStateToolCallImpl(a, ga.AgentName, ga.SharedState) })
	case ToolListState:
		return ga.runGeminiTool(call, func() (string, error) { return listStateToolCallImpl(ga.SharedState) })
	case ToolActivateSkill:
		return ga.runGeminiTool(call, func() (string, error) { return activateSkillToolCallImpl(a, &ga.ToolsUse) })
	case ToolAskUser:
		return ga.runGeminiTool(call, func() (string, error) { return askUserToolCallImpl(a) })
	case ToolExitPlanMode:
		return ga.runGeminiTool(call, func() (string, error) { return exitPlanModeToolCallImpl(a, &ga.ToolsUse) })
	case ToolSwitchAgent:
		return ga.geminiSwitchAgentToolCall(call, a)
	default:
		if ga.MCPClient != nil && ga.MCPClient.FindTool(call.Name) != nil {
			return ga.geminiMCPToolCall(call, a)
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
		ga.Status.ChangeTo(ga.NotifyChan, StreamNotify{Status: StatusWarn, Data: fmt.Sprintf("Model attempted to call unknown function: %s", call.Name)}, nil)
		return resp, nil
	}
}
