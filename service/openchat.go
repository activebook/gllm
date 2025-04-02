package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

func generateOpenAIStreamChan(apiKey, endPoint, modelName, systemPrompt, userPrompt string, temperature float32, images []*ImageData) error {

	// 1. Initialize the Client
	ctx := context.Background()
	// Create a client config with custom base URL
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = endPoint
	client := openai.NewClientWithConfig(config)

	// 2. Prepare the Messages for Chat Completion
	// Initialize messages slice with proper capacity
	messages := make([]openai.ChatCompletionMessage, 0, 2)

	// Only add system message if not empty
	if systemPrompt != "" {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		})
	}

	var userMessage openai.ChatCompletionMessage
	// Add image parts if available
	if len(images) > 0 {
		// Add user message
		userMessage = openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: "", // Empty string for multimodal
			MultiContent: []openai.ChatMessagePart{
				{
					Type: openai.ChatMessagePartTypeText,
					Text: userPrompt,
				},
			},
		}
		// Add all images
		for _, img := range images {
			// Skip nil images
			if img == nil {
				continue
			}

			// Create base64 image URL
			base64Data := base64.StdEncoding.EncodeToString(img.Data())
			imageURL := fmt.Sprintf("data:image/%s;base64,%s", img.Format(), base64Data)

			// Create and append image part
			imagePart := openai.ChatMessagePart{
				Type: "image_url",
				ImageURL: &openai.ChatMessageImageURL{
					URL: imageURL,
				},
			}
			userMessage.MultiContent = append(userMessage.MultiContent, imagePart)
		}
	} else {
		// For text only models, add user prompt directly
		// If use MultiContent(for multimodal), it could be error
		userMessage = openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: userPrompt, // only for text models
		}
	}
	messages = append(messages, userMessage)

	// 3. Create the Chat Completion Request for Streaming
	request := openai.ChatCompletionRequest{
		Model:       modelName,
		Messages:    messages,
		Temperature: temperature, // Directly use the float32 value here
		// MaxTokens:   150,         // Optional: limit output length
		// TopP:        1.0,         // Optional: nucleus sampling
		// N:           1,           // How many chat completion choices to generate for each input message.
		Stream: true, // CRITICAL: Enable streaming
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
	reasoning := false
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
			textPart := (response.Choices[0].Delta.Content)
			// For reasoning model, textPart could be empty
			// So we need to check if it's empty
			if textPart == "" {
				if !reasoning {
					proc <- StreamNotify{Status: StatusReasoning, Data: ""}
				}
				reasoning = true
				continue
			}
			if reasoning {
				reasoning = false
				proc <- StreamNotify{Status: StatusReasoningOver, Data: ""}
			}
			proc <- StreamNotify{Status: StatusData, Data: string(textPart)}
		}
	}
}

func generateOpenAIStreamWithSearchChan(apiKey, endPoint, modelName, systemPrompt, userPrompt string, temperature float32, images []*ImageData) error {

	// 1. Initialize the Client
	conversation := NewConversation(
		apiKey,
		endPoint,
		modelName,
	)

	// 2. Prepare the Messages for Chat Completion
	// Only add system message if not empty
	if systemPrompt != "" {
		conversation.messages = append(conversation.messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		})
	}

	var userMessage openai.ChatCompletionMessage
	// Add image parts if available
	if len(images) > 0 {
		// Add user message
		userMessage = openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: "", // Empty string for multimodal
			MultiContent: []openai.ChatMessagePart{
				{
					Type: openai.ChatMessagePartTypeText,
					Text: userPrompt,
				},
			},
		}
		// Add all images
		for _, img := range images {
			// Skip nil images
			if img == nil {
				continue
			}

			// Create base64 image URL
			base64Data := base64.StdEncoding.EncodeToString(img.Data())
			imageURL := fmt.Sprintf("data:image/%s;base64,%s", img.Format(), base64Data)

			// Create and append image part
			imagePart := openai.ChatMessagePart{
				Type: "image_url",
				ImageURL: &openai.ChatMessageImageURL{
					URL: imageURL,
				},
			}
			userMessage.MultiContent = append(userMessage.MultiContent, imagePart)
		}
	} else {
		// For text only models, add user prompt directly
		// If use MultiContent(for multimodal), it could be error
		userMessage = openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: userPrompt, // only for text models
		}
	}
	conversation.messages = append(conversation.messages, userMessage)

	// Process the conversation with recursive tool call handling
	err := conversation.ProcessConversation(0)
	if err != nil {
		proc <- StreamNotify{Status: StatusError}
		log.Printf("Error processing conversation: %v\n", err)
		return fmt.Errorf("error processing conversation: %v", err)
	}
	return nil
}

// Conversation manages the state of an ongoing conversation with an AI assistant
type Conversation struct {
	client        *openai.Client
	ctx           context.Context
	messages      []openai.ChatCompletionMessage
	model         string
	tools         []openai.Tool
	maxRecursions int
}

