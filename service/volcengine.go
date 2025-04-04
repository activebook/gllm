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

func getVolcFilePart(file *FileData) *model.ChatCompletionMessageContentPart {

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

func generateVolcStreamChan(apiKey, endPoint, modelName, systemPrompt, userPrompt string, temperature float32, files []*FileData) error {
	// 1. Initialize the Client
	ctx := context.Background()
	// Create a client config with custom base URL
	client := arkruntime.NewClientWithApiKey(
		apiKey,
		arkruntime.WithTimeout(30*time.Minute),
		arkruntime.WithBaseUrl(endPoint),
	)

	// 2. Prepare the Messages for Chat Completion
	// Initialize messages slice with proper capacity
	messages := make([]*model.ChatCompletionMessage, 0, 2)

	// Only add system message if not empty
	if systemPrompt != "" {
		messages = append(messages, &model.ChatCompletionMessage{
			Role: model.ChatMessageRoleSystem,
			Content: &model.ChatCompletionMessageContent{
				StringValue: volcengine.String(systemPrompt),
			},
		})
	}

	var userMessage *model.ChatCompletionMessage
	// Add user message
	userMessage = &model.ChatCompletionMessage{
		Role: model.ChatMessageRoleUser,
		Content: &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(userPrompt),
		},
	}
	messages = append(messages, userMessage)
	// Add image parts if available
	if len(files) > 0 {
		userMessage = &model.ChatCompletionMessage{
			Role:    model.ChatMessageRoleUser,
			Content: &model.ChatCompletionMessageContent{ListValue: []*model.ChatCompletionMessageContentPart{}},
		}
		// Add all files
		for _, file := range files {
			if file != nil {
				part := getVolcFilePart(file)
				if part != nil {
					userMessage.Content.ListValue = append(userMessage.Content.ListValue, part)
				}
			}
		}
		messages = append(messages, userMessage)
	}

	proc <- StreamNotify{Status: StatusProcessing}

	// 3. Create the Chat Completion Request for Streaming
	request := model.CreateChatCompletionRequest{
		Model:       modelName,
		Messages:    messages,
		Temperature: &temperature, // Directly use the float32 value here
		// MaxTokens:   150,         // Optional: limit output length
		// TopP:        1.0,         // Optional: nucleus sampling
		// N:           1,           // How many chat completion choices to generate for each input message.
	}

	// 4. Initiate Streaming Chat Completion
	stream, err := client.CreateChatCompletionStream(ctx, request)
	if err != nil {
		proc <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("failed to create chat completion stream: %v", err)}
		return err
	}
	// IMPORTANT: Always close the stream when done.
	defer stream.Close()

	// Signal that streaming has started
	proc <- StreamNotify{Status: StatusStarted}

	// 5. Process the Stream
	state := stateNormal
	for {
		response, err := stream.Recv()
		// Check for the end of the stream
		if errors.Is(err, io.EOF) {
			// Indicate stream end
			proc <- StreamNotify{Status: StatusFinished}
			return nil // Exit the loop when the stream is done
		}
		// Handle potential errors during streaming
		if err != nil {
			proc <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("error receiving stream response: %v", err)}
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
					proc <- StreamNotify{Status: StatusReasoning, Data: ""}
					state = stateReasoning
				}
			case stateReasoning:
				if delta.ReasoningContent == nil {
					proc <- StreamNotify{Status: StatusReasoningOver, Data: ""}
					state = stateNormal
				}
			}

			if delta.ReasoningContent != nil {
				// For reasoning model
				text := *delta.ReasoningContent
				proc <- StreamNotify{Status: StatusData, Data: text}
			} else {
				text := delta.Content
				proc <- StreamNotify{Status: StatusData, Data: text}
			}
		}
	}
}

func generateVolcStreamWithSearchChan(apiKey, endPoint, modelName, systemPrompt, userPrompt string, temperature float32, files []*FileData) error {

	// 1. Initialize the Client
	conversation := NewVolcChat(
		apiKey,
		endPoint,
		modelName,
		temperature,
	)

	// 2. Prepare the Messages for Chat Completion
	// Initialize messages slice with proper capacity
	messages := make([]*model.ChatCompletionMessage, 0, 2)

	// Only add system message if not empty
	if systemPrompt != "" {
		messages = append(messages, &model.ChatCompletionMessage{
			Role: model.ChatMessageRoleSystem,
			Content: &model.ChatCompletionMessageContent{
				StringValue: volcengine.String(systemPrompt),
			},
		})
	}

	var userMessage *model.ChatCompletionMessage
	// Add user message
	userMessage = &model.ChatCompletionMessage{
		Role: model.ChatMessageRoleUser,
		Content: &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(userPrompt),
		},
	}
	messages = append(messages, userMessage)
	// Add image parts if available
	if len(files) > 0 {
		userMessage = &model.ChatCompletionMessage{
			Role:    model.ChatMessageRoleUser,
			Content: &model.ChatCompletionMessageContent{ListValue: []*model.ChatCompletionMessageContentPart{}},
		}
		// Add all files
		for _, file := range files {
			if file != nil {
				part := getVolcFilePart(file)
				if part != nil {
					userMessage.Content.ListValue = append(userMessage.Content.ListValue, part)
				}
			}
		}
		messages = append(messages, userMessage)
	}
	conversation.messages = messages

	// Process the conversation with recursive tool call handling
	err := conversation.ProcessVolcChat()
	if err != nil {
		proc <- StreamNotify{Status: StatusError}
		Logf("Error processing conversation: %v\n", err)
		return fmt.Errorf("error processing conversation: %v", err)
	}
	return nil
}

