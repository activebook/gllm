package service

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

var (
	// RoleColors for message roles
	RoleColors = map[string]string{
		"message":   "\033[93m", // Bright Yellow
		"system":    "\033[33m", // Yellow
		"user":      "\033[32m", // Green
		"assistant": "\033[34m", // Blue
		"model":     "\033[34m", // Blue (Gemini equivalent to assistant)
		"function":  "\033[36m", // Cyan
		"tool":      "\033[36m", // Cyan (OpenAI tool responses)
	}

	// ContentTypeColors for special content
	ContentTypeColors = map[string]string{
		"function_call":     "\033[35m", // Magenta
		"function_response": "\033[35m", // Magenta
		"image":             "\033[31m", // Red
		"file_data":         "\033[31m", // Red
	}

	// Reset code to end coloring
	ResetColor = "\033[0m"

	// Gray for additional info like "... and X more messages"
	GrayColor = "\033[90m"
)

// Gemini message format
type GeminiMessage struct {
	Role  string       `json:"role"` // user, model
	Parts []GeminiPart `json:"parts"`
}

type GeminiPart struct {
	Type             string                  `json:"type"` // text, function_call, function_response, image, file_data, etc.
	Text             string                  `json:"text,omitempty"`
	Name             string                  `json:"name,omitempty"`             // for function calls
	Args             map[string]interface{}  `json:"args,omitempty"`             // for function calls
	InlineData       *GeminiInlineData       `json:"inlineData,omitempty"`       // for inline images
	FunctionCall     *GeminiFunctionCall     `json:"functionCall,omitempty"`     // for function responses
	FunctionResponse *GeminiFunctionResponse `json:"functionResponse,omitempty"` // for function responses
}

type GeminiFunctionCall struct {
	Name      string                 `json:"name,omitempty"`
	Arguments map[string]interface{} `json:"args,omitempty"` // JSON string
}

type GeminiFunctionResponse struct {
	Name     string                 `json:"name,omitempty"`
	Response map[string]interface{} `json:"response,omitempty"` // JSON string
}

type GeminiInlineData struct {
	MimeType string `json:"mimeType,omitempty"`
	Data     string `json:"data,omitempty"`
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
func DisplayGeminiConversationLog(data []byte, msgCount int, msgLength int) {
	var messages []GeminiMessage
	if err := json.Unmarshal(data, &messages); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing Gemini messages: %v\n", err)
		return
	}

	// Summary section (keeping it simple for now)
	fmt.Println("Summary:")
	fmt.Printf("  %sMessages: %d%s\n", ResetColor, len(messages), resetColor)

	var userCount, modelCount, functionCallCount, functionResponseCount, imageCount, fileCount int
	for _, msg := range messages {
		switch msg.Role {
		case "user":
			userCount++
		case "model":
			modelCount++
		}
		for _, part := range msg.Parts {
			switch {
			case part.FunctionCall != nil:
				functionCallCount++
			case part.FunctionResponse != nil:
				functionResponseCount++
			case part.InlineData != nil:
				mimeType := part.InlineData.MimeType
				if strings.HasPrefix(mimeType, "image/") {
					imageCount++
				} else {
					fileCount++
				}
			}
		}
	}

	fmt.Printf("  %sUser messages: %d%s\n", RoleColors["user"], userCount, ResetColor)
	fmt.Printf("  %sModel responses: %d%s\n", RoleColors["model"], modelCount, ResetColor)
	if functionCallCount > 0 {
		fmt.Printf("  %sFunction calls: %d%s\n", ContentTypeColors["function_call"], functionCallCount, ResetColor)
	}
	if functionResponseCount > 0 {
		fmt.Printf("  %sFunction responses: %d%s\n", ContentTypeColors["function_response"], functionResponseCount, ResetColor)
	}
	if imageCount > 0 {
		fmt.Printf("  %sImages: %d%s\n", ContentTypeColors["image"], imageCount, ResetColor)
	}
	if fileCount > 0 {
		fmt.Printf("  %sFiles: %d%s\n", ContentTypeColors["file_data"], fileCount, ResetColor)
	}

	// Conversation preview with colors
	if len(messages) > 0 {
		fmt.Println("\nConversation Preview:")
		messageLimit := min(msgCount, len(messages))
		// Adjust start index to show recent messages
		start := len(messages) - messageLimit
		if start > 0 {
			fmt.Printf("  %s... (%d) old messages ...%s\n", GrayColor, start, resetColor)
			fmt.Println()
		}
		for i := start; i < len(messages); i++ {
			msg := messages[i]
			// Apply color to role, default to no color if role not found
			roleColor := RoleColors[msg.Role]
			if roleColor == "" {
				roleColor = ""
			}
			fmt.Printf("  %s%s%s: ", roleColor, msg.Role, ResetColor)

			if len(msg.Parts) > 0 {
				for j, part := range msg.Parts {
					if j > 0 {
						fmt.Print(" + ")
					}
					switch {
					case part.FunctionCall != nil:
						fmt.Printf("%s[Function call: %s]%s", ContentTypeColors["function_call"], part.FunctionCall.Name, ResetColor)
						if len(part.FunctionCall.Arguments) > 0 {
							argStr, _ := json.Marshal(part.FunctionCall.Arguments)
							fmt.Printf(" args: %s", TruncateString(string(argStr), msgLength))
						}
					case part.FunctionResponse != nil:
						fmt.Printf("%s[Function response]%s", ContentTypeColors["function_response"], ResetColor)
						respPreview, _ := json.Marshal(part.FunctionResponse.Response)
						fmt.Printf(" data: %s", TruncateString(string(respPreview), msgLength))
					case part.InlineData != nil:
						mimeType := part.InlineData.MimeType
						if strings.HasPrefix(mimeType, "image/") {
							fmt.Printf("%s[Image content]%s", ContentTypeColors["image"], ResetColor)
						} else {
							fmt.Printf("%s[File]%s", ContentTypeColors["file_data"], ResetColor)
						}
					default:
						if part.Text != "" {
							// For gemini2, there is not a specific type for text, so we use the default text type
							preview := TruncateString(part.Text, msgLength)
							fmt.Printf("%s", preview)
						} else {
							fmt.Printf("[%s content]", part.Type)
						}
					}
					fmt.Println()
				}
				fmt.Println()
			} else {
				fmt.Println("[Empty message]")
			}
		}

		if len(messages) > messageLimit {
			fmt.Printf("  %s... and %d old messages before%s\n", GrayColor, len(messages)-messageLimit, ResetColor)
		}
	}
}

