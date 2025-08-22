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

// func (ag *Agent) InitOpenChatAgent() error {
// 	// 1. Initialize the Client
// 	ctx := context.Background()
// 	// Create a client config with custom base URL
// 	client := arkruntime.NewClientWithApiKey(
// 		ag.ApiKey,
// 		arkruntime.WithTimeout(30*time.Minute),
// 		arkruntime.WithBaseUrl(ag.EndPoint),
// 	)
// }

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
		embeddingTools := ag.getOpenChatTools()
		tools = append(tools, embeddingTools...)
	} else if ag.SearchEngine.UseSearch {
		// Only add the search tool if general tools are not enabled,
		// but the search flag is explicitly set.
		searchTool := ag.getOpenChatWebSearchTool()
		tools = append(tools, searchTool)
	}

	chat := &OpenChat{
		client:     client,
		ctx:        &ctx,
		tools:      tools,
		notify:     ag.NotifyChan,
		data:       ag.DataChan,
		proceed:    ag.ProceedChan,
		search:     &ag.SearchEngine,
		toolsUse:   &ag.ToolsUse,
		queries:    make([]string, 0),
		references: make([]map[string]interface{}, 0), // Updated to match new field type
		status:     &ag.Status,
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
	convo := GetOpenChatConversation()
	err := convo.Load()
	if err != nil {
		// Notify error and return
		ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusError, Data: fmt.Sprintf("failed to load conversation: %v", err)}, nil)
		return err
	}
	convo.PushMessages(messages) // Add new messages to the conversation

	// Process the chat with recursive tool call handling
	err = chat.process(ag)
	if err != nil {
		return fmt.Errorf("error processing chat: %v", err)
	}
	return nil
}

// Conversation manages the state of an ongoing conversation with an AI assistant
type OpenChat struct {
	client     *arkruntime.Client
	ctx        *context.Context
	tools      []*model.Tool
	notify     chan<- StreamNotify      // Sub Channel to send notifications
	data       chan<- StreamData        // Sub Channel to send data
	proceed    <-chan bool              // Main Channel to receive proceed signal
	search     *SearchEngine            // Search engine
	toolsUse   *ToolsUse                // Use tools
	queries    []string                 // List of queries to be sent to the AI assistant
	references []map[string]interface{} // keep track of the references
	status     *StatusStack             // Stack to manage streaming status
}

