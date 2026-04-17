package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/activebook/gllm/util"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/utils"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
)

func (ag *Agent) getOpenChatFilePart(file *FileData) *model.ChatCompletionMessageContentPart {

	var part *model.ChatCompletionMessageContentPart
	format := file.Format()
	// Handle based on file type
	if IsImageMIMEType(format) {
		// Create base64 image URL
		imageURL := util.BuildDataURL(file.Format(), file.Data())
		// Create and append image part
		part = &model.ChatCompletionMessageContentPart{
			Type: model.ChatCompletionMessageContentPartTypeImageURL,
			ImageURL: &model.ChatMessageImageURL{
				URL: imageURL,
			},
		}
	} else if IsVideoMIMEType(format) {
		// Create and append video part
		videoURL := util.BuildDataURL(file.Format(), file.Data())
		part = &model.ChatCompletionMessageContentPart{
			Type: model.ChatCompletionMessageContentPartTypeVideoURL,
			VideoURL: &model.ChatMessageVideoURL{
				URL: videoURL,
			},
		}
		util.LogDebugf("Created video part with type=%s, URL prefix=%s\n", part.Type, part.VideoURL.URL[:50])
	} else if IsTextMIMEType(format) {
		// Create and append text part
		part = &model.ChatCompletionMessageContentPart{
			Type: model.ChatCompletionMessageContentPartTypeText,
			Text: string(file.Data()),
		}
	} else {
		// Unknown file type, skip
		// Don't deal with pdf, xls
		// It needs upload to OpenAI's servers first, so we can't include them directly in a message.
	}
	return part
}

/*
 * Sort the messages by order:
 *   History (user/assistant/tool turns only — no system message)
 *   The system prompt is never persisted; it is injected fresh in process().
 */
func (ag *Agent) SortOpenChatMessagesByOrder() error {
	// Load previous messages if any
	err := ag.Session.Load()
	if err != nil {
		return err
	}

	// Get current history (pure dialogue, no system messages)
	history, _ := ag.Session.GetMessages().([]*model.ChatCompletionMessage)

	var userMessage *model.ChatCompletionMessage
	// Add user message
	userMessage = &model.ChatCompletionMessage{
		Role: model.ChatMessageRoleUser,
		Content: &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(ag.UserPrompt),
		}, Name: Ptr(""),
	}

	// Add File parts if available
	if len(ag.Files) > 0 {
		userMessage = &model.ChatCompletionMessage{
			Role:    model.ChatMessageRoleUser,
			Content: &model.ChatCompletionMessageContent{ListValue: []*model.ChatCompletionMessageContentPart{}},
			Name:    Ptr(""),
		}
		// Add all files
		for _, file := range ag.Files {
			if file != nil {
				part := ag.getOpenChatFilePart(file)
				if part != nil {
					userMessage.Content.ListValue = append(userMessage.Content.ListValue, part)
				}
			}
		}
	}
	// Add to history only if it's not empty
	if ag.UserPrompt != "" || len(ag.Files) > 0 {
		history = append(history, userMessage)
	}

	// Update the session with new messages
	ag.Session.SetMessages(history)
	// Save the session
	// Bugfix: save session after update messages
	// Because the system message could be modified, and added user message
	return ag.Session.Save()
}

// prependOpenChatSystemMessage builds the final API messages slice by prepending
// a fresh system message in-memory. Returns history unchanged when prompt is empty.
func prependOpenChatSystemMessage(systemPrompt string, history []*model.ChatCompletionMessage) []*model.ChatCompletionMessage {
	if systemPrompt == "" {
		return history
	}
	sysMsg := &model.ChatCompletionMessage{
		Role: model.ChatMessageRoleSystem,
		Content: &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(systemPrompt),
		},
		Name: Ptr(""),
	}
	messages := make([]*model.ChatCompletionMessage, 0, 1+len(history))
	messages = append(messages, sysMsg)
	messages = append(messages, history...)
	return messages
}

