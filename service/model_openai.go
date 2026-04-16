package service

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/activebook/gllm/util"
	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/packages/ssestream"
)

// extractDeltaReasoning extracts vendor-specific reasoning/thinking content from a
// streaming delta's ExtraFields. Providers like DeepSeek R1 use "reasoning_content";
// others (e.g. OpenRouter) use "reasoning".
//
// Key SDK mechanics:
//   - Extra/unknown JSON fields are captured in delta.JSON.ExtraFields.
//   - These fields are present but NOT Valid() — they sit outside the typed schema.
//   - Raw() returns the raw JSON value (e.g. `"some text"` or `null` or “ for omitted).
//   - We previously strings.Trim to unquote rather than json.Unmarshal, matching the streaming
//     incremental nature of the data, but encountered escaped \n and unicode concerns in practice.
func (oa *OpenAI) extractDeltaReasoning(delta *openai.ChatCompletionChunkChoiceDelta) string {
	if delta == nil {
		return ""
	}

	// Fast path: use cached key if found previously in this stream
	if oa.reasoningKey != "" {
		field, ok := delta.JSON.ExtraFields[oa.reasoningKey]
		if ok && !field.Valid() {
			raw := field.Raw()
			if raw != "" && raw != "null" {
				var unescaped string
				if err := json.Unmarshal([]byte(raw), &unescaped); err == nil {
					return unescaped
				}
			}
		}
		// If cached key failed for some reason, we give up (unlikely)
		return ""
	}

	for _, key := range []string{"reasoning_content", "reasoning"} {
		field, ok := delta.JSON.ExtraFields[key]
		if !ok || field.Valid() {
			continue // field absent, or it is a typed/valid field (shouldn't happen but guard anyway)
		}
		raw := field.Raw()
		if raw == "" || raw == "null" {
			continue // omitted or explicitly null — no reasoning this chunk
		}
		// Correctly unescape JSON string (handles \n, \t, etc)
		var unescaped string
		if err := json.Unmarshal([]byte(raw), &unescaped); err == nil {
			oa.reasoningKey = key // Cache the key for future chunks in this stream
			return unescaped
		}
		// Fallback if it fails
		continue
	}
	return ""
}

func (ag *Agent) getOpenAIFilePart(file *FileData) (openai.ChatCompletionContentPartUnionParam, bool) {

	var part openai.ChatCompletionContentPartUnionParam
	format := file.Format()
	// Handle based on file type
	if IsImageMIMEType(format) {
		// Create base64 image URL
		imageURL := util.BuildDataURL(file.Format(), file.Data())
		// Create and append image part
		part = openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{URL: imageURL})
		return part, true
	} else if IsAudioMIMEType(format) {
		audioFmt := "mp3"
		if strings.Contains(format, "wav") {
			audioFmt = "wav"
		}
		part = openai.InputAudioContentPart(openai.ChatCompletionContentPartInputAudioInputAudioParam{
			Data:   util.GetBase64String(file.Data()),
			Format: audioFmt,
		})
		return part, true
	} else if IsPDFMIMEType(format) {
		filename := "document.pdf"
		if file.Path() != "" {
			filename = filepath.Base(file.Path())
		}
		part = openai.FileContentPart(openai.ChatCompletionContentPartFileFileParam{
			FileData: openai.String(util.GetBase64String(file.Data())),
			Filename: openai.String(filename),
		})
		return part, true
	} else if IsTextMIMEType(format) {
		// Create and append text part
		part = openai.TextContentPart(string(file.Data()))
		return part, true
	} else {
		// Unknown file type, skip
		// Don't deal with xls
		// It needs upload to OpenAI's servers first, so we can't include them directly in a message.
		return part, false
	}
}

/*
 * Sort the messages by order:
 *   History (user/assistant/tool turns only — no system message)
 *   The system prompt is never persisted; it is injected fresh in process().
 */
