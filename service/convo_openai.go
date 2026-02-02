package service

import (
	"encoding/json"
	"fmt"

	openai "github.com/sashabaranov/go-openai"
)

/*
 * OpenAI Conversation
 */

// OpenAIConversation represents a conversation using OpenAI format
type OpenAIConversation struct {
	BaseConversation
	Messages []openai.ChatCompletionMessage
}

// PushMessages adds multiple messages to the conversation
func (c *OpenAIConversation) Push(messages ...interface{}) {
	for _, msg := range messages {
		switch v := msg.(type) {
		case openai.ChatCompletionMessage:
			c.Messages = append(c.Messages, v)
		case []openai.ChatCompletionMessage:
			c.Messages = append(c.Messages, v...)
		}
	}
}

func (c *OpenAIConversation) GetMessages() interface{} {
	return c.Messages
}

func (c *OpenAIConversation) SetMessages(messages interface{}) {
	if msgs, ok := messages.([]openai.ChatCompletionMessage); ok {
		c.Messages = msgs
	}
}

// Save persists the conversation to disk
func (c *OpenAIConversation) Save() error {
	if c.Name == "" || len(c.Messages) == 0 {
		return nil
	}

	// Most major systems (including ChatGPT and Google's Gemini) discard search results between turns
	// Always clear content for tool messages before saving to save tokens
	// Keep the "record" of the tool call (e.g., call_id: 123, tool: google_search) but drop the "body" of the result.
	empty := ""
	for i := range c.Messages {
		msg := &c.Messages[i]
		if msg.Role == openai.ChatMessageRoleTool {
			msg.Content = empty
		}
	}

	data, err := json.MarshalIndent(c.Messages, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize conversation: %w", err)
	}

	return c.writeFile(data)
}

// Load retrieves the conversation from disk
func (c *OpenAIConversation) Load() error {
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
		c.Messages = []openai.ChatCompletionMessage{}
		return nil
	}

	// Parse messages
	if err := json.Unmarshal(data, &c.Messages); err != nil {
		// If there's an error unmarshaling, it might be an old format
		return fmt.Errorf("failed to parse conversation file: %v", err)
	}

	if len(c.Messages) > 0 {
		msg := c.Messages[0]
		if msg.Content == "" && len(msg.ToolCalls) == 0 && msg.FunctionCall == nil && msg.ReasoningContent == "" {
			return fmt.Errorf("invalid conversation format: isn't a compatible format. '%s'", c.Path)
		}
	}

	return nil
}

// Clear removes all messages from the conversation
func (c *OpenAIConversation) Clear() error {
	c.Messages = []openai.ChatCompletionMessage{}
	return c.BaseConversation.Clear()
}
