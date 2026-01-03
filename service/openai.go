package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

func (ag *Agent) getOpenAIFilePart(file *FileData) *openai.ChatMessagePart {

	var part *openai.ChatMessagePart
	format := file.Format()
	// Handle based on file type
	if IsImageMIMEType(format) {
		// Create base64 image URL
		base64Data := base64.StdEncoding.EncodeToString(file.Data())
		//imageURL := fmt.Sprintf("data:image/%s;base64,%s", file.Format(), base64Data)
		// data:format;base64,base64Data
		imageURL := fmt.Sprintf("data:%s;base64,%s", file.Format(), base64Data)
		// Create and append image part
		part = &openai.ChatMessagePart{
			Type: openai.ChatMessagePartTypeImageURL,
			ImageURL: &openai.ChatMessageImageURL{
				URL: imageURL,
			},
		}
	} else if IsTextMIMEType(format) {
		// Create and append text part
		part = &openai.ChatMessagePart{
			Type: openai.ChatMessagePartTypeText,
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
func (ag *Agent) SortOpenAIMessagesByOrder() error {
	// Load previous messages if any
	err := ag.Convo.Load()
	if err != nil {
		// Notify error and return
		ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusError, Data: fmt.Sprintf("failed to load conversation: %v", err)}, nil)
		return err
	}

	// Get current history
	history, _ := ag.Convo.GetMessages().([]openai.ChatCompletionMessage)

	// Handle System Prompt
	if len(history) > 0 && history[0].Role == openai.ChatMessageRoleSystem {
		// Check for duplication
		// If the new system prompt is not empty and not included in the old one, append it
		if ag.SystemPrompt != "" && !strings.Contains(history[0].Content, ag.SystemPrompt) {
			history[0].Content += "\n" + ag.SystemPrompt
			Debugf("Modified System Prompt: %s", history[0].Content)
		} else {
			Debugf("Reuse System Prompt: %s", history[0].Content)
		}
	} else if ag.SystemPrompt != "" {
		Debugf("New System Prompt: %s", ag.SystemPrompt)
		// Prepend system prompt if provided
		sysMsg := openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: ag.SystemPrompt,
		}
		// Always prepend system prompt at the beginning of the history
		history = append([]openai.ChatCompletionMessage{sysMsg}, history...)
	}

	var userMessage openai.ChatCompletionMessage
	// Add File parts if available
	if len(ag.Files) > 0 {
		// Add user message
		userMessage = openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: "", // Empty string for multimodal
			MultiContent: []openai.ChatMessagePart{
				{
					Type: openai.ChatMessagePartTypeText,
					Text: ag.UserPrompt,
				},
			},
		}
		// Add all files
		for _, file := range ag.Files {
			if file != nil {
				part := ag.getOpenAIFilePart(file)
				if part != nil {
					userMessage.MultiContent = append(userMessage.MultiContent, *part)
				}
			}
		}
	} else {
		// For text only models, add user prompt directly
		// If use MultiContent(for multimodal), it could be error
		userMessage = openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: ag.UserPrompt, // only for text models
		}
	}
	history = append(history, userMessage)

	// Update the conversation with new messages
	ag.Convo.SetMessages(history)
	return nil
}

// GenerateOpenAIStream generates a streaming response using OpenAI API
func (ag *Agent) GenerateOpenAIStream() error {

	// Initialize the Client
	ctx := context.Background()
	// Create a client config with custom base URL
	config := openai.DefaultConfig(ag.Model.ApiKey)
	if ag.Model.EndPoint != "" {
		config.BaseURL = ag.Model.EndPoint
	}
	client := openai.NewClientWithConfig(config)

	// Create tools
	tools := []openai.Tool{}
	if len(ag.EnabledTools) > 0 {
		// Add embedding operation tools, which includes the web_search tool
		embeddingTools := ag.getOpenAIEmbeddingTools()
		tools = append(tools, embeddingTools...)
	}
	// OpenAI webtools and embedding tools are compatible
	if ag.SearchEngine.UseSearch {
		// Only add the search tool if general tools are not enabled,
		// but the search flag is explicitly set.
		searchTool := ag.getOpenAIWebSearchTool()
		tools = append(tools, searchTool)
	}
	if ag.MCPClient != nil {
		// Add MCP tools if MCP client is available
		mcpTools := ag.getOpenAIMCPTools()
		tools = append(tools, mcpTools...)
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
	}
	chat := &OpenAI{
		client: client,
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
		return fmt.Errorf("error processing chat: %v", err)
	}
	return nil
}