func (ag *Agent) SortOpenAIMessagesByOrder() error {
	// Load previous messages if any
	err := ag.Session.Load()
	if err != nil {
		return err
	}

	// Get current history (pure dialogue, no system messages)
	history, _ := ag.Session.GetMessages().([]openai.ChatCompletionMessageParamUnion)

	// Add File parts if available
	if len(ag.Files) > 0 {
		var parts []openai.ChatCompletionContentPartUnionParam
		if ag.UserPrompt != "" {
			parts = append(parts, openai.TextContentPart(ag.UserPrompt))
		}
		// Add all files
		for _, file := range ag.Files {
			if file != nil {
				part, ok := ag.getOpenAIFilePart(file)
				if ok {
					parts = append(parts, part)
				}
			}
		}
		if len(parts) > 0 {
			history = append(history, openai.UserMessage(parts))
		}
	} else {
		// For text only models, add user prompt directly
		if ag.UserPrompt != "" {
			history = append(history, openai.UserMessage(ag.UserPrompt))
		}
	}
	// Add to history only if it's not empty
	ag.Session.SetMessages(history)
	// Bugfix: save session after update messages
	// Because the system message could be modified, and added user message
	return ag.Session.Save()
}

// prependOpenAISystemMessage builds the final API messages slice by prepending
// a fresh system message in-memory. Returns history unchanged when prompt is empty.
func prependOpenAISystemMessage(systemPrompt string, history []openai.ChatCompletionMessageParamUnion) []openai.ChatCompletionMessageParamUnion {
	if systemPrompt == "" {
		return history
	}
	sysMsg := openai.SystemMessage(systemPrompt)
	messages := make([]openai.ChatCompletionMessageParamUnion, 0, 1+len(history))
	messages = append(messages, sysMsg)
	messages = append(messages, history...)
	return messages
}

// GenerateOpenAISync generates a single, non-streaming completion using OpenAI API.
// This is used for background tasks like context compression where streaming is unnecessary.
// systemPrompt is the system prompt to be used for the sync generation, it's majorly a role.
// the last message is the user prompt to do the task.
func (ag *Agent) GenerateOpenAISync(messages []openai.ChatCompletionMessageParamUnion, systemPrompt string) (string, error) {
	if ag.Ctx == nil {
		ag.Ctx = context.Background()
	}
	opts := []option.RequestOption{option.WithAPIKey(ag.Model.ApiKey)}
	if ag.Model.EndPoint != "" {
		opts = append(opts, option.WithBaseURL(ag.Model.EndPoint))
	}
	client := openai.NewClient(opts...)

	// Add system prompt
	messages = append([]openai.ChatCompletionMessageParamUnion{openai.SystemMessage(systemPrompt)}, messages...)

	req := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(ag.Model.Model),
		Messages: messages,
	}
	if ag.Model.Temperature != 0 {
		req.Temperature = openai.Float(float64(ag.Model.Temperature))
	}
	if ag.Model.TopP != 0 {
		req.TopP = openai.Float(float64(ag.Model.TopP))
	}

	if ag.Model.Seed != nil {
		req.Seed = openai.Int(int64(*ag.Model.Seed))
	}

	resp, err := client.Chat.Completions.New(ag.Ctx, req)
	if err != nil {
		return "", fmt.Errorf("sync chat completion error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no choices returned in sync response")
	}

	return resp.Choices[0].Message.Content, nil
}

// GenerateOpenAIStream generates a streaming response using OpenAI API
func (ag *Agent) GenerateOpenAIStream() error {
	// Initialize the Client
	// Create a client config with custom base URL
	clientOpts := []option.RequestOption{
		option.WithAPIKey(ag.Model.ApiKey),
	}
	if ag.Model.EndPoint != "" {
		clientOpts = append(clientOpts, option.WithBaseURL(ag.Model.EndPoint))
	}
	client := openai.NewClient(clientOpts...)

	// Create tools
	tools := []openai.ChatCompletionToolUnionParam{}
	if len(ag.EnabledTools) > 0 {
		// Add tools
		tools = ag.getOpenAITools()
	}
	if ag.MCPClient != nil {
		// Add MCP tools if MCP client is available
		mcpTools := ag.getOpenAIMCPTools()
		tools = append(tools, mcpTools...)
	}

	// Initialize sub-agent executor if SharedState is available
	var executor *SubAgentExecutor
	if ag.SharedState != nil {
		executor = NewSubAgentExecutor(ag.SharedState, ag.Session.GetTopSessionName(), ag.StdOutput, ag.FileOutput, ag.SSEOutput)
		defer executor.Shutdown()
	}

	op := OpenProcessor{
		notify:      ag.NotifyChan,
		data:        ag.DataChan,
		proceed:     ag.ProceedChan,
		search:      ag.SearchEngine,
		toolsUse:    &ag.ToolsUse,
		interaction: ag.Interaction,
		quiet:       ag.QuietMode,
		queries:     make([]string, 0),
		references:  make([]map[string]interface{}, 0), // Updated to match new field type
		status:      &ag.Status,
		mcpClient:   ag.MCPClient,
		fileHooks:   NewFileHooks(),
		// Sub-agent orchestration
		sharedState: ag.SharedState,
		executor:    executor,
		agentName:   ag.AgentName,
	}
	chat := &OpenAI{
		client: &client,
		tools:  tools,
		op:     &op,
	}

	// Prepare the Messages for Chat Completion
	err := ag.SortOpenAIMessagesByOrder()
	if err != nil {
		return fmt.Errorf("error sorting messages: %v", err)
	}

	// Signal that streaming has started
	// Wait for the main goroutine to tell sub-goroutine to proceed
	ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusStarted}, ag.ProceedChan)

	// Process the chat with recursive tool call handling
	err = chat.process(ag)
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

