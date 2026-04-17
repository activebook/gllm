package service

import (
	"encoding/json"
	"fmt"

	"github.com/activebook/gllm/util"
	openai "github.com/openai/openai-go/v3"
)

/*
 * OpenAI session
 */

// OpenAISession represents a session using OpenAI format
type OpenAISession struct {
	BaseSession
	Messages []openai.ChatCompletionMessageParamUnion
}

func (s *OpenAISession) GetMessages() interface{} {
	return s.Messages
}

func (s *OpenAISession) SetMessages(messages interface{}) {
	if msgs, ok := messages.([]openai.ChatCompletionMessageParamUnion); ok {
		s.Messages = msgs
	}
}

func (s *OpenAISession) MarshalMessages(messages []openai.ChatCompletionMessageParamUnion, dropToolContent bool) []byte {
	var data []byte
	for _, msg := range messages {
		rolePtr := msg.GetRole()
		if rolePtr != nil && *rolePtr == "system" {
			continue
		}

		line, err := json.Marshal(msg)
		if err != nil {
			util.LogWarnf("failed to serialize message: %v\n", err)
			continue
		}

		if dropToolContent && rolePtr != nil && *rolePtr == "tool" {
			var raw map[string]interface{}
			if err := json.Unmarshal(line, &raw); err == nil {
				raw["content"] = ""
				line, _ = json.Marshal(raw)
			}
		}

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
	var newmsgs []openai.ChatCompletionMessageParamUnion
	for _, msg := range messages {
		switch v := msg.(type) {
		case openai.ChatCompletionMessageParamUnion:
			newmsgs = append(newmsgs, v)
		case []openai.ChatCompletionMessageParamUnion:
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
		s.Messages = []openai.ChatCompletionMessageParamUnion{}
		return nil
	}

	// Parse each JSONL line as a message
	s.Messages = make([]openai.ChatCompletionMessageParamUnion, 0, len(lines))
	for i, line := range lines {
		var msg openai.ChatCompletionMessageParamUnion
		if err := json.Unmarshal(line, &msg); err != nil {
			return fmt.Errorf("failed to parse message at line %d: %w", i+1, err)
		}
		s.Messages = append(s.Messages, msg)
	}

	// Validate format
	if len(s.Messages) > 0 {
		msg := s.Messages[0]
		if rolePtr := msg.GetRole(); rolePtr == nil {
			return fmt.Errorf("invalid session format: isn't a compatible format. '%s'", s.Path)
		}
	}

	return nil
}

// Clear removes all messages from the session
func (s *OpenAISession) Clear() error {
	s.Messages = []openai.ChatCompletionMessageParamUnion{}
	return s.BaseSession.Clear()
}