func getOpenaiSearchTool() openai.Tool {

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

	searchFunc := openai.FunctionDefinition{
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
	searchTool := openai.Tool{
		Type:     openai.ToolTypeFunction,
		Function: &searchFunc,
	}

	return searchTool
}

// NewConversation creates a new conversation manager
func NewConversation(apiKey, baseURL, model string) *Conversation {
	config := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		config.BaseURL = baseURL
	}
	client := openai.NewClientWithConfig(config)

	// Create a tool with the function
	tool := getOpenaiSearchTool()

	return &Conversation{
		client:        client,
		ctx:           context.Background(),
		messages:      []openai.ChatCompletionMessage{},
		model:         model,
		tools:         []openai.Tool{tool},
		maxRecursions: 3, // Limit recursion depth to prevent infinite loops
	}
}

// ProcessConversation processes the conversation, handling tool calls recursively
func (c *Conversation) ProcessConversation(recursionDepth int) error {
	if recursionDepth > c.maxRecursions {
		// Too much calls, force the end
		proc <- StreamNotify{Status: StatusFinished}
		return nil
	}

	// Create the request
	req := openai.ChatCompletionRequest{
		Model:           c.model,
		Messages:        c.messages,
		Stream:          true,
		ReasoningEffort: "high",
		Tools:           c.tools,
	}

	// Make the streaming request
	stream, err := c.client.CreateChatCompletionStream(c.ctx, req)
	if err != nil {
		return fmt.Errorf("stream creation error: %v", err)
	}
	defer stream.Close()

	proc <- StreamNotify{Status: StatusStarted}

	// Process the stream and collect tool calls
	assistantMessage, toolCalls, err := c.processStream(stream)
	if err != nil {
		return fmt.Errorf("error processing stream: %v", err)
	}

	// Add the assistant's message to the conversation
	c.messages = append(c.messages, assistantMessage)

	// If there are tool calls, process them
	if len(toolCalls) > 0 {
		// Process each tool call
		for id, toolCall := range toolCalls {
			result, err := c.processToolCall(id, toolCall)
			if err != nil {
				log.Printf("Error processing tool call: %v\n", err)
				continue
			}

			// Add the tool response to the conversation
			c.messages = append(c.messages, result)
		}

		// Continue the conversation recursively
		return c.ProcessConversation(recursionDepth + 1)
	}

	// No more message
	proc <- StreamNotify{Status: StatusFinished}
	return nil
}

// processStream processes the stream and collects tool calls
func (c *Conversation) processStream(stream *openai.ChatCompletionStream) (openai.ChatCompletionMessage, map[string]openai.ToolCall, error) {
	assistantMessage := openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleAssistant,
	}
	toolCalls := make(map[string]openai.ToolCall)
	contentBuffer := strings.Builder{}
	lastCallId := ""

	for {
		response, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return assistantMessage, toolCalls, err
		}

		// Handle regular content
		if response.Choices[0].Delta.Content != "" {
			content := response.Choices[0].Delta.Content
			contentBuffer.WriteString(content)
			// Print content
			proc <- StreamNotify{Status: StatusData, Data: content}
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

	// Update the assistant message
	assistantMessage.Content = contentBuffer.String()

	// Add tool calls to the assistant message if there are any
	if len(toolCalls) > 0 {
		var assistantToolCalls []openai.ToolCall
		for _, tc := range toolCalls {
			assistantToolCalls = append(assistantToolCalls, tc)
		}
		assistantMessage.ToolCalls = assistantToolCalls
	}

	return assistantMessage, toolCalls, nil
}

// processToolCall processes a single tool call and returns a tool response message
func (c *Conversation) processToolCall(id string, toolCall openai.ToolCall) (openai.ChatCompletionMessage, error) {
	// Parse the query from the arguments
	var argsMap map[string]interface{}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &argsMap); err != nil {
		return openai.ChatCompletionMessage{}, fmt.Errorf("error parsing arguments: %v", err)
	}

	query, ok := argsMap["query"].(string)
	if !ok {
		return openai.ChatCompletionMessage{}, fmt.Errorf("query not found in arguments")
	}

	log.Printf("\nFunction Calling: %s(%+v)\n", toolCall.Function.Name, query)
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
		log.Printf("Error performing search: %v", err)
		return openai.ChatCompletionMessage{}, fmt.Errorf("error performing search: %v", err)
	}

	// Convert search results to JSON string
	resultsJSON, err := json.Marshal(data)
	if err != nil {
		// TODO: Potentially send an error FunctionResponse back to the model
		proc <- StreamNotify{Status: StatusFunctionCallingOver, Data: ""}
		return openai.ChatCompletionMessage{}, fmt.Errorf("error marshaling results: %v", err)
	}

	proc <- StreamNotify{Status: StatusFunctionCallingOver, Data: ""}
	// Create and return the tool response message
	return openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		Content:    string(resultsJSON),
		ToolCallID: id,
	}, nil
}