func (c *OpenChat) process(ag *Agent) error {
	convo := GetOpenChatConversation()

	var finalResp *model.ChatCompletionStreamResponse

	// Recursively process the conversation
	// Because the model can call tools multiple times
	i := 0
	for range ag.MaxRecursions {
		i++
		//Debugf("Processing conversation at times: %d\n", i)
		c.status.ChangeTo(c.notify, StreamNotify{Status: StatusProcessing}, c.proceed)

		// Create the request
		req := model.CreateChatCompletionRequest{
			Model:         ag.ModelName,
			Temperature:   &ag.Temperature,
			Messages:      convo.Messages,
			Tools:         c.tools,
			StreamOptions: &model.StreamOptions{IncludeUsage: true},
			// Thinking: &model.Thinking{
			// 	Type: model.ThinkingTypeAuto,
			// },
		}

		// Make the streaming request
		stream, err := c.client.CreateChatCompletionStream(*c.ctx, req)
		if err != nil {
			return fmt.Errorf("stream creation error: %v", err)
		}
		defer stream.Close()

		// Wait for the main goroutine to tell sub-goroutine to proceed
		c.status.ChangeTo(c.notify, StreamNotify{Status: StatusStarted}, c.proceed)

		// Process the stream and collect tool calls
		assistantMessage, toolCalls, resp, err := c.processStream(stream)
		if err != nil {
			return fmt.Errorf("error processing stream: %v", err)
		}

		// Add the assistant's message to the conversation
		convo.PushMessage(assistantMessage)

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
				convo.PushMessage(toolMessage)
			}
			// Continue the conversation recursively
		} else {
			// No function call and no model content
			// Get the last response
			finalResp = resp
			break
		}
	}

	// Add queries to the output if any
	if len(c.queries) > 0 {
		q := "\n\n" + ag.SearchEngine.RetrieveQueries(c.queries)
		c.data <- StreamData{Text: q, Type: DataTypeNormal}
	}
	// Add references to the output if any
	if len(c.references) > 0 {
		refs := "\n\n" + ag.SearchEngine.RetrieveReferences(c.references)
		c.data <- StreamData{Text: refs, Type: DataTypeNormal}
	}

	// Record token usage
	if finalResp != nil && finalResp.Usage != nil && ag.TokenUsage != nil {
		ag.TokenUsage.RecordTokenUsage(int(finalResp.Usage.PromptTokens),
			int(finalResp.Usage.CompletionTokens),
			int(finalResp.Usage.PromptTokensDetails.CachedTokens),
			int(finalResp.Usage.CompletionTokensDetails.ReasoningTokens),
			int(finalResp.Usage.TotalTokens))
	}

	// No more message
	// Save the conversation
	err := convo.Save()
	if err != nil {
		return fmt.Errorf("failed to save conversation: %v", err)
	}

	// Flush all data to the channel
	c.data <- StreamData{Type: DataTypeFinished}
	<-c.proceed
	// Notify that the stream is finished
	c.status.ChangeTo(c.notify, StreamNotify{Status: StatusFinished}, nil)
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
			switch c.status.Peek() {
			case StatusReasoning:
				// If reasoning content is empty, switch back to normal state
				// This is to handle the case where reasoning content is empty but we already have content
				// Aka, the model is done with reasoning content and starting to output normal content
				if delta.ReasoningContent == nil ||
					(*delta.ReasoningContent == "" && delta.Content != "") {
					c.status.ChangeTo(c.notify, StreamNotify{Status: StatusReasoningOver}, c.proceed)
				}
			default:
				// If reasoning content is not empty, switch to reasoning state
				if HasContent(delta.ReasoningContent) {
					c.status.ChangeTo(c.notify, StreamNotify{Status: StatusReasoning}, c.proceed)
				}
			}

			if HasContent(delta.ReasoningContent) {
				// For reasoning model
				text := *delta.ReasoningContent
				reasoningBuffer.WriteString(text)
				c.data <- StreamData{Text: text, Type: DataTypeReasoning}
			} else if delta.Content != "" {
				text := delta.Content
				contentBuffer.WriteString(text)
				c.data <- StreamData{Text: text, Type: DataTypeNormal}
			}

			// Handle tool calls in the stream
			if len(delta.ToolCalls) > 0 {
				for _, toolCall := range delta.ToolCalls {
					id := toolCall.ID
					functionName := toolCall.Function.Name

					// Skip if not our expected function
					// Because some model made up function name
					if functionName != "" && !AvailableEmbeddingTool(functionName) {
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
		if !strings.HasSuffix(content, "\n") {
			content = content + "\n"
		}
		assistantMessage.Content = &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(content),
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

	// Call function
	c.status.ChangeTo(c.notify, StreamNotify{Status: StatusFunctionCalling, Data: fmt.Sprintf("%s(%s)\n", toolCall.Function.Name, formatToolCallArguments(argsMap))}, c.proceed)

	var msg *model.ChatCompletionMessage
	var err error

	// Using a map for dispatch is cleaner and more extensible than a large switch statement.
	toolHandlers := map[string]func(*model.ToolCall, *map[string]interface{}) (*model.ChatCompletionMessage, error){
		"shell":               c.processShellToolCall,
		"web_fetch":           c.processWebFetchToolCall,
		"web_search":          c.processWebSearchToolCall,
		"read_file":           c.processReadFileToolCall,
		"write_file":          c.processWriteFileToolCall,
		"edit_file":           c.processEditFileToolCall,
		"create_directory":    c.processCreateDirectoryToolCall,
		"list_directory":      c.processListDirectoryToolCall,
		"delete_file":         c.processDeleteFileToolCall,
		"delete_directory":    c.processDeleteDirectoryToolCall,
		"move":                c.processMoveToolCall,
		"copy":                c.processCopyToolCall,
		"search_files":        c.processSearchFilesToolCall,
		"search_text_in_file": c.processSearchTextInFileToolCall,
		"read_multiple_files": c.processReadMultipleFilesToolCall,
	}

	if handler, ok := toolHandlers[toolCall.Function.Name]; ok {
		msg, err = handler(&toolCall, &argsMap)
	} else {
		msg = nil
		err = fmt.Errorf("unknown function name: %s", toolCall.Function.Name)
	}

	// Function call is done
	c.status.ChangeTo(c.notify, StreamNotify{Status: StatusFunctionCallingOver}, c.proceed)
	return msg, err
}
