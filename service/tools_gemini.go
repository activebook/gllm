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
func (ga *GeminiAgent) GeminiShowDiffConfirm(diff string) {
	// Function call is over
	ga.Status.ChangeTo(ga.NotifyChan, StreamNotify{Status: StatusFunctionCallingOver}, ga.ProceedChan)

	// Show the diff confirm
	ga.Status.ChangeTo(ga.NotifyChan, StreamNotify{Data: diff, Status: StatusDiffConfirm}, ga.ProceedChan)
}

// Diff close func
func (ga *GeminiAgent) GeminiCloseDiffConfirm() {
	// Confirm diff is over
	ga.Status.ChangeTo(ga.NotifyChan, StreamNotify{Status: StatusDiffConfirmOver}, ga.ProceedChan)
}

// Tool implementation functions for Gemini

/**
 * Bug note:
 * When only the error field is set without the output field.
 * The model often stops responding or returns empty responses in this scenario.
 * This appears to be a known problem with the Gemini API where:
 * - The model sometimes returns empty responses with finish_reason=STOP but no actual content
 * - This frequently happens during function calling, especially with error handling
 * - The API doesn't consistently handle cases where only error is set without output
 * Solution:
 * We need to ensure that the output field is always set, even if it's empty.
 * This is done by checking if the output is empty and setting it to the error message if it is.
 */

