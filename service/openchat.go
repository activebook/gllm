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

type streamState int

const (
	stateNormal streamState = iota
	stateReasoning
)

func (ll *LangLogic) GenerateOpenChatStream() error {
	if !ll.UseSearchTool {
		return ll.openchatStream()
	} else {
		return ll.openchatStreamWithSearch()
	}
}

func (ll *LangLogic) getOpenChatFilePart(file *FileData) *model.ChatCompletionMessageContentPart {

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

func (ll *LangLogic) openchatStream() error {
	// 1. Initialize the Client
	ctx := context.Background()
	// Create a client config with custom base URL
	client := arkruntime.NewClientWithApiKey(
		ll.ApiKey,
		arkruntime.WithTimeout(30*time.Minute),
		arkruntime.WithBaseUrl(ll.EndPoint),
	)

	// 2. Prepare the Messages for Chat Completion
	// Initialize messages slice with proper capacity
	messages := make([]*model.ChatCompletionMessage, 0, 2)

	// Only add system message if not empty
	if ll.SystemPrompt != "" {
		messages = append(messages, &model.ChatCompletionMessage{
			Role: model.ChatMessageRoleSystem,
			Content: &model.ChatCompletionMessageContent{
				StringValue: volcengine.String(ll.SystemPrompt),
			}, Name: Ptr(""),
		})
	}

	var userMessage *model.ChatCompletionMessage
	// Add user message
	userMessage = &model.ChatCompletionMessage{
		Role: model.ChatMessageRoleUser,
		Content: &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(ll.UserPrompt),
		}, Name: Ptr(""),
	}
	messages = append(messages, userMessage)
	// Add image parts if available
	if len(ll.Files) > 0 {
		userMessage = &model.ChatCompletionMessage{
			Role:    model.ChatMessageRoleUser,
			Content: &model.ChatCompletionMessageContent{ListValue: []*model.ChatCompletionMessageContentPart{}},
			Name:    Ptr(""),
		}
		// Add all files
		for _, file := range ll.Files {
			if file != nil {
				part := ll.getOpenChatFilePart(file)
				if part != nil {
					userMessage.Content.ListValue = append(userMessage.Content.ListValue, part)
				}
			}
		}
		messages = append(messages, userMessage)
	}

	// Signal that streaming has started
	ll.ProcChan <- StreamNotify{Status: StatusStarted}
	<-ll.ProceedChan // Wait for the main goroutine to tell sub-goroutine to proceed

	// Load previous messages if any
	convo := GetOpenChatConversation()
	err := convo.Load()
	if err != nil {
		ll.ProcChan <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("failed to load conversation: %v", err)}
		return err
	}
	convo.PushMessages(messages) // Add new messages to the conversation

	ll.ProcChan <- StreamNotify{Status: StatusProcessing}

	// 3. Create the Chat Completion Request for Streaming
	request := model.CreateChatCompletionRequest{
		Model:       ll.ModelName,
		Messages:    convo.Messages,
		Temperature: &ll.Temperature, // Directly use the float32 value here
		// MaxTokens:   150,         // Optional: limit output length
		// TopP:        1.0,         // Optional: nucleus sampling
		// N:           1,           // How many chat completion choices to generate for each input message.
	}

	// 4. Initiate Streaming Chat Completion
	stream, err := client.CreateChatCompletionStream(ctx, request)
	if err != nil {
		ll.ProcChan <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("failed to create chat completion stream: %v", err)}
		return err
	}
	// IMPORTANT: Always close the stream when done.
	defer stream.Close()

	// Signal that streaming has started
	ll.ProcChan <- StreamNotify{Status: StatusStarted}
	<-ll.ProceedChan // Wait for the main goroutine to tell sub-goroutine to proceed

	// 5. Process the Stream
	state := stateNormal
	assistContent := strings.Builder{}
	reasoningContent := strings.Builder{}
	for {
		response, err := stream.Recv()
		// Check for the end of the stream
		if errors.Is(err, io.EOF) {
			// Indicate stream end
			ll.ProcChan <- StreamNotify{Status: StatusData, Data: "\n"}
			break // Exit the loop when the stream is done
		}
		// Handle potential errors during streaming
		if err != nil {
			ll.ProcChan <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("error receiving stream response: %v", err)}
			return err
		}

		// Extract and print the text chunk from the response delta
		// For streaming, the actual content is in the Delta field
		if len(response.Choices) > 0 {
			delta := (response.Choices[0].Delta)

			// State transitions
			switch state {
			case stateNormal:
				if delta.ReasoningContent != nil {
					ll.ProcChan <- StreamNotify{Status: StatusReasoning, Data: ""}
					state = stateReasoning
				}
			case stateReasoning:
				if delta.ReasoningContent == nil {
					ll.ProcChan <- StreamNotify{Status: StatusReasoningOver, Data: ""}
					state = stateNormal
				}
			}

			if delta.ReasoningContent != nil {
				// For reasoning model
				text := *delta.ReasoningContent
				reasoningContent.WriteString(text)
				ll.ProcChan <- StreamNotify{Status: StatusReasoningData, Data: text}
			} else {
				text := delta.Content
				assistContent.WriteString(text)
				ll.ProcChan <- StreamNotify{Status: StatusData, Data: text}
			}
		}
	}

	// keep response content
	msg := model.ChatCompletionMessage{
		Role: model.ChatMessageRoleAssistant,
		Content: &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(assistContent.String()),
		}, Name: Ptr(""),
	}
	reasoning_content := reasoningContent.String()
	if reasoning_content != "" {
		msg.ReasoningContent = &reasoning_content
	}
	// Add the assistant's message to the conversation
	convo.PushMessage(&msg)
	err = convo.Save()
	if err != nil {
		return fmt.Errorf("failed to save conversation: %v", err)
	}
	ll.ProcChan <- StreamNotify{Status: StatusFinished}
	return err
}

