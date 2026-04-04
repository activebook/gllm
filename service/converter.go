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
 * 1. Text content and images are preserved via Parts.
 * 2. Reasoning is preserved.
 * 3. Tool calls and tool responses are discarded.
 * 4. Role normalization: "model" (Gemini) → "assistant"
 */
type UniversalMessage struct {
	Role      UniversalRole // "system", "user", "assistant"
	Parts     []UniversalPart
	Reasoning string // Thinking/reasoning content (if any)
}

type UniversalPartType string

const (
	PartTypeText  UniversalPartType = "text"
	PartTypeImage UniversalPartType = "image"
)

type UniversalPart struct {
	Type     UniversalPartType
	MIMEType string
	Text     string // Populated if Type is PartTypeText
	Data     []byte // Raw bytes, populated if Type is PartTypeImage
}

// GetTextContent returns all text parts concatenated.
func (um *UniversalMessage) GetTextContent() string {
	var text string
	for _, part := range um.Parts {
		if part.Type == PartTypeText {
			if text != "" {
				text += "\n"
			}
			text += part.Text
		}
	}
	return text
}

// HasContent returns true if the message has any text, reasoning, or media parts.
func (um *UniversalMessage) HasContent() bool {
	return len(um.Parts) > 0 || um.Reasoning != ""
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
// Extracts: Content text, MultiContent[].Text, Reasonings, and ImageURLs.
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

		// Try to extract content and reasoning via JSON marshaling to be safe and robust
		data, err := json.Marshal(msg)
		if err == nil {
			var raw map[string]interface{}
			if err := json.Unmarshal(data, &raw); err == nil {
				// Content as string
				if contentStr, ok := raw["content"].(string); ok && contentStr != "" {
					um.Parts = append(um.Parts, UniversalPart{Type: PartTypeText, Text: contentStr})
				} else if contentArr, ok := raw["content"].([]interface{}); ok {
					// Content as Array of parts
					for _, item := range contentArr {
						if partMap, ok := item.(map[string]interface{}); ok {
							partType, _ := partMap["type"].(string)
							if partType == "text" {
								if text, ok := partMap["text"].(string); ok && text != "" {
									um.Parts = append(um.Parts, UniversalPart{Type: PartTypeText, Text: text})
								}
							} else if partType == "image_url" {
								if imgMap, ok := partMap["image_url"].(map[string]interface{}); ok {
									if urlStr, ok := imgMap["url"].(string); ok {
										mimeType, decodedRaw, err := util.ParseDataURL(urlStr)
										if err == nil {
											um.Parts = append(um.Parts, UniversalPart{Type: PartTypeImage, MIMEType: mimeType, Data: decodedRaw})
										}
									}
								}
							}
						}
					}
				}

				// Reasoning Content
				if reasoning, ok := raw["reasoning_content"].(string); ok && reasoning != "" {
					um.Reasoning = reasoning
				} else if reasoning, ok := raw["reasoning"].(string); ok && reasoning != "" {
					um.Reasoning = reasoning
				}
			}
		}

		// Look for embedded <thought> tags wrapper text parts
		if um.Reasoning == "" {
			var rebuiltParts []UniversalPart
			for _, part := range um.Parts {
				if part.Type == PartTypeText {
					if thinkContent, cleanedContent := util.ExtractThinkTags(part.Text); thinkContent != "" {
						um.Reasoning = thinkContent
						if cleanedContent != "" {
							rebuiltParts = append(rebuiltParts, UniversalPart{Type: PartTypeText, Text: cleanedContent})
						}
					} else {
						rebuiltParts = append(rebuiltParts, part)
					}
				} else {
					rebuiltParts = append(rebuiltParts, part)
				}
			}
			um.Parts = rebuiltParts
		}

		if um.HasContent() {
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

		// Extract content
		if msg.Content != nil {
			if msg.Content.StringValue != nil {
				if *msg.Content.StringValue != "" {
					um.Parts = append(um.Parts, UniversalPart{Type: PartTypeText, Text: *msg.Content.StringValue})
				}
			} else if msg.Content.ListValue != nil {
				for _, part := range msg.Content.ListValue {
					if part.Type == model.ChatCompletionMessageContentPartTypeText && part.Text != "" {
						um.Parts = append(um.Parts, UniversalPart{Type: PartTypeText, Text: part.Text})
					} else if part.Type == model.ChatCompletionMessageContentPartTypeImageURL && part.ImageURL != nil {
						mimeType, decodedRaw, err := util.ParseDataURL(part.ImageURL.URL)
						if err == nil {
							um.Parts = append(um.Parts, UniversalPart{Type: PartTypeImage, MIMEType: mimeType, Data: decodedRaw})
						}
					}
				}
			}
		}

		if um.HasContent() {
			result = append(result, um)
		}
	}
	return result
}

