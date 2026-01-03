package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
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
		"reasoning":         reasoningColor,
		"reasoning_content": inReasoningColor,
		"reset":             resetColor,
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
	Thought          bool                    `json:"thought,omitempty"`          // for reasoning
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
	Role          string `json:"role"`    // system, user, assistant
	Content       string `json:"content"` // can be string or array for multimodal content
	MultiContent  []OpenAIContentItem
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

// Anthropic message format
type AnthropicMessage struct {
	Role    string                  `json:"role"`
	Content []AnthropicContentBlock `json:"content"`
}

type AnthropicContentBlock struct {
	Type   string      `json:"type"` // text, tool_use, tool_result, thinking
	Text   string      `json:"text,omitempty"`
	Source interface{} `json:"source,omitempty"` // for image
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

	// Try to detect OpenAI format (fallback)
	var openaiMsg OpenAIMessage
	if err := json.Unmarshal(messages[0], &openaiMsg); err == nil {
		// OpenAI messages must have a role and content or multi-content
		if openaiMsg.Role != "" && (openaiMsg.Content != "" || openaiMsg.MultiContent != nil) {
			return ModelProviderOpenAI
		}
	}

	// Try to detect Gemini format
	var geminiMsg GeminiMessage
	if err := json.Unmarshal(messages[0], &geminiMsg); err == nil {
		// Gemini messages must have a role and parts array
		if geminiMsg.Role != "" && len(geminiMsg.Parts) > 0 {
			return ModelProviderGemini
		}
	}

	// Try to detect Anthropic format
	var anthropicMsg AnthropicMessage
	if err := json.Unmarshal(messages[0], &anthropicMsg); err == nil {
		// Anthropic messages must have a role and content must be an array of blocks
		if anthropicMsg.Role != "" && len(anthropicMsg.Content) > 0 {
			return ModelProviderAnthropic
		}
	}

	return ModelProviderUnknown
}

// styleEachRune applies color to each rune individually except newlines.
// This ensures color is preserved across terminal wrapping and scrolling.
func styleEachRune(text string, color string) string {
	// trim leading and trailing newlines
	text = strings.Trim(text, "\n")
	var sb strings.Builder
	reset := resetColor
	for _, r := range text {
		if r == '\n' {
			sb.WriteRune(r)
			continue
		}
		sb.WriteString(color)
		sb.WriteRune(r)
		sb.WriteString(reset)
	}
	return sb.String()
}

