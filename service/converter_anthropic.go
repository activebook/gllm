package service

import (
	"encoding/json"
	"strings"

	"github.com/activebook/gllm/util"
	anthropic "github.com/anthropics/anthropic-sdk-go"
)

func (r UniversalRole) ConvertToAnthropic() anthropic.MessageParamRole {
	switch r {
	case UniversalRoleAssistant:
		return anthropic.MessageParamRoleAssistant
	case UniversalRoleUser, UniversalRoleTool:
		return anthropic.MessageParamRoleUser
	case UniversalRoleSystem:
		return anthropic.MessageParamRoleUser
	default:
		return anthropic.MessageParamRoleUser
	}
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
				// ToolResult
				var toolTextBuilder strings.Builder
				for _, resBlock := range v.Content {
					if resv := resBlock.OfText; resv != nil && resv.Text != "" {
						if toolTextBuilder.Len() > 0 {
							toolTextBuilder.WriteByte('\n')
						}
						toolTextBuilder.WriteString(resv.Text)
					}
				}
				toolText := toolTextBuilder.String()
				isErr := false
				if v.IsError.Valid() {
					isErr = v.IsError.Value
				}
				um.ToolResult = &UniversalToolResult{
					CallID:  v.ToolUseID,
					Output:  toolText,
					IsError: isErr,
				}
			} else if v := block.OfToolUse; v != nil {
				// ToolUse
				var argsMap map[string]interface{}
				if v.Input != nil {
					// Input is `any`. In Anthropic SDK this is usually a map already, but we robustly marshal/unmarshal
					if b, err := json.Marshal(v.Input); err == nil {
						json.Unmarshal(b, &argsMap)
					}
				}
				um.ToolCalls = append(um.ToolCalls, UniversalToolCall{
					ID:   v.ID,
					Name: v.Name,
					Args: argsMap,
				})
			}
		}

		if um.HasContent() {
			result = append(result, um)
		}
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

		// Tool calls (Assistant)
		for _, tc := range um.ToolCalls {
			blocks = append(blocks, anthropic.ContentBlockParamUnion{
				OfToolUse: &anthropic.ToolUseBlockParam{
					ID:    tc.ID,
					Name:  tc.Name,
					Input: tc.Args,
				},
			})
		}

		// Tool result (User)
		if um.ToolResult != nil {
			// Needs to wrap output in a TextBlockParam array
			resBlocks := []anthropic.ToolResultBlockParamContentUnion{
				{
					OfText: &anthropic.TextBlockParam{
						Text: um.ToolResult.Output,
					},
				},
			}
			block := anthropic.ToolResultBlockParam{
				ToolUseID: um.ToolResult.CallID,
				Content:   resBlocks,
			}
			if um.ToolResult.IsError {
				block.IsError = anthropic.Opt(true)
			}
			blocks = append(blocks, anthropic.ContentBlockParamUnion{
				OfToolResult: &block,
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