func (ll *LangLogic) openchatStreamWithSearch() error {

	// 1. Initialize the Client
	ctx := context.Background()
	// Create a client config with custom base URL
	client := arkruntime.NewClientWithApiKey(
		ll.ApiKey,
		arkruntime.WithTimeout(30*time.Minute),
		arkruntime.WithBaseUrl(ll.EndPoint),
	)

	// Create a tool with the function
	tool := ll.getOpenChatSearchTool()

	chat := &OpenChat{
		client:        client,
		ctx:           &ctx,
		model:         ll.ModelName,
		temperature:   ll.Temperature,
		tools:         []*model.Tool{tool},
		proc:          ll.ProcChan,
		proceed:       ll.ProceedChan,
		references:    make([]*map[string]interface{}, 0, 1),
		maxRecursions: 5, // Limit recursion depth to prevent infinite loops
	}

	// 2. Prepare the Messages for Chat Completion
	// Initialize messages slice with proper capacity
	messages := make([]*model.ChatCompletionMessage, 0, 2)

	// Only add system message if not empty
	if ll.SystemPrompt != "" {
		messages = append(messages, &model.ChatCompletionMessage{
			Role: model.ChatMessageRoleSystem,
			Content: &model.ChatCompletionMessageContent{
				StringValue: volcengine.String(ll.SystemPrompt),
			}, Name: Ptr(""),
		})
	}

	var userMessage *model.ChatCompletionMessage
	// Add user message
	userMessage = &model.ChatCompletionMessage{
		Role: model.ChatMessageRoleUser,
		Content: &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(ll.UserPrompt),
		}, Name: Ptr(""),
	}
	messages = append(messages, userMessage)
	// Add image parts if available
	if len(ll.Files) > 0 {
		userMessage = &model.ChatCompletionMessage{
			Role:    model.ChatMessageRoleUser,
			Content: &model.ChatCompletionMessageContent{ListValue: []*model.ChatCompletionMessageContentPart{}},
			Name:    Ptr(""),
		}
		// Add all files
		for _, file := range ll.Files {
			if file != nil {
				part := ll.getOpenChatFilePart(file)
				if part != nil {
					userMessage.Content.ListValue = append(userMessage.Content.ListValue, part)
				}
			}
		}
		messages = append(messages, userMessage)
	}

	// Signal that streaming has started
	ll.ProcChan <- StreamNotify{Status: StatusStarted}
	<-ll.ProceedChan // Wait for the main goroutine to tell sub-goroutine to proceed

	// Load previous messages if any
	convo := GetOpenChatConversation()
	err := convo.Load()
	if err != nil {
		ll.ProcChan <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("failed to load conversation: %v", err)}
		return err
	}
	convo.PushMessages(messages) // Add new messages to the conversation

	// Process the chat with recursive tool call handling
	err = chat.process()
	if err != nil {
		ll.ProcChan <- StreamNotify{Status: StatusError}
		Warnf("Error processing chat: %v\n", err)
		return fmt.Errorf("error processing chat: %v", err)
	}
	return nil
}

