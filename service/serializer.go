package service

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/activebook/gllm/data"
	anthropic "github.com/anthropics/anthropic-sdk-go"
	openai "github.com/sashabaranov/go-openai"
	gemini "google.golang.org/genai"
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
		"system":    data.RoleSystemColor,
		"user":      data.RoleUserColor,
		"assistant": data.RoleAssistantColor,
		"model":     data.RoleAssistantColor,
		"function":  data.ToolCallColor,
		"tool":      data.ToolCallColor,
	}

	ContentTypeColors = map[string]string{
		"function_call":     data.ToolCallColor,
		"function_response": data.ToolResponseColor,
		"image":             data.MediaColor,
		"file_data":         data.MediaColor,
		"reasoning":         data.ReasoningActiveColor,
		"reasoning_content": data.ReasoningDoneColor,
		"reset":             data.ResetSeq,
	}
}

// Detects if a message is definitely an OpenAI message
func DetectOpenAIKeyMessage(msg *openai.ChatCompletionMessage) bool {
	if msg.Role == "" {
		return false
	}
	// SystemRole is unique to OpenAI
	if msg.Role == openai.ChatMessageRoleSystem {
		return true
	}
	// ReasoningContent is unique to OpenAI
	if msg.ReasoningContent != "" {
		return true
	}
	// ToolCallID is unique to OpenAI
	if msg.ToolCallID != "" {
		return true
	}
	// ToolCalls is unique to OpenAI
	if len(msg.ToolCalls) > 0 {
		return true
	}
	// ImageURL is unique to OpenAI
	if len(msg.MultiContent) > 0 {
		for _, content := range msg.MultiContent {
			if content.ImageURL != nil {
				return true
			}
		}
	}
	return false
}

// Detects if a message is definitely a Gemini message
func DetectGeminiKeyMessage(msg *gemini.Content) bool {
	if msg.Role == "" {
		return false
	}
	// RoleModel is unique to Gemini
	if msg.Role == gemini.RoleModel {
		return true
	}
	// Parts is unique to Gemini
	if len(msg.Parts) > 0 {
		return true
	}
	return false
}

// Detects if a message is definitely an Anthropic message
func DetectAnthropicKeyMessage(msg *anthropic.MessageParam) bool {
	if msg.Role == "" {
		return false
	}
	if msg.Role != anthropic.MessageParamRoleUser && msg.Role != anthropic.MessageParamRoleAssistant {
		return false
	}
	for _, block := range msg.Content {
		if v := block.OfText; v != nil {
			// For pure text content, Anthropic and OpenAI multimodal messages have identical JSON structure
			// So we need to check for other fields to determine if it's Anthropic
			continue
		} else if v := block.OfImage; v != nil {
			return true
		} else if v := block.OfDocument; v != nil {
			return true
		} else if v := block.OfToolResult; v != nil {
			return true
		} else if v := block.OfToolUse; v != nil {
			return true
		} else if v := block.OfThinking; v != nil {
			return true
		} else if v := block.OfRedactedThinking; v != nil {
			return true
		}
	}
	return false
}

/*
 * Detects the conversation provider based on message format.
 * Supports both JSONL (preferred) and legacy JSON array formats.
 */
