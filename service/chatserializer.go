package service

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/generative-ai-go/genai"
)

// Define a universal message structure
type ChatMessage struct {
	Role    string    `json:"role"`    // "system", "user", or "assistant"
	Content string    `json:"content"` // Message content
	Time    time.Time `json:"time"`    // Timestamp
}

// Define a wrapper structure for the entire chat
type ChatHistory struct {
	SystemPrompt string        `json:"system_prompt,omitempty"` // Store system prompt separately
	Messages     []ChatMessage `json:"messages"`                // Store conversation messages
	ModelName    string        `json:"model_name,omitempty"`    // Store the model used
}

// Function to convert Gemini chat to unified format
func ConvertGeminiHistory(contents []*genai.Content, modelName string, systemPrompt string) ChatHistory {
	history := ChatHistory{
		SystemPrompt: systemPrompt,
		ModelName:    modelName, // Assuming this exists
	}

	for _, msg := range contents {
		role := "user"
		if msg.Role == "model" {
			role = "assistant"
		}

		content := ""
		for _, part := range msg.Parts {
			if text, ok := part.(genai.Text); ok {
				content += string(text)
			}
			// Handle other part types as needed
		}

		history.Messages = append(history.Messages, ChatMessage{
			Role:    role,
			Content: content,
			Time:    time.Now(), // You might want to extract timestamps if available
		})
	}

	return history
}

func SimpleSaveGeminiChatHistory(contents []*genai.Content, path string) error {
	data, err := json.Marshal(contents)
	if err != nil {
		return err
	}
	// To unmarshal later:
	var history []*genai.Content
	if err := json.Unmarshal(data, &history); err != nil {
		log.Fatalf("failed to unmarshal history: %v", err)
	}

	return os.WriteFile(path, data, 0644)
}

func SimpleLoadGeminiChatHistory(path string) ([]*genai.Content, error) {
	// Read from storage
	historyJSON, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Parse back to history structure
	var contentsJSON []genai.Content
	if err := json.Unmarshal(historyJSON, &contentsJSON); err != nil {
		fmt.Println("Error parsing JSON:", err)
		return nil, err
	}
	// Convert intermediate structure to genai.Content
	contents := make([]*genai.Content, 0, len(contentsJSON))
	for _, c := range contentsJSON {
		contents = append(contents, &c)
	}
	return contents, nil
}

// Function to save the unified format
func SaveChatHistory(history ChatHistory) error {
	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile("chat_history.json", data, 0644)
}

func LoadChatHistory(path string) (*ChatHistory, error) {
	// Read from storage
	historyJSON, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Parse back to history structure
	var history ChatHistory
	if err := json.Unmarshal(historyJSON, &history); err != nil {
		return nil, err
	}
	return &history, nil
}

// Function to load from unified format to Gemini chat
func LoadChatHistoryToGemini(history ChatHistory, model *genai.GenerativeModel) ([]*genai.Content, error) {
	// Set system instruction
	if history.SystemPrompt != "" {
		model.SystemInstruction = &genai.Content{
			Parts: []genai.Part{genai.Text(history.SystemPrompt)},
		}
	}

	// Add messages
	contents := make([]*genai.Content, 0, len(history.Messages))
	for _, msg := range history.Messages {
		// content := &genai.Content{
		// 	Parts: []genai.Part{genai.Text(msg.Content)},
		// }

		if msg.Role == "user" {
			// Add user message
			contents = append(contents, &genai.Content{
				Role:  "user",
				Parts: []genai.Part{genai.Text(msg.Content)},
			})
		} else if msg.Role == "assistant" {
			// Add model response
			contents = append(contents, &genai.Content{
				Role:  "model",
				Parts: []genai.Part{genai.Text(msg.Content)},
			})
		}
		// System messages are handled via SystemInstruction, not in history
	}

	return contents, nil
}

// Function to convert to OpenAI format would be similar
