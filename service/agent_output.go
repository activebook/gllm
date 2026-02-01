package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/internal/ui"
	"github.com/charmbracelet/lipgloss"
)

/*
WriteText writes the given text to the Agent's Std, Markdown, and OutputFile writers if they are set.
*/
func (ag *Agent) WriteText(text string) {
	if ag.Std != nil {
		ag.Std.Writef("%s", text)
		ag.LastWrittenData = text
	}
	if ag.Markdown != nil {
		ag.Markdown.Writef("%s", text)
	}
	if ag.OutputFile != nil {
		ag.OutputFile.Writef("%s", text)
	}
}

/*
StartReasoning notifies the user and logs to file that the agent has started thinking.
It writes a status message to both Std and OutputFile if they are available.
*/
func (ag *Agent) StartReasoning() {
	if ag.Std != nil {
		if ag.Verbose {
			ag.Std.Writeln(data.ReasoningActiveColor + "Thinking ↓")
		} else {
			// Only output thinking indicator under non-verbose mode
			ag.Std.Writeln(data.ReasoningActiveColor + "Thinking..." + data.ResetSeq)
		}
	}
	if ag.OutputFile != nil {
		ag.OutputFile.Writeln("Thinking ↓")
	}
}

func (ag *Agent) CompleteReasoning() {
	if ag.Std != nil {
		if ag.Verbose {
			ag.Std.Writeln(data.ResetSeq + data.ReasoningActiveColor + "✓" + data.ResetSeq)
		}
	}
	if ag.OutputFile != nil {
		ag.OutputFile.Writeln("✓")
	}
}

/*
WriteReasoning writes the provided reasoning text to both the standard output and an output file, applying specific formatting to each if they are available.
*/
func (ag *Agent) WriteReasoning(text string) {
	if ag.Std != nil {
		// Only output reasoning content under verbose
		if ag.Verbose {
			ag.Std.Writef("%s%s", data.ReasoningDoneColor, text)
			ag.LastWrittenData = text
		}
	}
	if ag.OutputFile != nil {
		ag.OutputFile.Writef("%s", text)
	}
}

func (ag *Agent) WriteMarkdown() {
	// Render the markdown
	if ag.Markdown != nil {
		if ag.Std != nil {
			ag.Markdown.Render(ag.Std)
		}
	}
}

func (ag *Agent) WriteUsage() {
	// Render the token usage
	if ag.TokenUsage != nil {
		if ag.Std != nil {
			ag.TokenUsage.Render(ag.Std)
		}
	}
}

func (ag *Agent) WriteDiffConfirm(text string) {
	// Only write to stdout
	if ag.Std != nil {
		ag.Std.Writeln(text)
	}
}

