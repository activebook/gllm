package service

import (
	"context"
	"fmt"

	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"github.com/google/generative-ai-go/genai"
)

func generateGeminiStreamChan(apiKey, modelName, systemPrompt, userPrompt string, temperature float32, images []*ImageData, ch chan<- StreamNotify) error {
	// Setup the Gemini client
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		ch <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("Failed to create client: %v", err)}
		return err
	}
	defer client.Close()

	// Create the model and generate content
	model := client.GenerativeModel(modelName)

	// Configure Model Parameters
	// System Instruction (System Prompt)
	if systemPrompt != "" {
		model.SystemInstruction = genai.NewUserContent(genai.Text(systemPrompt))
	}
	model.SetTemperature(temperature)

	parts := []genai.Part{genai.Text(userPrompt)}
	for _, imageData := range images {
		// Check if the image data is empty
		if imageData != nil {
			// Convert the image data to a blob
			parts = append(parts, genai.ImageData(imageData.Format(), imageData.Data()))
		}
	}

	// Signal that streaming has started
	ch <- StreamNotify{Status: StatusStarted}

	// Because gemini wouldn't show reasoning content, so we need to wait here
	ch <- StreamNotify{Status: StatusReasoning, Data: ""}

	iter := model.GenerateContentStream(ctx, parts...)

	ch <- StreamNotify{Status: StatusReasoningOver, Data: ""}

	// Stream the responses
	for {
		resp, err := iter.Next()
		if err == iterator.Done {
			// Signal that streaming is complete
			ch <- StreamNotify{Status: StatusFinished}
			return nil
		}

		if err != nil {
			ch <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("Generation error: %v", err)}
			return err
		}

		// Process and send content
		for _, candidate := range resp.Candidates {
			for _, part := range candidate.Content.Parts {
				if textPart, ok := part.(genai.Text); ok {
					ch <- StreamNotify{Status: StatusData, Data: string(textPart)}
				}
			}
		}
	}
}

// Functions that start with lowercase letters (like printSection) are unexported and only visible within the same package.
// Functions that start with uppercase letters (like PrintSection) are exported and can be used by other packages that import your package.
// generateStreamText connects to the Google AI API and streams the generated text.
func generateGeminiStream(apiKey, modelName, systemPrompt, userPrompt string, temperature float32) error {
	// 1. Initialize the Client
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer client.Close() // Ensure client resources are released

	// 2. Select the Model
	model := client.GenerativeModel(modelName)

	// 3. Configure Model Parameters
	// System Instruction (System Prompt)
	if systemPrompt != "" {
		model.SystemInstruction = genai.NewUserContent(genai.Text(systemPrompt))
	}
	model.SetTemperature(temperature)

	// 4. Prepare the User Prompt Content
	promptContent := genai.Text(userPrompt)

	// 5. Initiate Streaming Generation
	iter := model.GenerateContentStream(ctx, promptContent)

	// 6. Process the Stream
	for {
		resp, err := iter.Next()
		if err == iterator.Done {
			break // End of stream
		}
		if err != nil {
			return fmt.Errorf("streaming iteration failed: %w", err)
		}

		// Extract and print text from the response chunk
		// Responses usually have one candidate for streaming.
		if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
			for _, part := range resp.Candidates[0].Content.Parts {
				if text, ok := part.(genai.Text); ok {
					fmt.Print(text) // Print the chunk immediately without a newline
				} else {
					//log.Printf("Warning: Received non-text part: %T", part)
				}
			}
		} else {
			// This might happen if the response is blocked due to safety settings,
			// or if there's an issue. Check resp.PromptFeedback if needed.

			//log.Printf("Warning: Received response chunk with no candidates or content.")
			if resp.PromptFeedback != nil {
				//log.Printf("Prompt Feedback: BlockReason=%v, SafetyRatings=%v", resp.PromptFeedback.BlockReason, resp.PromptFeedback.SafetyRatings)
			}
		}
	}

	// Add a final newline for cleaner terminal output after the stream finishes
	fmt.Println()

	return nil
}
