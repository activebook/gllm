package service

import (
	"encoding/json"
	"fmt"

	"github.com/activebook/gllm/util"
	"github.com/anthropics/anthropic-sdk-go"
)

/*
 * Anthropic session
 */

// AnthropicSession represents a session using Anthropic format
type AnthropicSession struct {
	BaseSession
	Messages []anthropic.MessageParam
}

func (s *AnthropicSession) GetMessages() interface{} {
	return s.Messages
}

func (s *AnthropicSession) SetMessages(messages interface{}) {
	if msgs, ok := messages.([]anthropic.MessageParam); ok {
		s.Messages = msgs
	}
}

func (s *AnthropicSession) MarshalMessages(messages []anthropic.MessageParam, dropToolContent bool) []byte {
	// The industry's current answer is basically "save everything by default, then compress/prune when it gets too big."
	// The complete session history, including your prompts and the model's responses, all tool executions (inputs and outputs).
	empty := ""
	var data []byte
	for _, msg := range messages {
		// Shallow copy the message first
		msgCopy := msg

		// Check if this message has any tool results that need content clearing
		hasToolResult := false
		if msg.Content != nil {
			for _, block := range msg.Content {
				if block.OfToolResult != nil {
					hasToolResult = true
					break
				}
			}
		}

		// Only deep copy Content slice if we need to modify tool results
		if hasToolResult && dropToolContent {
			msgCopy.Content = make([]anthropic.ContentBlockParamUnion, len(msg.Content))
			for j, block := range msg.Content {
				if block.OfToolResult != nil {
					// Create a NEW ToolResultBlockParam with empty content
					msgCopy.Content[j] = anthropic.ContentBlockParamUnion{
						OfToolResult: &anthropic.ToolResultBlockParam{
							ToolUseID: block.OfToolResult.ToolUseID,
							IsError:   block.OfToolResult.IsError,
							Content: []anthropic.ToolResultBlockParamContentUnion{
								{
									OfText: &anthropic.TextBlockParam{
										Text: empty,
									},
								},
							},
						},
					}
				} else {
					msgCopy.Content[j] = block
				}
			}
		}

		// Marshal to compact JSON
		line, err := json.Marshal(msgCopy)
		if err != nil {
			util.LogWarnf("failed to serialize message: %v\n", err)
			continue
		}

		// Write all lines as JSONL (one message per line)
		data = append(data, line...)
		data = append(data, '\n')
	}
	return data
}

// PushMessages adds multiple messages to the session (high performance)
// Uses append-mode for incremental saves using JSONL format (one message per line)
func (s *AnthropicSession) Push(messages ...interface{}) error {
	if len(messages) == 0 {
		return nil
	}

	newmsgs := []anthropic.MessageParam{}
	for _, msg := range messages {
		switch v := msg.(type) {
		case anthropic.MessageParam:
			newmsgs = append(newmsgs, v)
		case []anthropic.MessageParam:
			newmsgs = append(newmsgs, v...)
		}
	}

	// Always append to in-memory messages (needed for tool-call loop in single-turn mode)
	s.Messages = append(s.Messages, newmsgs...)

	// Only persist to file if session has a name
	if s.Name == "" {
		return nil
	}
	data := s.MarshalMessages(newmsgs, false)
	return s.appendFile(data)
}

// Save persists the session to disk using JSONL format (one message per line).
func (s *AnthropicSession) Save() error {
	if s.Name == "" || len(s.Messages) == 0 {
		return nil
	}

	data := s.MarshalMessages(s.Messages, false)
	return s.writeFile(data)
}

// Load retrieves the session from disk (JSONL format).
func (s *AnthropicSession) Load() error {
	if s.Name == "" {
		return nil
	}

	lines, err := s.readFile()
	if err != nil {
		return err
	}

	// Handle empty or non-existent files
	if len(lines) == 0 {
		s.Messages = []anthropic.MessageParam{}
		return nil
	}

	// Parse each JSONL line as a message
	s.Messages = make([]anthropic.MessageParam, 0, len(lines))
	for i, line := range lines {
		var msg anthropic.MessageParam
		if err := json.Unmarshal(line, &msg); err != nil {
			return fmt.Errorf("failed to parse message at line %d: %w", i+1, err)
		}
		s.Messages = append(s.Messages, msg)
	}

	return nil
}

// Clear removes all messages from the session
func (s *AnthropicSession) Clear() error {
	s.Messages = []anthropic.MessageParam{}
	return s.BaseSession.Clear()
}
