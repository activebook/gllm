package service

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"

	"github.com/activebook/gllm/util"
	"google.golang.org/genai"
)

func (ag *Agent) getGeminiFilePart(file *FileData) genai.Part {

	mimeType := file.Format()
	data := file.Data()
	// Create appropriate part based on file type
	switch {
	case IsImageMIMEType(mimeType):
		// Handle image files
		return genai.Part{InlineData: &genai.Blob{Data: data, MIMEType: mimeType}}
	case IsPDFMIMEType(mimeType):
		// Handle PDF files.
		return genai.Part{InlineData: &genai.Blob{Data: data, MIMEType: mimeType}}
	case IsExcelMIMEType(mimeType):
		// Handle Excel files.
		return genai.Part{InlineData: &genai.Blob{Data: data, MIMEType: mimeType}}
	case IsAudioMIMEType(mimeType):
		// Handle audio files.
		return genai.Part{InlineData: &genai.Blob{Data: data, MIMEType: mimeType}}
	case IsVideoMIMEType(mimeType):
		// Handle video files.
		return genai.Part{InlineData: &genai.Blob{Data: data, MIMEType: mimeType}}
	case IsTextMIMEType(mimeType):
		// Handle plain text files.
		return genai.Part{Text: string(data)}
	default:
		// Default to plain text for other file types.
		text := string(data)
		if len(text) > 0 {
			return genai.Part{Text: text}
		} else {
			return genai.Part{}
		}
	}
}

type Gemini struct {
	// With *Agent embedded, Gemini Agent automatically has access to all of Agent's fields and methods.
	// *Agent // Embedded pointer to Agent
	client *genai.Client
	op     *OpenProcessor
}

func (ag *Agent) initGeminiAgent(ctx context.Context) (*Gemini, error) {
	// Setup the Gemini client
	// In multi-turn session, even though we create it each time
	// it can still be cached for advanced gemini models such as 2.5 flash/pro
	// so it's a server side job
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  ag.Model.ApiKey,
		Backend: genai.BackendGeminiAPI,
		HTTPOptions: genai.HTTPOptions{
			// BaseURL specifies the base URL for the API endpoint. If empty, defaults
			// to "https://generativelanguage.googleapis.com/"
			// APIVersion specifies the version of the API to use. If empty, defaults to "v1beta"
			BaseURL: ag.Model.EndPoint,
		},
	})
	if err != nil {
		return nil, err
	}
	return &Gemini{
		client: client,
	}, nil
}

func (ag *Agent) SortGeminiMessagesByOrder() error {
	// Load previous messages if any
	err := ag.Session.Load()
	if err != nil {
		return err
	}

	messages, _ := ag.Session.GetMessages().([]*genai.Content)

	var parts []*genai.Part

	if ag.UserPrompt != "" {
		parts = append(parts, &genai.Part{Text: ag.UserPrompt})
	}
	for _, file := range ag.Files {
		// Check if the file data is empty
		if file != nil {
			// Convert the file data to a blob
			part := ag.getGeminiFilePart(file)
			if part.Text != "" || part.InlineData != nil {
				parts = append(parts, &part)
			}
		}
	}

	if len(parts) > 0 {
		// Construct Input Content from streamParts
		content := &genai.Content{
			Role:  genai.RoleUser,
			Parts: parts,
		}
		messages = append(messages, content)
	}

	// Save messages to session
	ag.Session.SetMessages(messages)
	// Bugfix: save session after update messages
	// Although system message wouldn't needed, but it's better to save it for consistency
	return ag.Session.Save()
}