// ParseAnthropicMessages converts Anthropic messages to universal format.
func ParseAnthropicMessages(messages []anthropic.MessageParam) []UniversalMessage {
	var result []UniversalMessage
	for _, msg := range messages {
		um := UniversalMessage{
			Role: ConvertToUniversalRole(string(msg.Role)),
		}

		for _, block := range msg.Content {
			if v := block.OfText; v != nil && v.Text != "" {
				um.Parts = append(um.Parts, UniversalPart{Type: PartTypeText, Text: v.Text})
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
			} else if v := block.OfImage; v != nil && v.Source.OfBase64 != nil && v.Source.OfBase64.Data != "" {
				rawBytes, err := util.DecodeBase64String(v.Source.OfBase64.Data)
				if err == nil {
					um.Parts = append(um.Parts, UniversalPart{Type: PartTypeImage, MIMEType: string(v.Source.OfBase64.MediaType), Data: rawBytes})
				}
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
					um.Parts = append(um.Parts, UniversalPart{Type: PartTypeText, Text: toolText})
				}
			}
		}

		if um.HasContent() {
			result = append(result, um)
		}
	}
	return result
}

// ParseGeminiMessages converts Gemini messages to universal format.
func ParseGeminiMessages(messages []*gemini.Content) []UniversalMessage {
	var result []UniversalMessage
	for _, msg := range messages {
		um := UniversalMessage{
			Role: ConvertToUniversalRole(msg.Role),
		}

		for _, part := range msg.Parts {
			if part.FunctionCall != nil {
				continue
			}

			// Extract inline data
			if part.InlineData != nil {
				if IsImageMIMEType(part.InlineData.MIMEType) {
					um.Parts = append(um.Parts, UniversalPart{Type: PartTypeImage, MIMEType: part.InlineData.MIMEType, Data: part.InlineData.Data})
				}
				continue
			}

			// Extract function response
			if part.FunctionResponse != nil {
				if part.FunctionResponse.Response != nil {
					if respBytes, err := json.Marshal(part.FunctionResponse.Response); err == nil && string(respBytes) != "{}" {
						um.Parts = append(um.Parts, UniversalPart{Type: PartTypeText, Text: string(respBytes)})
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
					um.Parts = append(um.Parts, UniversalPart{Type: PartTypeText, Text: part.Text})
				}
			}
		}

		if um.HasContent() {
			result = append(result, um)
		}
	}
	return result
}

// ==============================
// Building Functions (Universal → Target)
// ==============================

// BuildOpenAIMessages converts universal messages to OpenAI format.
func BuildOpenAIMessages(messages []UniversalMessage) []openai.ChatCompletionMessageParamUnion {
	var result []openai.ChatCompletionMessageParamUnion
	for _, um := range messages {
		role := um.Role.ConvertToOpenAI()

		switch role {
		case model.ChatMessageRoleAssistant:
			result = append(result, openai.AssistantMessage(um.GetTextContent()))
		case model.ChatMessageRoleSystem:
			result = append(result, openai.SystemMessage(um.GetTextContent()))
		default:
			// Ensure we use parts if there are media components
			hasMedia := false
			var targetParts []openai.ChatCompletionContentPartUnionParam

			for _, part := range um.Parts {
				if part.Type == PartTypeText {
					targetParts = append(targetParts, openai.TextContentPart(part.Text))
				} else if part.Type == PartTypeImage {
					hasMedia = true
					dataURL := util.BuildDataURL(part.MIMEType, part.Data)
					targetParts = append(targetParts, openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{URL: dataURL}))
				}
			}

			if hasMedia {
				result = append(result, openai.UserMessage(targetParts))
			} else {
				// Fallback to simple string for text-only messages to maintain clean JSON
				result = append(result, openai.UserMessage(um.GetTextContent()))
			}
		}
	}
	return result
}

