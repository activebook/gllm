package service

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const (
	// Model types
	ModelGemini           = "gemini"
	ModelOpenAI           = "openai"
	ModelOpenAICompatible = "openai-compatible"
	ModelUnknown          = "unknown"

	// Content preview length
	MaxPreviewLength = 100
)

// Gemini message format
type GeminiMessage struct {
	Role  string       `json:"role"` // user, model
	Parts []GeminiPart `json:"parts"`
}

type GeminiPart struct {
	Type             string                 `json:"type"` // text, function_call, function_response, image, file_data, etc.
	Text             string                 `json:"text,omitempty"`
	Name             string                 `json:"name,omitempty"`              // for function calls
	Args             map[string]interface{} `json:"args,omitempty"`              // for function calls
	FileData         map[string]interface{} `json:"file_data,omitempty"`         // for file attachments
	InlineData       map[string]interface{} `json:"inline_data,omitempty"`       // for inline images
	FunctionResponse map[string]interface{} `json:"function_response,omitempty"` // for function responses
}

// OpenAI message format with enhanced support for function calls and files
type OpenAIMessage struct {
	Role          string              `json:"role"`    // system, user, assistant
	Content       interface{}         `json:"content"` // can be string or array for multimodal content
	ReasonContent string              `json:"reasoning_content,omitempty"`
	Name          string              `json:"name,omitempty"`
	FunctionCall  *OpenAIFunctionCall `json:"function_call,omitempty"`
	ToolCalls     []OpenAIToolCall    `json:"tool_calls,omitempty"`
	ToolCallId    string              `json:"tool_call_id,omitempty"` // For function response
}

type OpenAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

type OpenAIToolCall struct {
	Id       string             `json:"id"`
	Type     string             `json:"type"` // typically "function"
	Function OpenAIFunctionCall `json:"function"`
}

// OpenAI content item for multimodal messages
type OpenAIContentItem struct {
	Type     string          `json:"type"` // text, image_url, etc.
	Text     string          `json:"text,omitempty"`
	ImageUrl *OpenAIImageURL `json:"image_url,omitempty"`
}

type OpenAIImageURL struct {
	Url string `json:"url"` // Can be data URL or HTTP URL
}

// Detects the conversation provider based on message format
func DetectMessageProvider(data []byte) string {
	// Try to unmarshal as array of messages
	var messages []json.RawMessage
	if err := json.Unmarshal(data, &messages); err != nil {
		return ModelUnknown
	}

	if len(messages) == 0 {
		return ModelUnknown
	}

	// Try to detect Gemini format
	var geminiMsg GeminiMessage
	if err := json.Unmarshal(messages[0], &geminiMsg); err == nil {
		if geminiMsg.Role != "" && len(geminiMsg.Parts) > 0 {
			return ModelGemini
		}
	}

	// Try to detect OpenAI format
	var openaiMsg OpenAIMessage
	if err := json.Unmarshal(messages[0], &openaiMsg); err == nil {
		if openaiMsg.Role != "" {
			return ModelOpenAI
		}
	}

	return ModelUnknown
}

// Display summary of Gemini conversation
func DisplayGeminiConversationLog(data []byte) {
	var messages []GeminiMessage
	if err := json.Unmarshal(data, &messages); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing Gemini messages: %v\n", err)
		return
	}

	fmt.Printf("Messages: %d\n", len(messages))

	// Count message types
	var userCount, modelCount, functionCallCount, functionResponseCount, imageCount, fileCount int
	for _, msg := range messages {
		switch msg.Role {
		case "user":
			userCount++
		case "model":
			modelCount++
		}

		// Check for special content types
		for _, part := range msg.Parts {
			switch part.Type {
			case "function_call":
				functionCallCount++
			case "function_response":
				functionResponseCount++
			case "image":
				imageCount++
			case "file_data":
				fileCount++
			}

			// Also check for inline data which might be images
			if part.InlineData != nil {
				if mimeType, ok := part.InlineData["mime_type"].(string); ok {
					if strings.HasPrefix(mimeType, "image/") {
						imageCount++
					}
				}
			}
		}
	}

	fmt.Printf("User messages: %d\n", userCount)
	fmt.Printf("Model responses: %d\n", modelCount)
	if functionCallCount > 0 {
		fmt.Printf("Function calls: %d\n", functionCallCount)
	}
	if functionResponseCount > 0 {
		fmt.Printf("Function responses: %d\n", functionResponseCount)
	}
	if imageCount > 0 {
		fmt.Printf("Images: %d\n", imageCount)
	}
	if fileCount > 0 {
		fmt.Printf("Files: %d\n", fileCount)
	}

	// Show conversation preview
	if len(messages) > 0 {
		fmt.Println("\nConversation Preview:")
		messageLimit := min(10, len(messages))

		for i := 0; i < messageLimit; i++ {
			msg := messages[i]
			fmt.Printf("  %s: ", msg.Role)

			// Show preview of content
			if len(msg.Parts) > 0 {
				for j, part := range msg.Parts {
					if j > 0 {
						fmt.Print(" + ")
					}

					switch part.Type {
					case "text":
						preview := TruncateString(part.Text, MaxPreviewLength)
						fmt.Printf("%s", preview)
					case "function_call":
						fmt.Printf("[Function call: %s]", part.Name)
						// Print a few args as preview
						if len(part.Args) > 0 {
							argStr, _ := json.Marshal(part.Args)
							fmt.Printf(" args: %s", TruncateString(string(argStr), 30))
						}
					case "function_response":
						fmt.Printf("[Function response]")
						respPreview, _ := json.Marshal(part.Args)
						fmt.Printf(" data: %s", TruncateString(string(respPreview), 30))
					case "image":
						fmt.Printf("[Image content]")
					case "file_data":
						if filename, ok := getFileName(part.FileData); ok {
							fmt.Printf("[File: %s]", filename)
						} else {
							fmt.Printf("[File]")
						}
					default:
						fmt.Printf("[%s content]", part.Type)
					}
				}
				fmt.Println()
			} else {
				fmt.Println("[Empty message]")
			}
		}

		if len(messages) > messageLimit {
			fmt.Printf("  ... and %d more messages\n", len(messages)-messageLimit)
		}
	}
}