// OpenAI manages the state of an ongoing session with an AI assistant
type OpenAI struct {
	client       *openai.Client
	tools        []openai.ChatCompletionToolUnionParam
	op           *OpenProcessor
	reasoningKey string // Cache for the vendor-specific reasoning extra field key
}

func (oa *OpenAI) process(ag *Agent) error {
	// Recursively process the session
	// Because the model can call tools multiple times
	i := 0
	for range ag.MaxRecursions {
		i++
		//Debugf("Processing session at times: %d\n", i)
		oa.op.status.ChangeTo(oa.op.notify, StreamNotify{Status: StatusProcessing}, oa.op.proceed)

		// Get all history messages - MUST be inside loop to pick up newly pushed messages.
		// Session only holds pure dialogue (user/assistant/tool turns).
		messages, _ := ag.Session.GetMessages().([]openai.ChatCompletionMessageParamUnion)

		// Apply context window management.
		util.LogDebugf("Context messages: [%d]\n", len(messages))
		pruned, truncated, err := ag.Context.PruneMessages(messages, ag.SystemPrompt, oa.tools)
		if err != nil {
			return fmt.Errorf("failed to prune context: %w", err)
		}
		messages = pruned.([]openai.ChatCompletionMessageParamUnion)

		if truncated {
			util.LogWarnf("Context limit reached: oldest messages removed or summarized (%s). Consider using /compress or summarizing manually.\n", ag.Context.GetStrategy())
			util.LogDebugf("Context messages after truncation: [%d]\n", len(messages))
			// Save back only non-system messages to keep the session clean.
			ag.Session.SetMessages(messages)
			if err := ag.Session.Save(); err != nil {
				return fmt.Errorf("failed to save truncated session: %w", err)
			}
		}

		// Prepend the fresh system prompt in-memory only — never persisted.
		messages = prependOpenAISystemMessage(ag.SystemPrompt, messages)

		// Create the request
		req := openai.ChatCompletionNewParams{
			Model:       openai.ChatModel(ag.Model.Model),
			Temperature: openai.Float(float64(ag.Model.Temperature)),
			TopP:        openai.Float(float64(ag.Model.TopP)),
			Messages:    messages,
		}

		// Tools
		if len(oa.tools) > 0 {
			req.Tools = oa.tools
		}

		// Add seed if provided
		if ag.Model.Seed != nil {
			req.Seed = openai.Int(int64(*ag.Model.Seed))
		}

		// Add reasoning effort if thinking is enabled
		if effort := ag.ThinkingLevel.ToOpenAIReasoningEffort(); effort != "" {
			req.ReasoningEffort = openai.ReasoningEffort(effort)
		}
		if ag.TokenUsage != nil {
			req.StreamOptions = openai.ChatCompletionStreamOptionsParam{IncludeUsage: openai.Bool(true)}
		}

		// Make the streaming request
		stream := oa.client.Chat.Completions.NewStreaming(ag.Ctx, req)
		// Bug: do NOT use defer here — deferreds accumulate until process() returns,
		// so inside a loop each iteration would stack up an open stream. Close explicitly.
		// defer stream.Close()

		// Wait for the main goroutine to tell sub-goroutine to proceed
		oa.op.status.ChangeTo(oa.op.notify, StreamNotify{Status: StatusStarted}, oa.op.proceed)

		// Process the stream and collect tool calls
		assistantMessage, toolCalls, resp, err := oa.processStream(stream)
		stream.Close() // Bugfix: Close immediately after consuming to release the HTTP connection
		if err != nil {
			return fmt.Errorf("error processing stream: %v", err)
		}

		// Record token usage
		// The final response contains the token usage metainfo
		addUpOpenAITokenUsage(ag, resp)

		// Add the assistant's message to the session
		err = oa.saveToSession(ag, assistantMessage)
		if err != nil {
			return err
		}

		// If there are tool calls, process them
		if len(toolCalls) > 0 {
			// Process each tool call
			for _, toolCall := range toolCalls {
				toolMessage, err := oa.processToolCall(toolCall)
				if err != nil {
					// Switch agent signal, pop up
					if IsSwitchAgentError(err) {
						// Bugfix: left an "orphan" tool_call that had no matching tool result.
						// Add tool message to session to fix this.
						oa.saveToSession(ag, toolMessage)
						return err
					}
					if IsUserCancelError(err) {
						// User cancelled tool call, pop up
						oa.saveToSession(ag, toolMessage)
						return err
					}
					ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusWarn, Data: fmt.Sprintf("Failed to process tool call: %v", err)}, nil)
				}
				// IMPORTANT: Even error happened still add an error response message to maintain session integrity
				// The API requires every tool_call to have a corresponding tool response
				// Add the tool response to the session
				err = oa.saveToSession(ag, toolMessage)
				if err != nil {
					return err
				}
			}
			// Continue the session recursively
		} else {
			// No function call and no model content
			break
		}
	}

	// Add queries to the output if any
	if len(oa.op.queries) > 0 {
		q := "\n\n" + ag.SearchEngine.RetrieveQueries(oa.op.queries)
		oa.op.data <- StreamData{Text: q, Type: DataTypeNormal}
	}
	// Add references to the output if any
	if len(oa.op.references) > 0 {
		refs := "\n\n" + ag.SearchEngine.RetrieveReferences(oa.op.references)
		oa.op.data <- StreamData{Text: refs, Type: DataTypeNormal}
	}

	// Flush all data to the channel
	oa.op.data <- StreamData{Type: DataTypeFinished}
	<-oa.op.proceed
	// Notify that the stream is finished
	oa.op.status.ChangeTo(oa.op.notify, StreamNotify{Status: StatusFinished}, nil)
	return nil
}