// BuildOpenChatMessages converts universal messages to OpenChat format.
func BuildOpenChatMessages(messages []UniversalMessage) []*model.ChatCompletionMessage {
	var result []*model.ChatCompletionMessage
	for _, um := range messages {
		reasoning := um.Reasoning

		msg := &model.ChatCompletionMessage{
			Role: um.Role.ConvertToOpenChat(),
		}

		var targetParts []*model.ChatCompletionMessageContentPart
		hasMedia := false

		for _, part := range um.Parts {
			if part.Type == PartTypeText {
				targetParts = append(targetParts, &model.ChatCompletionMessageContentPart{
					Type: model.ChatCompletionMessageContentPartTypeText,
					Text: part.Text,
				})
			} else if part.Type == PartTypeImage {
				hasMedia = true
				dataURL := util.BuildDataURL(part.MIMEType, part.Data)
				targetParts = append(targetParts, &model.ChatCompletionMessageContentPart{
					Type:     model.ChatCompletionMessageContentPartTypeImageURL,
					ImageURL: &model.ChatMessageImageURL{URL: dataURL},
				})
			}
		}

		if hasMedia {
			msg.Content = &model.ChatCompletionMessageContent{ListValue: targetParts}
		} else if text := um.GetTextContent(); text != "" {
			// Deep copy local ref
			textVal := text
			msg.Content = &model.ChatCompletionMessageContent{StringValue: &textVal}
		}

		if reasoning != "" {
			msg.ReasoningContent = &reasoning
		}

		result = append(result, msg)
	}
	return result
}

// BuildAnthropicMessages converts universal messages to Anthropic format.
func BuildAnthropicMessages(messages []UniversalMessage) []anthropic.MessageParam {
	var result []anthropic.MessageParam

	for _, um := range messages {
		var blocks []anthropic.ContentBlockParamUnion

		// Add reasoning as thinking block
		if um.Reasoning != "" {
			blocks = append(blocks, anthropic.ContentBlockParamUnion{
				OfThinking: &anthropic.ThinkingBlockParam{
					Thinking: um.Reasoning,
				},
			})
		}

		for _, part := range um.Parts {
			if part.Type == PartTypeText {
				blocks = append(blocks, anthropic.ContentBlockParamUnion{
					OfText: &anthropic.TextBlockParam{
						Text: part.Text,
					},
				})
			} else if part.Type == PartTypeImage {
				b64 := util.GetBase64String(part.Data)
				imgBlock := anthropic.NewImageBlockBase64(part.MIMEType, b64)
				blocks = append(blocks, imgBlock)
			}
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
func BuildGeminiMessages(messages []UniversalMessage) []*gemini.Content {
	var result []*gemini.Content

	for _, um := range messages {
		var parts []*gemini.Part

		// Add reasoning as thought part
		if um.Reasoning != "" {
			parts = append(parts, &gemini.Part{
				Text:    um.Reasoning,
				Thought: true,
			})
		}

		for _, part := range um.Parts {
			if part.Type == PartTypeText {
				parts = append(parts, &gemini.Part{Text: part.Text})
			} else if part.Type == PartTypeImage {
				parts = append(parts, &gemini.Part{
					InlineData: &gemini.Blob{MIMEType: part.MIMEType, Data: part.Data},
				})
			}
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
