package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
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

// In current openchat api, we can't use cached tokens
// The context api and response api are not available for current golang lib
func (ag *Agent) GenerateOpenChatStream() error {

	// 1. Initialize the Client
	ctx := context.Background()
	// Create a client config with custom base URL
	client := arkruntime.NewClientWithApiKey(
		ag.ApiKey,
		arkruntime.WithTimeout(30*time.Minute),
		arkruntime.WithBaseUrl(ag.EndPoint),
	)

	// Create a tool with the function
	tools := []*model.Tool{}
	if ag.ToolsUse.Enable {
		// Add embedding operation tools, which includes the web_search tool
		embeddingTools := ag.getOpenChatEmbeddingTools()
		tools = append(tools, embeddingTools...)
	}
	// Openchat webtools and embedding tools are compatible
	if ag.SearchEngine.UseSearch {
		// Only add the search tool if general tools are not enabled,
		// but the search flag is explicitly set.
		searchTool := ag.getOpenChatWebSearchTool()
		tools = append(tools, searchTool)
	}
	if ag.MCPClient != nil {
		// Add MCP tools if MCP client is available
		mcpTools := ag.getOpenChatMCPTools()
		tools = append(tools, mcpTools...)
	}

	op := OpenProcessor{
		ctx:        ctx,
		notify:     ag.NotifyChan,
		data:       ag.DataChan,
		proceed:    ag.ProceedChan,
		search:     &ag.SearchEngine,
		toolsUse:   &ag.ToolsUse,
		queries:    make([]string, 0),
		references: make([]map[string]interface{}, 0), // Updated to match new field type
		status:     &ag.Status,
		mcpClient:  ag.MCPClient,
	}
	chat := &OpenChat{
		client: client,
		tools:  tools,
		op:     &op,
	}

	// 2. Prepare the Messages for Chat Completion
	// Initialize messages slice with proper capacity
	messages := make([]*model.ChatCompletionMessage, 0, 2)

	// Only add system message if not empty
	if ag.SystemPrompt != "" {
		messages = append(messages, &model.ChatCompletionMessage{
			Role: model.ChatMessageRoleSystem,
			Content: &model.ChatCompletionMessageContent{
				StringValue: volcengine.String(ag.SystemPrompt),
			}, Name: Ptr(""),
		})
	}

	var userMessage *model.ChatCompletionMessage
	// Add user message
	userMessage = &model.ChatCompletionMessage{
		Role: model.ChatMessageRoleUser,
		Content: &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(ag.UserPrompt),
		}, Name: Ptr(""),
	}
	messages = append(messages, userMessage)
	// Add image parts if available
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
		messages = append(messages, userMessage)
	}

	// Signal that streaming has started
	// Wait for the main goroutine to tell sub-goroutine to proceed
	ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusStarted}, ag.ProceedChan)

	// Load previous messages if any
	err := ag.Convo.Load()
	if err != nil {
		// Notify error and return
		ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusError, Data: fmt.Sprintf("failed to load conversation: %v", err)}, nil)
		return err
	}
	ag.Convo.Push(messages) // Add new messages to the conversation

	// Process the chat with recursive tool call handling
	err = chat.process(ag)
	if err != nil {
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
	// For some models, there isn't thinking property
	// So we need to check whether to add it or not
	thinkProperty := true

	// Recursively process the conversation
	// Because the model can call tools multiple times
	i := 0
	for range ag.MaxRecursions {
		i++
		//Debugf("Processing conversation at times: %d\n", i)
		c.op.status.ChangeTo(c.op.notify, StreamNotify{Status: StatusProcessing}, c.op.proceed)

		// Set whether to use thinking mode
		var thinking *model.Thinking
		if thinkProperty {
			if ag.ThinkMode {
				thinking = &model.Thinking{
					Type: model.ThinkingTypeEnabled,
				}
			} else {
				// For some models, it must explicitly tell it not to use thinking mode
				thinking = &model.Thinking{
					Type: model.ThinkingTypeDisabled,
				}
			}
		}
		// Get all history messages
		messages, _ := ag.Convo.GetMessages().([]*model.ChatCompletionMessage)

		// Apply context window management
		// This ensures we don't exceed the model's context window
		cm := NewContextManagerForModel(ag.ModelName, StrategyTruncateOldest)
		messages, truncated := cm.PrepareOpenChatMessages(messages)
		if truncated {
			ag.Warn("Context trimmed to fit model limits")
			// Update the conversation with truncated messages
			ag.Convo.SetMessages(messages)
		}

		// Create the request with thinking mode
		req := model.CreateChatCompletionRequest{
			Model:       ag.ModelName,
			Temperature: &ag.Temperature,
			TopP:        &ag.TopP,
			Messages:    messages,
			Tools:       c.tools,
			Thinking:    thinking,
		}

		// Include token usage if tracking is enabled
		if ag.TokenUsage != nil {
			req.StreamOptions = &model.StreamOptions{IncludeUsage: true}
		}

		// Make the streaming request
		stream, err := c.client.CreateChatCompletionStream(c.op.ctx, req)

		// If thinking mode caused an error, try again without thinking mode
		if err != nil && req.Thinking != nil {
			// Create request without thinking mode
			// Because some models don't support the thinking property
			req.Thinking = nil
			stream, err = c.client.CreateChatCompletionStream(c.op.ctx, req)
			if err != nil {
				return fmt.Errorf("stream creation error: %v", err)
			}
			// This model cannot add thinking property
			thinkProperty = false
		} else if err != nil {
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
					if functionName != "" && !AvailableEmbeddingTool(functionName) && !AvailableSearchTool(functionName) && !AvailableMCPTool(functionName, c.op.mcpClient) {
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
		for _, tc := range toolCalls {
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
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &argsMap); err != nil {
		return nil, fmt.Errorf("error parsing arguments: %v", err)
	}

	var args string
	if toolCall.Function.Name == "edit_file" || toolCall.Function.Name == "write_file" {
		// Don't show content(the modified content could be too long)
		args = formatToolCallArguments(argsMap, []string{"content", "edits"})
	} else {
		args = formatToolCallArguments(argsMap, []string{})
	}

	// Call function
	c.op.status.ChangeTo(c.op.notify, StreamNotify{Status: StatusFunctionCalling, Data: fmt.Sprintf("%s(%s)\n", toolCall.Function.Name, args)}, c.op.proceed)

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
	}

	if handler, ok := toolHandlers[toolCall.Function.Name]; ok {
		// Handle embedding tool calls
		msg, err = handler(&toolCall, &argsMap)
	} else if c.op.mcpClient != nil && c.op.mcpClient.FindTool(toolCall.Function.Name) != nil {
		// Handle MCP tool calls
		msg, err = c.op.OpenChatMCPToolCall(&toolCall, &argsMap)
	} else {
		msg = nil
		err = fmt.Errorf("unknown function name: %s", toolCall.Function.Name)
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
	//Warnf("addUpTokenUsage - PromptTokenCount: %d, CompletionTokenCount: %d, TotalTokenCount: %d", resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)
	if resp != nil && resp.Usage != nil && ag.TokenUsage != nil {
		ag.TokenUsage.RecordTokenUsage(int(resp.Usage.PromptTokens),
			int(resp.Usage.CompletionTokens),
			int(resp.Usage.PromptTokensDetails.CachedTokens),
			int(resp.Usage.CompletionTokensDetails.ReasoningTokens),
			int(resp.Usage.TotalTokens))
	}
}

func (ag *Agent) getOpenChatEmbeddingTools() []*model.Tool {
	var tools []*model.Tool

	// Get generic tools and convert them to OpenChat tools
	genericTools := getOpenEmbeddingTools()
	for _, genericTool := range genericTools {
		tools = append(tools, genericTool.ToOpenChatTool())
	}

	return tools
}

func (ag *Agent) getOpenChatWebSearchTool() *model.Tool {
	// Get generic web search tool and convert it to OpenChat tool
	genericTool := getOpenWebSearchTool()
	return genericTool.ToOpenChatTool()
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