// saveToSession processes the session save
// We need to save the session after each message is sent to the client
// Because model supports interleaved tool calls and responses, aka ReAct
// If error happened or user cancelled, in order to maintain session integrity, we need to save the session
// So that we can resume the session from the last saved state
func (oa *OpenAI) saveToSession(ag *Agent, message openai.ChatCompletionMessageParamUnion) error {
	// Add the assistant's message to the session
	err := ag.Session.Push(message)
	if err != nil {
		return fmt.Errorf("failed to save session: %v", err)
	}
	return nil
}

// processStream processes the stream and collects tool calls
func (oa *OpenAI) processStream(stream *ssestream.Stream[openai.ChatCompletionChunk]) (openai.ChatCompletionMessageParamUnion, []openai.ChatCompletionMessageToolCallUnionParam, *openai.ChatCompletionChunk, error) {
	var assistantMessage openai.ChatCompletionAssistantMessageParam

	type ToolCallBuilder struct {
		ID        string
		Name      string
		Arguments string
	}
	toolCallsMap := make(map[string]*ToolCallBuilder)
	var orderedToolCallIDs []string

	contentBuffer := strings.Builder{}
	reasoningBuffer := strings.Builder{}
	lastCallId := ""
	var finalResp *openai.ChatCompletionChunk

	for stream.Next() {
		response := stream.Current()
		// Get the final response
		finalResp = &response

		// Handle regular content
		if len(response.Choices) > 0 {
			delta := response.Choices[0].Delta

			// (The official SDK param struct has no reasoning_content field.)
			// Extract vendor reasoning content (DeepSeek R1 → "reasoning_content",
			// OpenRouter/others → "reasoning") from ExtraFields.
			if rcText := oa.extractDeltaReasoning(&delta); rcText != "" {
				reasoningBuffer.WriteString(rcText)
				if oa.op.status.Peek() != StatusReasoning {
					// If reasoning content is not empty, switch to reasoning state
					oa.op.status.ChangeTo(oa.op.notify, StreamNotify{Status: StatusReasoning}, oa.op.proceed)
				}
				oa.op.data <- StreamData{Text: rcText, Type: DataTypeReasoning}
			}

			if delta.Content != "" {
				text := delta.Content
				contentBuffer.WriteString(text)
				if oa.op.status.Peek() == StatusReasoning {
					// If regular content arrives while we're reasoning, transition away
					oa.op.status.ChangeTo(oa.op.notify, StreamNotify{Status: StatusReasoningOver}, oa.op.proceed)
				}
				oa.op.data <- StreamData{Text: text, Type: DataTypeNormal}
			}

			// Handle tool calls in the stream
			if len(delta.ToolCalls) > 0 {
				for _, toolCall := range delta.ToolCalls {
					id := toolCall.ID
					functionName := toolCall.Function.Name

					// Skip if not our expected function
					// Because some model made up function name
					if functionName != "" && !IsAvailableOpenTool(functionName) && !IsAvailableMCPTool(functionName, oa.op.mcpClient) {
						util.LogWarnf("Skipping tool call with unknown function name: %s\n", functionName)
						continue
					}

					// Handle streaming tool call parts
					if id == "" && lastCallId != "" {
						// Continue with previous tool call
						if tc, exists := toolCallsMap[lastCallId]; exists {
							tc.Arguments += toolCall.Function.Arguments
						}
					} else if id != "" {
						// Create or update a tool call
						lastCallId = id
						if tc, exists := toolCallsMap[id]; exists {
							tc.Arguments += toolCall.Function.Arguments
						} else {
							// Prepare to receive tool call arguments
							orderedToolCallIDs = append(orderedToolCallIDs, id)
							toolCallsMap[id] = &ToolCallBuilder{
								ID:        id,
								Name:      functionName,
								Arguments: toolCall.Function.Arguments,
							}
						}
					}
				}
			}
		}
	}

	if err := stream.Err(); err != nil {
		return openai.ChatCompletionMessageParamUnion{OfAssistant: &assistantMessage}, nil, nil, fmt.Errorf("error receiving stream data: %v", err)
	}

	// Update the assistant reasoning message
	reasoningContent := reasoningBuffer.String()

	// Set the content of the assistant message
	content := contentBuffer.String()
	if content != "" || reasoningContent != "" {
		// Also try extracting inline <think> tags (DeepSeek / Qwen streaming format)
		if thinkContent, cleanedContent := util.ExtractThinkTags(content); thinkContent != "" {
			if reasoningContent != "" {
				reasoningContent = reasoningContent + "\n" + thinkContent
			} else {
				reasoningContent = thinkContent
			}
			content = cleanedContent
		}

		// Inject reasoning back into the content wrapped in think tags
		// This preserves it natively in the session file without custom wrappers,
		// and passes it back to the model context seamlessly!
		content = util.InjectThinkTags(content, reasoningContent)

		if content != "" && !strings.HasSuffix(content, "\n") {
			content = content + "\n"
		}
		assistantMessage.Content.OfString = param.NewOpt(content)
	}

	// Convert tool calls map to slice
	var assistantToolCalls []openai.ChatCompletionMessageToolCallUnionParam
	for _, id := range orderedToolCallIDs {
		tc := toolCallsMap[id]
		assistantToolCalls = append(assistantToolCalls, openai.ChatCompletionMessageToolCallUnionParam{
			OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
				ID: tc.ID,
				Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
					Name:      tc.Name,
					Arguments: tc.Arguments,
				},
			},
		})
	}
	// Add tool calls to the assistant message if there are any
	if len(assistantToolCalls) > 0 {
		assistantMessage.ToolCalls = assistantToolCalls
	}

	return openai.ChatCompletionMessageParamUnion{OfAssistant: &assistantMessage}, assistantToolCalls, finalResp, nil
}

