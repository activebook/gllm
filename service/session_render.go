package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/io"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	model "github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	gemini "google.golang.org/genai"
)

// RenderSessionHistory loads and formats existing messages when resuming a REPL session.
func RenderSessionHistory(agent *data.AgentConfig, name string) string {
	if name == "" {
		return ""
	}

	sessionData, err := ReadSessionContent(name)
	if err != nil || len(bytes.TrimSpace(sessionData)) == 0 {
		return "" // brand-new session or unreadable file — start fresh silently
	}

	_, provider, _ := CheckSessionFormat(agent, sessionData)
	if provider == ModelProviderUnknown {
		return ""
	}

	switch provider {
	case ModelProviderGemini:
		return renderGeminiSessionHistory(sessionData)
	case ModelProviderAnthropic:
		return renderAnthropicSessionHistory(sessionData)
	case ModelProviderOpenAI, ModelProviderOpenAICompatible:
		return renderOpenAISessionHistory(sessionData)
	default:
		return ""
	}
}

// -------------------------------------------------------------------------
// Common Rendering Helpers
// -------------------------------------------------------------------------

func renderUserBlock(text string) string {
	tcol := io.GetTerminalWidth()
	return lipgloss.NewStyle().
		Background(lipgloss.Color(data.CurrentTheme.Background)).
		Foreground(lipgloss.Color(data.CurrentTheme.Foreground)).
		Width(tcol).
		Padding(1, 2).
		Margin(0, 0, 1, 0).
		Render(text)
}

func renderMediaTag(tag string) string {
	return data.MediaColor + "[" + tag + "]" + data.ResetSeq
}

func renderThinkingBlock(content string) string {
	var sb strings.Builder
	sb.WriteString(data.ReasoningTagColor + "Thinking ↓" + data.ResetSeq + "\n")
	for _, line := range strings.Split(strings.TrimSpace(content), "\n") {
		sb.WriteString(data.ReasoningTextColor + "  " + line + data.ResetSeq + "\n")
	}
	sb.WriteString(data.ReasoningTagColor + "✓" + data.ResetSeq + "\n")
	return sb.String()
}

func renderToolCallBox(name string, args interface{}) string {
	tcol := io.GetTerminalWidth() - 8
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(data.BorderHex)).
		Padding(0, 1)

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(data.SectionHex)).Bold(true)

	argsStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(data.DetailHex)).Width(tcol)

	// extractFirstArg is a helper in agent_output.go that handles both maps and strings
	detail := extractFirstArg(args)

	var content string
	if detail == "" {
		content = titleStyle.Render(name)
	} else {
		content = fmt.Sprintf("%s\n%s", titleStyle.Render(name), argsStyle.Render(detail))
	}

	return style.Render(content)
}

func renderMarkdown(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	tr, err := glamour.NewTermRenderer(glamour.WithStandardStyle(data.MostSimilarGlamourStyle()))
	if err != nil {
		tr, _ = glamour.NewTermRenderer(glamour.WithAutoStyle())
	}
	out, err := tr.Render(text)
	if err != nil {
		return text // fallback to raw
	}
	return out
}

// -------------------------------------------------------------------------
// Gemini Renderer
// -------------------------------------------------------------------------

func renderGeminiSessionHistory(input []byte) string {
	var sb strings.Builder
	lines := bytes.Split(input, []byte("\n"))

	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		var msg gemini.Content
		if err := json.Unmarshal(line, &msg); err != nil {
			continue
		}

		switch msg.Role {
		case gemini.RoleUser:
			var userText []string
			for _, part := range msg.Parts {
				if part.Text != "" {
					userText = append(userText, part.Text)
				} else if part.InlineData != nil {
					if strings.HasPrefix(part.InlineData.MIMEType, "image/") {
						userText = append(userText, renderMediaTag("Image"))
					} else if strings.HasPrefix(part.InlineData.MIMEType, "audio/") {
						userText = append(userText, renderMediaTag("Audio"))
					} else {
						userText = append(userText, renderMediaTag("Document"))
					}
				} else if part.FileData != nil {
					if strings.HasPrefix(part.FileData.MIMEType, "image/") {
						userText = append(userText, renderMediaTag("Image"))
					} else if strings.HasPrefix(part.FileData.MIMEType, "audio/") {
						userText = append(userText, renderMediaTag("Audio"))
					} else {
						userText = append(userText, renderMediaTag("Document"))
					}
				}
			}
			if len(userText) > 0 {
				sb.WriteString(renderUserBlock(strings.Join(userText, "\n")))
				sb.WriteString("\n")
			}
		case gemini.RoleModel:
			var markdownBuf strings.Builder

			for _, part := range msg.Parts {
				if part.Thought {
					sb.WriteString(renderThinkingBlock(part.Text))
					// sb.WriteString("\n")
				} else if part.FunctionCall != nil {
					sb.WriteString(renderToolCallBox(part.FunctionCall.Name, part.FunctionCall.Args))
					sb.WriteString("\n")
				} else if part.FunctionResponse != nil {
					// silently skip function response
				} else if part.Text != "" {
					markdownBuf.WriteString(part.Text)
				}
			}

			// flush markdown
			if markdownBuf.Len() > 0 {
				sb.WriteString(renderMarkdown(markdownBuf.String()))
			}
		}
	}

	return sb.String()
}

