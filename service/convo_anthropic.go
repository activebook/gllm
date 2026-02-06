package service

import (
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
)

/*
 * Anthropic Conversation
 */

// AnthropicConversation represents a conversation using Anthropic format
type AnthropicConversation struct {
	BaseConversation
	Messages []anthropic.MessageParam
}

func (c *AnthropicConversation) GetMessages() interface{} {
	return c.Messages
}

func (c *AnthropicConversation) SetMessages(messages interface{}) {
	if msgs, ok := messages.([]anthropic.MessageParam); ok {
		c.Messages = msgs
	}
}

func (c *AnthropicConversation) MarshalMessages(messages []anthropic.MessageParam) []byte {
	// Important: We only deep copy tool result content that we modify
	// model needs complete original message, which includes tool content to generate assistant response
	// but we don't need tool content in conversation file to save tokens
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
		if hasToolResult {
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
			Warnf("failed to serialize message: %w", err)
			continue
		}

		// Write all lines as JSONL (one message per line)
		data = append(data, line...)
		data = append(data, '\n')
	}
	return data
}

// PushMessages adds multiple messages to the conversation (high performance)
// Uses append-mode for incremental saves using JSONL format (one message per line)
func (c *AnthropicConversation) Push(messages ...interface{}) error {
	if c.Name == "" || len(messages) == 0 {
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

	// append new messages to c.Messages
	c.Messages = append(c.Messages, newmsgs...)

	// append new messages to file
	data := c.MarshalMessages(newmsgs)
	return c.appendFile(data)
}

// Save persists the conversation to disk using JSONL format (one message per line).
func (c *AnthropicConversation) Save() error {
	if c.Name == "" || len(c.Messages) == 0 {
		return nil
	}

	data := c.MarshalMessages(c.Messages)
	return c.writeFile(data)
}

// Load retrieves the conversation from disk (JSONL format).
func (c *AnthropicConversation) Load() error {
	if c.Name == "" {
		return nil
	}

	lines, err := c.readFile()
	if err != nil {
		return err
	}

	// Handle empty or non-existent files
	if len(lines) == 0 {
		c.Messages = []anthropic.MessageParam{}
		return nil
	}

	// Parse each JSONL line as a message
	c.Messages = make([]anthropic.MessageParam, 0, len(lines))
	for i, line := range lines {
		var msg anthropic.MessageParam
		if err := json.Unmarshal(line, &msg); err != nil {
			return fmt.Errorf("failed to parse message at line %d: %w", i+1, err)
		}
		c.Messages = append(c.Messages, msg)
	}

	return nil
}

// Clear removes all messages from the conversation
func (c *AnthropicConversation) Clear() error {
	c.Messages = []anthropic.MessageParam{}
	return c.BaseConversation.Clear()
}