// GenerateGeminiSync generates a single, non-streaming completion using the Gemini API.
// This is used for background tasks like context compression where streaming is unnecessary.
// systemPrompt is the system prompt to be used for the sync generation, it's majorly a role.
// the last message is the user prompt to do the task.
func (ag *Agent) GenerateGeminiSync(messages []*genai.Content, systemPrompt string) (string, error) {
	ctx := context.Background()
	ga, err := ag.initGeminiAgent(ctx)
	if err != nil {
		return "", err
	}

	config := &genai.GenerateContentConfig{
		Temperature: &ag.Model.Temperature,
		TopP:        &ag.Model.TopP,
	}
	if ag.Model.Seed != nil {
		config.Seed = ag.Model.Seed
	}
	if systemPrompt != "" {
		config.SystemInstruction = &genai.Content{Parts: []*genai.Part{{Text: systemPrompt}}}
	}

	resp, err := ga.client.Models.GenerateContent(ctx, ag.Model.Model, messages, config)
	if err != nil {
		return "", fmt.Errorf("sync chat completion error: %w", err)
	}

	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return "", fmt.Errorf("no candidates returned in sync response")
	}

	var textContent string
	for _, part := range resp.Candidates[0].Content.Parts {
		if part.Text != "" {
			textContent += part.Text
		}
	}

	return textContent, nil
}

func (ag *Agent) GenerateGeminiStream() error {
	var err error
	// Check the setup of Gemini client
	ctx := context.Background()
	ga, err := ag.initGeminiAgent(ctx)
	if err != nil {
		return err
	}

	// Initialize sub-agent executor if SharedState is available
	var executor *SubAgentExecutor
	if ag.SharedState != nil {
		executor = NewSubAgentExecutor(ag.SharedState, ag.Session.GetTopSessionName())
		defer executor.Shutdown()
	}

	op := OpenProcessor{
		ctx:        ctx,
		notify:     ag.NotifyChan,
		data:       ag.DataChan,
		proceed:    ag.ProceedChan,
		search:     ag.SearchEngine,
		toolsUse:   &ag.ToolsUse,
		queries:    make([]string, 0),
		references: make([]map[string]interface{}, 0),
		status:     &ag.Status,
		mcpClient:  ag.MCPClient,
		fileHooks:  NewFileHooks(),
		// Sub-agent orchestration
		sharedState: ag.SharedState,
		executor:    executor,
		agentName:   ag.AgentName,
	}
	ga.op = &op

	// Configure Model Parameters
	thinking := ag.ThinkingLevel.ToGeminiConfig(ag.Model.Model)

	// Create the model and generate content
	config := genai.GenerateContentConfig{
		Temperature:    &ag.Model.Temperature,
		TopP:           &ag.Model.TopP,
		ThinkingConfig: thinking,
		Tools:          []*genai.Tool{
			// Placeholder
			//{CodeExecution: &genai.ToolCodeExecution{}},
			//{GoogleSearch: &genai.GoogleSearch{}},
		},
	}

	// Add seed if provided
	if ag.Model.Seed != nil {
		config.Seed = ag.Model.Seed
	}
	// System Instruction (System Prompt)
	if ag.SystemPrompt != "" {
		config.SystemInstruction = &genai.Content{Parts: []*genai.Part{{Text: ag.SystemPrompt}}}
	}

	var tool *genai.Tool
	if len(ag.EnabledTools) > 0 {
		// get tools(functions)
		tool = ag.getGeminiTools()
	}
	if ag.MCPClient != nil {
		// Append MCP tools(functions) to the existing tools
		if mcpTool := getGeminiMCPTools(ag.MCPClient); mcpTool != nil {
			tool = appendGeminiTool(tool, mcpTool)
		}
	}
	// Add function tools to config
	if tool != nil {
		config.Tools = append(config.Tools, tool)
	}
	if ag.SearchEngine.UseSearch {
		// Add web search tool to config
		tool = ga.getGeminiWebSearchTool()
		config.Tools = append(config.Tools, tool)
	}
	// // Incompatible tools yet
	// if ag.UseCodeTool {
	// 	// Remember: CodeExecution and GoogleSearch cannot be enabled at the same time
	// 	config.Tools = append(config.Tools, ga.getGeminiCodeExecTool())
	// }

	// Check whether to show warning message
	if len(config.Tools) > 1 {
		// Function tools and Google Search cannot be enabled simultaneously.
		// Function call is not compatible with Google Search tool
		// Only keep the first one
		config.Tools = config.Tools[:1]
		ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusStarted}, ag.ProceedChan)
		ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusWarn,
			Data: fmt.Sprint("Function tools are not compatible with Google Search tool, so Google Search is unavailable now.\n" +
				"Please disable tools if you want to use Google Search.")}, nil)
	}

	// Prepare the Messages for Chat Completion
	err = ag.SortGeminiMessagesByOrder()
	if err != nil {
		return fmt.Errorf("error sorting messages: %v", err)
	}

	// Signal that streaming has started
	// Wait for the main goroutine to tell sub-goroutine to proceed
	ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusStarted}, ag.ProceedChan)

	// Process the chat with recursive tool call handling
	err = ga.process(ag, &config)
	if err != nil {
		// Switch agent signal
		if IsSwitchAgentError(err) {
			return err
		}
		// User cancel signal
		if IsUserCancelError(err) {
			return err
		}
		return fmt.Errorf("error processing chat: %v", err)
	}
	return nil
}