func DetectMessageProviderByContent(input []byte) string {
	// Try to unmarshal as array of messages
	var messages []json.RawMessage

	// JSONL format: parse each line as a separate JSON object
	lines := bytes.Split(input, []byte("\n"))
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		if json.Valid(line) {
			messages = append(messages, json.RawMessage(line))
		}
	}

	provider := ModelProviderUnknown
	if len(messages) == 0 {
		return provider
	}

	// Try to detect Gemini format
	var geminiMsg gemini.Content
	for _, msg := range messages {
		if err := json.Unmarshal(msg, &geminiMsg); err == nil {
			// Gemini messages must have a role and parts array
			// If parts length aren't 0, then it must be gemini
			if DetectGeminiKeyMessage(&geminiMsg) {
				provider = ModelProviderGemini
				break
			} else {
				// The first message can detect gemini or not, if not, break
				break
			}
		}
	}
	if provider != ModelProviderUnknown {
		return provider
	}

	// Try to detect Anthropic format
	// Bugfix:
	// The Anthropic SDK has a custom decoder that automatically handles string content by converting it to an array of content blocks.
	// So we cannot rely on Content[] along to detect whether it's anthropic or not
	// Because "content": "hi" would be converted Content[{"text":"hi"}] automatically
	var anthropicMsg anthropic.MessageParam
	for _, msg := range messages {
		if err := json.Unmarshal(msg, &anthropicMsg); err == nil {
			// For Anthropic messages, we must find the first key message
			if DetectAnthropicKeyMessage(&anthropicMsg) {
				provider = ModelProviderAnthropic
				break
			} else if anthropicMsg.Role != anthropic.MessageParamRoleUser && anthropicMsg.Role != anthropic.MessageParamRoleAssistant {
				// If role is not user or assistant, it's not anthropic
				// Remember: anthropic only has two roles, no system and tools
				provider = ModelProviderUnknown
				break
			}
		}
	}
	if provider != ModelProviderUnknown {
		return provider
	}

	// Try to detect OpenAI format (fallback)
	var openaiMsg openai.ChatCompletionMessage
	for _, msg := range messages {
		if err := json.Unmarshal(msg, &openaiMsg); err == nil {
			// OpenAI messages must have a role
			if DetectOpenAIKeyMessage(&openaiMsg) {
				provider = ModelProviderOpenAI
				break
			} else if openaiMsg.Role != "" && (openaiMsg.Content != "" || len(openaiMsg.MultiContent) > 0) {
				// If role exists, check whether it's pure text content
				// If so, we can consider it OpenAICompatible (Pure text content)
				provider = ModelProviderOpenAICompatible
				// don't break, continue to check the next message
			}
		}
	}

	return provider
}

// Detects the conversation provider based on message format using a scanner
// This is more efficient for large files as it doesn't read the entire file into memory
func DetectMessageProvider(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ModelProviderUnknown
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// Increase buffer size for long messages
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	provider := ModelProviderUnknown

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		if !json.Valid(line) {
			continue
		}

		// Gemini Check
		var geminiMsg gemini.Content
		if err := json.Unmarshal(line, &geminiMsg); err == nil {
			if DetectGeminiKeyMessage(&geminiMsg) {
				return ModelProviderGemini
			}
		}

		// Anthropic Check
		var anthropicMsg anthropic.MessageParam
		if err := json.Unmarshal(line, &anthropicMsg); err == nil {
			if DetectAnthropicKeyMessage(&anthropicMsg) {
				return ModelProviderAnthropic
			}
		}

		// OpenAI Check
		var openaiMsg openai.ChatCompletionMessage
		if err := json.Unmarshal(line, &openaiMsg); err == nil {
			if DetectOpenAIKeyMessage(&openaiMsg) {
				return ModelProviderOpenAI
			} else if openaiMsg.Role != "" && (openaiMsg.Content != "" || len(openaiMsg.MultiContent) > 0) {
				return ModelProviderOpenAICompatible
			}
		}
	}

	return provider
}

// styleEachRune applies color to each rune individually except newlines.
// This ensures color is preserved across terminal wrapping and scrolling.
// Added indent parameter to support multi-line indentation.
func styleEachRune(text string, color string, indent string) string {
	// trim leading and trailing newlines
	text = strings.Trim(text, "\n")
	var sb strings.Builder
	reset := data.ResetSeq
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if i > 0 {
			sb.WriteRune('\n')
			sb.WriteString(indent)
		}
		for _, r := range line {
			sb.WriteString(color)
			sb.WriteRune(r)
			sb.WriteString(reset)
		}
	}
	return sb.String()
}

// indentText ensures every line in the text is prefixed with the given indent.
func indentText(text string, indent string) string {
	text = strings.Trim(text, "\n")
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if i > 0 {
			lines[i] = indent + line
		}
	}
	return strings.Join(lines, "\n")
}

