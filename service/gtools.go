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

// Tool definitions for Gemini 2
func (ag *Agent) getGemini2EmbeddingTools(includeMCP bool) *genai.Tool {
	openTools := getOpenEmbeddingTools()
	var funcs []*genai.FunctionDeclaration

	// Track registered tool names to prevent duplicates
	registeredNames := make(map[string]bool)

	for _, openTool := range openTools {
		geminiTool := openTool.ToGeminiFunctions()
		funcs = append(funcs, geminiTool)
		registeredNames[geminiTool.Name] = true
	}

	// Add MCP tools if requested and client is available
	if includeMCP && ag.MCPClient != nil {
		mcpTools := getMCPTools(ag.MCPClient)
		for _, mcpTool := range mcpTools {
			geminiTool := mcpTool.ToGeminiFunctions()
			// Skip MCP tools that have the same name as built-in tools to avoid Gemini duplicate function declaration error
			if registeredNames[geminiTool.Name] {
				continue
			}
			funcs = append(funcs, geminiTool)
			registeredNames[geminiTool.Name] = true
		}
	}

	// The Gemini API expects all function declarations to be grouped together under a single Tool object.
	return &genai.Tool{
		FunctionDeclarations: funcs,
	}
}

func (ag *Agent) getGemini2MCPTools() *genai.Tool {
	if ag.MCPClient == nil {
		return nil
	}
	mcpTools := getMCPTools(ag.MCPClient)
	var funcs []*genai.FunctionDeclaration

	for _, mcpTool := range mcpTools {
		geminiTool := mcpTool.ToGeminiFunctions()
		funcs = append(funcs, geminiTool)
	}

	return &genai.Tool{
		FunctionDeclarations: funcs,
	}
}

func (ag *Agent) getGemini2WebSearchTool() *genai.Tool {
	// return google embedding search tool
	tool := &genai.Tool{GoogleSearch: &genai.GoogleSearch{}}
	return tool
}

func (ag *Agent) getGemini2CodeExecTool() *genai.Tool {
	// return google embedding search tool
	tool := &genai.Tool{CodeExecution: &genai.ToolCodeExecution{}}
	return tool
}

// Tool implementation functions for Gemini 2

func (ag *Agent) Gemini2ReadFileToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
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
	if err != nil {
		return nil, err
	}

	resp.Response = map[string]any{
		"output": response,
		"error":  "",
	}
	return &resp, nil
}

func (ag *Agent) Gemini2WriteFileToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
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
	response, err := writeFileToolCallImpl(&argsMap, &ag.ToolsUse, ag.gemini2ShowDiffConfirm, ag.gemini2CloseDiffConfirm)
	if err != nil {
		return nil, err
	}

	resp.Response = map[string]any{
		"output": response,
		"error":  "",
	}
	return &resp, nil
}

func (ag *Agent) Gemini2CreateDirectoryToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
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
	if err != nil {
		return nil, err
	}

	resp.Response = map[string]any{
		"output": response,
		"error":  "",
	}
	return &resp, nil
}

func (ag *Agent) Gemini2ListDirectoryToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
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
	if err != nil {
		return nil, err
	}

	resp.Response = map[string]any{
		"output": response,
		"error":  "",
	}
	return &resp, nil
}

func (ag *Agent) Gemini2DeleteFileToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
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
	response, err := deleteFileToolCallImpl(&argsMap, &ag.ToolsUse)
	if err != nil {
		return nil, err
	}

	resp.Response = map[string]any{
		"output": response,
		"error":  "",
	}
	return &resp, nil
}

func (ag *Agent) Gemini2DeleteDirectoryToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
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
	response, err := deleteDirectoryToolCallImpl(&argsMap, &ag.ToolsUse)
	if err != nil {
		return nil, err
	}

	resp.Response = map[string]any{
		"output": response,
		"error":  "",
	}
	return &resp, nil
}