// OpenAI manages the state of an ongoing conversation with an AI assistant
type OpenAI struct {
	client *openai.Client
	tools  []openai.Tool
	op     *OpenProcessor
}

func (oa *OpenAI) process(ag *Agent) error {
	// Context Management
	truncated := false
	cm := NewContextManagerForModel(ag.Model.ModelName, StrategyTruncateOldest)

	// Recursively process the conversation
	// Because the model can call tools multiple times
	i := 0
	for range ag.MaxRecursions {
		i++
		//Debugf("Processing conversation at times: %d\n", i)
		oa.op.status.ChangeTo(oa.op.notify, StreamNotify{Status: StatusProcessing}, oa.op.proceed)

		// Get all history messages - MUST be inside loop to pick up newly pushed messages
		messages, _ := ag.Convo.GetMessages().([]openai.ChatCompletionMessage)

		// Apply context window management
		// This ensures we don't exceed the model's context window
		Debugf("Context messages: [%d]", len(messages))
		messages, truncated = cm.PrepareOpenAIMessages(messages, oa.tools)
		if truncated {
			ag.Warn("Context trimmed to fit model limits")
			Debugf("Context messages after truncation: [%d]", len(messages))
			// Update the conversation with truncated messages
			ag.Convo.SetMessages(messages)
		}

		// Create the request
		req := openai.ChatCompletionRequest{
			Model:       ag.Model.ModelName,
			Temperature: ag.Model.Temperature,
			TopP:        ag.Model.TopP,
			Messages:    messages,
			Tools:       oa.tools,
			Stream:      true,
		}

		// Add seed if provided
		if ag.Model.Seed != nil {
			seedInt32 := int(*ag.Model.Seed)
			req.Seed = &seedInt32
		}

		// Add reasoning effort if think mode is enabled
		if ag.ThinkMode {
			req.ReasoningEffort = "high"
		}
		if ag.TokenUsage != nil {
			req.StreamOptions = &openai.StreamOptions{IncludeUsage: true}
		}

		// Make the streaming request
		stream, err := oa.client.CreateChatCompletionStream(oa.op.ctx, req)
		if err != nil {
			return fmt.Errorf("stream creation error: %v", err)
		}
		defer stream.Close()

		// Wait for the main goroutine to tell sub-goroutine to proceed
		oa.op.status.ChangeTo(oa.op.notify, StreamNotify{Status: StatusStarted}, oa.op.proceed)

		// Process the stream and collect tool calls
		assistantMessage, toolCalls, resp, err := oa.processStream(stream)
		if err != nil {
			return fmt.Errorf("error processing stream: %v", err)
		}

		// Record token usage
		// The final response contains the token usage metainfo
		addUpOpenAITokenUsage(ag, resp)

		// Add the assistant's message to the conversation
		ag.Convo.Push(assistantMessage)

		// If there are tool calls, process them
		if len(toolCalls) > 0 {
			// Process each tool call
			for _, toolCall := range toolCalls {
				toolMessage, err := oa.processToolCall(toolCall)
				if err != nil {
					ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusWarn, Data: fmt.Sprintf("Failed to process tool call: %v", err)}, nil)
					// Send error info to user but continue processing other tool calls
					continue
				}
				// Add the tool response to the conversation
				ag.Convo.Push(toolMessage)
			}
			// Continue the conversation recursively
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

	// No more message
	// Save the conversation
	err := ag.Convo.Save()
	if err != nil {
		return fmt.Errorf("failed to save conversation: %v", err)
	}

	// Flush all data to the channel
	oa.op.data <- StreamData{Type: DataTypeFinished}
	<-oa.op.proceed
	// Notify that the stream is finished
	oa.op.status.ChangeTo(oa.op.notify, StreamNotify{Status: StatusFinished}, nil)
	return nil
}

