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

// PushMessages adds multiple messages to the conversation
func (c *OpenChatConversation) Push(messages ...interface{}) {
	for _, msg := range messages {
		switch v := msg.(type) {
		case *model.ChatCompletionMessage:
			c.Messages = append(c.Messages, v)
		case []*model.ChatCompletionMessage:
			c.Messages = append(c.Messages, v...)
		}
	}
}

func (c *OpenChatConversation) GetMessages() interface{} {
	return c.Messages
}

func (c *OpenChatConversation) SetMessages(messages interface{}) {
	if msgs, ok := messages.([]*model.ChatCompletionMessage); ok {
		c.Messages = msgs
	}
}

// Save persists the conversation to disk
func (c *OpenChatConversation) Save() error {
	if c.Name == "" || len(c.Messages) == 0 {
		return nil // Don't create empty files
	}

	// Most major systems (including ChatGPT and Google's Gemini) discard search results between turns
	// Always clear content for tool messages before saving to save tokens
	// Keep the "record" of the tool call (e.g., call_id: 123, tool: google_search) but drop the "body" of the result.
	empty := ""
	for _, msg := range c.Messages {
		if msg.Role == model.ChatMessageRoleTool {
			//msg.Content = nil // or "" if Content is a string
			msg.Content = &model.ChatCompletionMessageContent{StringValue: &empty}
		}
	}

	data, err := json.MarshalIndent(c.Messages, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize conversation: %w", err)
	}

	return c.writeFile(data)
}

// Load retrieves the conversation from disk
func (c *OpenChatConversation) Load() error {
	if c.Name == "" {
		return nil
	}

	data, err := c.readFile()
	if err != nil || data == nil {
		return err
	}

	// Handle empty files
	if len(data) == 0 {
		c.Messages = []*model.ChatCompletionMessage{}
		return nil
	}

	err = json.Unmarshal(data, &c.Messages)
	if err != nil {
		return fmt.Errorf("failed to deserialize conversation: %w", err)
	}

	if len(c.Messages) > 0 {
		msg := c.Messages[0]
		// Check whether all nil (aka not correct format)
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
