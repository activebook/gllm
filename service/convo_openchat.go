package service

import (
	"encoding/json"
	"fmt"

	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
)

/*
 * OpenChat Conversation
 */

// OpenChatConversation manages conversations for Volcengine model
type OpenChatConversation struct {
	BaseConversation
	Messages []*model.ChatCompletionMessage
}

func (c *OpenChatConversation) GetMessages() interface{} {
	return c.Messages
}

func (c *OpenChatConversation) SetMessages(messages interface{}) {
	if msgs, ok := messages.([]*model.ChatCompletionMessage); ok {
		c.Messages = msgs
	}
}

func (c *OpenChatConversation) MarshalMessages(messages []*model.ChatCompletionMessage) []byte {
	// Most major systems (including ChatGPT and Google's Gemini) discard search results between turns
	// Always clear content for tool messages before saving to save tokens
	// Keep the "record" of the tool call (e.g., call_id: 123, tool: google_search) but drop the "body" of the result.
	// Important: We need to copy the message, otherwise it will modify the original message
	// model need complete original message, which includes tool content to generate assistant response
	// but we don't need tool content in conversation file to save tokens
	empty := ""
	var data []byte
	for _, msg := range messages {
		// Copy message and clear tool content to save tokens
		formatted := *msg
		if msg.Role == model.ChatMessageRoleTool {
			formatted.Content = &model.ChatCompletionMessageContent{StringValue: &empty}
		}

		// Marshal to compact JSON
		line, err := json.Marshal(formatted)
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
func (c *OpenChatConversation) Push(messages ...interface{}) error {
	if c.Name == "" || len(messages) == 0 {
		return nil
	}

	newmsgs := []*model.ChatCompletionMessage{}
	for _, msg := range messages {
		switch v := msg.(type) {
		case *model.ChatCompletionMessage:
			newmsgs = append(newmsgs, v)
		case []*model.ChatCompletionMessage:
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
func (c *OpenChatConversation) Save() error {
	if c.Name == "" || len(c.Messages) == 0 {
		return nil
	}

	data := c.MarshalMessages(c.Messages)
	return c.writeFile(data)
}

// Load retrieves the conversation from disk (JSONL format).
func (c *OpenChatConversation) Load() error {
	if c.Name == "" {
		return nil
	}

	lines, err := c.readFile()
	if err != nil {
		return err
	}

	// Handle empty or non-existent files
	if len(lines) == 0 {
		c.Messages = []*model.ChatCompletionMessage{}
		return nil
	}

	// Parse each JSONL line as a message
	c.Messages = make([]*model.ChatCompletionMessage, 0, len(lines))
	for i, line := range lines {
		var msg model.ChatCompletionMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			return fmt.Errorf("failed to parse message at line %d: %w", i+1, err)
		}
		c.Messages = append(c.Messages, &msg)
	}

	// Validate format
	if len(c.Messages) > 0 {
		msg := c.Messages[0]
		if msg.Content == nil && msg.FunctionCall == nil && msg.ReasoningContent == nil && msg.Name == nil && len(msg.ToolCalls) == 0 {
			return fmt.Errorf("invalid conversation format: isn't a compatible format. '%s'", c.Path)
		}
	}

	return nil
}

// Clear removes all messages from the conversation
func (c *OpenChatConversation) Clear() error {
	c.Messages = []*model.ChatCompletionMessage{}
	return c.BaseConversation.Clear()
}
