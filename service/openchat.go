package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

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
		base64Data := base64.StdEncoding.EncodeToString(file.Data())
		//imageURL := fmt.Sprintf("data:image/%s;base64,%s", file.Format(), base64Data)
		// data:format;base64,base64Data
		imageURL := fmt.Sprintf("data:%s;base64,%s", file.Format(), base64Data)
		// Create and append image part
		part = &model.ChatCompletionMessageContentPart{
			Type: model.ChatCompletionMessageContentPartTypeImageURL,
			ImageURL: &model.ChatMessageImageURL{
				URL: imageURL,
			},
		}
	} else if IsVideoMIMEType(format) {
		// Create and append video part
		base64Data := base64.StdEncoding.EncodeToString(file.Data())
		Debugf("Processing video file: format=%s, data length=%d, base64 length=%d", format, len(file.Data()), len(base64Data))
		part = &model.ChatCompletionMessageContentPart{
			Type: model.ChatCompletionMessageContentPartTypeVideoURL,
			VideoURL: &model.ChatMessageVideoURL{
				URL: fmt.Sprintf("data:%s;base64,%s", format, base64Data),
			},
		}
		Debugf("Created video part with type=%s, URL prefix=%s", part.Type, part.VideoURL.URL[:50])
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
 * Sort the messages by order
 * 1. System Prompt -- always at the top
 * 2. History Prompts
 *    - User Prompt
 *    - Assistant Prompt
 */
func (ag *Agent) SortOpenChatMessagesByOrder() error {
	// Load previous messages if any
	err := ag.Convo.Load()
	if err != nil {
		// Notify error and return
		ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusError, Data: fmt.Sprintf("failed to load conversation: %v", err)}, nil)
		return err
	}

	// Get current history
	history, _ := ag.Convo.GetMessages().([]*model.ChatCompletionMessage)

	// Handle System Prompt
	if len(history) > 0 && history[0].Role == model.ChatMessageRoleSystem {
		// Check for duplication
		currentSysContent := ""
		if history[0].Content != nil && history[0].Content.StringValue != nil {
			currentSysContent = *history[0].Content.StringValue
		}

		// If the new system prompt is not empty and not included in the old one, append it
		if ag.SystemPrompt != "" && !strings.Contains(currentSysContent, ag.SystemPrompt) {
			newContent := currentSysContent + "\n" + ag.SystemPrompt
			history[0].Content = &model.ChatCompletionMessageContent{
				StringValue: volcengine.String(newContent),
			}
			Debugf("Modified System Prompt: %s", *history[0].Content.StringValue)
		} else {
			Debugf("Reuse System Prompt: %s", *history[0].Content.StringValue)
		}
	} else if ag.SystemPrompt != "" {
		Debugf("New System Prompt: %s", ag.SystemPrompt)
		// Prepend system prompt
		sysMsg := &model.ChatCompletionMessage{
			Role: model.ChatMessageRoleSystem,
			Content: &model.ChatCompletionMessageContent{
				StringValue: volcengine.String(ag.SystemPrompt),
			},
			Name: Ptr(""),
		}
		// Always prepend system prompt at the beginning of the history
		history = append([]*model.ChatCompletionMessage{sysMsg}, history...)
	}

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

	// Update the conversation with new messages
	ag.Convo.SetMessages(history)
	return nil
}

// In current openchat api, we can't use cached tokens
// The context api and response api are not available for current golang lib
func (ag *Agent) GenerateOpenChatStream() error {

	// Initialize the Client
	ctx := context.Background()
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
		executor = NewSubAgentExecutor(ag.SharedState, MaxWorkersParalleled)
	}

	op := OpenProcessor{
		ctx:        ctx,
		notify:     ag.NotifyChan,
		data:       ag.DataChan,
		proceed:    ag.ProceedChan,
		search:     ag.SearchEngine,
		toolsUse:   &ag.ToolsUse,
		queries:    make([]string, 0),
		references: make([]map[string]interface{}, 0), // Updated to match new field type
		status:     &ag.Status,
		mcpClient:  ag.MCPClient,
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
		return fmt.Errorf("error processing chat: %v", err)
	}
	return nil
}

// Conversation manages the state of an ongoing conversation with an AI assistant
type OpenChat struct {
	client *arkruntime.Client
	tools  []*model.Tool
	op     *OpenProcessor
}