// Display summary of OpenAI conversation with enhanced details
func DisplayOpenAIConversationLog(data []byte) {
	var messages []OpenAIMessage
	if err := json.Unmarshal(data, &messages); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing OpenAI messages: %v\n", err)
		return
	}

	fmt.Printf("Messages: %d\n", len(messages))

	// Count message types
	var systemCount, userCount, assistantCount int
	var functionCallCount, functionResponseCount, imageCount int

	for _, msg := range messages {
		switch msg.Role {
		case "system":
			systemCount++
		case "user":
			userCount++
		case "assistant":
			assistantCount++
		case "function", "tool":
			functionResponseCount++
		}

		// Check for function calls
		if msg.FunctionCall != nil {
			functionCallCount++
		}

		// Check for tool calls (newer API)
		if len(msg.ToolCalls) > 0 {
			functionCallCount += len(msg.ToolCalls)
		}

		// Check for images in content
		if contentItems, ok := msg.Content.([]interface{}); ok {
			for _, item := range contentItems {
				if contentMap, ok := item.(map[string]interface{}); ok {
					if contentType, ok := contentMap["type"].(string); ok && contentType == "image_url" {
						imageCount++
					}
				}
			}
		}
	}

	fmt.Printf("System messages: %d\n", systemCount)
	fmt.Printf("User messages: %d\n", userCount)
	fmt.Printf("Assistant responses: %d\n", assistantCount)
	if functionCallCount > 0 {
		fmt.Printf("Function/tool calls: %d\n", functionCallCount)
	}
	if functionResponseCount > 0 {
		fmt.Printf("Function/tool responses: %d\n", functionResponseCount)
	}
	if imageCount > 0 {
		fmt.Printf("Images: %d\n", imageCount)
	}

	// Show conversation preview
	if len(messages) > 0 {
		fmt.Println("\nConversation Preview:")
		messageLimit := min(10, len(messages))

		for i := 0; i < messageLimit; i++ {
			msg := messages[i]
			fmt.Printf("  %s", msg.Role)

			// Show name for function calls if present
			if msg.Name != "" {
				fmt.Printf(" (%s)", msg.Name)
			}
			fmt.Print(": ")

			// Handle different content types
			switch content := msg.Content.(type) {
			case string:
				preview := TruncateString(content, MaxPreviewLength)
				fmt.Printf("%s", preview)
			case []interface{}:
				// Handle multimodal content
				fmt.Print("[Multimodal content: ")
				for j, item := range content {
					if j > 0 {
						fmt.Print(", ")
					}

					if contentMap, ok := item.(map[string]interface{}); ok {
						if contentType, ok := contentMap["type"].(string); ok {
							fmt.Print(contentType)
							if contentType == "text" {
								if text, ok := contentMap["text"].(string); ok {
									fmt.Printf(" (%s)", TruncateString(text, 20))
								}
							}
						}
					}
				}
				fmt.Print("]")
			case nil:
				// Content might be nil for function calls
				if msg.Role == "assistant" {
					fmt.Print("[No text content]")
				}
			default:
				fmt.Print("[Unknown content format]")
			}

			// Show function call details
			if msg.FunctionCall != nil {
				fmt.Printf(" [Function call: %s", msg.FunctionCall.Name)
				if msg.FunctionCall.Arguments != "" {
					fmt.Printf(", args: %s", TruncateString(msg.FunctionCall.Arguments, 30))
				}
				fmt.Print("]")
			}

			// Show tool call details
			if len(msg.ToolCalls) > 0 {
				fmt.Printf(" [Tool calls: ")
				for j, tool := range msg.ToolCalls {
					if j > 0 {
						fmt.Print(", ")
					}
					fmt.Printf("%s (id: %s)", tool.Function.Name, tool.Id)
				}
				fmt.Print("]")
			}

			// Show tool response details
			if msg.ToolCallId != "" {
				fmt.Printf(" [Response to tool call: %s]", msg.ToolCallId)
			}

			fmt.Println()
		}

		if len(messages) > messageLimit {
			fmt.Printf("  ... and %d more messages\n", len(messages)-messageLimit)
		}
	}
}

func getFileName(fileData map[string]interface{}) (string, bool) {
	if fileData == nil {
		return "", false
	}

	// Check common fields that might contain filename
	possibleFields := []string{"file_name", "filename", "name", "title"}
	for _, field := range possibleFields {
		if val, ok := fileData[field].(string); ok && val != "" {
			return val, true
		}
	}

	return "", false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