// GenerateOpenChatSync generates a single, non-streaming completion using the Volcengine API.
// This is used for background tasks like context compression where streaming is unnecessary.
// systemPrompt is the system prompt to be used for the sync generation, it's majorly a role.
// the last message is the user prompt to do the task.
func (ag *Agent) GenerateOpenChatSync(messages []*model.ChatCompletionMessage, systemPrompt string) (string, error) {
	if ag.Ctx == nil {
		ag.Ctx = context.Background()
	}
	client := arkruntime.NewClientWithApiKey(
		ag.Model.ApiKey,
		arkruntime.WithTimeout(30*time.Minute),
		arkruntime.WithBaseUrl(ag.Model.EndPoint),
	)

	// Add system prompt
	messages = append([]*model.ChatCompletionMessage{{
		Role: model.ChatMessageRoleSystem,
		Content: &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(systemPrompt),
		},
		Name: Ptr(""),
	}}, messages...)

	req := model.CreateChatCompletionRequest{
		Model:       ag.Model.Model,
		Temperature: &ag.Model.Temperature,
		TopP:        &ag.Model.TopP,
		Messages:    messages,
	}

	resp, err := client.CreateChatCompletion(ag.Ctx, req)
	if err != nil {
		return "", fmt.Errorf("sync chat completion error: %w", err)
	}

	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == nil || resp.Choices[0].Message.Content.StringValue == nil {
		return "", fmt.Errorf("no choices returned in sync response")
	}

	return *resp.Choices[0].Message.Content.StringValue, nil
}

// In current openchat api, we can't use cached tokens
// The context api and response api are not available for current golang lib
func (ag *Agent) GenerateOpenChatStream() error {
	// Initialize the Client
	// Create a client config with custom base URL
	client := arkruntime.NewClientWithApiKey(
		ag.Model.ApiKey,
		arkruntime.WithTimeout(30*time.Minute),
		arkruntime.WithBaseUrl(ag.Model.EndPoint),
	)

	// Create a tool with the function
	tools := []*model.Tool{}
	if len(ag.EnabledTools) > 0 {
		// Add tools
		tools = ag.getOpenChatTools()
	}
	if ag.MCPClient != nil {
		// Add MCP tools if MCP client is available
		mcpTools := ag.getOpenChatMCPTools()
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
	chat := &OpenChat{
		client: client,
		tools:  tools,
		op:     &op,
	}

	// Prepare the Messages for Chat Completion
	err := ag.SortOpenChatMessagesByOrder()
	if err != nil {
		return fmt.Errorf("error sorting messages: %v", err)
	}

	// Signal that streaming has started
	// Wait for the main goroutine to tell sub-goroutine to proceed
	ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusStarted}, ag.ProceedChan)

	// Process the chat with recursive tool call handling
	err = chat.process(ag)
	if err != nil {
		// Switch agent signal, pop up
		if IsSwitchAgentError(err) {
			return err
		}
		// User cancel signal, pop up
		if IsUserCancelError(err) {
			return err
		}
		return fmt.Errorf("error processing chat: %v", err)
	}
	return nil
}

// session manages the state of an ongoing session with an AI assistant
type OpenChat struct {
	client *arkruntime.Client
	tools  []*model.Tool
	op     *OpenProcessor
}