// RenderGeminiConversationLog returns a string summary of Gemini conversation
func RenderGeminiConversationLog(data []byte) string {
	var sb strings.Builder
	var messages []GeminiMessage
	if err := json.Unmarshal(data, &messages); err != nil {
		return fmt.Sprintf("Error parsing Gemini messages: %v\n", err)
	}

	// Summary section
	sb.WriteString("Summary:\n")
	sb.WriteString(fmt.Sprintf("  %sMessages: %d%s\n", resetColor, len(messages), resetColor))

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

	sb.WriteString(fmt.Sprintf("  %sUser messages: %d%s\n", RoleColors["user"], userCount, resetColor))
	sb.WriteString(fmt.Sprintf("  %sModel responses: %d%s\n", RoleColors["model"], modelCount, resetColor))
	if functionCallCount > 0 {
		sb.WriteString(fmt.Sprintf("  %sFunction calls: %d%s\n", ContentTypeColors["function_call"], functionCallCount, resetColor))
	}
	if functionResponseCount > 0 {
		sb.WriteString(fmt.Sprintf("  %sFunction responses: %d%s\n", ContentTypeColors["function_response"], functionResponseCount, resetColor))
	}
	if imageCount > 0 {
		sb.WriteString(fmt.Sprintf("  %sImages: %d%s\n", ContentTypeColors["image"], imageCount, resetColor))
	}
	if fileCount > 0 {
		sb.WriteString(fmt.Sprintf("  %sFiles: %d%s\n", ContentTypeColors["file_data"], fileCount, resetColor))
	}

	// Conversation content
	if len(messages) > 0 {
		sb.WriteString("\nConversation Content:\n")
		for _, msg := range messages {
			// Apply color to role
			roleColor := RoleColors[msg.Role]
			if roleColor == "" {
				roleColor = ""
			}
			sb.WriteString(fmt.Sprintf("  %s%s%s: ", roleColor, msg.Role, resetColor))

			if len(msg.Parts) > 0 {
				for j, part := range msg.Parts {
					if j > 0 {
						sb.WriteString("    ")
					}
					switch {
					case part.FunctionCall != nil:
						sb.WriteString(fmt.Sprintf("%s[Function call: %s]%s", ContentTypeColors["function_call"], part.FunctionCall.Name, resetColor))
						if len(part.FunctionCall.Arguments) > 0 {
							argStr, _ := json.MarshalIndent(part.FunctionCall.Arguments, "    ", "  ")
							sb.WriteString(fmt.Sprintf(" args: %s", string(argStr)))
						}
					case part.FunctionResponse != nil:
						sb.WriteString(fmt.Sprintf("%s[Function response]%s", ContentTypeColors["function_response"], resetColor))
						respPreview, _ := json.MarshalIndent(part.FunctionResponse.Response, "    ", "  ")
						sb.WriteString(fmt.Sprintf(" data: %s", string(respPreview)))
					case part.InlineData != nil:
						mimeType := part.InlineData.MimeType
						if strings.HasPrefix(mimeType, "image/") {
							sb.WriteString(fmt.Sprintf("%s[Image content]%s", ContentTypeColors["image"], resetColor))
						} else {
							sb.WriteString(fmt.Sprintf("%s[File]%s", ContentTypeColors["file_data"], resetColor))
						}
					case part.Thought:
						sb.WriteString(fmt.Sprintf("\n    %sThinking ↓%s", ContentTypeColors["reasoning"], ContentTypeColors["reset"]))
						sb.WriteString(fmt.Sprintf("\n    %s", styleEachRune(part.Text, ContentTypeColors["reasoning_content"])))
						sb.WriteString(fmt.Sprintf("\n    %s✓%s\n", ContentTypeColors["reasoning"], ContentTypeColors["reset"]))
					default:
						if part.Text != "" {
							sb.WriteString(part.Text)
						} else {
							sb.WriteString(fmt.Sprintf("[%s content]", part.Type))
						}
					}
				}
				sb.WriteString("\n\n")
			} else {
				sb.WriteString("[Empty message]\n\n")
			}
		}
	}
	return sb.String()
}

