package service

import (
	"encoding/json"

	"github.com/activebook/gllm/util"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
)

func (r UniversalRole) ConvertToOpenChat() string {
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

		// Extract tool calls
		if msg.Role == model.ChatMessageRoleAssistant {
			for _, tc := range msg.ToolCalls {
				var argsMap map[string]interface{}
				if tc.Function.Arguments != "" {
					json.Unmarshal([]byte(tc.Function.Arguments), &argsMap)
				}
				um.ToolCalls = append(um.ToolCalls, UniversalToolCall{
					ID:   tc.ID,
					Name: tc.Function.Name,
					Args: argsMap,
				})
			}
		} else if msg.Role == model.ChatMessageRoleTool {
			output := ""
			if msg.Content != nil && msg.Content.StringValue != nil {
				output = *msg.Content.StringValue
			}
			um.ToolResult = &UniversalToolResult{
				CallID: msg.ToolCallID,
				Output: output,
			}
			// Clear parts since we extracted the content into ToolResult
			um.Parts = nil
		}

		if um.HasContent() {
			result = append(result, um)
		}
	}
	return result
}

// BuildOpenChatMessages converts universal messages to OpenChat format.
func BuildOpenChatMessages(messages []UniversalMessage) []*model.ChatCompletionMessage {
	var result []*model.ChatCompletionMessage
	for _, um := range messages {
		msg := &model.ChatCompletionMessage{
			Role: um.Role.ConvertToOpenChat(),
		}

		if um.ToolResult != nil {
			msg.Role = model.ChatMessageRoleTool
		}

		reasoning := um.Reasoning
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
		} else if text := um.GetTextContent(); text != "" || len(um.ToolCalls) == 0 {
			// Only set content if we have text OR there are no tool calls (required for normal messages)
			// For assistant messages with tool calls, content can be omitted
			textVal := text
			msg.Content = &model.ChatCompletionMessageContent{StringValue: &textVal}
		}

		if reasoning != "" {
			msg.ReasoningContent = &reasoning
		}

		if len(um.ToolCalls) > 0 && msg.Role == model.ChatMessageRoleAssistant {
			var toolCalls []*model.ToolCall
			for _, tc := range um.ToolCalls {
				argsStr := "{}"
				if b, err := json.Marshal(tc.Args); err == nil && string(b) != "null" {
					argsStr = string(b)
				}
				toolCalls = append(toolCalls, &model.ToolCall{
					ID:   tc.ID,
					Type: model.ToolTypeFunction,
					Function: model.FunctionCall{
						Name:      tc.Name,
						Arguments: argsStr,
					},
				})
			}
			// When tools are used, content needs to be an empty string if nothing was generated as text (as per volcengine spec)
			if msg.Content == nil {
				emptyStr := ""
				msg.Content = &model.ChatCompletionMessageContent{StringValue: &emptyStr}
			}
			msg.ToolCalls = toolCalls
		} else if um.ToolResult != nil && msg.Role == model.ChatMessageRoleTool {
			msg.ToolCallID = um.ToolResult.CallID
		}

		result = append(result, msg)
	}
	return result
}