func (c *OpenChat) process(ag *Agent) error {
	// Recursively process the session
	// Because the model can call tools multiple times
	i := 0
	for range ag.MaxRecursions {
		i++
		//Debugf("Processing session at times: %d\n", i)
		c.op.status.ChangeTo(c.op.notify, StreamNotify{Status: StatusProcessing}, c.op.proceed)

		// Note on Thinking Mode:
		// While we set up the request correctly here with model.Thinking, the Volcengine SDK (openchat.go)
		// handles the response parsing. If the SDK does not correctly map the API's "reasoning_content" field
		// (or if the API uses a different field than the SDK expects), thinking tokens will not be visible.
		// In contrast, openai.go uses the generic go-openai library which handles standard "reasoning_content" fields
		// robustly. If thinking tokens are missing in OpenChat, switch to provider: openai.

		// Get all history messages - MUST be inside loop to pick up newly pushed messages.
		// Session only holds pure dialogue (user/assistant/tool turns).
		messages, _ := ag.Session.GetMessages().([]*model.ChatCompletionMessage)

		// Apply context window management.
		util.LogDebugf("Context messages: [%d]\n", len(messages))
		pruned, truncated, err := ag.Context.PruneMessages(messages, ag.SystemPrompt, c.tools)
		if err != nil {
			return fmt.Errorf("failed to prune context: %w", err)
		}
		messages = pruned.([]*model.ChatCompletionMessage)

		if truncated {
			util.LogWarnf("Context limit reached: oldest messages removed or summarized (%s). Consider using /compress or summarizing manually.\n", ag.Context.GetStrategy())
			util.LogDebugf("Context messages after truncation: [%d]\n", len(messages))
			// Session write-back is clean: system message not yet prepended at this point.
			ag.Session.SetMessages(messages)
			if err := ag.Session.Save(); err != nil {
				return fmt.Errorf("failed to save truncated session: %w", err)
			}
		}

		// Prepend the fresh system prompt in-memory only — never persisted.
		messages = prependOpenChatSystemMessage(ag.SystemPrompt, messages)

		// Set thinking mode using ThinkingLevel conversion
		thinking, reasoningEffort := ag.ThinkingLevel.ToOpenChatParams()

		// Create the request with thinking mode
		req := model.CreateChatCompletionRequest{
			Model:           ag.Model.Model,
			Temperature:     &ag.Model.Temperature,
			TopP:            &ag.Model.TopP,
			Messages:        messages,
			Tools:           c.tools,
			Thinking:        thinking,
			ReasoningEffort: reasoningEffort,
		}

		// Include token usage if tracking is enabled
		if ag.TokenUsage != nil {
			req.StreamOptions = &model.StreamOptions{IncludeUsage: true}
		}

		// Make the streaming request
		stream, err := c.client.CreateChatCompletionStream(ag.Ctx, req)
		if err != nil {
			// Try to extract detailed API error information
			var apiErr *model.APIError
			if errors.As(err, &apiErr) {
				// APIError contains detailed error information (code and message)
				return fmt.Errorf("stream creation error: code=%s, message=%s", apiErr.Code, apiErr.Message)
			}
			// Fallback to checking for generic RequestError
			var reqErr *model.RequestError
			if errors.As(err, &reqErr) {
				// Check for 400 Bad Request which often implies invalid parameters or unsupported features (like tools)
				if reqErr.HTTPStatusCode == 400 && len(c.tools) > 0 {
					return fmt.Errorf("stream creation error: %v (Hint: The model might not support the requested tools/function calling. Try disabling tools or switching models)", err)
				}
			}

			// Fallback to generic error
			return fmt.Errorf("stream creation error: %v", err)
		}
		// Bugfix: do NOT use defer here — deferreds accumulate until process() returns,
		// so inside a loop each iteration would stack up an open stream. Close explicitly.
		// defer stream.Close()

		// Wait for the main goroutine to tell sub-goroutine to proceed
		c.op.status.ChangeTo(c.op.notify, StreamNotify{Status: StatusStarted}, c.op.proceed)

		// Process the stream and collect tool calls
		assistantMessage, toolCalls, resp, err := c.processStream(stream)
		stream.Close() // Bugfix: Close immediately after consuming to release the HTTP connection
		if err != nil {
			return fmt.Errorf("error processing stream: %v", err)
		}

		// Record token usage
		// The final response contains the token usage metainfo
		ag.addUpOpenChatTokenUsage(resp)

		// Add the assistant's message to the session
		err = c.saveToSession(ag, assistantMessage)
		if err != nil {
			return err
		}

		// If there are tool calls, process them
		if len(*toolCalls) > 0 {
			// Process each tool call
			for _, toolCall := range *toolCalls {
				toolMessage, err := c.processToolCall(toolCall)
				if err != nil {
					// Switch agent signal, pop up
					if IsSwitchAgentError(err) {
						// Bugfix: left an "orphan" tool_call that had no matching tool result.
						// Add tool message to session to fix this.
						c.saveToSession(ag, toolMessage)
						return err
					}
					if IsUserCancelError(err) {
						// User cancelled tool call, pop up
						c.saveToSession(ag, toolMessage)
						return err
					}
					ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusWarn, Data: fmt.Sprintf("Failed to process tool call: %v", err)}, nil)
				}
				// IMPORTANT: Even error happened still add an error response message to maintain session integrity
				// The API requires every tool_call to have a corresponding tool response
				// Add the tool response to the session
				err = c.saveToSession(ag, toolMessage)
				if err != nil {
					return err
				}
			}
			// Continue the session recursively
		} else {
			// No function call and no model content
			if assistantMessage.Content == nil {
				// Check whether there is no Content at all
				// buxfix: it would be null, in some cases the history with null content is not allowed
				// so we add an empty string. toolcall Content is nil is ok.
				assistantMessage.Content = &model.ChatCompletionMessageContent{StringValue: Ptr("")}
			}
			break
		}
	}

	// Add queries to the output if any
	if len(c.op.queries) > 0 {
		q := "\n\n" + ag.SearchEngine.RetrieveQueries(c.op.queries)
		c.op.data <- StreamData{Text: q, Type: DataTypeNormal}
	}
	// Add references to the output if any
	if len(c.op.references) > 0 {
		refs := "\n\n" + ag.SearchEngine.RetrieveReferences(c.op.references)
		c.op.data <- StreamData{Text: refs, Type: DataTypeNormal}
	}

	// Flush all data to the channel
	c.op.data <- StreamData{Type: DataTypeFinished}
	<-c.op.proceed
	// Notify that the stream is finished
	c.op.status.ChangeTo(c.op.notify, StreamNotify{Status: StatusFinished}, nil)
	return nil
}