func (ga *Gemini) process(ag *Agent, config *genai.GenerateContentConfig) error {
	// Stream the responses
	references := make([]map[string]interface{}, 0, 1)
	queries := make([]string, 0, 1)

	// Use maxRecursions from LangLogic
	maxRecursions := ag.MaxRecursions
	for i := 0; i < maxRecursions; i++ {
		ga.op.status.ChangeTo(ga.op.notify, StreamNotify{Status: StatusProcessing}, ga.op.proceed)

		// Create a chat session - this is the important part
		// Get all history messages - MUST be inside loop to pick up newly pushed messages
		messages, _ := ag.Session.GetMessages().([]*genai.Content)

		// Context Management
		// Directly truncate on the messages
		util.Debugf("Context messages: [%d]\n", len(messages))
		// check context limit and prune if needed
		pruned, truncated, err := ag.Context.PruneMessages(messages, ag.SystemPrompt, config.Tools)
		if err != nil {
			return fmt.Errorf("failed to prune context: %w", err)
		}
		messages = pruned.([]*genai.Content)

		if truncated {
			util.Warnf("Context limit reached: oldest messages removed or summarized (%s). Consider using /compress or summarizing manually.\n", ag.Context.GetStrategy())
			util.Debugf("Context messages after truncation: [%d]\n", len(messages))
			ag.Session.SetMessages(messages)
			if err := ag.Session.Save(); err != nil {
				return fmt.Errorf("failed to save truncated session: %w", err)
			}
		}

		// Stream the response
		stream := ga.client.Models.GenerateContentStream(ga.op.ctx, ag.Model.Model, messages, config)
		// Wait for the main goroutine to tell sub-goroutine to proceed
		ga.op.status.ChangeTo(ga.op.notify, StreamNotify{Status: StatusStarted}, ga.op.proceed)

		// Process the stream and collect tool calls
		modelContent, resp, err := ga.processStream(stream, &references, &queries)
		if err != nil {
			return err
		}

		// Update History
		err = ga.saveToSession(ag, modelContent)
		if err != nil {
			return err
		}

		// Record token usage
		ag.addUpGeminiTokenUsage(resp)

		// Check for function calls in the model content
		funcCalls := []*genai.FunctionCall{}
		for _, part := range modelContent.Parts {
			if part.FunctionCall != nil {
				funcCalls = append(funcCalls, part.FunctionCall)
			}
		}

		// No further calls
		if len(funcCalls) == 0 {
			break
		}

		for _, funcCall := range funcCalls {
			// Handle tool call
			funcResp, err := ga.processToolCall(funcCall)
			if err != nil {
				// Switch agent signal, pop up
				if IsSwitchAgentError(err) {
					// Add the response part to satisfy history integrity
					ga.saveToSession(ag, funcResp)
					return err
				}
				if IsUserCancelError(err) {
					// Add the response part to satisfy history integrity
					ga.saveToSession(ag, funcResp)
					return err
				}
				ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusWarn, Data: fmt.Sprintf("Failed to process tool call: %v", err)}, nil)
			}
			// Bugfix: Even error happened, we still need to send the function response back through the chat session
			// Send function response back through the chat session
			err = ga.saveToSession(ag, funcResp)
			if err != nil {
				return err
			}
		}
	}

	// Add queries to the output if any
	if len(queries) > 0 {
		q := "\n\n" + ag.SearchEngine.RetrieveQueries(queries)
		ag.DataChan <- StreamData{Text: q, Type: DataTypeNormal}
	}

	// Add references to the output if any
	if len(references) > 0 {
		refs := "\n\n" + ag.SearchEngine.RetrieveReferences(references)
		ag.DataChan <- StreamData{Text: refs, Type: DataTypeNormal}
	}

	// Flush all data to the channel
	ag.DataChan <- StreamData{Type: DataTypeFinished}
	<-ag.ProceedChan
	// Signal that streaming is finished
	ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusFinished}, nil)
	return nil
}