// processToolCall processes a single tool call and returns a tool response message
func (oa *OpenAI) processToolCall(toolCall openai.ChatCompletionMessageToolCallUnionParam) (openai.ChatCompletionMessageParamUnion, error) {
	// Parse the query from the arguments
	var argsMap map[string]interface{}
	fnCall := toolCall.GetFunction()
	if fnCall == nil {
		return openai.ToolMessage("Error: unsupported tool call type", ""), fmt.Errorf("unsupported tool call type")
	}

	argsStr := fnCall.Arguments
	if strings.TrimSpace(argsStr) == "" {
		argsStr = "{}"
	}

	if err := json.Unmarshal([]byte(argsStr), &argsMap); err != nil {
		return openai.ChatCompletionMessageParamUnion{}, fmt.Errorf("error parsing arguments: %v", err)
	}

	var filteredArgs map[string]interface{}
	if fnCall.Name == ToolEditFile || fnCall.Name == ToolWriteFile || fnCall.Name == ToolAskUser {
		// Don't show content(the modified content could be too long)
		filteredArgs = FilterOpenToolArguments(argsMap, []string{"content", "edits", "options", "question_type"})
	} else {
		filteredArgs = FilterOpenToolArguments(argsMap, []string{})
	}

	// Call function
	// Create structured data for the UI
	toolCallData := map[string]interface{}{
		"function": fnCall.Name,
		"args":     filteredArgs,
	}
	jsonData, _ := json.Marshal(toolCallData)
	oa.op.status.ChangeTo(oa.op.notify, StreamNotify{Status: StatusFunctionCalling, Data: string(jsonData)}, oa.op.proceed)

	// Convert ChatCompletionMessageToolCallUnionParam to ChatCompletionMessageToolCallUnion for dispatch
	toolCallUnion := openai.ChatCompletionMessageToolCallUnion{
		ID:   toolCall.OfFunction.ID,
		Type: "function",
		Function: openai.ChatCompletionMessageFunctionToolCallFunction{
			Name:      toolCall.OfFunction.Function.Name,
			Arguments: toolCall.OfFunction.Function.Arguments,
		},
	}

	var msg openai.ChatCompletionMessageParamUnion
	var err error
	// Dispatch tool call
	msg, err = oa.op.dispatchOpenAIToolCall(toolCallUnion, &argsMap)

	// Function call is done
	oa.op.status.ChangeTo(oa.op.notify, StreamNotify{Status: StatusFunctionCallingOver}, oa.op.proceed)
	return msg, err
}