func (c *OpenChat) process(ag *Agent) error {
	// Context Management
	truncated := false
	cm := NewContextManagerForModel(ag.Model.ModelName, StrategyTruncateOldest)

	// Recursively process the conversation
	// Because the model can call tools multiple times
	i := 0
	for range ag.MaxRecursions {
		i++
		//Debugf("Processing conversation at times: %d\n", i)
		c.op.status.ChangeTo(c.op.notify, StreamNotify{Status: StatusProcessing}, c.op.proceed)

		// Note on Thinking Mode:
		// While we set up the request correctly here with model.Thinking, the Volcengine SDK (openchat.go)
		// handles the response parsing. If the SDK does not correctly map the API's "reasoning_content" field
		// (or if the API uses a different field than the SDK expects), thinking tokens will not be visible.
		// In contrast, openai.go uses the generic go-openai library which handles standard "reasoning_content" fields
		// robustly. If thinking tokens are missing in OpenChat, switch to provider: openai.

		// Get all history messages
		messages, _ := ag.Convo.GetMessages().([]*model.ChatCompletionMessage)

		// Apply context window management
		// This ensures we don't exceed the model's context window
		Debugf("Context messages: [%d]", len(messages))
		messages, truncated = cm.PrepareOpenChatMessages(messages, c.tools)
		if truncated {
			ag.Warn("Context trimmed to fit model limits")
			Debugf("Context messages after truncation: [%d]", len(messages))
			// Update the conversation with truncated messages
			ag.Convo.SetMessages(messages)
		}

		// Set thinking mode using ThinkingLevel conversion
		thinking, reasoningEffort := ag.ThinkingLevel.ToOpenChatParams()

		// Create the request with thinking mode
		req := model.CreateChatCompletionRequest{
			Model:           ag.Model.ModelName,
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
		stream, err := c.client.CreateChatCompletionStream(c.op.ctx, req)
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
		defer stream.Close()

		// Wait for the main goroutine to tell sub-goroutine to proceed
		c.op.status.ChangeTo(c.op.notify, StreamNotify{Status: StatusStarted}, c.op.proceed)

		// Process the stream and collect tool calls
		assistantMessage, toolCalls, resp, err := c.processStream(stream)
		if err != nil {
			return fmt.Errorf("error processing stream: %v", err)
		}

		// Record token usage
		// The final response contains the token usage metainfo
		ag.addUpOpenChatTokenUsage(resp)

		// Add the assistant's message to the conversation
		ag.Convo.Push(assistantMessage)

		// If there are tool calls, process them
		if len(*toolCalls) > 0 {
			// Process each tool call
			for _, toolCall := range *toolCalls {
				toolMessage, err := c.processToolCall(toolCall)
				if err != nil {
					// Switch agent signal, pop up
					if IsSwitchAgentError(err) {
						// Bugfix: left an "orphan" tool_call that had no matching tool result.
						// Add tool message to conversation to fix this.
						ag.Convo.Push(toolMessage)
						return err
					}
					ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusWarn, Data: fmt.Sprintf("Failed to process tool call: %v", err)}, nil)
				}
				// IMPORTANT: Even error happened still add an error response message to maintain conversation integrity
				// The API requires every tool_call to have a corresponding tool response
				// Add the tool response to the conversation
				ag.Convo.Push(toolMessage)
			}
			// Continue the conversation recursively
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

	// No more message
	// Save the conversation
	err := ag.Convo.Save()
	if err != nil {
		return fmt.Errorf("failed to save conversation: %v", err)
	}

	// Flush all data to the channel
	c.op.data <- StreamData{Type: DataTypeFinished}
	<-c.op.proceed
	// Notify that the stream is finished
	c.op.status.ChangeTo(c.op.notify, StreamNotify{Status: StatusFinished}, nil)
	return nil
}

// processStream processes the stream and collects tool calls
func (c *OpenChat) processStream(stream *utils.ChatCompletionStreamReader) (*model.ChatCompletionMessage, *map[string]model.ToolCall, *model.ChatCompletionStreamResponse, error) {
	assistantMessage := model.ChatCompletionMessage{
		Role: model.ChatMessageRoleAssistant,
		Name: Ptr(""),
	}
	toolCalls := make(map[string]model.ToolCall)
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

			// State transitions
			switch c.op.status.Peek() {
			case StatusReasoning:
				// If reasoning content is empty, switch back to normal state
				// This is to handle the case where reasoning content is empty but we already have content
				// Aka, the model is done with reasoning content and starting to output normal content
				if delta.ReasoningContent == nil ||
					(*delta.ReasoningContent == "" && delta.Content != "") {
					c.op.status.ChangeTo(c.op.notify, StreamNotify{Status: StatusReasoningOver}, c.op.proceed)
				}
			default:
				// If reasoning content is not empty, switch to reasoning state
				if HasContent(delta.ReasoningContent) {
					c.op.status.ChangeTo(c.op.notify, StreamNotify{Status: StatusReasoning}, c.op.proceed)
				}
			}

			if HasContent(delta.ReasoningContent) {
				// For reasoning model
				text := *delta.ReasoningContent
				reasoningBuffer.WriteString(text)
				c.op.data <- StreamData{Text: text, Type: DataTypeReasoning}
			} else if delta.Content != "" {
				text := delta.Content
				contentBuffer.WriteString(text)
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
						Warnf("Skipping tool call with unknown function name: %s", functionName)
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
	reasoning_content := reasoningBuffer.String()
	if reasoning_content != "" {
		assistantMessage.ReasoningContent = &reasoning_content
	}
	// Set the content of the assistant message
	content := contentBuffer.String()
	if content != "" {
		// Extract <think> tags from content if present
		// Some providers embed reasoning in <think>...</think> tags instead of
		// using a separate reasoning_content field
		if thinkContent, cleanedContent := ExtractThinkTags(content); thinkContent != "" {
			// Prepend extracted thinking to existing reasoning content
			if reasoning_content != "" {
				fullReasoning := thinkContent + "\n" + reasoning_content
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
		for id, tc := range toolCalls {
			// Sanitize arguments to handle cases like "}{" or trailing garbage
			cleanedArgs := sanitizeToolArgs(tc.Function.Arguments)
			if cleanedArgs != tc.Function.Arguments {
				Debugf("Sanitized tool arguments for %s: %s -> %s", tc.Function.Name, tc.Function.Arguments, cleanedArgs)
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
		Debugf("Failed to parse tool call arguments. Function: %s, Raw arguments: %s", toolCall.Function.Name, toolCall.Function.Arguments)
		return nil, fmt.Errorf("error parsing arguments: %v (raw: %s)", err, toolCall.Function.Arguments)
	}

	var filteredArgs map[string]interface{}
	if toolCall.Function.Name == "edit_file" || toolCall.Function.Name == "write_file" {
		// Don't show content(the modified content could be too long)
		filteredArgs = FilterOpenToolArguments(argsMap, []string{"content", "edits"})
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

	// Using a map for dispatch is cleaner and more extensible than a large switch statement.
	toolHandlers := map[string]func(*model.ToolCall, *map[string]interface{}) (*model.ChatCompletionMessage, error){
		"shell":               c.op.OpenChatShellToolCall,
		"web_fetch":           c.op.OpenChatWebFetchToolCall,
		"web_search":          c.op.OpenChatWebSearchToolCall,
		"read_file":           c.op.OpenChatReadFileToolCall,
		"write_file":          c.op.OpenChatWriteFileToolCall,
		"edit_file":           c.op.OpenChatEditFileToolCall,
		"create_directory":    c.op.OpenChatCreateDirectoryToolCall,
		"list_directory":      c.op.OpenChatListDirectoryToolCall,
		"delete_file":         c.op.OpenChatDeleteFileToolCall,
		"delete_directory":    c.op.OpenChatDeleteDirectoryToolCall,
		"move":                c.op.OpenChatMoveToolCall,
		"copy":                c.op.OpenChatCopyToolCall,
		"search_files":        c.op.OpenChatSearchFilesToolCall,
		"search_text_in_file": c.op.OpenChatSearchTextInFileToolCall,
		"read_multiple_files": c.op.OpenChatReadMultipleFilesToolCall,
		"list_memory":         c.op.OpenChatListMemoryToolCall,
		"save_memory":         c.op.OpenChatSaveMemoryToolCall,
		"switch_agent":        c.op.OpenChatSwitchAgentToolCall,
		"list_agent":          c.op.OpenChatListAgentToolCall,
		"spawn_subagents":     c.op.OpenChatSpawnSubAgentsToolCall,
		"get_state":           c.op.OpenChatGetStateToolCall,
		"set_state":           c.op.OpenChatSetStateToolCall,
		"list_state":          c.op.OpenChatListStateToolCall,
		"activate_skill":      c.op.OpenChatActivateSkillToolCall,
	}

	if handler, ok := toolHandlers[toolCall.Function.Name]; ok {
		// Handle embedding tool calls
		msg, err = handler(&toolCall, &argsMap)
	} else if c.op.mcpClient != nil && c.op.mcpClient.FindTool(toolCall.Function.Name) != nil {
		// Handle MCP tool calls
		msg, err = c.op.OpenChatMCPToolCall(&toolCall, &argsMap)
	} else {
		// Unknown function: return error message to model and warn user
		errorMsg := fmt.Sprintf("Error: Unknown function '%s'. This function is not available. Please use one of the available functions from the tool list.", toolCall.Function.Name)
		msg = &model.ChatCompletionMessage{
			Role: "tool",
			Content: &model.ChatCompletionMessageContent{
				StringValue: volcengine.String(errorMsg),
			},
			ToolCallID: toolCall.ID,
		}
		// Warn the user
		c.op.status.ChangeTo(c.op.notify, StreamNotify{Status: StatusWarn, Data: fmt.Sprintf("Model attempted to call unknown function: %s", toolCall.Function.Name)}, nil)
		err = nil // Don't stop conversation - let model see the error and adjust
	}

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