// -------------------------------------------------------------------------
// Anthropic Renderer
// -------------------------------------------------------------------------

func renderAnthropicSessionHistory(input []byte) string {
	var sb strings.Builder
	lines := bytes.Split(input, []byte("\n"))

	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		var msg anthropic.MessageParam
		if err := json.Unmarshal(line, &msg); err != nil {
			continue
		}

		switch msg.Role {
		case anthropic.MessageParamRoleUser:
			var userText []string
			for _, block := range msg.Content {
				if v := block.OfText; v != nil {
					userText = append(userText, v.Text)
				} else if block.OfImage != nil {
					userText = append(userText, renderMediaTag("Image"))
				} else if block.OfDocument != nil {
					userText = append(userText, renderMediaTag("Document"))
				} else if block.OfToolResult != nil {
					// silently skip user tool results
				}
			}
			if len(userText) > 0 {
				sb.WriteString(renderUserBlock(strings.Join(userText, "\n")))
				sb.WriteString("\n")
			}
		case anthropic.MessageParamRoleAssistant:
			var markdownBuf strings.Builder

			for _, block := range msg.Content {
				if v := block.OfThinking; v != nil {
					sb.WriteString(renderThinkingBlock(v.Thinking))
					// sb.WriteString("\n")
				} else if v := block.OfRedactedThinking; v != nil {
					sb.WriteString(renderThinkingBlock(v.Data))
					// sb.WriteString("\n")
				} else if v := block.OfToolUse; v != nil {
					sb.WriteString(renderToolCallBox(v.Name, v.Input))
					sb.WriteString("\n")
				} else if v := block.OfText; v != nil {
					markdownBuf.WriteString(v.Text)
				}
			}

			if markdownBuf.Len() > 0 {
				sb.WriteString(renderMarkdown(markdownBuf.String()))
			}
		}
	}

	return sb.String()
}

// -------------------------------------------------------------------------
// OpenAI / OpenChat Renderer
// -------------------------------------------------------------------------

func renderOpenAISessionHistory(input []byte) string {
	var sb strings.Builder
	lines := bytes.Split(input, []byte("\n"))

	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		var msg model.ChatCompletionMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			continue
		}

		if msg.Role == model.ChatMessageRoleSystem || msg.Role == model.ChatMessageRoleTool || msg.Role == "function" {
			// silently skip
			continue
		}

		switch msg.Role {
		case model.ChatMessageRoleUser:
			var userText []string
			if msg.Content != nil {
				if msg.Content.StringValue != nil && *msg.Content.StringValue != "" {
					userText = append(userText, *msg.Content.StringValue)
				} else if len(msg.Content.ListValue) > 0 {
					for _, part := range msg.Content.ListValue {
						switch part.Type {
						case model.ChatCompletionMessageContentPartTypeText:
							userText = append(userText, part.Text)
						case model.ChatCompletionMessageContentPartTypeImageURL:
							userText = append(userText, renderMediaTag("Image"))
						case "input_audio":
							userText = append(userText, renderMediaTag("Audio"))
						case "file":
							userText = append(userText, renderMediaTag("Document"))
						}
					}
				}
			}
			if len(userText) > 0 {
				sb.WriteString(renderUserBlock(strings.Join(userText, "\n")))
				sb.WriteString("\n")
			}
		case model.ChatMessageRoleAssistant:
			if msg.ReasoningContent != nil && *msg.ReasoningContent != "" {
				sb.WriteString(renderThinkingBlock(*msg.ReasoningContent))
				// sb.WriteString("\n")
			}

			if len(msg.ToolCalls) > 0 {
				for _, tool := range msg.ToolCalls {
					var rawArgs interface{}
					// Try to unmarshal args if it's JSON text, otherwise pass as string
					if err := json.Unmarshal([]byte(tool.Function.Arguments), &rawArgs); err != nil {
						rawArgs = tool.Function.Arguments
					}
					sb.WriteString(renderToolCallBox(tool.Function.Name, rawArgs))
					sb.WriteString("\n")
				}
			}

			var markdownBuf strings.Builder
			if msg.Content != nil {
				if msg.Content.StringValue != nil && *msg.Content.StringValue != "" {
					markdownBuf.WriteString(*msg.Content.StringValue)
				} else if len(msg.Content.ListValue) > 0 {
					for _, part := range msg.Content.ListValue {
						if part.Type == model.ChatCompletionMessageContentPartTypeText {
							markdownBuf.WriteString(part.Text)
						}
					}
				}
			}

			if markdownBuf.Len() > 0 {
				sb.WriteString(renderMarkdown(markdownBuf.String()))
			}
		}
	}

	return sb.String()
}
