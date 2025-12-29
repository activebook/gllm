package service

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

var (
	// RoleColors for message roles (initialized in init)
	RoleColors map[string]string

	// ContentTypeColors for special content (initialized in init)
	ContentTypeColors map[string]string
)

func init() {
	// Initialize the maps after the color variables are populated by color.go's init()
	RoleColors = map[string]string{
		"system":    roleSystemColor,
		"user":      roleUserColor,
		"assistant": roleAssistantColor,
		"model":     roleAssistantColor,
		"function":  toolCallColor,
		"tool":      toolCallColor,
	}

	ContentTypeColors = map[string]string{
		"function_call":     toolCallColor,
		"function_response": toolResponseColor,
		"image":             mediaColor,
		"file_data":         mediaColor,
	}
}

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
		return ModelProviderUnknown
	}

	if len(messages) == 0 {
		return ModelProviderUnknown
	}

	// Try to detect Gemini format
	var geminiMsg GeminiMessage
	if err := json.Unmarshal(messages[0], &geminiMsg); err == nil {
		if geminiMsg.Role != "" && len(geminiMsg.Parts) > 0 {
			return ModelProviderGemini
		}
	}

	// Try to detect OpenAI format
	var openaiMsg OpenAIMessage
	if err := json.Unmarshal(messages[0], &openaiMsg); err == nil {
		if openaiMsg.Role != "" {
			return ModelProviderOpenAI
		}
	}

	return ModelProviderUnknown
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
	fmt.Printf("  %sMessages: %d%s\n", resetColor, len(messages), resetColor)

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

	fmt.Printf("  %sUser messages: %d%s\n", RoleColors["user"], userCount, resetColor)
	fmt.Printf("  %sModel responses: %d%s\n", RoleColors["model"], modelCount, resetColor)
	if functionCallCount > 0 {
		fmt.Printf("  %sFunction calls: %d%s\n", ContentTypeColors["function_call"], functionCallCount, resetColor)
	}
	if functionResponseCount > 0 {
		fmt.Printf("  %sFunction responses: %d%s\n", ContentTypeColors["function_response"], functionResponseCount, resetColor)
	}
	if imageCount > 0 {
		fmt.Printf("  %sImages: %d%s\n", ContentTypeColors["image"], imageCount, resetColor)
	}
	if fileCount > 0 {
		fmt.Printf("  %sFiles: %d%s\n", ContentTypeColors["file_data"], fileCount, resetColor)
	}

	// Conversation preview with colors
	if len(messages) > 0 {
		fmt.Println("\nConversation Preview:")
		messageLimit := min(msgCount, len(messages))
		// Adjust start index to show recent messages
		start := len(messages) - messageLimit
		if start > 0 {
			fmt.Printf("  %s... (%d) old messages ...%s\n", greyColor, start, resetColor)
			fmt.Println()
		}
		for i := start; i < len(messages); i++ {
			msg := messages[i]
			// Apply color to role, default to no color if role not found
			roleColor := RoleColors[msg.Role]
			if roleColor == "" {
				roleColor = ""
			}
			fmt.Printf("  %s%s%s: ", roleColor, msg.Role, resetColor)

			if len(msg.Parts) > 0 {
				for j, part := range msg.Parts {
					if j > 0 {
						fmt.Print(" + ")
					}
					switch {
					case part.FunctionCall != nil:
						fmt.Printf("%s[Function call: %s]%s", ContentTypeColors["function_call"], part.FunctionCall.Name, resetColor)
						if len(part.FunctionCall.Arguments) > 0 {
							argStr, _ := json.Marshal(part.FunctionCall.Arguments)
							fmt.Printf(" args: %s", TruncateString(string(argStr), msgLength))
						}
					case part.FunctionResponse != nil:
						fmt.Printf("%s[Function response]%s", ContentTypeColors["function_response"], resetColor)
						respPreview, _ := json.Marshal(part.FunctionResponse.Response)
						fmt.Printf(" data: %s", TruncateString(string(respPreview), msgLength))
					case part.InlineData != nil:
						mimeType := part.InlineData.MimeType
						if strings.HasPrefix(mimeType, "image/") {
							fmt.Printf("%s[Image content]%s", ContentTypeColors["image"], resetColor)
						} else {
							fmt.Printf("%s[File]%s", ContentTypeColors["file_data"], resetColor)
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
			fmt.Printf("  %s... and %d old messages before%s\n", greyColor, len(messages)-messageLimit, resetColor)
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
	fmt.Printf("  %sMessages: %d%s\n", resetColor, len(messages), resetColor)

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

	fmt.Printf("  %sSystem messages: %d%s\n", RoleColors["system"], systemCount, resetColor)
	fmt.Printf("  %sUser messages: %d%s\n", RoleColors["user"], userCount, resetColor)
	fmt.Printf("  %sAssistant responses: %d%s\n", RoleColors["assistant"], assistantCount, resetColor)
	if functionCallCount > 0 {
		fmt.Printf("  %sFunction/tool calls: %d%s\n", ContentTypeColors["function_call"], functionCallCount, resetColor)
	}
	if functionResponseCount > 0 {
		fmt.Printf("  %sFunction/tool responses: %d%s\n", ContentTypeColors["function_response"], functionResponseCount, resetColor)
	}
	if imageCount > 0 {
		fmt.Printf("  %sImages: %d%s\n", ContentTypeColors["image"], imageCount, resetColor)
	}

	// Conversation preview with colors
	if len(messages) > 0 {
		fmt.Println("\nConversation Preview: Recent")
		messageLimit := min(msgCount, len(messages))
		// Adjust start index to show recent messages
		start := len(messages) - messageLimit
		if start > 0 {
			fmt.Printf("  %s... (%d) old messages ...%s\n", greyColor, start, resetColor)
			fmt.Println()
		}
		for i := start; i < len(messages); i++ {
			msg := messages[i]
			// Apply color to role
			roleColor := RoleColors[msg.Role]
			if roleColor == "" {
				roleColor = ""
			}
			fmt.Printf("  %s%s%s", roleColor, msg.Role, resetColor)

			if msg.Name != "" {
				fmt.Printf(" (%s)", msg.Name)
			}
			fmt.Print(": ")

			// Output the reasoning content if it exists
			if msg.ReasonContent != "" {
				fmt.Printf("\n    %sThinking ↓%s", completeColor, resetColor)
				fmt.Printf("\n    %s%s%s", inReasoningColor, TruncateString(msg.ReasonContent, msgLength), resetColor)
				fmt.Printf("\n    %s✓%s\n", completeColor, resetColor)
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
							switch contentType {
							case "text":
								if text, ok := contentMap["text"].(string); ok {
									fmt.Printf("text (%s)", TruncateString(text, msgLength))
								}
							case "image_url":
								fmt.Printf("%simage%s", ContentTypeColors["image"], resetColor)
							default:
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
				fmt.Printf(" %s[Function call: %s]%s", ContentTypeColors["function_call"], msg.FunctionCall.Name, resetColor)
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
				fmt.Printf("]%s", resetColor)
			}

			// Tool response details
			if msg.ToolCallId != "" {
				fmt.Printf(" %s[Response to tool call: %s]%s", ContentTypeColors["function_response"], msg.ToolCallId, resetColor)
			}

			fmt.Println()
			fmt.Println()
		}

		if len(messages) > messageLimit {
			fmt.Printf("  %s... and %d old messages before%s\n", greyColor, len(messages)-messageLimit, resetColor)
		}
	}
}