// In an agentic workflow with multi-turn interactions:
// Each turn involves streaming responses from the LLM
// Each response may contain tool calls that trigger additional processing
// New responses are generated based on tool call results
// Each of these interactions consumes tokens that should be tracked
func addUpOpenAITokenUsage(ag *Agent, resp *openai.ChatCompletionChunk) {
	// For openai model, cache read tokens are not included in the usage
	// Because cached tokens already in the prompt tokens, so we don't need to count them
	// Thought tokens are also included in the prompt tokens
	// So the total tokens is the sum of prompt tokens and completion tokens
	if resp != nil && ag.TokenUsage != nil && resp.JSON.Usage.Valid() {
		usage := resp.Usage
		cachedTokens := usage.PromptTokensDetails.CachedTokens
		thoughtTokens := usage.CompletionTokensDetails.ReasoningTokens
		ag.TokenUsage.CachedTokensInPrompt = true
		ag.TokenUsage.RecordTokenUsage(
			int(usage.PromptTokens),
			int(usage.CompletionTokens),
			int(cachedTokens),
			int(thoughtTokens),
			int(usage.TotalTokens))
	}
}

// getOpenAITools returns the tools for OpenAI
func (ag *Agent) getOpenAITools() []openai.ChatCompletionToolUnionParam {
	var tools []openai.ChatCompletionToolUnionParam

	// Get filtered tools based on agent's enabled tools list
	genericTools := GetOpenToolsFiltered(ag.EnabledTools)
	for _, genericTool := range genericTools {
		tools = append(tools, genericTool.ToOpenAITool())
	}

	return tools
}

func (ag *Agent) getOpenAIMCPTools() []openai.ChatCompletionToolUnionParam {
	var tools []openai.ChatCompletionToolUnionParam
	// Add MCP tools if client is available
	if ag.MCPClient != nil {
		mcpTools := getMCPTools(ag.MCPClient)
		for _, mcpTool := range mcpTools {
			tools = append(tools, mcpTool.ToOpenAITool())
		}
	}

	return tools
}