// RenderGeminiConversationLog returns a string summary of Gemini conversation (JSONL or JSON array format)
func RenderGeminiConversationLog(input []byte) string {
	var sb strings.Builder
	var messages []gemini.Content

	// JSONL format: parse each line
	lines := bytes.Split(input, []byte("\n"))
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var msg gemini.Content
		if err := json.Unmarshal(line, &msg); err != nil {
			return fmt.Sprintf("Error parsing Gemini message: %v\n", err)
		}
		messages = append(messages, msg)
	}

	// Summary section
	sb.WriteString("Summary:\n")
	sb.WriteString(fmt.Sprintf("  %sMessages: %d%s\n", data.ResetSeq, len(messages), data.ResetSeq))

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
				mimeType := part.InlineData.MIMEType
				if strings.HasPrefix(mimeType, "image/") {
					imageCount++
				} else {
					fileCount++
				}
			}
		}
	}

	sb.WriteString(fmt.Sprintf("  %sUser messages: %d%s\n", RoleColors["user"], userCount, data.ResetSeq))
	sb.WriteString(fmt.Sprintf("  %sModel responses: %d%s\n", RoleColors["model"], modelCount, data.ResetSeq))
	if functionCallCount > 0 {
		sb.WriteString(fmt.Sprintf("  %sFunction calls: %d%s\n", ContentTypeColors["function_call"], functionCallCount, data.ResetSeq))
	}
	if functionResponseCount > 0 {
		sb.WriteString(fmt.Sprintf("  %sFunction responses: %d%s\n", ContentTypeColors["function_response"], functionResponseCount, data.ResetSeq))
	}
	if imageCount > 0 {
		sb.WriteString(fmt.Sprintf("  %sImages: %d%s\n", ContentTypeColors["image"], imageCount, data.ResetSeq))
	}
	if fileCount > 0 {
		sb.WriteString(fmt.Sprintf("  %sFiles: %d%s\n", ContentTypeColors["file_data"], fileCount, data.ResetSeq))
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
			sb.WriteString(fmt.Sprintf("  %s%s%s: ", roleColor, msg.Role, data.ResetSeq))

			if len(msg.Parts) > 0 {
				for j, part := range msg.Parts {
					if j > 0 {
						sb.WriteString("    ")
					}
					switch {
					case part.FunctionCall != nil:
						sb.WriteString(fmt.Sprintf("\n    %s[Function call: %s]%s", ContentTypeColors["function_call"], part.FunctionCall.Name, data.ResetSeq))
						if len(part.FunctionCall.Args) > 0 {
							argStr, _ := json.MarshalIndent(part.FunctionCall.Args, "    ", "  ")
							sb.WriteString(fmt.Sprintf("\n    args: %s", string(argStr)))
						}
					case part.FunctionResponse != nil:
						sb.WriteString(fmt.Sprintf("\n    %s[Function response]%s", ContentTypeColors["function_response"], data.ResetSeq))
						respPreview, _ := json.MarshalIndent(part.FunctionResponse.Response, "    ", "  ")
						sb.WriteString(fmt.Sprintf("\n    data: %s", string(respPreview)))
					case part.InlineData != nil:
						mimeType := part.InlineData.MIMEType
						if strings.HasPrefix(mimeType, "image/") {
							sb.WriteString(fmt.Sprintf("\n    %s[Image content]%s", ContentTypeColors["image"], data.ResetSeq))
						} else {
							sb.WriteString(fmt.Sprintf("\n    %s[File]%s", ContentTypeColors["file_data"], data.ResetSeq))
						}
					case part.Thought:
						sb.WriteString(fmt.Sprintf("\n    %sThinking ↓%s", ContentTypeColors["reasoning"], ContentTypeColors["reset"]))
						sb.WriteString(fmt.Sprintf("\n    %s", styleEachRune(part.Text, ContentTypeColors["reasoning_content"], "    ")))
						sb.WriteString(fmt.Sprintf("\n    %s✓%s\n", ContentTypeColors["reasoning"], ContentTypeColors["reset"]))
					default:
						if part.Text != "" {
							sb.WriteString("\n    ")
							sb.WriteString(indentText(part.Text, "    "))
						}
					}
				}
				sb.WriteString("\n\n")
			} else {
				sb.WriteString("\n    [Empty message]\n\n")
			}
		}
	}
	return sb.String()
}