// RenderOpenAIConversationLog returns a string summary of OpenAI conversation
func RenderOpenAIConversationLog(data []byte) string {
	var sb strings.Builder
	var messages []OpenAIMessage
	if err := json.Unmarshal(data, &messages); err != nil {
		return fmt.Sprintf("Error parsing OpenAI messages: %v\n", err)
	}

	// Summary section
	sb.WriteString("Summary:\n")
	sb.WriteString(fmt.Sprintf("  %sMessages: %d%s\n", resetColor, len(messages), resetColor))

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

		for _, item := range msg.MultiContent {
			if item.Type == "image_url" {
				imageCount++
			}
		}
	}

	sb.WriteString(fmt.Sprintf("  %sSystem messages: %d%s\n", RoleColors["system"], systemCount, resetColor))
	sb.WriteString(fmt.Sprintf("  %sUser messages: %d%s\n", RoleColors["user"], userCount, resetColor))
	sb.WriteString(fmt.Sprintf("  %sAssistant responses: %d%s\n", RoleColors["assistant"], assistantCount, resetColor))
	if functionCallCount > 0 {
		sb.WriteString(fmt.Sprintf("  %sFunction/tool calls: %d%s\n", ContentTypeColors["function_call"], functionCallCount, resetColor))
	}
	if functionResponseCount > 0 {
		sb.WriteString(fmt.Sprintf("  %sFunction/tool responses: %d%s\n", ContentTypeColors["function_response"], functionResponseCount, resetColor))
	}
	if imageCount > 0 {
		sb.WriteString(fmt.Sprintf("  %sImages: %d%s\n", ContentTypeColors["image"], imageCount, resetColor))
	}

	// Conversation content
	if len(messages) > 0 {
		sb.WriteString("\nConversation Content:\n")
		for _, msg := range messages {
			// Apply color to role
			roleColor := RoleColors[msg.Role]
			if roleColor == "" {
				roleColor = ""
			}
			sb.WriteString(fmt.Sprintf("  %s%s%s", roleColor, msg.Role, resetColor))

			if msg.Name != "" {
				sb.WriteString(fmt.Sprintf(" (%s)", msg.Name))
			}
			sb.WriteString(": ")

			// Output the reasoning content if it exists
			if msg.ReasonContent != "" {
				sb.WriteString(fmt.Sprintf("\n    %sThinking ↓%s", ContentTypeColors["reasoning"], ContentTypeColors["reset"]))
				sb.WriteString(fmt.Sprintf("\n    %s", styleEachRune(msg.ReasonContent, ContentTypeColors["reasoning_content"])))
				sb.WriteString(fmt.Sprintf("\n    %s✓%s\n", ContentTypeColors["reasoning"], ContentTypeColors["reset"]))
			}

			if msg.Content != "" {
				sb.WriteString(msg.Content)
			}

			if len(msg.MultiContent) > 0 {
				sb.WriteString("[Multimodal content: ")
				for j, item := range msg.MultiContent {
					if j > 0 {
						sb.WriteString(", ")
					}
					if item.Type == "text" {
						sb.WriteString(fmt.Sprintf("text (%s)", item.Text))
					}
					if item.Type == "image_url" {
						sb.WriteString(fmt.Sprintf("%simage%s", ContentTypeColors["image"], resetColor))
					}
				}
				sb.WriteString("]")
			}

			// Function call details
			if msg.FunctionCall != nil {
				sb.WriteString(fmt.Sprintf(" %s[Function call: %s]%s", ContentTypeColors["function_call"], msg.FunctionCall.Name, resetColor))
				if msg.FunctionCall.Arguments != "" {
					sb.WriteString(fmt.Sprintf(" args: %s", msg.FunctionCall.Arguments))
				}
			}

			// Tool call details
			if len(msg.ToolCalls) > 0 {
				sb.WriteString(fmt.Sprintf(" %s[Tool calls: ", ContentTypeColors["function_call"]))
				for j, tool := range msg.ToolCalls {
					if j > 0 {
						sb.WriteString(", ")
					}
					sb.WriteString(fmt.Sprintf("%s (id: %s)", tool.Function.Name, tool.Id))
				}
				sb.WriteString(fmt.Sprintf("]%s", resetColor))
			}

			// Tool response details
			if msg.ToolCallId != "" {
				sb.WriteString(fmt.Sprintf(" %s[Response to tool call: %s]%s", ContentTypeColors["function_response"], msg.ToolCallId, resetColor))
			}

			sb.WriteString("\n\n")
		}
	}
	return sb.String()
}

