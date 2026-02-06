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

func (c *OpenAIConversation) GetMessages() interface{} {
	return c.Messages
}

func (c *OpenAIConversation) SetMessages(messages interface{}) {
	if msgs, ok := messages.([]openai.ChatCompletionMessage); ok {
		c.Messages = msgs
	}
}

func (c *OpenAIConversation) MarshalMessages(messages []openai.ChatCompletionMessage) []byte {
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
		formatted := msg
		if msg.Role == openai.ChatMessageRoleTool {
			formatted.Content = empty
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
func (c *OpenAIConversation) Push(messages ...interface{}) error {
	if c.Name == "" || len(messages) == 0 {
		return nil
	}

	// append new messages to c.Messages
	newmsgs := []openai.ChatCompletionMessage{}
	for _, msg := range messages {
		switch v := msg.(type) {
		case openai.ChatCompletionMessage:
			newmsgs = append(newmsgs, v)
		case []openai.ChatCompletionMessage:
			newmsgs = append(newmsgs, v...)
		}
	}
	c.Messages = append(c.Messages, newmsgs...)

	// append new messages to file
	data := c.MarshalMessages(newmsgs)
	return c.appendFile(data)
}

// Save persists the conversation to disk using JSONL format (one message per line).
func (c *OpenAIConversation) Save() error {
	if c.Name == "" || len(c.Messages) == 0 {
		return nil
	}

	// Write all messages to file
	data := c.MarshalMessages(c.Messages)
	return c.writeFile(data)
}

// Load retrieves the conversation from disk (JSONL format).
func (c *OpenAIConversation) Load() error {
	if c.Name == "" {
		return nil
	}

	lines, err := c.readFile()
	if err != nil {
		return err
	}

	// Handle empty or non-existent files
	if len(lines) == 0 {
		c.Messages = []openai.ChatCompletionMessage{}
		return nil
	}

	// Parse each JSONL line as a message
	c.Messages = make([]openai.ChatCompletionMessage, 0, len(lines))
	for i, line := range lines {
		var msg openai.ChatCompletionMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			return fmt.Errorf("failed to parse message at line %d: %w", i+1, err)
		}
		c.Messages = append(c.Messages, msg)
	}

	// Validate format
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