// saveToSession processes the session save
// We need to save the session after each message is sent to the client
// Because model supports interleaved tool calls and responses, aka ReAct
// If error happened or user cancelled, in order to maintain session integrity, we need to save the session
// So that we can resume the session from the last saved state
func (ga *Gemini) saveToSession(ag *Agent, content *genai.Content) error {
	// Save the session history(curated)
	err := ag.Session.Push(content)
	if err != nil {
		return fmt.Errorf("failed to save session: %v", err)
	}
	return nil
}

func (ga *Gemini) processStream(stream iter.Seq2[*genai.GenerateContentResponse, error],
	refs *[]map[string]interface{},
	queries *[]string) (*genai.Content, *genai.GenerateContentResponse, error) {

	modelContent := &genai.Content{
		Role:  genai.RoleModel,
		Parts: []*genai.Part{},
	}
	var finalResp *genai.GenerateContentResponse

	for resp, err := range stream {
		if err != nil {
			return nil, nil, err
		}

		// Process and send content
		for _, candidate := range resp.Candidates {
			if candidate.Content == nil {
				continue // Skip candidates with nil content, bugfix: panic on nil content
			}
			// Process content
			for _, part := range candidate.Content.Parts {
				// Accumulate parts for history ONLY if they have content
				// The API returns Error 400 if we send back a part with no initialized 'data' field
				if part.Text != "" || part.FunctionCall != nil || part.FunctionResponse != nil ||
					part.InlineData != nil || part.FileData != nil ||
					part.ExecutableCode != nil || part.CodeExecutionResult != nil {

					// Check for unknown tools BEFORE saving to history to prevent "orphan" tool calls (calls with no response)
					if part.FunctionCall != nil {
						funcName := part.FunctionCall.Name
						if funcName != "" && !IsAvailableOpenTool(funcName) && !IsAvailableMCPTool(funcName, ga.op.mcpClient) {
							// Skip unknown tools so they don't pollute history and cause 400 errors (Missing function response)
							util.Warnf("Skipping tool call with unknown function name: %s\n", funcName)
							continue
						}
					}

					modelContent.Parts = append(modelContent.Parts, part)
				}

				// Record function call, but don't process here
				if part.FunctionCall != nil {
					continue
				}

				// State transitions
				switch ga.op.status.Peek() {
				case StatusReasoning:
					if !part.Thought {
						ga.op.status.ChangeTo(ga.op.notify, StreamNotify{Status: StatusReasoningOver}, ga.op.proceed)
					}
				default:
					if part.Thought {
						ga.op.status.ChangeTo(ga.op.notify, StreamNotify{Status: StatusReasoning}, ga.op.proceed)
					}
				}

				// Actual text data (don't trim text, because we need to keep the spaces between them)
				if part.Thought && part.Text != "" {
					// Reasoning data
					ga.op.data <- StreamData{Text: (part.Text), Type: DataTypeReasoning}
				} else if part.Text != "" {
					// Normal text data
					ga.op.data <- StreamData{Text: (part.Text), Type: DataTypeNormal}
				}
			}

			// Add references to the output if any
			if candidate.GroundingMetadata != nil {
				appendReferences(candidate.GroundingMetadata, refs)
				*queries = append(*queries, candidate.GroundingMetadata.WebSearchQueries...)
			}
		}

		// If we have a final response, save it
		// It has usage metadata
		finalResp = resp
	}
	return modelContent, finalResp, nil
}

