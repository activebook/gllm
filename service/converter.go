package service

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/activebook/gllm/util"
	anthropic "github.com/anthropics/anthropic-sdk-go"
	openai "github.com/openai/openai-go/v3"
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
	// Openai uses string "assistant", "user", "system"
	// we use model instead
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
	case model.ChatMessageRoleAssistant:
		return UniversalRoleAssistant
	case model.ChatMessageRoleTool, "function":
		return UniversalRoleAssistant
	case model.ChatMessageRoleUser:
		return UniversalRoleUser
	case model.ChatMessageRoleSystem:
		return UniversalRoleSystem
	default:
		return UniversalRole(role)
	}
}

// ==============================
// Parsing Functions (Source → Universal)
// ==============================

// ParseOpenAIMessages converts OpenAI messages to universal format.
// Extracts: Content, MultiContent[].Text, ReasoningContent, Tool Responses as text
// Ignores: ToolCalls, FunctionCall, ImageURL
func ParseOpenAIMessages(messages []openai.ChatCompletionMessageParamUnion) []UniversalMessage {
	var result []UniversalMessage
	for _, msg := range messages {
		rolePtr := msg.GetRole()
		if rolePtr == nil {
			continue
		}
		role := *rolePtr
		um := UniversalMessage{
			Role: ConvertToUniversalRole(role),
		}

		// Try to extract content and reasoning content via JSON marshaling to be safe and robust
		// since ChatCompletionMessageParamUnion is complex and reasoning_content might be an extra field.
		data, err := json.Marshal(msg)
		if err == nil {
			var raw map[string]interface{}
			if err := json.Unmarshal(data, &raw); err == nil {
				// Content as string
				if contentStr, ok := raw["content"].(string); ok && contentStr != "" {
					um.Content = contentStr
				} else if contentArr, ok := raw["content"].([]interface{}); ok {
					// Content as Array of parts
					for _, item := range contentArr {
						if partMap, ok := item.(map[string]interface{}); ok {
							if partType, ok := partMap["type"].(string); ok && partType == "text" {
								if text, ok := partMap["text"].(string); ok && text != "" {
									if um.Content != "" {
										um.Content += "\n"
									}
									um.Content += text
								}
							}
						}
					}
				}

				// Reasoning Content (Deepseek / Custom parameter — only if not already extracted)
				if reasoning, ok := raw["reasoning_content"].(string); ok && reasoning != "" {
					um.Reasoning = reasoning
				} else if reasoning, ok := raw["reasoning"].(string); ok && reasoning != "" {
					um.Reasoning = reasoning
				}
			}
		}

		// Look for embedded <thought> tags (OpenAI compatibility workaround)
		if um.Reasoning == "" && um.Content != "" {
			if thinkContent, cleanedContent := util.ExtractThinkTags(um.Content); thinkContent != "" {
				um.Reasoning = thinkContent
				um.Content = cleanedContent
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
func ParseOpenChatMessages(messages []model.ChatCompletionMessage) []UniversalMessage {
	var result []UniversalMessage
	for _, msg := range messages {
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
// Extracts: OfText blocks, OfThinking/OfRedactedThinking blocks, OfToolResult as text
// Ignores: OfToolUse, OfImage, OfDocument
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
			} else if v := block.OfToolResult; v != nil {
				// Extract tool result content as plain text
				toolText := ""
				for _, resBlock := range v.Content {
					if resv := resBlock.OfText; resv != nil && resv.Text != "" {
						if toolText != "" {
							toolText += "\n"
						}
						toolText += resv.Text
					}
				}
				if toolText != "" {
					if um.Content != "" {
						um.Content += "\n"
					}
					um.Content += toolText
				}
			}
			// Skip: OfToolUse, OfImage, OfDocument
		}

		// Only add if there's actual content
		if um.Content != "" || um.Reasoning != "" {
			result = append(result, um)
		}
	}
	return result
}

// ParseGeminiMessages converts Gemini messages to universal format.
// Extracts: Parts.Text, Parts.Thought, FunctionResponse as text
// Ignores: FunctionCall, InlineData
// Maps: "model" → "assistant"
func ParseGeminiMessages(messages []*gemini.Content) []UniversalMessage {
	var result []UniversalMessage
	for _, msg := range messages {
		um := UniversalMessage{
			Role: ConvertToUniversalRole(msg.Role),
		}

		for _, part := range msg.Parts {
			// Skip function calls
			if part.FunctionCall != nil {
				continue
			}
			// Skip inline data (images, files)
			if part.InlineData != nil {
				continue
			}

			// Extract function response
			if part.FunctionResponse != nil {
				if part.FunctionResponse.Response != nil {
					// Need to serialize the map to string to preserve as text context
					if respBytes, err := json.Marshal(part.FunctionResponse.Response); err == nil && string(respBytes) != "{}" {
						if um.Content != "" {
							um.Content += "\n"
						}
						um.Content += string(respBytes)
					}
				}
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
func BuildOpenAIMessages(messages []UniversalMessage) []openai.ChatCompletionMessageParamUnion {
	var result []openai.ChatCompletionMessageParamUnion
	for _, um := range messages {
		role := um.Role.ConvertToOpenAI()

		switch role {
		case model.ChatMessageRoleAssistant:
			// Reasoning is not set on Request Params in v3.
			result = append(result, openai.AssistantMessage(um.Content))
		case model.ChatMessageRoleSystem:
			result = append(result, openai.SystemMessage(um.Content))
		default:
			result = append(result, openai.UserMessage(um.Content))
		}
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

// parseJSONL parses strictly JSONL format (one JSON object per line)
func parseJSONL[T any](data []byte, target *[]T) error {
	lines := bytes.Split(data, []byte("\n"))
	for i, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var item T
		if err := json.Unmarshal(line, &item); err != nil {
			return fmt.Errorf("line %d: %w", i+1, err)
		}
		*target = append(*target, item)
	}
	return nil
}

// marshalJSONL encodes a slice as JSONL (one JSON object per line)
func marshalJSONL[T any](messages []T) ([]byte, error) {
	var buf bytes.Buffer
	for _, msg := range messages {
		line, err := json.Marshal(msg)
		if err != nil {
			return nil, err
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	return buf.Bytes(), nil
}

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
		var msgs []openai.ChatCompletionMessageParamUnion
		if err := parseJSONL(data, &msgs); err != nil {
			return nil, fmt.Errorf("failed to parse OpenAI messages: %w", err)
		}
		uniMsgs = ParseOpenAIMessages(msgs)

	case ModelProviderOpenAICompatible:
		var msgs []model.ChatCompletionMessage
		if err := parseJSONL(data, &msgs); err != nil {
			return nil, fmt.Errorf("failed to parse OpenChat messages: %w", err)
		}
		uniMsgs = ParseOpenChatMessages(msgs)

	case ModelProviderAnthropic:
		var msgs []anthropic.MessageParam
		if err := parseJSONL(data, &msgs); err != nil {
			return nil, fmt.Errorf("failed to parse Anthropic messages: %w", err)
		}
		uniMsgs = ParseAnthropicMessages(msgs)

	case ModelProviderGemini:
		var msgs []*gemini.Content
		if err := parseJSONL(data, &msgs); err != nil {
			return nil, fmt.Errorf("failed to parse Gemini messages: %w", err)
		}
		uniMsgs = ParseGeminiMessages(msgs)

	default:
		return nil, fmt.Errorf("unsupported source provider: %s", sourceProvider)
	}

	// Step 2: Build target format and marshal as JSONL
	switch targetProvider {
	case ModelProviderOpenAI:
		newmsgs := BuildOpenAIMessages(uniMsgs)
		return marshalJSONL(newmsgs)

	case ModelProviderOpenAICompatible:
		newmsgs := BuildOpenChatMessages(uniMsgs)
		return marshalJSONL(newmsgs)

	case ModelProviderAnthropic:
		newmsgs := BuildAnthropicMessages(uniMsgs)
		return marshalJSONL(newmsgs)

	case ModelProviderGemini:
		newmsgs := BuildGeminiMessages(uniMsgs)
		return marshalJSONL(newmsgs)

	default:
		return nil, fmt.Errorf("unsupported target provider: %s", targetProvider)
	}
}