func (ag *Agent) WriteFunctionCall(text string) {
	if ag.Std != nil {
		// Attempt to parse text as JSON
		// The text is expected to be in format "function_name(arguments)" or just raw text
		// But in our new implementation, we will pass a JSON string: {"function": name, "args": args}

		type ToolCallData struct {
			Function string      `json:"function"`
			Args     interface{} `json:"args"`
		}

		var output string
		var toolData ToolCallData
		err := json.Unmarshal([]byte(text), &toolData)
		if err != nil {
			// Fallback to original text if not JSON
			output = data.ToolCallColor + text + data.ResetSeq
		}

		// Make sure we have enough space for the border
		tcol := ui.GetTerminalWidth() - 8

		// Structured data available
		// Use lipgloss to render
		style := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(data.BorderHex)). // Tool Border
			Padding(0, 1).
			Margin(0, 0)

		titleStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(data.SectionHex)). // Tool Title
			Bold(true)

		argsStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(data.DetailHex)).Width(tcol) // Tool Args

		var content string
		var purposeVal string

		// For built-in tools, we have a map of args
		// We will try to extract purpose/description and command separately
		if argsMap, ok := toolData.Args.(map[string]interface{}); ok {
			// 1. Identify Purpose
			// MCP tool calls may not have purpose/description
			if v, ok := argsMap["purpose"].(string); ok {
				purposeVal = v
			}

			// 2. Identify Command (everything else)
			var commandParts []string

			// Then grab any args that look like command parts
			// keep the k=v pairs format for command
			for k, v := range argsMap {
				if k == "purpose" {
					continue
				} else if k == "need_confirm" {
					continue
				}
				var val string
				switch v.(type) {
				case map[string]interface{}, []interface{}, []map[string]interface{}:
					// Pretty print complex types
					bytes, _ := json.MarshalIndent(v, "      ", "  ")
					val = fmt.Sprintf("%s = %s", k, string(bytes))
				default:
					// Simple types
					val = fmt.Sprintf("%s = %v", k, v)
				}
				commandParts = append(commandParts, val)
			}

			commandVal := strings.Join(commandParts, "\n")

			// Render logic
			// Title (Function Name) -> Cyan Bold
			// Command -> White (With keys)
			// Purpose -> Gray, Dim, Wrapped

			cmdStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(data.LabelHex)).Width(tcol)      // Cmd Label
			purposeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(data.DetailHex)).Width(tcol) // Cmd Purpose

			var parts []string
			parts = append(parts, titleStyle.Render(toolData.Function))

			if commandVal != "" {
				parts = append(parts, cmdStyle.Render(commandVal))
			}

			if purposeVal != "" {
				parts = append(parts, purposeStyle.Render(purposeVal))
			}

			content = strings.Join(parts, "\n")
		}

		// Fallback if content is still empty
		if content == "" {
			// Convert Args back to string for display
			var argsStr string
			if s, ok := toolData.Args.(string); ok {
				argsStr = s
			} else {
				bytes, _ := json.MarshalIndent(toolData.Args, "", "  ")
				argsStr = string(bytes)
			}
			content = fmt.Sprintf("%s\n%s", titleStyle.Render(toolData.Function), argsStyle.Render(argsStr))
		}

		// Render logic based on Verbose flag
		if ag.Verbose {
			// Render with border and style and details
			output = style.Render(content)
		} else {
			// Simple layout for non-verbose: [call tool: tool_name]
			if purposeVal != "" {
				content = fmt.Sprintf("%s:%s", toolData.Function, purposeVal)
			} else {
				content = fmt.Sprintf("%s...", toolData.Function)
			}
			output = data.ToolCallColor + content + data.ResetSeq
		}

		ag.Std.Writeln(output)
	}
	if ag.OutputFile != nil {
		ag.OutputFile.Writef("\n%s\n", text)
	}
}

func (ag *Agent) WriteEnd() {
	// Ensure output ends with a newline to prevent shell from displaying %
	// the % character in shells like zsh when output doesn't end with newline
	//if ag.Std != nil && ag.Markdown == nil && ag.TokenUsage == nil {
	if ag.Std != nil {
		if !EndWithNewline(ag.LastWrittenData) {
			ag.Std.Writeln(data.ResetSeq)
		}
	}
}

func (ag *Agent) StartIndicator(text string) {
	if ag.Std != nil {
		// fmt.Println("Start Indicator From Agent")
		ui.GetIndicator().Start(text)
	}
}

func (ag *Agent) StopIndicator() {
	if ag.Std != nil {
		// fmt.Println("Stop Indicator From Agent")
		ui.GetIndicator().Stop()
	}
}

func (ag *Agent) Error(text string) {
	// ignore stdout, because CallAgent will return the error
	// if ag.Std != nil {
	// 	Errorf("Agent: %v\n", text)
	// }
	if ag.OutputFile != nil {
		ag.OutputFile.Writef("\n%s\n", text)
	}
}

func (ag *Agent) Warn(text string) {
	if ag.Std != nil {
		Warnf("%s", text)
	}
	if ag.OutputFile != nil {
		ag.OutputFile.Writef("\n%s\n", text)
	}
}