func (ga *Gemini) processToolCall(call *genai.FunctionCall) (*genai.Content, error) {

	var filteredArgs map[string]interface{}
	if call.Name == ToolEditFile || call.Name == ToolWriteFile || call.Name == ToolAskUser {
		// Don't show content(the modified content could be too long)
		filteredArgs = FilterOpenToolArguments(call.Args, []string{"content", "edits", "options", "question_type"})
	} else {
		filteredArgs = FilterOpenToolArguments(call.Args, []string{})
	}

	// Call function
	// Create structured data for the UI
	toolCallData := map[string]interface{}{
		"function": call.Name,
		"args":     filteredArgs,
	}
	jsonData, _ := json.Marshal(toolCallData)
	ga.op.status.ChangeTo(ga.op.notify, StreamNotify{Status: StatusFunctionCalling, Data: string(jsonData)}, ga.op.proceed)

	var resp *genai.FunctionResponse
	var err error
	// Dispatch tool call - call.Args is map[string]any which is identical to map[string]interface{}
	resp, err = ga.op.dispatchGeminiToolCall(call, &call.Args)

	// Function response only has one part
	respPart := genai.Part{FunctionResponse: resp}
	respContent := &genai.Content{
		Role:  genai.RoleUser, // In Gemini, tool responses are sent as 'user' role
		Parts: []*genai.Part{&respPart},
	}

	// Function call is done
	ga.op.status.ChangeTo(ga.op.notify, StreamNotify{Status: StatusFunctionCallingOver}, ga.op.proceed)
	return respContent, err
}

// In an agentic workflow with multi-turn interactions:
// Each turn involves streaming responses from the LLM
// Each response may contain tool calls that trigger additional processing
// New responses are generated based on tool call results
// Each of these interactions consumes tokens that should be tracked
func (ag *Agent) addUpGeminiTokenUsage(resp *genai.GenerateContentResponse) {
	if resp != nil && resp.UsageMetadata != nil && ag.TokenUsage != nil {
		// For gemini model, cache read tokens are not included in the usage metadata
		// The total number of tokens for the entire request. This is the sum of `prompt_token_count`,
		// `candidates_token_count`, `tool_use_prompt_token_count`, and `thoughts_token_count`.
		ag.TokenUsage.CachedTokensInPrompt = true
		ag.TokenUsage.RecordTokenUsage(int(resp.UsageMetadata.PromptTokenCount),
			int(resp.UsageMetadata.CandidatesTokenCount),
			int(resp.UsageMetadata.CachedContentTokenCount),
			int(resp.UsageMetadata.ThoughtsTokenCount),
			int(resp.UsageMetadata.TotalTokenCount))
	}
}

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
func (ag *Agent) getGeminiTools() *genai.Tool {
	// Get filtered tools based on agent's enabled tools list
	openTools := GetOpenToolsFiltered(ag.EnabledTools)
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

func (ga *Gemini) getGeminiWebSearchTool() *genai.Tool {
	// return google embedding search tool
	tool := &genai.Tool{GoogleSearch: &genai.GoogleSearch{}}
	return tool
}

func (ga *Gemini) getGeminiCodeExecTool() *genai.Tool {
	// return google embedding search tool
	tool := &genai.Tool{CodeExecution: &genai.ToolCodeExecution{}}
	return tool
}

func appendReferences(metadata *genai.GroundingMetadata, refs *[]map[string]interface{}) {
	// Process grounding metadata to extract references
	// Check if we have grounding chunks
	if len(metadata.GroundingChunks) > 0 {
		// Build a single map with a "results" key as expected by RetrieveReferences
		results := make([]any, 0, len(metadata.GroundingChunks))
		for _, chunk := range metadata.GroundingChunks {
			// Check if the web chunk exists before accessing its fields
			if chunk.Web != nil {
				// Use URI as the displayLink since that's what users typically see
				results = append(results, map[string]any{
					"title":       chunk.Web.Title,
					"link":        chunk.Web.URI,
					"displayLink": chunk.Web.Title,
				})
			}
		}

		// Only append if we have valid results
		if len(results) > 0 {
			// Track references
			*refs = append(*refs, map[string]any{"results": results})
		}
	}
}
