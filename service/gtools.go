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
func (ga *Gemini2Agent) getGemini2EmbeddingTools(includeMCP bool) *genai.Tool {
	// Get filtered tools based on agent's enabled tools list
	openTools := GetOpenEmbeddingToolsFiltered(ga.EnabledTools)
	var funcs []*genai.FunctionDeclaration

	// Track registered tool names to prevent duplicates
	registeredNames := make(map[string]bool)

	for _, openTool := range openTools {
		geminiTool := openTool.ToGeminiFunctions()
		funcs = append(funcs, geminiTool)
		registeredNames[geminiTool.Name] = true
	}

	// Add MCP tools if requested and client is available
	if includeMCP && ga.MCPClient != nil {
		mcpTools := getMCPTools(ga.MCPClient)
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

func (ga *Gemini2Agent) getGemini2MCPTools() *genai.Tool {
	if ga.MCPClient == nil {
		return nil
	}
	mcpTools := getMCPTools(ga.MCPClient)
	var funcs []*genai.FunctionDeclaration

	for _, mcpTool := range mcpTools {
		geminiTool := mcpTool.ToGeminiFunctions()
		funcs = append(funcs, geminiTool)
	}

	return &genai.Tool{
		FunctionDeclarations: funcs,
	}
}

func (ga *Gemini2Agent) getGemini2WebSearchTool() *genai.Tool {
	// return google embedding search tool
	tool := &genai.Tool{GoogleSearch: &genai.GoogleSearch{}}
	return tool
}

func (ga *Gemini2Agent) getGemini2CodeExecTool() *genai.Tool {
	// return google embedding search tool
	tool := &genai.Tool{CodeExecution: &genai.ToolCodeExecution{}}
	return tool
}

// Diff confirm func
func (ga *Gemini2Agent) gemini2ShowDiffConfirm(diff string) {
	// Function call is over
	ga.Status.ChangeTo(ga.NotifyChan, StreamNotify{Status: StatusFunctionCallingOver}, ga.ProceedChan)

	// Show the diff confirm
	ga.Status.ChangeTo(ga.NotifyChan, StreamNotify{Data: diff, Status: StatusDiffConfirm}, ga.ProceedChan)
}

// Diff close func
func (ga *Gemini2Agent) gemini2CloseDiffConfirm() {
	// Confirm diff is over
	ga.Status.ChangeTo(ga.NotifyChan, StreamNotify{Status: StatusDiffConfirmOver}, ga.ProceedChan)
}

// Tool implementation functions for Gemini 2

func (ga *Gemini2Agent) Gemini2ReadFileToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
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

func (ga *Gemini2Agent) Gemini2WriteFileToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
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
	response, err := writeFileToolCallImpl(&argsMap, &ga.ToolsUse, ga.gemini2ShowDiffConfirm, ga.gemini2CloseDiffConfirm)
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

func (ga *Gemini2Agent) Gemini2CreateDirectoryToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
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

func (ga *Gemini2Agent) Gemini2ListDirectoryToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
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

func (ga *Gemini2Agent) Gemini2DeleteFileToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
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

func (ga *Gemini2Agent) Gemini2DeleteDirectoryToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
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

func (ga *Gemini2Agent) Gemini2MCPToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
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

func (ga *Gemini2Agent) Gemini2MoveToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
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

func (ga *Gemini2Agent) Gemini2SearchFilesToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
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

func (ga *Gemini2Agent) Gemini2SearchTextInFileToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
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

func (ga *Gemini2Agent) Gemini2ReadMultipleFilesToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
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

func (ga *Gemini2Agent) Gemini2ShellToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
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

func (ga *Gemini2Agent) Gemini2WebFetchToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
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

func (ga *Gemini2Agent) Gemini2EditFileToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
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
	response, err := editFileToolCallImpl(&argsMap, &ga.ToolsUse, ga.gemini2ShowDiffConfirm, ga.gemini2CloseDiffConfirm)
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

func (ga *Gemini2Agent) Gemini2CopyToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
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

func (ga *Gemini2Agent) Gemini2ListMemoryToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
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

func (ga *Gemini2Agent) Gemini2SaveMemoryToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
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

func (ga *Gemini2Agent) Gemini2SwitchAgentToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
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

func (ga *Gemini2Agent) Gemini2ListAgentToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
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

func (ga *Gemini2Agent) Gemini2CallAgentToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
	resp := genai.FunctionResponse{
		ID:   call.ID,
		Name: call.Name,
	}

	// Convert args
	argsMap := make(map[string]interface{})
	for k, v := range call.Args {
		argsMap[k] = v
	}

	response, err := callAgentToolCallImpl(&argsMap, ga.executor)
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

func (ga *Gemini2Agent) Gemini2GetStateToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
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

func (ga *Gemini2Agent) Gemini2SetStateToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
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

func (ga *Gemini2Agent) Gemini2ListStateToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {
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