// RenderOpenAIConversationLog returns a string summary of OpenAI conversation (JSONL or JSON array format)
func RenderOpenAIConversationLog(input []byte) string {
	var sb strings.Builder
	var messages []openai.ChatCompletionMessage

	// JSONL format: parse each line
	lines := bytes.Split(input, []byte("\n"))
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var msg openai.ChatCompletionMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			return fmt.Sprintf("Error parsing OpenAI message: %v\n", err)
		}
		messages = append(messages, msg)
	}

	// Summary section
	sb.WriteString("Summary:\n")
	sb.WriteString(fmt.Sprintf("  %sMessages: %d%s\n", data.ResetSeq, len(messages), data.ResetSeq))

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

	sb.WriteString(fmt.Sprintf("  %sSystem messages: %d%s\n", RoleColors["system"], systemCount, data.ResetSeq))
	sb.WriteString(fmt.Sprintf("  %sUser messages: %d%s\n", RoleColors["user"], userCount, data.ResetSeq))
	sb.WriteString(fmt.Sprintf("  %sAssistant responses: %d%s\n", RoleColors["assistant"], assistantCount, data.ResetSeq))
	if functionCallCount > 0 {
		sb.WriteString(fmt.Sprintf("  %sFunction/tool calls: %d%s\n", ContentTypeColors["function_call"], functionCallCount, data.ResetSeq))
	}
	if functionResponseCount > 0 {
		sb.WriteString(fmt.Sprintf("  %sFunction/tool responses: %d%s\n", ContentTypeColors["function_response"], functionResponseCount, data.ResetSeq))
	}
	if imageCount > 0 {
		sb.WriteString(fmt.Sprintf("  %sImages: %d%s\n", ContentTypeColors["image"], imageCount, data.ResetSeq))
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
			sb.WriteString(fmt.Sprintf("  %s%s%s", roleColor, msg.Role, data.ResetSeq))

			if msg.Name != "" {
				sb.WriteString(fmt.Sprintf(" (%s)", msg.Name))
			}
			sb.WriteString(": ")

			// Output the reasoning content if it exists
			if msg.ReasoningContent != "" {
				sb.WriteString(fmt.Sprintf("\n    %sThinking ↓%s", ContentTypeColors["reasoning"], ContentTypeColors["reset"]))
				sb.WriteString(fmt.Sprintf("\n    %s", styleEachRune(msg.ReasoningContent, ContentTypeColors["reasoning_content"], "    ")))
				sb.WriteString(fmt.Sprintf("\n    %s✓%s\n", ContentTypeColors["reasoning"], ContentTypeColors["reset"]))
			}

			if msg.Content != "" {
				sb.WriteString("\n    ")
				sb.WriteString(indentText(msg.Content, "    "))
			}

			if len(msg.MultiContent) > 0 {
				// sb.WriteString("\n    Multimodal content: ")
				for j, item := range msg.MultiContent {
					if j > 0 {
						sb.WriteString(", ")
					}
					if item.Type == "text" {
						sb.WriteString(fmt.Sprintf("\n    %s", indentText(item.Text, "    ")))
					}
					if item.Type == "image_url" {
						sb.WriteString(fmt.Sprintf("\n    %simage%s", ContentTypeColors["image"], data.ResetSeq))
					}
				}
				// sb.WriteString("\n    ")
			}

			// Function call details
			if msg.FunctionCall != nil {
				sb.WriteString(fmt.Sprintf("\n    %s[Function call: %s]%s", ContentTypeColors["function_call"], msg.FunctionCall.Name, data.ResetSeq))
				if msg.FunctionCall.Arguments != "" {
					sb.WriteString(fmt.Sprintf(" args: %s", msg.FunctionCall.Arguments))
				}
			}

			// Tool call details
			if len(msg.ToolCalls) > 0 {
				sb.WriteString(fmt.Sprintf("\n    %s[Tool calls: ", ContentTypeColors["function_call"]))
				for j, tool := range msg.ToolCalls {
					if j > 0 {
						sb.WriteString(", ")
					}
					sb.WriteString(fmt.Sprintf("%s (id: %s)", tool.Function.Name, tool.ID))
				}
				sb.WriteString(fmt.Sprintf("]%s", data.ResetSeq))
			}

			// Tool response details
			if msg.ToolCallID != "" {
				sb.WriteString(fmt.Sprintf("\n    %s[Response to tool call: %s]%s", ContentTypeColors["function_response"], msg.ToolCallID, data.ResetSeq))
			}

			sb.WriteString("\n\n")
		}
	}
	return sb.String()
}

