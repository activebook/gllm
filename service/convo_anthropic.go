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

// PushMessages adds multiple messages to the conversation
func (c *AnthropicConversation) Push(messages ...interface{}) {
	for _, msg := range messages {
		switch v := msg.(type) {
		case anthropic.MessageParam:
			c.Messages = append(c.Messages, v)
		case []anthropic.MessageParam:
			c.Messages = append(c.Messages, v...)
		}
	}
}

func (c *AnthropicConversation) GetMessages() interface{} {
	return c.Messages
}

func (c *AnthropicConversation) SetMessages(messages interface{}) {
	if msgs, ok := messages.([]anthropic.MessageParam); ok {
		c.Messages = msgs
	}
}

// Save persists the conversation to disk
func (c *AnthropicConversation) Save() error {
	if c.Name == "" || len(c.Messages) == 0 {
		return nil
	}

	// Important: We only deep copy tool result content that we modify
	// model needs complete original message, which includes tool content to generate assistant response
	// but we don't need tool content in conversation file to save tokens
	empty := ""
	formatMessages := make([]anthropic.MessageParam, len(c.Messages))
	for i, msg := range c.Messages {
		// Shallow copy the message first - efficient since we're not modifying most fields
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
					// Only here we create a NEW ToolResultBlockParam with empty content
					msgCopy.Content[j] = anthropic.ContentBlockParamUnion{
						OfToolResult: &anthropic.ToolResultBlockParam{
							ToolUseID: block.OfToolResult.ToolUseID,
							IsError:   block.OfToolResult.IsError,
							// Replace content with empty - this is a NEW slice, not modifying original
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
					// No change needed, shallow copy the block
					msgCopy.Content[j] = block
				}
			}
		}

		formatMessages[i] = msgCopy
	}

	data, err := json.MarshalIndent(formatMessages, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize conversation: %w", err)
	}

	return c.writeFile(data)
}

// Load retrieves the conversation from disk
func (c *AnthropicConversation) Load() error {
	if c.Name == "" {
		return nil
	}

	// read file
	data, err := c.readFile()
	if err != nil || data == nil {
		return err
	}

	// Handle empty files
	if len(data) == 0 {
		c.Messages = []anthropic.MessageParam{}
		return nil
	}

	// Parse messages
	if err := json.Unmarshal(data, &c.Messages); err != nil {
		return fmt.Errorf("failed to parse conversation file: %v", err)
	}

	return nil
}

// Clear removes all messages from the conversation
func (c *AnthropicConversation) Clear() error {
	c.Messages = []anthropic.MessageParam{}
	return c.BaseConversation.Clear()
}
