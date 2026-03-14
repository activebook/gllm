package service

import (
	"encoding/json"
	"fmt"

	"github.com/activebook/gllm/util"
	openai "github.com/sashabaranov/go-openai"
)

/*
 * OpenAI session
 */

// OpenAISession represents a session using OpenAI format
type OpenAISession struct {
	BaseSession
	Messages []openai.ChatCompletionMessage
}

func (s *OpenAISession) GetMessages() interface{} {
	return s.Messages
}

func (s *OpenAISession) SetMessages(messages interface{}) {
	if msgs, ok := messages.([]openai.ChatCompletionMessage); ok {
		s.Messages = msgs
	}
}

func (s *OpenAISession) MarshalMessages(messages []openai.ChatCompletionMessage, dropToolContent bool) []byte {
	// The industry's current answer is basically "save everything by default, then compress/prune when it gets too big."
	// The complete session history, including your prompts and the model's responses, all tool executions (inputs and outputs).
	// Important: We need to copy the message, otherwise it will modify the original message
	// model need complete original message, which includes tool content to generate assistant response
	// but we don't need tool content in session file to save tokens

	empty := ""
	var data []byte
	for _, msg := range messages {
		// Never persist system messages — they are always injected fresh
		// at request time from ag.SystemPrompt. Persisting them would cause bloat
		// and stale-prompt drift across multi-turn sessions.
		if msg.Role == openai.ChatMessageRoleSystem {
			continue
		}
		// Copy message and clear tool content to save tokens
		formatted := msg
		if dropToolContent && msg.Role == openai.ChatMessageRoleTool {
			formatted.Content = empty
		}

		// Marshal to compact JSON
		line, err := json.Marshal(formatted)
		if err != nil {
			util.Warnf("failed to serialize message: %v\n", err)
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
func (s *OpenAISession) Push(messages ...interface{}) error {
	if len(messages) == 0 {
		return nil
	}

	// append new messages to s.Messages
	newmsgs := []openai.ChatCompletionMessage{}
	for _, msg := range messages {
		switch v := msg.(type) {
		case openai.ChatCompletionMessage:
			newmsgs = append(newmsgs, v)
		case []openai.ChatCompletionMessage:
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
func (s *OpenAISession) Save() error {
	if s.Name == "" || len(s.Messages) == 0 {
		return nil
	}

	// Write all messages to file
	data := s.MarshalMessages(s.Messages, false)
	return s.writeFile(data)
}

// Load retrieves the session from disk (JSONL format).
func (s *OpenAISession) Load() error {
	if s.Name == "" {
		return nil
	}

	lines, err := s.readFile()
	if err != nil {
		return err
	}

	// Handle empty or non-existent files
	if len(lines) == 0 {
		s.Messages = []openai.ChatCompletionMessage{}
		return nil
	}

	// Parse each JSONL line as a message
	s.Messages = make([]openai.ChatCompletionMessage, 0, len(lines))
	for i, line := range lines {
		var msg openai.ChatCompletionMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			return fmt.Errorf("failed to parse message at line %d: %w", i+1, err)
		}
		s.Messages = append(s.Messages, msg)
	}

	// Validate format
	if len(s.Messages) > 0 {
		msg := s.Messages[0]
		if msg.Content == "" && len(msg.ToolCalls) == 0 && msg.FunctionCall == nil && msg.ReasoningContent == "" {
			return fmt.Errorf("invalid session format: isn't a compatible format. '%s'", s.Path)
		}
	}

	return nil
}

// Clear removes all messages from the session
func (s *OpenAISession) Clear() error {
	s.Messages = []openai.ChatCompletionMessage{}
	return s.BaseSession.Clear()
}