// RenderAnthropicConversationLog returns a string summary of Anthropic conversation (JSONL or JSON array format)
func RenderAnthropicConversationLog(input []byte) string {
	var sb strings.Builder
	var messages []anthropic.MessageParam

	// JSONL format: parse each line
	lines := bytes.Split(input, []byte("\n"))
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var msg anthropic.MessageParam
		if err := json.Unmarshal(line, &msg); err != nil {
			return fmt.Sprintf("Error parsing Anthropic message: %v\n", err)
		}
		messages = append(messages, msg)
	}

	// Summary section
	sb.WriteString("Summary:\n")
	sb.WriteString(fmt.Sprintf("  %sMessages: %d%s\n", data.ResetSeq, len(messages), data.ResetSeq))

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

	sb.WriteString(fmt.Sprintf("  %sUser messages: %d%s\n", RoleColors["user"], userCount, data.ResetSeq))
	sb.WriteString(fmt.Sprintf("  %sAssistant messages: %d%s\n", RoleColors["assistant"], assistantCount, data.ResetSeq))
	if toolUseCount > 0 {
		sb.WriteString(fmt.Sprintf("  %sTool uses: %d%s\n", ContentTypeColors["function_call"], toolUseCount, data.ResetSeq))
	}
	if toolResultCount > 0 {
		sb.WriteString(fmt.Sprintf("  %sTool results: %d%s\n", ContentTypeColors["function_response"], toolResultCount, data.ResetSeq))
	}

	// Conversation content
	if len(messages) > 0 {
		sb.WriteString("\nConversation Content:\n")
		for _, msg := range messages {
			role := string(msg.Role)

			// Color
			roleColor := RoleColors[role]
			if roleColor == "" {
				roleColor = data.ResetSeq
			}
			sb.WriteString(fmt.Sprintf("  %s%s%s: ", roleColor, role, data.ResetSeq))

			for j, block := range msg.Content {
				if j > 0 {
					sb.WriteString("\n    ") // Indent for subsequent blocks
				}

				if v := block.OfText; v != nil {
					if j == 0 {
						sb.WriteString("\n    ")
					}
					sb.WriteString(indentText(v.Text, "    "))
				} else if v := block.OfImage; v != nil {
					sb.WriteString(fmt.Sprintf("\n    %s[Image]%s", ContentTypeColors["image"], data.ResetSeq))
				} else if v := block.OfThinking; v != nil {
					sb.WriteString(fmt.Sprintf("\n    %sThinking ↓%s", ContentTypeColors["reasoning"], ContentTypeColors["reset"]))
					sb.WriteString(fmt.Sprintf("\n    %s", styleEachRune(v.Thinking, ContentTypeColors["reasoning_content"], "    ")))
					sb.WriteString(fmt.Sprintf("\n    %s✓%s", ContentTypeColors["reasoning"], ContentTypeColors["reset"]))
				} else if v := block.OfRedactedThinking; v != nil {
					sb.WriteString(fmt.Sprintf("\n    %sThinking (Redacted) ↓%s", ContentTypeColors["reasoning"], ContentTypeColors["reset"]))
					sb.WriteString(fmt.Sprintf("\n    %s", styleEachRune(v.Data, ContentTypeColors["reasoning_content"], "    ")))
					sb.WriteString(fmt.Sprintf("\n    %s✓%s", ContentTypeColors["reasoning"], ContentTypeColors["reset"]))
				} else if v := block.OfToolUse; v != nil {
					sb.WriteString(fmt.Sprintf("\n    %s[Tool Use: %s]%s", ContentTypeColors["function_call"], v.Name, data.ResetSeq))
					// Input
					inputJSON, _ := json.MarshalIndent(v.Input, "    ", "  ")
					sb.WriteString(fmt.Sprintf("\n    input: %s", string(inputJSON)))
				} else if v := block.OfToolResult; v != nil {
					sb.WriteString(fmt.Sprintf("\n    %s[Tool Result: ID=%s]%s", ContentTypeColors["function_response"], v.ToolUseID, data.ResetSeq))
					// Content
					contentJSON, _ := json.MarshalIndent(v.Content, "    ", "  ")
					sb.WriteString(fmt.Sprintf("\n    content: %s", string(contentJSON)))
				} else {
					sb.WriteString("\n    [Unknown Block]")
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
