package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	openai "github.com/sashabaranov/go-openai"
)

func generateOpenAIStreamChan(apiKey, endPoint, modelName, systemPrompt, userPrompt string, temperature float32, images []*ImageData, ch chan<- StreamNotify) error {

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

	// Add user message
	userMessage := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: "", // Empty string for multimodal
		MultiContent: []openai.ChatMessagePart{
			{
				Type: "text",
				Text: userPrompt,
			},
		},
	}

	// Add image parts if available
	if len(images) > 0 {
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
		ch <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("failed to create chat completion stream: %v", err)}
		return err
	}
	// IMPORTANT: Always close the stream when done.
	defer stream.Close()

	// Signal that streaming has started
	ch <- StreamNotify{Status: StatusStarted}

	// 5. Process the Stream
	for {
		response, err := stream.Recv()
		// Check for the end of the stream
		if errors.Is(err, io.EOF) {
			// Indicate stream end
			ch <- StreamNotify{Status: StatusFinished}
			return nil // Exit the loop when the stream is done
		}
		// Handle potential errors during streaming
		if err != nil {
			ch <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("error receiving stream response: %v", err)}
			return err
		}

		// Extract and print the text chunk from the response delta
		// For streaming, the actual content is in the Delta field
		if len(response.Choices) > 0 {
			textPart := (response.Choices[0].Delta.Content)
			ch <- StreamNotify{Status: StatusData, Data: string(textPart)}
		}
	}
}

// generateOpenAIStream connects to the OpenAI API and streams the generated text.
func generateOpenAIStream(apiKey, endPoint, modelName, systemPrompt, userPrompt string, temperature float32) error {
	// 1. Initialize the Client
	ctx := context.Background()
	// Create a client config with custom base URL
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = endPoint
	client := openai.NewClientWithConfig(config)

	// 2. Prepare the Messages for Chat Completion
	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: userPrompt,
		},
	}

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
		return fmt.Errorf("failed to create chat completion stream: %w", err)
	}
	// IMPORTANT: Always close the stream when done.
	defer stream.Close()

	// 5. Process the Stream
	for {
		response, err := stream.Recv()
		// Check for the end of the stream
		if errors.Is(err, io.EOF) {
			// fmt.Println("\nStream finished.") // Optional: Indicate stream end
			break // Exit the loop when the stream is done
		}
		// Handle potential errors during streaming
		if err != nil {
			return fmt.Errorf("error receiving stream response: %w", err)
		}

		// Extract and print the text chunk from the response delta
		// For streaming, the actual content is in the Delta field
		if len(response.Choices) > 0 {
			fmt.Print(response.Choices[0].Delta.Content)
		}
	}

	// Add a final newline for cleaner terminal output
	fmt.Println()

	return nil
}
