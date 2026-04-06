package service

import (
	"encoding/json"

	gemini "google.golang.org/genai"
)

func (r UniversalRole) ConvertToGemini() string {
	switch r {
	case UniversalRoleAssistant:
		return gemini.RoleModel
	case UniversalRoleUser, UniversalRoleTool:
		return gemini.RoleUser
	case UniversalRoleSystem:
		return gemini.RoleUser
	default:
		return string(r)
	}
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
				um.ToolCalls = append(um.ToolCalls, UniversalToolCall{
					ID:   part.FunctionCall.ID,
					Name: part.FunctionCall.Name,
					Args: part.FunctionCall.Args,
				})
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
				output := ""
				if part.FunctionResponse.Response != nil {
					if outStr, ok := part.FunctionResponse.Response["output"].(string); ok {
						output = outStr
					} else {
						// fallback to serialize the entire response
						if respBytes, err := json.Marshal(part.FunctionResponse.Response); err == nil {
							output = string(respBytes)
						}
					}
				}
				um.ToolResult = &UniversalToolResult{
					CallID: part.FunctionResponse.ID,
					Name:   part.FunctionResponse.Name,
					Output: output,
				}
				continue
			}

			// Extract text content
			if part.Text != "" {
				if part.Thought {
					if len(um.Reasoning) > 0 {
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
					InlineData: &gemini.Blob{
						MIMEType: part.MIMEType,
						Data:     part.Data,
					},
				})
			}
		}

		for _, tc := range um.ToolCalls {
			parts = append(parts, &gemini.Part{
				FunctionCall: &gemini.FunctionCall{
					ID:   tc.ID,
					Name: tc.Name,
					Args: tc.Args,
				},
			})
		}

		if um.ToolResult != nil {
			parts = append(parts, &gemini.Part{
				FunctionResponse: &gemini.FunctionResponse{
					ID:       um.ToolResult.CallID,
					Name:     um.ToolResult.Name,
					Response: map[string]any{"output": um.ToolResult.Output},
				},
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