// saveToSession processes the session save
// We need to save the session after each message is sent to the client
// Because model supports interleaved tool calls and responses, aka ReAct
// If error happened or user cancelled, in order to maintain session integrity, we need to save the session
// So that we can resume the session from the last saved state
func (c *OpenChat) saveToSession(ag *Agent, message *model.ChatCompletionMessage) error {
	// Add the assistant's message to the session
	err := ag.Session.Push(message)
	if err != nil {
		return fmt.Errorf("failed to save session: %v", err)
	}
	return nil
}

// processStream processes the stream and collects tool calls
func (c *OpenChat) processStream(stream *utils.ChatCompletionStreamReader) (*model.ChatCompletionMessage, *map[string]model.ToolCall, *model.ChatCompletionStreamResponse, error) {
	assistantMessage := model.ChatCompletionMessage{
		Role: model.ChatMessageRoleAssistant,
		Name: Ptr(""),
	}
	toolCalls := make(map[string]model.ToolCall)
	var orderedToolCallIDs []string
	contentBuffer := strings.Builder{}
	reasoningBuffer := strings.Builder{}
	lastCallId := ""
	var finalResp *model.ChatCompletionStreamResponse

	for {
		response, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, nil, fmt.Errorf("error receiving stream data: %v", err)
		}
		// Get the final response
		finalResp = &response

		// Handle regular content
		if len(response.Choices) > 0 {
			delta := (response.Choices[0].Delta)

			if util.HasContent(delta.ReasoningContent) {
				text := *delta.ReasoningContent
				reasoningBuffer.WriteString(text)
				if c.op.status.Peek() != StatusReasoning {
					// If reasoning content is not empty, switch to reasoning state
					c.op.status.ChangeTo(c.op.notify, StreamNotify{Status: StatusReasoning}, c.op.proceed)
				}
				c.op.data <- StreamData{Text: text, Type: DataTypeReasoning}
			}

			if delta.Content != "" {
				text := delta.Content
				contentBuffer.WriteString(text)
				if c.op.status.Peek() == StatusReasoning {
					// If regular content arrives while we're reasoning, transition away
					c.op.status.ChangeTo(c.op.notify, StreamNotify{Status: StatusReasoningOver}, c.op.proceed)
				}
				c.op.data <- StreamData{Text: text, Type: DataTypeNormal}
			}

			// Handle tool calls in the stream
			if len(delta.ToolCalls) > 0 {
				for _, toolCall := range delta.ToolCalls {
					id := toolCall.ID
					functionName := toolCall.Function.Name

					// Skip if not our expected function
					// Because some model made up function name
					if functionName != "" && !IsAvailableOpenTool(functionName) && !IsAvailableMCPTool(functionName, c.op.mcpClient) {
						util.LogWarnf("Skipping tool call with unknown function name: %s\n", functionName)
						continue
					}

					// Handle streaming tool call parts
					if id == "" && lastCallId != "" {
						// Continue with previous tool call
						if tc, exists := toolCalls[lastCallId]; exists {
							tc.Function.Arguments += toolCall.Function.Arguments
							toolCalls[lastCallId] = tc
						}
					} else if id != "" {
						// Create or update a tool call
						lastCallId = id
						if tc, exists := toolCalls[id]; exists {
							tc.Function.Arguments += toolCall.Function.Arguments
							toolCalls[id] = tc
						} else {
							// Prepare to receive tool call arguments
							orderedToolCallIDs = append(orderedToolCallIDs, id)
							toolCalls[id] = model.ToolCall{
								ID:   id,
								Type: model.ToolTypeFunction,
								Function: model.FunctionCall{
									Name:      functionName,
									Arguments: toolCall.Function.Arguments,
								},
							}
						}
					}
				}
			}
		}
	}

	// Update the assistant reasoning message
	reasoningContent := reasoningBuffer.String()
	if reasoningContent != "" {
		assistantMessage.ReasoningContent = &reasoningContent
	}
	// Set the content of the assistant message
	content := contentBuffer.String()
	if content != "" {
		// Extract <think> tags from content if present
		// Some providers embed reasoning in <think>...</think> tags instead of
		// using a separate reasoning_content field
		if thinkContent, cleanedContent := util.ExtractThinkTags(content); thinkContent != "" {
			// Prepend extracted thinking to existing reasoning content
			if reasoningContent != "" {
				fullReasoning := reasoningContent + "\n" + thinkContent
				assistantMessage.ReasoningContent = &fullReasoning
			} else {
				assistantMessage.ReasoningContent = &thinkContent
			}
			content = cleanedContent
		}

		if content != "" {
			if !strings.HasSuffix(content, "\n") {
				content = content + "\n"
			}
			assistantMessage.Content = &model.ChatCompletionMessageContent{
				StringValue: volcengine.String(content),
			}
		}
	}

	// Add tool calls to the assistant message if there are any
	if len(toolCalls) > 0 {
		var assistantToolCalls []*model.ToolCall
		for _, id := range orderedToolCallIDs {
			tc := toolCalls[id]
			// Sanitize arguments to handle cases like "}{" or trailing garbage
			cleanedArgs := sanitizeToolArgs(tc.Function.Arguments)
			if cleanedArgs != tc.Function.Arguments {
				util.LogDebugf("Sanitized tool arguments for %s: %s -> %s\n", tc.Function.Name, tc.Function.Arguments, cleanedArgs)
				tc.Function.Arguments = cleanedArgs
				// Update the map as well so local execution uses the clean version
				toolCalls[id] = tc
			}
			assistantToolCalls = append(assistantToolCalls, &tc)
		}
		assistantMessage.ToolCalls = assistantToolCalls
	}

	return &assistantMessage, &toolCalls, finalResp, nil
}