// Conversation manages the state of an ongoing conversation with an AI assistant
type VolcChat struct {
	client        *arkruntime.Client
	ctx           *context.Context
	messages      []*model.ChatCompletionMessage
	model         string
	temperature   float32
	tools         []*model.Tool
	maxRecursions int
	references    []*map[string]interface{} // keep track of the references
}

func getVolcSearchTool() *model.Tool {

	engine := GetSearchEngine()
	switch engine {
	case GoogleSearchEngine:
		// Use Google Search Engine
	case BingSearchEngine:
		// Use Bing Search Engine
	case TavilySearchEngine:
		// Use Tavily Search Engine
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

// NewConversation creates a new conversation manager
func NewVolcChat(apiKey, baseURL, modelName string, temperature float32) *VolcChat {
	// 1. Initialize the Client
	ctx := context.Background()
	// Create a client config with custom base URL
	client := arkruntime.NewClientWithApiKey(
		apiKey,
		arkruntime.WithTimeout(30*time.Minute),
		arkruntime.WithBaseUrl(baseURL),
	)

	// Create a tool with the function
	tool := getVolcSearchTool()

	return &VolcChat{
		client:        client,
		ctx:           &ctx,
		messages:      []*model.ChatCompletionMessage{},
		model:         modelName,
		temperature:   temperature,
		tools:         []*model.Tool{tool},
		references:    make([]*map[string]interface{}, 0, 1),
		maxRecursions: 5, // Limit recursion depth to prevent infinite loops
	}
}

func (c *VolcChat) ProcessVolcChat() error {
	// only allow 3 recursions
	i := 0
	for range c.maxRecursions {
		proc <- StreamNotify{Status: StatusProcessing}

		i++
		Debugf("Processing conversation at times: %d\n", i)

		// Create the request
		req := model.CreateChatCompletionRequest{
			Model:       c.model,
			Temperature: &c.temperature,
			Messages:    c.messages,
			Tools:       c.tools,
		}

		// Make the streaming request
		stream, err := c.client.CreateChatCompletionStream(*c.ctx, req)
		if err != nil {
			return fmt.Errorf("stream creation error: %v", err)
		}
		defer stream.Close()

		proc <- StreamNotify{Status: StatusStarted}

		// Process the stream and collect tool calls
		assistantMessage, toolCalls, err := c.processVolcStream(stream)
		if err != nil {
			return fmt.Errorf("error processing stream: %v", err)
		}

		// Add the assistant's message to the conversation
		c.messages = append(c.messages, assistantMessage)

		// If there are tool calls, process them
		if len(*toolCalls) > 0 {
			// Process each tool call
			for id, toolCall := range *toolCalls {
				result, err := c.processVolcToolCall(id, toolCall)
				if err != nil {
					Logf("Error processing tool call: %v\n", err)
					continue
				}
				// Add the tool response to the conversation
				c.messages = append(c.messages, result)
			}
			// Continue the conversation recursively
		} else {
			// No function call and no model content
			break
		}
	}
	if len(c.references) > 0 {
		refs := "\n\n" + RetrieveReferences(c.references)
		proc <- StreamNotify{Status: StatusData, Data: refs}
	}
	// No more message
	proc <- StreamNotify{Status: StatusFinished}
	return nil
}

// processStream processes the stream and collects tool calls
func (c *VolcChat) processVolcStream(stream *utils.ChatCompletionStreamReader) (*model.ChatCompletionMessage, *map[string]model.ToolCall, error) {
	assistantMessage := model.ChatCompletionMessage{
		Role: model.ChatMessageRoleAssistant,
	}
	toolCalls := make(map[string]model.ToolCall)
	contentBuffer := strings.Builder{}
	lastCallId := ""

	state := stateNormal
	for {
		response, err := stream.Recv()
		if err == io.EOF {
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
					proc <- StreamNotify{Status: StatusReasoning, Data: ""}
					state = stateReasoning
				}
			case stateReasoning:
				if delta.ReasoningContent == nil {
					proc <- StreamNotify{Status: StatusReasoningOver, Data: ""}
					state = stateNormal
				}
			}

			if delta.ReasoningContent != nil {
				// For reasoning model
				text := *delta.ReasoningContent
				contentBuffer.WriteString(text)
				proc <- StreamNotify{Status: StatusData, Data: text}
			} else if delta.Content != "" {
				text := delta.Content
				contentBuffer.WriteString(text)
				proc <- StreamNotify{Status: StatusData, Data: text}
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

	// Update the assistant message
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
func (c *VolcChat) processVolcToolCall(id string, toolCall model.ToolCall) (*model.ChatCompletionMessage, error) {
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
	proc <- StreamNotify{Status: StatusFunctionCalling, Data: ""}

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
	default:
	}

	if err != nil {
		proc <- StreamNotify{Status: StatusFunctionCallingOver, Data: ""}
		Logf("Error performing search: %v", err)
		return nil, fmt.Errorf("error performing search: %v", err)
	}
	// keep the search results for references
	c.references = append(c.references, &data)

	// Convert search results to JSON string
	resultsJSON, err := json.Marshal(data)
	if err != nil {
		// TODO: Potentially send an error FunctionResponse back to the model
		proc <- StreamNotify{Status: StatusFunctionCallingOver, Data: ""}
		return nil, fmt.Errorf("error marshaling results: %v", err)
	}

	proc <- StreamNotify{Status: StatusFunctionCallingOver, Data: ""}
	// Create and return the tool response message
	return &model.ChatCompletionMessage{
		Role: model.ChatMessageRoleTool,
		Content: &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(string(resultsJSON)),
		},
		ToolCallID: id,
	}, nil
}