// RenderAnthropicConversationLog returns a string summary of Anthropic conversation
func RenderAnthropicConversationLog(data []byte) string {
	var sb strings.Builder
	var messages []anthropic.MessageParam
	if err := json.Unmarshal(data, &messages); err != nil {
		return fmt.Sprintf("Error parsing Anthropic messages: %v\n", err)
	}

	// Summary section
	sb.WriteString("Summary:\n")
	sb.WriteString(fmt.Sprintf("  %sMessages: %d%s\n", resetColor, len(messages), resetColor))

	var userCount, assistantCount int
	var toolUseCount, toolResultCount int

	for _, msg := range messages {
		switch msg.Role {
		case anthropic.MessageParamRoleUser:
			userCount++
		case anthropic.MessageParamRoleAssistant:
			assistantCount++
		}

		for _, block := range msg.Content {
			if block.OfToolUse != nil {
				toolUseCount++
			} else if block.OfToolResult != nil {
				toolResultCount++
			}
		}
	}

	sb.WriteString(fmt.Sprintf("  %sUser messages: %d%s\n", RoleColors["user"], userCount, resetColor))
	sb.WriteString(fmt.Sprintf("  %sAssistant messages: %d%s\n", RoleColors["assistant"], assistantCount, resetColor))
	if toolUseCount > 0 {
		sb.WriteString(fmt.Sprintf("  %sTool uses: %d%s\n", ContentTypeColors["function_call"], toolUseCount, resetColor))
	}
	if toolResultCount > 0 {
		sb.WriteString(fmt.Sprintf("  %sTool results: %d%s\n", ContentTypeColors["function_response"], toolResultCount, resetColor))
	}

	// Conversation content
	if len(messages) > 0 {
		sb.WriteString("\nConversation Content:\n")
		for _, msg := range messages {
			role := string(msg.Role)

			// Color
			roleColor := RoleColors[role]
			if roleColor == "" {
				roleColor = resetColor
			}
			sb.WriteString(fmt.Sprintf("  %s%s%s: ", roleColor, role, resetColor))

			for j, block := range msg.Content {
				if j > 0 {
					// sb.WriteString(" + ")
				}

				if v := block.OfText; v != nil {
					sb.WriteString(v.Text)
				} else if v := block.OfImage; v != nil {
					sb.WriteString(fmt.Sprintf("%s[Image]%s", ContentTypeColors["image"], resetColor))
				} else if v := block.OfThinking; v != nil {
					sb.WriteString(fmt.Sprintf("\n    %sThinking ↓%s", ContentTypeColors["reasoning"], ContentTypeColors["reset"]))
					sb.WriteString(fmt.Sprintf("\n    %s", styleEachRune(v.Thinking, ContentTypeColors["reasoning_content"])))
					sb.WriteString(fmt.Sprintf("\n    %s✓%s", ContentTypeColors["reasoning"], ContentTypeColors["reset"]))
				} else if v := block.OfRedactedThinking; v != nil {
					sb.WriteString(fmt.Sprintf("\n    %sThinking (Redacted) ↓%s", ContentTypeColors["reasoning"], ContentTypeColors["reset"]))
					sb.WriteString(fmt.Sprintf("\n    %s", styleEachRune(v.Data, ContentTypeColors["reasoning_content"])))
					sb.WriteString(fmt.Sprintf("\n    %s✓%s", ContentTypeColors["reasoning"], ContentTypeColors["reset"]))
				} else if v := block.OfToolUse; v != nil {
					sb.WriteString(fmt.Sprintf(" %s[Tool Use: %s]%s", ContentTypeColors["function_call"], v.Name, resetColor))
					// Input
					inputJSON, _ := json.MarshalIndent(v.Input, "    ", "  ")
					sb.WriteString(fmt.Sprintf(" input: %s", string(inputJSON)))
				} else if v := block.OfToolResult; v != nil {
					sb.WriteString(fmt.Sprintf(" %s[Tool Result: ID=%s]%s", ContentTypeColors["function_response"], v.ToolUseID, resetColor))
					// Content
					contentJSON, _ := json.MarshalIndent(v.Content, "    ", "  ")
					sb.WriteString(fmt.Sprintf(" content: %s", string(contentJSON)))
				} else {
					sb.WriteString("[Unknown Block]")
				}

				if j < len(msg.Content)-1 {
					sb.WriteString("\n    ") // Indent for next block
				}
			}
			sb.WriteString("\n\n")
		}
	}
	return sb.String()
}
