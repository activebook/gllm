package service

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/activebook/gllm/data"
	anthropic "github.com/anthropics/anthropic-sdk-go"
	openai "github.com/openai/openai-go/v3"
	arkmodel "github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
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
		"reasoning":         data.ReasoningTagColor,
		"reasoning_content": data.ReasoningTextColor,
		"reset":             data.ResetSeq,
	}
}

// Detects if a message is definitely an OpenAI message
func DetectOpenAIKeyMessage(msg *openai.ChatCompletionMessageParamUnion) bool {
	rolePtr := msg.GetRole()
	if rolePtr == nil {
		return false
	}
	role := *rolePtr

	// System/Tool/Function roles are unique signals for OpenAI-style messages
	if role == "system" || role == "tool" || role == "function" {
		return true
	}

	// ToolCallID is present in 'tool' role messages
	if id := msg.GetToolCallID(); id != nil && *id != "" {
		return true
	}

	// ToolCalls in 'assistant' role messages
	if len(msg.GetToolCalls()) > 0 {
		return true
	}

	// Check content for OpenAI-specific parts like image_url in user messages
	if role == "user" && msg.OfUser != nil {
		for _, part := range msg.OfUser.Content.OfArrayOfContentParts {
			if part.OfImageURL != nil {
				return true
			}
		}
	}

	// OpenAI-compatible reasoning tags are a definitive signal for GLLM's internal handling
	if role == "assistant" && msg.OfAssistant != nil {
		if content := msg.OfAssistant.Content.OfString.Value; content != "" && strings.Contains(content, "</think>") {
			return true
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

// DetectMessageProviderFromLine detects the provider of a message from its JSON representation.
func DetectMessageProviderFromLine(line []byte) string {
	line = bytes.TrimSpace(line)
	if len(line) == 0 || !json.Valid(line) {
		return ModelProviderUnknown
	}

	// Fast pre-check using raw JSON map for efficiency and non-standard fields.
	var raw map[string]interface{}
	if err := json.Unmarshal(line, &raw); err == nil {
		if provider := DetectMessageProviderFromRaw(raw); provider != ModelProviderUnknown {
			return provider
		}
	}

	// 1. Check Gemini (Most unique structure "parts")
	var geminiMsg gemini.Content
	if err := json.Unmarshal(line, &geminiMsg); err == nil {
		if DetectGeminiKeyMessage(&geminiMsg) {
			return ModelProviderGemini
		}
	}

	// 2. Check Anthropic
	var anthropicMsg anthropic.MessageParam
	if err := json.Unmarshal(line, &anthropicMsg); err == nil {
		if DetectAnthropicKeyMessage(&anthropicMsg) {
			return ModelProviderAnthropic
		}
	}

	// 3. Fallback to full SDK unmarshal check
	var openaiMsg openai.ChatCompletionMessageParamUnion
	if err := json.Unmarshal(line, &openaiMsg); err == nil {
		if DetectOpenAIKeyMessage(&openaiMsg) {
			return ModelProviderOpenAI
		}
		// Weak check: plain role present → OpenAI-compatible
		rolePtr := openaiMsg.GetRole()
		if rolePtr != nil && *rolePtr != "" {
			return ModelProviderOpenAICompatible
		}
	}

	return ModelProviderUnknown
}

// DetectMessageProviderFromRaw performs a fast-path check using a raw JSON map.
// This is used for performance (avoiding multi-type reflection unmarshals)
// and for detecting vendor-specific keys that aren't in standard SDK schemas.
func DetectMessageProviderFromRaw(raw map[string]interface{}) string {
	// Definitive OpenAI/OpenChat signals
	role, _ := raw["role"].(string)
	if role == "tool" || role == "function" || role == "system" {
		return ModelProviderOpenAI
	}
	if _, ok := raw["reasoning_content"]; ok {
		// reasoning_content is only supported by OpenChat/compatible providers
		return ModelProviderOpenAICompatible
	}
	if _, ok := raw["tool_call_id"]; ok {
		return ModelProviderOpenAI
	}

	// Multimodal content parts
	if contentArr, ok := raw["content"].([]interface{}); ok {
		for _, item := range contentArr {
			if m, ok := item.(map[string]interface{}); ok {
				if m["type"] == "image_url" {
					return ModelProviderOpenAI
				}
			}
		}
	}

	// Anthropic uses role 'user' and 'assistant' but also has 'content' array.
	// Gemini uses 'parts' instead of 'content' and uses role 'model' instead of 'assistant'.
	if _, ok := raw["parts"]; ok {
		if role == "user" || role == "model" {
			return ModelProviderGemini
		}
	}

	return ModelProviderUnknown
}

/*
 * Detects the session provider based on message format.
 * Supports both JSONL (preferred) and legacy JSON array formats.
 */
func DetectMessageProviderByContent(input []byte) string {
	// 1. Try to unmarshal as array of messages (Legacy Format)
	// var arrayMessages []json.RawMessage
	// if err := json.Unmarshal(input, &arrayMessages); err == nil && len(arrayMessages) > 0 {
	// 	// It's a valid JSON array, detect provider from the messages
	// 	var weakMatch bool
	// 	for _, msg := range arrayMessages {
	// 		provider := DetectMessageProviderFromLine(msg)
	// 		if provider != ModelProviderUnknown && provider != ModelProviderOpenAICompatible {
	// 			return provider // Found definitive match
	// 		}
	// 		if provider == ModelProviderOpenAICompatible {
	// 			weakMatch = true
	// 		}
	// 	}
	// 	if weakMatch {
	// 		return ModelProviderOpenAICompatible
	// 	}
	// 	return ModelProviderUnknown
	// }

	// 2. JSONL format: parse each line
	lines := bytes.Split(input, []byte("\n"))
	var weakMatch bool
	for _, line := range lines {
		provider := DetectMessageProviderFromLine(line)
		if provider != ModelProviderUnknown && provider != ModelProviderOpenAICompatible {
			return provider // Found definitive match
		}
		if provider == ModelProviderOpenAICompatible {
			weakMatch = true
		}
	}

	if weakMatch {
		return ModelProviderOpenAICompatible
	}

	return ModelProviderUnknown
}

// DetectMessageProvider detects the session provider by scanning the JSONL file.
// Uses bufio.Reader to handle arbitrarily long lines (e.g. base64-encoded images)
// without hitting bufio.Scanner's fixed token-size ceiling.
func DetectMessageProvider(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ModelProviderUnknown
	}
	defer file.Close()

	reader := bufio.NewReaderSize(file, 64*1024)
	var weakMatch bool

	for {
		line, err := reader.ReadBytes('\n')
		line = bytes.TrimSpace(line)

		if len(line) > 0 {
			provider := DetectMessageProviderFromLine(line)
			if provider != ModelProviderUnknown && provider != ModelProviderOpenAICompatible {
				return provider
			}
			if provider == ModelProviderOpenAICompatible {
				weakMatch = true
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return ModelProviderUnknown
		}
	}

	if weakMatch {
		return ModelProviderOpenAICompatible
	}
	return ModelProviderUnknown
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

// RenderGeminiSessionLog returns a string summary of Gemini session (JSONL or JSON array format)
func RenderGeminiSessionLog(input []byte) string {
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

	// session content
	if len(messages) > 0 {
		sb.WriteString("\nsession Content:\n")
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
						respName, _ := json.MarshalIndent(part.FunctionResponse.Name, "    ", "  ")
						sb.WriteString(fmt.Sprintf("\n    name: %s", string(respName)))
						if part.FunctionResponse.Response != nil {
							respData, _ := json.MarshalIndent(part.FunctionResponse.Response, "    ", "  ")
							sb.WriteString(fmt.Sprintf("\n    data: %s", string(respData)))
						}
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

// RenderOpenAISessionLog returns a string summary of OpenAI session (JSONL or JSON array format)
func RenderOpenAISessionLog(input []byte) string {
	var sb strings.Builder
	var messages []*arkmodel.ChatCompletionMessage

	// JSONL format: parse each line
	lines := bytes.Split(input, []byte("\n"))
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var msg arkmodel.ChatCompletionMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			return fmt.Sprintf("Error parsing OpenAI message: %v\n", err)
		}
		messages = append(messages, &msg)
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

		functionCallCount += len(msg.ToolCalls)

		// Check content for images
		if msg.Content != nil && msg.Content.ListValue != nil {
			for _, part := range msg.Content.ListValue {
				if part.Type == arkmodel.ChatCompletionMessageContentPartTypeImageURL {
					imageCount++
				}
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

	// session content
	if len(messages) > 0 {
		sb.WriteString("\nsession Content:\n")
		for _, msg := range messages {
			// Apply color to role
			roleColor := RoleColors[msg.Role]
			sb.WriteString(fmt.Sprintf("  %s%s%s", roleColor, msg.Role, data.ResetSeq))

			if msg.Name != nil && *msg.Name != "" {
				sb.WriteString(fmt.Sprintf(" (%s)", *msg.Name))
			}
			sb.WriteString(": ")

			// Output format and order is like this:
			// reasoning
			// tool response if exists
			// content
			// tool calls if exists

			// Output the reasoning content if it exists
			if msg.ReasoningContent != nil && *msg.ReasoningContent != "" {
				sb.WriteString(fmt.Sprintf("\n    %sThinking ↓%s", ContentTypeColors["reasoning"], ContentTypeColors["reset"]))
				sb.WriteString(fmt.Sprintf("\n    %s", styleEachRune(*msg.ReasoningContent, ContentTypeColors["reasoning_content"], "    ")))
				sb.WriteString(fmt.Sprintf("\n    %s✓%s\n", ContentTypeColors["reasoning"], ContentTypeColors["reset"]))
			}

			// Tool response details
			if msg.ToolCallID != "" {
				sb.WriteString(fmt.Sprintf("\n    %s[Response to tool call: %s]%s", ContentTypeColors["function_response"], msg.ToolCallID, data.ResetSeq))
			}

			if msg.Content != nil {
				if msg.Content.StringValue != nil && *msg.Content.StringValue != "" {
					sb.WriteString("\n    ")
					sb.WriteString(indentText(*msg.Content.StringValue, "    "))
				} else if len(msg.Content.ListValue) > 0 {
					for j, part := range msg.Content.ListValue {
						if j > 0 {
							sb.WriteString(", ")
						}
						switch part.Type {
						case arkmodel.ChatCompletionMessageContentPartTypeText:
							if part.Text != "" {
								sb.WriteString(fmt.Sprintf("\n    %s", indentText(part.Text, "    ")))
							}
						case arkmodel.ChatCompletionMessageContentPartTypeImageURL:
							sb.WriteString(fmt.Sprintf("\n    %s[Image]%s", ContentTypeColors["image"], data.ResetSeq))
						case arkmodel.ChatCompletionMessageContentPartType("input_audio"):
							sb.WriteString(fmt.Sprintf("\n    %s[Audio]%s", ContentTypeColors["image"], data.ResetSeq))
						case arkmodel.ChatCompletionMessageContentPartType("file"):
							sb.WriteString(fmt.Sprintf("\n    %s[PDF Document]%s", ContentTypeColors["image"], data.ResetSeq))
						}
					}
				}
			}

			// Tool call details, print at the end
			if len(msg.ToolCalls) > 0 {
				for _, tool := range msg.ToolCalls {
					sb.WriteString("\n")
					sb.WriteString(fmt.Sprintf("    %s[Tool calls: ", ContentTypeColors["function_call"]))
					sb.WriteString(fmt.Sprintf("%s (id: %s)", tool.Function.Name, tool.ID))
					sb.WriteString(fmt.Sprintf("]%s\n", data.ResetSeq))
					if tool.Function.Arguments != "" {
						// we should write arguments in a pretty way
						var args map[string]interface{}
						if err := json.Unmarshal([]byte(tool.Function.Arguments), &args); err == nil {
							argStr, _ := json.MarshalIndent(args, "    ", "  ")
							sb.WriteString(fmt.Sprintf("    args: %s", string(argStr)))
						} else {
							sb.WriteString(fmt.Sprintf("    args: %s", tool.Function.Arguments))
						}
					}
				}
			}

			sb.WriteString("\n\n")
		}
	}
	return sb.String()
}

// RenderAnthropicSessionLog returns a string summary of Anthropic session (JSONL or JSON array format)
func RenderAnthropicSessionLog(input []byte) string {
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

	// session content
	if len(messages) > 0 {
		sb.WriteString("\nsession Content:\n")
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
				} else if v := block.OfDocument; v != nil {
					sb.WriteString(fmt.Sprintf("\n    %s[PDF Document]%s", ContentTypeColors["image"], data.ResetSeq))
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