// processStream processes the stream and collects tool calls
func (oa *OpenAI) processStream(stream *openai.ChatCompletionStream) (openai.ChatCompletionMessage, []openai.ToolCall, *openai.ChatCompletionStreamResponse, error) {
	assistantMessage := openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleAssistant,
	}
	toolCalls := make(map[string]openai.ToolCall)
	contentBuffer := strings.Builder{}
	reasoningBuffer := strings.Builder{}
	lastCallId := ""
	var finalResp *openai.ChatCompletionStreamResponse

	for {
		response, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return assistantMessage, nil, nil, fmt.Errorf("error receiving stream data: %v", err)
		}
		// Get the final response
		finalResp = &response

		// Handle regular content
		if len(response.Choices) > 0 {
			delta := response.Choices[0].Delta

			// State transitions
			switch oa.op.status.Peek() {
			case StatusReasoning:
				// If reasoning content is empty, switch back to normal state
				// This is to handle the case where reasoning content is empty but we already have content
				// Aka, the model is done with reasoning content and starting to output normal content
				if delta.ReasoningContent == "" && delta.Content != "" {
					oa.op.status.ChangeTo(oa.op.notify, StreamNotify{Status: StatusReasoningOver}, oa.op.proceed)
				}
			default:
				// If reasoning content is not empty, switch to reasoning state
				if delta.ReasoningContent != "" {
					oa.op.status.ChangeTo(oa.op.notify, StreamNotify{Status: StatusReasoning}, oa.op.proceed)
				}
			}

			if delta.ReasoningContent != "" {
				// For reasoning model
				text := delta.ReasoningContent
				reasoningBuffer.WriteString(text)
				oa.op.data <- StreamData{Text: text, Type: DataTypeReasoning}
			} else if delta.Content != "" {
				text := delta.Content
				contentBuffer.WriteString(text)
				oa.op.data <- StreamData{Text: text, Type: DataTypeNormal}
			}

			// Handle tool calls in the stream
			if len(delta.ToolCalls) > 0 {
				for _, toolCall := range delta.ToolCalls {
					id := toolCall.ID
					functionName := toolCall.Function.Name

					// Skip if not our expected function
					// Because some model made up function name
					if functionName != "" && !AvailableEmbeddingTool(functionName) && !AvailableSearchTool(functionName) && !AvailableMCPTool(functionName, oa.op.mcpClient) {
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
							toolCalls[id] = openai.ToolCall{
								ID:   id,
								Type: openai.ToolTypeFunction,
								Function: openai.FunctionCall{
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
		assistantMessage.ReasoningContent = reasoning_content
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
				assistantMessage.ReasoningContent = thinkContent + "\n" + reasoning_content
			} else {
				assistantMessage.ReasoningContent = thinkContent
			}
			content = cleanedContent
		}

		if content != "" {
			if !strings.HasSuffix(content, "\n") {
				content = content + "\n"
			}
			assistantMessage.Content = content
		}
	}

	// Convert tool calls map to slice
	var assistantToolCalls []openai.ToolCall
	for _, tc := range toolCalls {
		assistantToolCalls = append(assistantToolCalls, tc)
	}
	// Add tool calls to the assistant message if there are any
	if len(assistantToolCalls) > 0 {
		assistantMessage.ToolCalls = assistantToolCalls
	}

	return assistantMessage, assistantToolCalls, finalResp, nil
}

// processToolCall processes a single tool call and returns a tool response message
func (oa *OpenAI) processToolCall(toolCall openai.ToolCall) (openai.ChatCompletionMessage, error) {
	// Parse the query from the arguments
	var argsMap map[string]interface{}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &argsMap); err != nil {
		return openai.ChatCompletionMessage{}, fmt.Errorf("error parsing arguments: %v", err)
	}

	var filteredArgs map[string]interface{}
	if toolCall.Function.Name == "edit_file" || toolCall.Function.Name == "write_file" {
		// Don't show content(the modified content could be too long)
		filteredArgs = FilterToolArguments(argsMap, []string{"content", "edits"})
	} else {
		filteredArgs = FilterToolArguments(argsMap, []string{})
	}

	// Call function
	// Create structured data for the UI
	toolCallData := map[string]interface{}{
		"function": toolCall.Function.Name,
		"args":     filteredArgs,
	}
	jsonData, _ := json.Marshal(toolCallData)
	oa.op.status.ChangeTo(oa.op.notify, StreamNotify{Status: StatusFunctionCalling, Data: string(jsonData)}, oa.op.proceed)

	var msg openai.ChatCompletionMessage
	var err error

	// Using a map for dispatch is cleaner and more extensible than a large switch statement.
	toolHandlers := map[string]func(openai.ToolCall, *map[string]interface{}) (openai.ChatCompletionMessage, error){
		"shell":               oa.op.OpenAIShellToolCall,
		"web_fetch":           oa.op.OpenAIWebFetchToolCall,
		"web_search":          oa.op.OpenAIWebSearchToolCall,
		"read_file":           oa.op.OpenAIReadFileToolCall,
		"write_file":          oa.op.OpenAIWriteFileToolCall,
		"edit_file":           oa.op.OpenAIEditFileToolCall,
		"create_directory":    oa.op.OpenAICreateDirectoryToolCall,
		"list_directory":      oa.op.OpenAIListDirectoryToolCall,
		"delete_file":         oa.op.OpenAIDeleteFileToolCall,
		"delete_directory":    oa.op.OpenAIDeleteDirectoryToolCall,
		"move":                oa.op.OpenAIMoveToolCall,
		"copy":                oa.op.OpenAICopyToolCall,
		"search_files":        oa.op.OpenAISearchFilesToolCall,
		"search_text_in_file": oa.op.OpenAISearchTextInFileToolCall,
		"read_multiple_files": oa.op.OpenAIReadMultipleFilesToolCall,
		"list_memory":         oa.op.OpenAIListMemoryToolCall,
		"save_memory":         oa.op.OpenAISaveMemoryToolCall,
	}

	if handler, ok := toolHandlers[toolCall.Function.Name]; ok {
		msg, err = handler(toolCall, &argsMap)
	} else if oa.op.mcpClient != nil && oa.op.mcpClient.FindTool(toolCall.Function.Name) != nil {
		// Handle MCP tool calls
		msg, err = oa.op.OpenAIMCPToolCall(toolCall, &argsMap)
	} else {
		// Unknown function: return error message to model and warn user
		errorMsg := fmt.Sprintf("Error: Unknown function '%s'. This function is not available. Please use one of the available functions from the tool list.", toolCall.Function.Name)
		msg = openai.ChatCompletionMessage{
			Role:       openai.ChatMessageRoleTool,
			Content:    errorMsg,
			ToolCallID: toolCall.ID,
		}
		// Warn the user
		oa.op.status.ChangeTo(oa.op.notify, StreamNotify{Status: StatusWarn, Data: fmt.Sprintf("Model attempted to call unknown function: %s", toolCall.Function.Name)}, nil)
		err = nil // Don't stop conversation - let model see the error and adjust
	}

	// Function call is done
	oa.op.status.ChangeTo(oa.op.notify, StreamNotify{Status: StatusFunctionCallingOver}, oa.op.proceed)
	return msg, err
}

// In an agentic workflow with multi-turn interactions:
// Each turn involves streaming responses from the LLM
// Each response may contain tool calls that trigger additional processing
// New responses are generated based on tool call results
// Each of these interactions consumes tokens that should be tracked
func addUpOpenAITokenUsage(ag *Agent, resp *openai.ChatCompletionStreamResponse) {
	// For openai model, cache read tokens are not included in the usage
	// Because cached tokens already in the prompt tokens, so we don't need to count them
	// Thought tokens are also included in the prompt tokens
	// So the total tokens is the sum of prompt tokens and completion tokens
	if resp != nil && resp.Usage != nil && ag.TokenUsage != nil {
		cachedTokens := 0
		thoughtTokens := 0
		if resp.Usage.PromptTokensDetails != nil {
			cachedTokens = int(resp.Usage.PromptTokensDetails.CachedTokens)
		}
		if resp.Usage.CompletionTokensDetails != nil {
			thoughtTokens = int(resp.Usage.CompletionTokensDetails.ReasoningTokens)
		}
		ag.TokenUsage.CachedTokensInPrompt = true
		ag.TokenUsage.RecordTokenUsage(
			int(resp.Usage.PromptTokens),
			int(resp.Usage.CompletionTokens),
			int(cachedTokens),
			int(thoughtTokens),
			int(resp.Usage.TotalTokens))
	}
}

// getOpenAIEmbeddingTools returns the embedding tools for OpenAI
func (ag *Agent) getOpenAIEmbeddingTools() []openai.Tool {
	var tools []openai.Tool

	// Get filtered tools based on agent's enabled tools list
	genericTools := GetOpenEmbeddingToolsFiltered(ag.EnabledTools)
	for _, genericTool := range genericTools {
		tools = append(tools, genericTool.ToOpenAITool())
	}

	return tools
}

// getOpenAIWebSearchTool returns the web search tool for OpenAI
func (ag *Agent) getOpenAIWebSearchTool() openai.Tool {
	// Get generic web search tool and convert it to OpenAI tool
	genericTool := getOpenWebSearchTool()
	return genericTool.ToOpenAITool()
}

func (ag *Agent) getOpenAIMCPTools() []openai.Tool {
	var tools []openai.Tool
	// Add MCP tools if client is available
	if ag.MCPClient != nil {
		mcpTools := getMCPTools(ag.MCPClient)
		for _, mcpTool := range mcpTools {
			tools = append(tools, mcpTool.ToOpenAITool())
		}
	}

	return tools
}