// Conversation manages the state of an ongoing conversation with an AI assistant
type OpenChat struct {
	client        *arkruntime.Client
	ctx           *context.Context
	model         string
	temperature   float32
	tools         []*model.Tool
	proc          chan<- StreamNotify // Sub Channel to send notifications
	proceed       <-chan bool         // Main Channel to receive proceed signal
	maxRecursions int
	queries       []string                  // List of queries to be sent to the AI assistant
	references    []*map[string]interface{} // keep track of the references
}

func (ll *LangLogic) getOpenChatSearchTool() *model.Tool {

	engine := GetSearchEngine()
	switch engine {
	case GoogleSearchEngine:
		// Use Google Search Engine
	case BingSearchEngine:
		// Use Bing Search Engine
	case TavilySearchEngine:
		// Use Tavily Search Engine
	case NoneSearchEngine:
		// Use None Search Engine
	default:
	}

	searchFunc := model.FunctionDefinition{
		Name:        "web_search",
		Description: "Retrieve the most relevant and up-to-date information from the web.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "The search term or question to find information about.",
				},
			},
			"required": []string{"query"},
		},
	}
	searchTool := model.Tool{
		Type:     model.ToolTypeFunction,
		Function: &searchFunc,
	}

	return &searchTool
}

func (c *OpenChat) process() error {
	convo := GetOpenChatConversation()

	// only allow 3 recursions
	i := 0
	for range c.maxRecursions {
		i++
		Debugf("Processing conversation at times: %d\n", i)

		c.proc <- StreamNotify{Status: StatusProcessing}

		// Create the request
		req := model.CreateChatCompletionRequest{
			Model:       c.model,
			Temperature: &c.temperature,
			Messages:    convo.Messages,
			Tools:       c.tools,
		}

		// Make the streaming request
		stream, err := c.client.CreateChatCompletionStream(*c.ctx, req)
		if err != nil {
			return fmt.Errorf("stream creation error: %v", err)
		}
		defer stream.Close()

		c.proc <- StreamNotify{Status: StatusStarted}
		<-c.proceed // Wait for the main goroutine to tell sub-goroutine to proceed

		// Process the stream and collect tool calls
		assistantMessage, toolCalls, err := c.processStream(stream)
		if err != nil {
			return fmt.Errorf("error processing stream: %v", err)
		}

		// Add the assistant's message to the conversation
		convo.PushMessage(assistantMessage)

		// If there are tool calls, process them
		if len(*toolCalls) > 0 {
			// Process each tool call
			for id, toolCall := range *toolCalls {
				toolMessage, err := c.processToolCall(id, toolCall)
				if err != nil {
					Warnf("Processing tool call: %v\n", err)
					continue
				}
				// Add the tool response to the conversation
				convo.PushMessage(toolMessage)
			}
			// Continue the conversation recursively
		} else {
			// No function call and no model content
			break
		}
	}

	// Add queries to the output if any
	if len(c.queries) > 0 {
		q := "\n\n" + RetrieveQueries(c.queries)
		c.proc <- StreamNotify{Status: StatusData, Data: q}
	}
	// Add references to the output if any
	if len(c.references) > 0 {
		refs := "\n\n" + RetrieveReferences(c.references)
		c.proc <- StreamNotify{Status: StatusData, Data: refs}
	}
	// No more message
	// Save the conversation
	err := convo.Save()
	if err != nil {
		return fmt.Errorf("failed to save conversation: %v", err)
	}
	c.proc <- StreamNotify{Status: StatusFinished}
	return nil
}