func (ga *GeminiAgent) GeminiReadFileToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	// Convert genai.FunctionCall.Args to map[string]interface{}
	argsMap := make(map[string]interface{})
	for k, v := range call.Args {
		argsMap[k] = v
	}

	// Call shared implementation
	response, err := readFileToolCallImpl(&argsMap)
	error := ""
	if err != nil {
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

func (ga *GeminiAgent) GeminiWriteFileToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	// Convert genai.FunctionCall.Args to map[string]interface{}
	argsMap := make(map[string]interface{})
	for k, v := range call.Args {
		argsMap[k] = v
	}

	// Call shared implementation
	response, err := writeFileToolCallImpl(&argsMap, &ga.ToolsUse, ga.GeminiShowDiffConfirm, ga.GeminiCloseDiffConfirm)
	error := ""
	if err != nil {
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

func (ga *GeminiAgent) GeminiCreateDirectoryToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	// Convert genai.FunctionCall.Args to map[string]interface{}
	argsMap := make(map[string]interface{})
	for k, v := range call.Args {
		argsMap[k] = v
	}

	// Call shared implementation
	response, err := createDirectoryToolCallImpl(&argsMap)
	error := ""
	if err != nil {
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

func (ga *GeminiAgent) GeminiListDirectoryToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	// Convert genai.FunctionCall.Args to map[string]interface{}
	argsMap := make(map[string]interface{})
	for k, v := range call.Args {
		argsMap[k] = v
	}

	// Call shared implementation
	response, err := listDirectoryToolCallImpl(&argsMap)
	error := ""
	if err != nil {
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

func (ga *GeminiAgent) GeminiDeleteFileToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	// Convert genai.FunctionCall.Args to map[string]interface{}
	argsMap := make(map[string]interface{})
	for k, v := range call.Args {
		argsMap[k] = v
	}

	// Call shared implementation
	response, err := deleteFileToolCallImpl(&argsMap, &ga.ToolsUse)
	error := ""
	if err != nil {
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

func (ga *GeminiAgent) GeminiDeleteDirectoryToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	// Convert genai.FunctionCall.Args to map[string]interface{}
	argsMap := make(map[string]interface{})
	for k, v := range call.Args {
		argsMap[k] = v
	}

	// Call shared implementation
	response, err := deleteDirectoryToolCallImpl(&argsMap, &ga.ToolsUse)
	error := ""
	if err != nil {
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

func (ga *GeminiAgent) GeminiMCPToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
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

	// Convert genai.FunctionCall.Args to map[string]interface{}
	argsMap := make(map[string]interface{})
	for k, v := range call.Args {
		argsMap[k] = v
	}

	// Call the MCP tool
	result, err := ga.MCPClient.CallTool(call.Name, argsMap)
	if err != nil {
		// Wrap error in response
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

func (ga *GeminiAgent) GeminiMoveToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	// Convert genai.FunctionCall.Args to map[string]interface{}
	argsMap := make(map[string]interface{})
	for k, v := range call.Args {
		argsMap[k] = v
	}

	// Call shared implementation
	response, err := moveToolCallImpl(&argsMap, &ga.ToolsUse)
	error := ""
	if err != nil {
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

func (ga *GeminiAgent) GeminiSearchFilesToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	// Convert genai.FunctionCall.Args to map[string]interface{}
	argsMap := make(map[string]interface{})
	for k, v := range call.Args {
		argsMap[k] = v
	}

	// Call shared implementation
	response, err := searchFilesToolCallImpl(&argsMap)
	error := ""
	if err != nil {
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

func (ga *GeminiAgent) GeminiSearchTextInFileToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	// Convert genai.FunctionCall.Args to map[string]interface{}
	argsMap := make(map[string]interface{})
	for k, v := range call.Args {
		argsMap[k] = v
	}

	// Call shared implementation
	response, err := searchTextInFileToolCallImpl(&argsMap)
	error := ""
	if err != nil {
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

func (ga *GeminiAgent) GeminiReadMultipleFilesToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	// Convert genai.FunctionCall.Args to map[string]interface{}
	argsMap := make(map[string]interface{})
	for k, v := range call.Args {
		argsMap[k] = v
	}

	// Call shared implementation
	response, err := readMultipleFilesToolCallImpl(&argsMap)
	error := ""
	if err != nil {
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

func (ga *GeminiAgent) GeminiShellToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	// Convert genai.FunctionCall.Args to map[string]interface{}
	argsMap := make(map[string]interface{})
	for k, v := range call.Args {
		argsMap[k] = v
	}

	// Call shared implementation
	response, err := shellToolCallImpl(&argsMap, &ga.ToolsUse)
	error := ""
	if err != nil {
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

func (ga *GeminiAgent) GeminiWebFetchToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	// Convert genai.FunctionCall.Args to map[string]interface{}
	argsMap := make(map[string]interface{})
	for k, v := range call.Args {
		argsMap[k] = v
	}

	// Call shared implementation
	response, err := webFetchToolCallImpl(&argsMap)
	error := ""
	if err != nil {
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

func (ga *GeminiAgent) GeminiEditFileToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	// Convert genai.FunctionCall.Args to map[string]interface{}
	argsMap := make(map[string]interface{})
	for k, v := range call.Args {
		argsMap[k] = v
	}

	// Call shared implementation
	response, err := editFileToolCallImpl(&argsMap, &ga.ToolsUse, ga.GeminiShowDiffConfirm, ga.GeminiCloseDiffConfirm)
	error := ""
	if err != nil {
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

func (ga *GeminiAgent) GeminiCopyToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	// Convert genai.FunctionCall.Args to map[string]interface{}
	argsMap := make(map[string]interface{})
	for k, v := range call.Args {
		argsMap[k] = v
	}

	// Call shared implementation
	response, err := copyToolCallImpl(&argsMap, &ga.ToolsUse)
	error := ""
	if err != nil {
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

func (ga *GeminiAgent) GeminiListMemoryToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	// Call shared implementation (no args needed)
	response, err := listMemoryToolCallImpl()
	error := ""
	if err != nil {
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

func (ga *GeminiAgent) GeminiSaveMemoryToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	// Convert genai.FunctionCall.Args to map[string]interface{}
	argsMap := make(map[string]interface{})
	for k, v := range call.Args {
		argsMap[k] = v
	}

	// Call shared implementation
	response, err := saveMemoryToolCallImpl(&argsMap)
	error := ""
	if err != nil {
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

func (ga *GeminiAgent) GeminiSwitchAgentToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	// Convert genai.FunctionCall.Args to map[string]interface{}
	argsMap := make(map[string]interface{})
	for k, v := range call.Args {
		argsMap[k] = v
	}

	// Call shared implementation
	response, err := switchAgentToolCallImpl(&argsMap, &ga.ToolsUse)
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

func (ga *GeminiAgent) GeminiListAgentToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	response, err := listAgentToolCallImpl()
	error := ""
	if err != nil {
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

func (ga *GeminiAgent) GeminiSpawnSubAgentsToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	// Convert args
	argsMap := make(map[string]interface{})
	for k, v := range call.Args {
		argsMap[k] = v
	}

	response, err := spawnSubAgentsToolCallImpl(&argsMap, &ga.ToolsUse, ga.executor)
	error := ""
	if err != nil {
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

func (ga *GeminiAgent) GeminiGetStateToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	argsMap := make(map[string]interface{})
	for k, v := range call.Args {
		argsMap[k] = v
	}

	response, err := getStateToolCallImpl(&argsMap, ga.SharedState)
	error := ""
	if err != nil {
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

func (ga *GeminiAgent) GeminiSetStateToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	argsMap := make(map[string]interface{})
	for k, v := range call.Args {
		argsMap[k] = v
	}

	response, err := setStateToolCallImpl(&argsMap, ga.AgentName, ga.SharedState)
	error := ""
	if err != nil {
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

func (ga *GeminiAgent) GeminiListStateToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	response, err := listStateToolCallImpl(ga.SharedState)
	error := ""
	if err != nil {
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

func (ga *GeminiAgent) GeminiActivateSkillToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	argsMap := make(map[string]interface{})
	for k, v := range call.Args {
		argsMap[k] = v
	}

	response, err := activateSkillToolCallImpl(&argsMap, &ga.ToolsUse)
	error := ""
	if err != nil {
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
