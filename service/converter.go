package service

import (
	"encoding/json"
	"fmt"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	openai "github.com/sashabaranov/go-openai"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	gemini "google.golang.org/genai"
)

/*
 * UniversalMessage is a provider-agnostic representation of a chat message.
 * It extracts only the essential semantic content for cross-provider conversion.
 *
 * Key Design Decisions:
 * 1. Only text content and reasoning are preserved.
 * 2. Tool calls, tool responses, images, and other multimodal content are discarded.
 * 3. Role normalization: "model" (Gemini) → "assistant"
 */
type UniversalMessage struct {
	Role      UniversalRole // "system", "user", "assistant"
	Content   string        // Main text content
	Reasoning string        // Thinking/reasoning content (if any)
}

type UniversalRole string

const (
	UniversalRoleSystem    UniversalRole = "system"
	UniversalRoleUser      UniversalRole = "user"
	UniversalRoleAssistant UniversalRole = "assistant"
)

func (r UniversalRole) String() string {
	return string(r)
}

func (r UniversalRole) ConvertToOpenAI() string {
	switch r {
	case UniversalRoleAssistant:
		return openai.ChatMessageRoleAssistant
	case UniversalRoleUser:
		return openai.ChatMessageRoleUser
	case UniversalRoleSystem:
		return openai.ChatMessageRoleSystem
	default:
		return string(r)
	}
}

func (r UniversalRole) ConvertToOpenChat() string {
	switch r {
	case UniversalRoleAssistant:
		return model.ChatMessageRoleAssistant
	case UniversalRoleUser:
		return model.ChatMessageRoleUser
	case UniversalRoleSystem:
		return model.ChatMessageRoleSystem
	default:
		return string(r)
	}
}

func (r UniversalRole) ConvertToAnthropic() anthropic.MessageParamRole {
	switch r {
	case UniversalRoleAssistant:
		return anthropic.MessageParamRoleAssistant
	case UniversalRoleUser:
		return anthropic.MessageParamRoleUser
	case UniversalRoleSystem:
		return anthropic.MessageParamRoleUser
	default:
		return anthropic.MessageParamRoleUser
	}
}

func (r UniversalRole) ConvertToGemini() string {
	switch r {
	case UniversalRoleAssistant:
		return gemini.RoleModel
	case UniversalRoleUser:
		return gemini.RoleUser
	case UniversalRoleSystem:
		return gemini.RoleUser
	default:
		return string(r)
	}
}

func ConvertToUniversalRole(role string) UniversalRole {
	switch role {
	case gemini.RoleModel:
		return UniversalRoleAssistant
	case openai.ChatMessageRoleAssistant:
		return UniversalRoleAssistant
	case openai.ChatMessageRoleUser:
		return UniversalRoleUser
	case openai.ChatMessageRoleSystem:
		return UniversalRoleSystem
	default:
		return UniversalRole(role)
	}
}

// ==============================
// Parsing Functions (Source → Universal)
// ==============================

// ParseOpenAIMessages converts OpenAI messages to universal format.
// Extracts: Content, MultiContent[].Text, ReasoningContent
// Ignores: ToolCalls, FunctionCall, ImageURL
func ParseOpenAIMessages(messages []openai.ChatCompletionMessage) []UniversalMessage {
	var result []UniversalMessage
	for _, msg := range messages {
		// Skip tool/function responses
		if msg.Role == openai.ChatMessageRoleTool || msg.Role == openai.ChatMessageRoleFunction {
			continue
		}
		// Skip messages with tool calls but no content
		if len(msg.ToolCalls) > 0 && msg.Content == "" && len(msg.MultiContent) == 0 {
			continue
		}

		um := UniversalMessage{
			Role:      ConvertToUniversalRole(msg.Role),
			Reasoning: msg.ReasoningContent,
		}

		// Extract text content
		if msg.Content != "" {
			um.Content = msg.Content
		} else if len(msg.MultiContent) > 0 {
			// Extract only text parts from multimodal content
			for _, part := range msg.MultiContent {
				if part.Type == openai.ChatMessagePartTypeText && part.Text != "" {
					if um.Content != "" {
						um.Content += "\n"
					}
					um.Content += part.Text
				}
			}
		}

		// Only add if there's actual content
		if um.Content != "" || um.Reasoning != "" {
			result = append(result, um)
		}
	}
	return result
}