// processToolCall processes a single tool call and returns a tool response message
func (c *OpenChat) processToolCall(toolCall model.ToolCall) (*model.ChatCompletionMessage, error) {
	// Parse the query from the arguments
	var argsMap map[string]interface{}
	argsStr := toolCall.Function.Arguments
	if strings.TrimSpace(argsStr) == "" {
		argsStr = "{}"
	}

	if err := json.Unmarshal([]byte(argsStr), &argsMap); err != nil {
		// Log the malformed JSON for debugging
		util.LogDebugf("Failed to parse tool call arguments. Function: %s, Raw arguments: %s\n", toolCall.Function.Name, toolCall.Function.Arguments)
		return nil, fmt.Errorf("error parsing arguments: %v (raw: %s)", err, toolCall.Function.Arguments)
	}

	var filteredArgs map[string]interface{}
	if toolCall.Function.Name == ToolEditFile || toolCall.Function.Name == ToolWriteFile || toolCall.Function.Name == ToolAskUser {
		// Don't show content(the modified content could be too long)
		filteredArgs = FilterOpenToolArguments(argsMap, []string{"content", "edits", "options", "question_type"})
	} else {
		filteredArgs = FilterOpenToolArguments(argsMap, []string{})
	}

	// Call function
	// Create structured data for the UI
	toolCallData := map[string]interface{}{
		"function": toolCall.Function.Name,
		"args":     filteredArgs,
	}
	jsonData, _ := json.Marshal(toolCallData)
	c.op.status.ChangeTo(c.op.notify, StreamNotify{Status: StatusFunctionCalling, Data: string(jsonData)}, c.op.proceed)

	var msg *model.ChatCompletionMessage
	var err error
	// Dispatch tool call
	msg, err = c.op.dispatchOpenChatToolCall(&toolCall, &argsMap)

	// Function call is done
	c.op.status.ChangeTo(c.op.notify, StreamNotify{Status: StatusFunctionCallingOver}, c.op.proceed)
	return msg, err
}