// Display summary of OpenAI conversation
func DisplayOpenAIConversationLog(data []byte, msgCount int, msgLength int) {
	var messages []OpenAIMessage
	if err := json.Unmarshal(data, &messages); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing OpenAI messages: %v\n", err)
		return
	}

	// Summary section
	fmt.Println("Summary:")
	fmt.Printf("  %sMessages: %d%s\n", ResetColor, len(messages), ResetColor)

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
		if msg.FunctionCall != nil {
			functionCallCount++
		}
		if len(msg.ToolCalls) > 0 {
			functionCallCount += len(msg.ToolCalls)
		}
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

	fmt.Printf("  %sSystem messages: %d%s\n", RoleColors["system"], systemCount, ResetColor)
	fmt.Printf("  %sUser messages: %d%s\n", RoleColors["user"], userCount, ResetColor)
	fmt.Printf("  %sAssistant responses: %d%s\n", RoleColors["assistant"], assistantCount, ResetColor)
	if functionCallCount > 0 {
		fmt.Printf("  %sFunction/tool calls: %d%s\n", ContentTypeColors["function_call"], functionCallCount, ResetColor)
	}
	if functionResponseCount > 0 {
		fmt.Printf("  %sFunction/tool responses: %d%s\n", ContentTypeColors["function_response"], functionResponseCount, ResetColor)
	}
	if imageCount > 0 {
		fmt.Printf("  %sImages: %d%s\n", ContentTypeColors["image"], imageCount, ResetColor)
	}

	// Conversation preview with colors
	if len(messages) > 0 {
		fmt.Println("\nConversation Preview: Recent")
		messageLimit := min(msgCount, len(messages))
		// Adjust start index to show recent messages
		start := len(messages) - messageLimit
		if start > 0 {
			fmt.Printf("  %s... (%d) old messages ...%s\n", GrayColor, start, resetColor)
			fmt.Println()
		}
		for i := start; i < len(messages); i++ {
			msg := messages[i]
			// Apply color to role
			roleColor := RoleColors[msg.Role]
			if roleColor == "" {
				roleColor = ""
			}
			fmt.Printf("  %s%s%s", roleColor, msg.Role, ResetColor)

			if msg.Name != "" {
				fmt.Printf(" (%s)", msg.Name)
			}
			fmt.Print(": ")

			// Output the reasoning content if it exists
			if msg.ReasonContent != "" {
				fmt.Printf("\n    %sThinking ↓%s", completeColor, ResetColor)
				fmt.Printf("\n    %s%s%s", inReasoningColor, TruncateString(msg.ReasonContent, msgLength), ResetColor)
				fmt.Printf("\n    %s✓%s\n", completeColor, ResetColor)
				fmt.Printf("    ")
			}

			switch content := msg.Content.(type) {
			case string:
				preview := TruncateString(content, msgLength)
				fmt.Printf("%s", preview)
			case []interface{}:
				fmt.Print("[Multimodal content: ")
				for j, item := range content {
					if j > 0 {
						fmt.Print(", ")
					}
					if contentMap, ok := item.(map[string]interface{}); ok {
						if contentType, ok := contentMap["type"].(string); ok {
							if contentType == "text" {
								if text, ok := contentMap["text"].(string); ok {
									fmt.Printf("text (%s)", TruncateString(text, msgLength))
								}
							} else if contentType == "image_url" {
								fmt.Printf("%simage%s", ContentTypeColors["image"], ResetColor)
							} else {
								fmt.Print(contentType)
							}
						}
					}
				}
				fmt.Print("]")
			case nil:
				if msg.Role == "assistant" {
					fmt.Print("[No text content]")
				}
			default:
				fmt.Print("[Unknown content format]")
			}

			// Function call details
			if msg.FunctionCall != nil {
				fmt.Printf(" %s[Function call: %s]%s", ContentTypeColors["function_call"], msg.FunctionCall.Name, ResetColor)
				if msg.FunctionCall.Arguments != "" {
					fmt.Printf(" args: %s", TruncateString(msg.FunctionCall.Arguments, msgLength))
				}
			}

			// Tool call details
			if len(msg.ToolCalls) > 0 {
				fmt.Printf(" %s[Tool calls: ", ContentTypeColors["function_call"])
				for j, tool := range msg.ToolCalls {
					if j > 0 {
						fmt.Print(", ")
					}
					fmt.Printf("%s (id: %s)", tool.Function.Name, tool.Id)
				}
				fmt.Printf("]%s", ResetColor)
			}

			// Tool response details
			if msg.ToolCallId != "" {
				fmt.Printf(" %s[Response to tool call: %s]%s", ContentTypeColors["function_response"], msg.ToolCallId, ResetColor)
			}

			fmt.Println()
			fmt.Println()
		}

		if len(messages) > messageLimit {
			fmt.Printf("  %s... and %d old messages before%s\n", GrayColor, len(messages)-messageLimit, ResetColor)
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