// ParseOpenChatMessages converts OpenChat (Volcengine) messages to universal format.
func ParseOpenChatMessages(messages []*model.ChatCompletionMessage) []UniversalMessage {
	var result []UniversalMessage
	for _, msg := range messages {
		// Skip tool responses
		if msg.Role == model.ChatMessageRoleTool {
			continue
		}
		// Skip messages with tool calls but no content
		if len(msg.ToolCalls) > 0 && (msg.Content == nil || msg.Content.StringValue == nil || *msg.Content.StringValue == "") {
			continue
		}

		um := UniversalMessage{
			Role: ConvertToUniversalRole(msg.Role),
		}

		// Extract reasoning
		if msg.ReasoningContent != nil && *msg.ReasoningContent != "" {
			um.Reasoning = *msg.ReasoningContent
		}

		// Extract text content
		if msg.Content != nil {
			if msg.Content.StringValue != nil {
				um.Content = *msg.Content.StringValue
			} else if msg.Content.ListValue != nil {
				// Extract text from list content
				for _, part := range msg.Content.ListValue {
					if part.Type == model.ChatCompletionMessageContentPartTypeText && part.Text != "" {
						if um.Content != "" {
							um.Content += "\n"
						}
						um.Content += part.Text
					}
				}
			}
		}

		// Only add if there's actual content
		if um.Content != "" || um.Reasoning != "" {
			result = append(result, um)
		}
	}
	return result
}

// ParseAnthropicMessages converts Anthropic messages to universal format.
// Extracts: OfText blocks, OfThinking/OfRedactedThinking blocks
// Ignores: OfToolUse, OfToolResult, OfImage, OfDocument
func ParseAnthropicMessages(messages []anthropic.MessageParam) []UniversalMessage {
	var result []UniversalMessage
	for _, msg := range messages {
		um := UniversalMessage{
			Role: ConvertToUniversalRole(string(msg.Role)),
		}

		for _, block := range msg.Content {
			if v := block.OfText; v != nil && v.Text != "" {
				if um.Content != "" {
					um.Content += "\n"
				}
				um.Content += v.Text
			} else if v := block.OfThinking; v != nil && v.Thinking != "" {
				if um.Reasoning != "" {
					um.Reasoning += "\n"
				}
				um.Reasoning += v.Thinking
			} else if v := block.OfRedactedThinking; v != nil && v.Data != "" {
				if um.Reasoning != "" {
					um.Reasoning += "\n"
				}
				um.Reasoning += "[Redacted Thinking]" + v.Data
			}
			// Skip: OfToolUse, OfToolResult, OfImage, OfDocument
		}

		// Only add if there's actual content
		if um.Content != "" || um.Reasoning != "" {
			result = append(result, um)
		}
	}
	return result
}

// ParseGeminiMessages converts Gemini messages to universal format.
// Extracts: Parts.Text, Parts.Thought
// Ignores: FunctionCall, FunctionResponse, InlineData
// Maps: "model" → "assistant"
func ParseGeminiMessages(messages []*gemini.Content) []UniversalMessage {
	var result []UniversalMessage
	for _, msg := range messages {
		um := UniversalMessage{
			Role: ConvertToUniversalRole(msg.Role),
		}

		for _, part := range msg.Parts {
			// Skip function calls and responses
			if part.FunctionCall != nil || part.FunctionResponse != nil {
				continue
			}
			// Skip inline data (images, files)
			if part.InlineData != nil {
				continue
			}

			// Extract text content
			if part.Text != "" {
				if part.Thought {
					if um.Reasoning != "" {
						um.Reasoning += "\n"
					}
					um.Reasoning += part.Text
				} else {
					if um.Content != "" {
						um.Content += "\n"
					}
					um.Content += part.Text
				}
			}
		}

		// Only add if there's actual content
		if um.Content != "" || um.Reasoning != "" {
			result = append(result, um)
		}
	}
	return result
}

// ==============================
// Building Functions (Universal → Target)
// ==============================

// BuildOpenAIMessages converts universal messages to OpenAI format.
// Preserves: system role, Content, ReasoningContent
func BuildOpenAIMessages(messages []UniversalMessage) []openai.ChatCompletionMessage {
	var result []openai.ChatCompletionMessage
	for _, um := range messages {
		msg := openai.ChatCompletionMessage{
			Role: um.Role.ConvertToOpenAI(),
		}
		if um.Content != "" {
			msg.Content = um.Content
		}
		if um.Reasoning != "" {
			msg.ReasoningContent = um.Reasoning
		}
		result = append(result, msg)
	}
	return result
}

// BuildOpenChatMessages converts universal messages to OpenChat (Volcengine) format.
// Preserves: system role, Content, ReasoningContent
func BuildOpenChatMessages(messages []UniversalMessage) []*model.ChatCompletionMessage {
	var result []*model.ChatCompletionMessage
	for _, um := range messages {
		// Need local copies for pointer references
		content := um.Content
		reasoning := um.Reasoning

		// Map string role to SDK constant
		// The Role field expects a SDK constant string
		msg := &model.ChatCompletionMessage{
			Role: um.Role.ConvertToOpenChat(),
		}
		if um.Content != "" {
			msg.Content = &model.ChatCompletionMessageContent{StringValue: &content}
		}
		if um.Reasoning != "" {
			msg.ReasoningContent = &reasoning
		}

		result = append(result, msg)
	}
	return result
}