func (ag *Agent) Gemini2MCPToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	if ag.MCPClient == nil {
		return nil, fmt.Errorf("MCP client not initialized")
	}

	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	// Convert genai.FunctionCall.Args to map[string]interface{}
	argsMap := make(map[string]interface{})
	for k, v := range call.Args {
		argsMap[k] = v
	}

	// Call the MCP tool
	result, err := ag.MCPClient.CallTool(call.Name, argsMap)
	if err != nil {
		return nil, fmt.Errorf("MCP tool call failed: %v", err)
	}

	// Right now, gemini2 only support string response
	// It cannot support other types(image/audio, etc.)
	// So even though it can get base64 encoded data and MIME type
	// It cannot recognize it.
	resp.Response = map[string]any{
		"output": result,
		"error":  "",
	}
	return &resp, nil
}

func (ag *Agent) Gemini2MoveToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
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
	response, err := moveToolCallImpl(&argsMap, &ag.ToolsUse)
	if err != nil {
		return nil, err
	}

	resp.Response = map[string]any{
		"output": response,
		"error":  "",
	}
	return &resp, nil
}

func (ag *Agent) Gemini2SearchFilesToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
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
	if err != nil {
		return nil, err
	}

	resp.Response = map[string]any{
		"output": response,
		"error":  "",
	}
	return &resp, nil
}

func (ag *Agent) Gemini2SearchTextInFileToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
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
	if err != nil {
		return nil, err
	}

	resp.Response = map[string]any{
		"output": response,
		"error":  "",
	}
	return &resp, nil
}

func (ag *Agent) Gemini2ReadMultipleFilesToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
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
	if err != nil {
		return nil, err
	}

	resp.Response = map[string]any{
		"output": response,
		"error":  "",
	}
	return &resp, nil
}

func (ag *Agent) Gemini2ShellToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
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
	response, err := shellToolCallImpl(&argsMap, &ag.ToolsUse)
	if err != nil {
		return nil, err
	}

	resp.Response = map[string]any{
		"output": response,
		"error":  "",
	}
	return &resp, nil
}

func (ag *Agent) Gemini2WebFetchToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
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
	if err != nil {
		return nil, err
	}

	resp.Response = map[string]any{
		"output": response,
		"error":  "",
	}
	return &resp, nil
}

// func (ag *Agent) Gemini2EditFileToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
// 	resp := genai.FunctionResponse{
// 		ID:   call.ID,
// 		Name: call.Name,
// 	}

// 	// Convert genai.FunctionCall.Args to map[string]interface{}
// 	argsMap := make(map[string]interface{})
// 	for k, v := range call.Args {
// 		argsMap[k] = v
// 	}

// 	// Call shared implementation
// 	response, err := editFileToolCallImpl(&argsMap, &ag.ToolsUse)
// 	if err != nil {
// 		return nil, err
// 	}

// 	resp.Response = map[string]any{
// 		"output": response,
// 		"error":  "",
// 	}
// 	return &resp, nil
// }

func (ag *Agent) Gemini2EditFileToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
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
	response, err := editFileToolCallImpl(&argsMap, &ag.ToolsUse, ag.gemini2ShowDiffConfirm, ag.gemini2CloseDiffConfirm)
	if err != nil {
		return nil, err
	}

	resp.Response = map[string]any{
		"output": response,
		"error":  "",
	}
	return &resp, nil
}

func (ag *Agent) Gemini2CopyToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
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
	response, err := copyToolCallImpl(&argsMap, &ag.ToolsUse)
	if err != nil {
		return nil, err
	}

	resp.Response = map[string]any{
		"output": response,
		"error":  "",
	}
	return &resp, nil
}

// Diff confirm func
func (ag *Agent) gemini2ShowDiffConfirm(diff string) {
	// Function call is over
	ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusFunctionCallingOver}, ag.ProceedChan)

	// Show the diff confirm
	ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Data: diff, Status: StatusDiffConfirm}, ag.ProceedChan)
}

// Diff close func
func (ag *Agent) gemini2CloseDiffConfirm() {
	// Confirm diff is over
	ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusDiffConfirmOver}, ag.ProceedChan)
}

func (ag *Agent) Gemini2ListMemoryToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	// Call shared implementation (no args needed)
	response, err := listMemoryToolCallImpl()
	if err != nil {
		return nil, err
	}

	resp.Response = map[string]any{
		"output": response,
		"error":  "",
	}
	return &resp, nil
}

func (ag *Agent) Gemini2SaveMemoryToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
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
	if err != nil {
		return nil, err
	}

	resp.Response = map[string]any{
		"output": response,
		"error":  "",
	}
	return &resp, nil
}
