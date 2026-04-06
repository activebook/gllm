package service

import (
	"encoding/json"

	"github.com/activebook/gllm/util"
	openai "github.com/openai/openai-go/v3"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
)

func (r UniversalRole) ConvertToOpenAI() string {
	// Openai uses string "assistant", "user", "system", "tool"
	switch r {
	case UniversalRoleAssistant:
		return model.ChatMessageRoleAssistant
	case UniversalRoleUser:
		return model.ChatMessageRoleUser
	case UniversalRoleSystem:
		return model.ChatMessageRoleSystem
	case UniversalRoleTool:
		return model.ChatMessageRoleTool
	default:
		return string(r)
	}
}

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

		// Marshal each message exactly once; reuse `raw` for all extraction branches
		// (content, reasoning, tool_calls, tool_call_id) to avoid redundant work.
		var raw map[string]interface{}
		if data, err := json.Marshal(msg); err == nil {
			json.Unmarshal(data, &raw) //nolint:errcheck — unmarshal of our own marshal output
		}

		if raw != nil {
			// Content as string
			if contentStr, ok := raw["content"].(string); ok && contentStr != "" {
				um.Parts = append(um.Parts, UniversalPart{Type: PartTypeText, Text: contentStr})
			} else if contentArr, ok := raw["content"].([]interface{}); ok {
				// Content as array of parts
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

			// Reasoning content
			if reasoning, ok := raw["reasoning_content"].(string); ok && reasoning != "" {
				um.Reasoning = reasoning
			} else if reasoning, ok := raw["reasoning"].(string); ok && reasoning != "" {
				um.Reasoning = reasoning
			}

			// Tool calls — assistant role (reuse raw; no second marshal)
			if role == model.ChatMessageRoleAssistant {
				if toolCallsRaw, ok := raw["tool_calls"].([]interface{}); ok {
					for _, tcRaw := range toolCallsRaw {
						if tcObj, ok := tcRaw.(map[string]interface{}); ok {
							id, _ := tcObj["id"].(string)
							if funcObj, ok := tcObj["function"].(map[string]interface{}); ok {
								name, _ := funcObj["name"].(string)
								argsStr, _ := funcObj["arguments"].(string)

								var argsMap map[string]interface{}
								if argsStr != "" {
									json.Unmarshal([]byte(argsStr), &argsMap) //nolint:errcheck
								}

								um.ToolCalls = append(um.ToolCalls, UniversalToolCall{
									ID:   id,
									Name: name,
									Args: argsMap,
								})
							}
						}
					}
				}
			}

			// Tool result — tool role (reuse raw; no second marshal)
			if role == model.ChatMessageRoleTool {
				toolCallID, _ := raw["tool_call_id"].(string)
				contentStr, _ := raw["content"].(string)
				um.ToolResult = &UniversalToolResult{
					CallID: toolCallID,
					// Name is absent in OpenAI tool result messages; reconstructed during correlation.
					Output: contentStr,
				}
				// Clear parts: content is now owned by ToolResult
				um.Parts = nil
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

// BuildOpenAIMessages converts universal messages to OpenAI format.
func BuildOpenAIMessages(messages []UniversalMessage) []openai.ChatCompletionMessageParamUnion {
	var result []openai.ChatCompletionMessageParamUnion
	for _, um := range messages {
		role := um.Role.ConvertToOpenAI()

		// Tool results must be emitted as "tool" role in OpenAI, regardless of whether
		// they were parsed from a user message (e.g. Anthropic) or tool message.
		if um.ToolResult != nil {
			role = model.ChatMessageRoleTool
		}

		switch role {
		case model.ChatMessageRoleAssistant:
			// Inject reasoning back as <think>...</think> tags — the symmetric inverse of the
			// ExtractThinkTags parse pass. OpenAI's request API has no dedicated reasoning_content
			// field, so this is the only portable round-trip format.
			content := util.InjectThinkTags(um.GetTextContent(), um.Reasoning)

			if len(um.ToolCalls) > 0 {
				var toolCallsRaw []map[string]interface{}
				for _, tc := range um.ToolCalls {
					argsStr := "{}"
					if b, err := json.Marshal(tc.Args); err == nil && string(b) != "null" {
						argsStr = string(b)
					}
					toolCallsRaw = append(toolCallsRaw, map[string]interface{}{
						"id":   tc.ID,
						"type": "function",
						"function": map[string]interface{}{
							"name":      tc.Name,
							"arguments": argsStr,
						},
					})
				}

				// Reconstruct via JSON to set tool_calls without fighting the internal struct unions.
				msgRaw := map[string]interface{}{
					"role":       "assistant",
					"content":    content,
					"tool_calls": toolCallsRaw,
				}
				var p openai.ChatCompletionMessageParamUnion
				if b, err := json.Marshal(msgRaw); err == nil {
					json.Unmarshal(b, &p) //nolint:errcheck
					result = append(result, p)
				}
			} else {
				result = append(result, openai.AssistantMessage(content))
			}
		case model.ChatMessageRoleSystem:
			result = append(result, openai.SystemMessage(um.GetTextContent()))
		case model.ChatMessageRoleTool:
			if um.ToolResult != nil {
				result = append(result, openai.ToolMessage(um.ToolResult.Output, um.ToolResult.CallID))
			}
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