// BuildAnthropicMessages converts universal messages to Anthropic format.
// Handles: System role is inlined into the first user message.
// Preserves: OfText, OfThinking blocks
func BuildAnthropicMessages(messages []UniversalMessage) []anthropic.MessageParam {
	var result []anthropic.MessageParam

	for _, um := range messages {
		var blocks []anthropic.ContentBlockParamUnion

		// Add reasoning as thinking block (for assistant messages)
		if um.Reasoning != "" {
			blocks = append(blocks, anthropic.ContentBlockParamUnion{
				OfThinking: &anthropic.ThinkingBlockParam{
					Thinking: um.Reasoning,
				},
			})
		}

		// Add content as text block
		content := um.Content
		if content != "" {
			blocks = append(blocks, anthropic.ContentBlockParamUnion{
				OfText: &anthropic.TextBlockParam{
					Text: content,
				},
			})
		}

		if len(blocks) > 0 {
			msg := anthropic.MessageParam{
				Role:    um.Role.ConvertToAnthropic(),
				Content: blocks,
			}
			result = append(result, msg)
		}
	}
	return result
}

// BuildGeminiMessages converts universal messages to Gemini format.
// Handles: System role is inlined into the first user message.
// Preserves: Parts with Text, Thought
// Maps: "assistant" → "model"
func BuildGeminiMessages(messages []UniversalMessage) []*gemini.Content {
	var result []*gemini.Content

	for _, um := range messages {
		var parts []*gemini.Part

		// Add reasoning as thought part (for model messages)
		if um.Reasoning != "" {
			parts = append(parts, &gemini.Part{
				Text:    um.Reasoning,
				Thought: true,
			})
		}

		// Add content as text part
		content := um.Content
		if content != "" {
			parts = append(parts, &gemini.Part{
				Text: content,
			})
		}

		if len(parts) > 0 {
			msg := &gemini.Content{
				Role:  um.Role.ConvertToGemini(),
				Parts: parts,
			}
			result = append(result, msg)
		}
	}
	return result
}

// ==============================
// High-Level Conversion Function
// ==============================

// ConvertMessages parses source provider data and builds target provider messages.
// Returns the converted data encoded as JSON.
//
// Supported source/target providers:
// - ModelProviderOpenAI
// - ModelProviderOpenAICompatible (OpenChat)
// - ModelProviderAnthropic
// - ModelProviderGemini
func ConvertMessages(data []byte, sourceProvider, targetProvider string) ([]byte, error) {
	if sourceProvider == targetProvider {
		// No conversion needed
		return data, nil
	}

	// Check compatible providers (OpenAI and OpenChat use same format for text)
	if (sourceProvider == ModelProviderOpenAI || sourceProvider == ModelProviderOpenAICompatible) &&
		(targetProvider == ModelProviderOpenAI || targetProvider == ModelProviderOpenAICompatible) {
		// Direct copy for compatible providers
		return data, nil
	}

	// Step 1: Parse source data to universal format
	var uniMsgs []UniversalMessage

	switch sourceProvider {
	case ModelProviderOpenAI:
		var msgs []openai.ChatCompletionMessage
		if err := json.Unmarshal(data, &msgs); err != nil {
			return nil, fmt.Errorf("failed to parse OpenAI messages: %w", err)
		}
		uniMsgs = ParseOpenAIMessages(msgs)

	case ModelProviderOpenAICompatible:
		// Try OpenAI format first (most OpenChat convos use this)
		var msgs []openai.ChatCompletionMessage
		if err := json.Unmarshal(data, &msgs); err != nil {
			return nil, fmt.Errorf("failed to parse OpenChat messages: %w", err)
		}
		uniMsgs = ParseOpenAIMessages(msgs)

	case ModelProviderAnthropic:
		var msgs []anthropic.MessageParam
		if err := json.Unmarshal(data, &msgs); err != nil {
			return nil, fmt.Errorf("failed to parse Anthropic messages: %w", err)
		}
		uniMsgs = ParseAnthropicMessages(msgs)

	case ModelProviderGemini:
		var msgs []*gemini.Content
		if err := json.Unmarshal(data, &msgs); err != nil {
			return nil, fmt.Errorf("failed to parse Gemini messages: %w", err)
		}
		uniMsgs = ParseGeminiMessages(msgs)

	default:
		return nil, fmt.Errorf("unsupported source provider: %s", sourceProvider)
	}

	// Step 2: Build target format from universal
	var targetData interface{}

	switch targetProvider {
	case ModelProviderOpenAI:
		targetData = BuildOpenAIMessages(uniMsgs)

	case ModelProviderOpenAICompatible:
		targetData = BuildOpenChatMessages(uniMsgs)

	case ModelProviderAnthropic:
		targetData = BuildAnthropicMessages(uniMsgs)

	case ModelProviderGemini:
		targetData = BuildGeminiMessages(uniMsgs)

	default:
		return nil, fmt.Errorf("unsupported target provider: %s", targetProvider)
	}

	// Step 3: Marshal to JSON
	result, err := json.MarshalIndent(targetData, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to serialize converted messages: %w", err)
	}

	return result, nil
}