// processStream processes the stream and collects tool calls
func (c *OpenChat) processStream(stream *utils.ChatCompletionStreamReader) (*model.ChatCompletionMessage, *map[string]model.ToolCall, error) {
	assistantMessage := model.ChatCompletionMessage{
		Role: model.ChatMessageRoleAssistant,
		Name: Ptr(""),
	}
	toolCalls := make(map[string]model.ToolCall)
	contentBuffer := strings.Builder{}
	reasoningBuffer := strings.Builder{}
	lastCallId := ""

	state := stateNormal
	for {
		response, err := stream.Recv()
		if err == io.EOF {
			c.proc <- StreamNotify{Status: StatusData, Data: "\n"}
			break
		}
		if err != nil {
			return nil, nil, err
		}

		// Handle regular content
		if len(response.Choices) > 0 {
			delta := (response.Choices[0].Delta)

			// State transitions
			switch state {
			case stateNormal:
				if delta.ReasoningContent != nil {
					c.proc <- StreamNotify{Status: StatusReasoning, Data: ""}
					state = stateReasoning
				}
			case stateReasoning:
				if delta.ReasoningContent == nil {
					c.proc <- StreamNotify{Status: StatusReasoningOver, Data: ""}
					state = stateNormal
				}
			}

			if delta.ReasoningContent != nil {
				// For reasoning model
				text := *delta.ReasoningContent
				reasoningBuffer.WriteString(text)
				c.proc <- StreamNotify{Status: StatusReasoningData, Data: text}
			} else if delta.Content != "" {
				text := delta.Content
				contentBuffer.WriteString(text)
				c.proc <- StreamNotify{Status: StatusData, Data: text}
			}
		}

		// Handle tool calls in the stream
		if len(response.Choices[0].Delta.ToolCalls) > 0 {
			for _, toolCall := range response.Choices[0].Delta.ToolCalls {
				id := toolCall.ID
				functionName := toolCall.Function.Name

				// Skip if not our expected function
				// Because some model made up function name
				if functionName != "" && functionName != "web_search" {
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

	// Update the assistant reasoning message
	reasoning_content := reasoningBuffer.String()
	if reasoning_content != "" {
		assistantMessage.ReasoningContent = &reasoning_content
	}
	// Set the content of the assistant message
	assistantMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(contentBuffer.String()),
	}

	// Add tool calls to the assistant message if there are any
	if len(toolCalls) > 0 {
		var assistantToolCalls []*model.ToolCall
		for _, tc := range toolCalls {
			assistantToolCalls = append(assistantToolCalls, &tc)
		}
		assistantMessage.ToolCalls = assistantToolCalls
	}

	return &assistantMessage, &toolCalls, nil
}

// processToolCall processes a single tool call and returns a tool response message
func (c *OpenChat) processToolCall(id string, toolCall model.ToolCall) (*model.ChatCompletionMessage, error) {
	// Parse the query from the arguments
	var argsMap map[string]interface{}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &argsMap); err != nil {
		return nil, fmt.Errorf("error parsing arguments: %v", err)
	}

	query, ok := argsMap["query"].(string)
	if !ok {
		return nil, fmt.Errorf("query not found in arguments")
	}

	Debugf("\nFunction Calling: %s(%+v)\n", toolCall.Function.Name, query)
	c.proc <- StreamNotify{Status: StatusFunctionCalling, Data: ""}

	// Call the search function
	engine := GetSearchEngine()
	var data map[string]any
	var err error
	switch engine {
	case GoogleSearchEngine:
		// Use Google Search Engine
		data, err = GoogleSearch(query)
	case BingSearchEngine:
		// Use Bing Search Engine
		data, err = BingSearch(query)
	case TavilySearchEngine:
		// Use Tavily Search Engine
		data, err = TavilySearch(query)
	case NoneSearchEngine:
		// Use None Search Engine
		data, err = NoneSearch(query)
	default:
	}

	if err != nil {
		c.proc <- StreamNotify{Status: StatusFunctionCallingOver, Data: ""}
		<-c.proceed
		Warnf("Performing search: %v", err)
		return nil, fmt.Errorf("error performing search: %v", err)
	}
	// keep the search results for references
	c.queries = append(c.queries, query)
	c.references = append(c.references, &data)

	// Convert search results to JSON string
	resultsJSON, err := json.Marshal(data)
	if err != nil {
		// TODO: Potentially send an error FunctionResponse back to the model
		c.proc <- StreamNotify{Status: StatusFunctionCallingOver, Data: ""}
		<-c.proceed
		return nil, fmt.Errorf("error marshaling results: %v", err)
	}

	c.proc <- StreamNotify{Status: StatusFunctionCallingOver, Data: ""}
	<-c.proceed
	// Create and return the tool response message
	return &model.ChatCompletionMessage{
		Role: model.ChatMessageRoleTool,
		Content: &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(string(resultsJSON)),
		}, Name: Ptr(""),
		ToolCallID: id,
	}, nil
}