// In an agentic workflow with multi-turn interactions:
// Each turn involves streaming responses from the LLM
// Each response may contain tool calls that trigger additional processing
// New responses are generated based on tool call results
// Each of these interactions consumes tokens that should be tracked
func (ag *Agent) addUpOpenChatTokenUsage(resp *model.ChatCompletionStreamResponse) {
	// For openchat model, cache read tokens are not included in the usage
	// Because cached tokens already in the prompt tokens, so we don't need to count them
	// Thought tokens are also included in the prompt tokens
	// So the total tokens is the sum of prompt tokens and completion tokens
	if resp != nil && resp.Usage != nil && ag.TokenUsage != nil {
		ag.TokenUsage.CachedTokensInPrompt = true
		ag.TokenUsage.RecordTokenUsage(int(resp.Usage.PromptTokens),
			int(resp.Usage.CompletionTokens),
			int(resp.Usage.PromptTokensDetails.CachedTokens),
			int(resp.Usage.CompletionTokensDetails.ReasoningTokens),
			int(resp.Usage.TotalTokens))
	}
}

func (ag *Agent) getOpenChatTools() []*model.Tool {
	var tools []*model.Tool

	// Get filtered tools based on agent's enabled tools list
	genericTools := GetOpenToolsFiltered(ag.EnabledTools)
	for _, genericTool := range genericTools {
		tools = append(tools, genericTool.ToOpenChatTool())
	}

	return tools
}

func (ag *Agent) getOpenChatMCPTools() []*model.Tool {
	var tools []*model.Tool
	// Add MCP tools if client is available
	if ag.MCPClient != nil {
		mcpTools := getMCPTools(ag.MCPClient)
		for _, mcpTool := range mcpTools {
			tools = append(tools, mcpTool.ToOpenChatTool())
		}
	}
	return tools
}

// sanitizeToolArgs attempts to clean up malformed JSON tool arguments
// It primarily handles cases where the model outputs multiple JSON objects or trailing garbage
// e.g. Log Evidence: Raw arguments: {"command": "...", ...}{}
func sanitizeToolArgs(args string) string {
	if args == "" {
		return args
	}
	// Try to decode as a single JSON object
	var obj map[string]interface{}
	dec := json.NewDecoder(strings.NewReader(args))
	if err := dec.Decode(&obj); err == nil {
		// If successfully decoded one object, re-marshal it
		// This strictly keeps only the valid JSON object and removes any trailing data
		if cleaned, err := json.Marshal(obj); err == nil {
			return string(cleaned)
		}
	}
	// If decoding fails, return original and let downstream handle the error
	return args
}
