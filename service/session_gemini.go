package service

import (
	"encoding/json"
	"fmt"

	"google.golang.org/genai"
)

/*
 * Google Gemini session
 */
// GeminiSession manages sessions for Google's Gemini model
type GeminiSession struct {
	BaseSession
	Messages []*genai.Content
}

// consolidateTextParts merges consecutive text parts to reduce fragmentation
// from streaming responses while preserving non-text parts (function calls, etc.)
func (s *GeminiSession) consolidateTextParts(parts []*genai.Part) []*genai.Part {
	if len(parts) == 0 {
		return parts
	}

	consolidated := make([]*genai.Part, 0, len(parts))
	var textBuffer string

	for _, part := range parts {
		if part.Text != "" {
			// Accumulate consecutive text parts
			textBuffer += part.Text
		} else {
			// Non-text part encountered: flush accumulated text, then add the part
			if textBuffer != "" {
				consolidated = append(consolidated, &genai.Part{Text: textBuffer})
				textBuffer = ""
			}
			consolidated = append(consolidated, part)
		}
	}

	// Flush any remaining text
	if textBuffer != "" {
		consolidated = append(consolidated, &genai.Part{Text: textBuffer})
	}

	return consolidated
}

func (s *GeminiSession) GetMessages() interface{} {
	return s.Messages
}

func (s *GeminiSession) SetMessages(messages interface{}) {
	if msgs, ok := messages.([]*genai.Content); ok {
		s.Messages = msgs
	}
}

func (s *GeminiSession) MarshalMessages(messages []*genai.Content, dropToolContent bool) []byte {
	// The industry's current answer is basically "save everything by default, then compress/prune when it gets too bis."
	// The complete session history, including your prompts and the model's responses, all tool executions (inputs and outputs).
	var data []byte
	for _, content := range messages {
		// First, consolidate consecutive text parts from streaming
		consolidatedParts := s.consolidateTextParts(content.Parts)

		// Check if this message has any function responses that need clearing
		hasFunctionResponse := false
		for _, part := range consolidatedParts {
			if part.FunctionResponse != nil {
				hasFunctionResponse = true
				break
			}
		}

		// Create formatted message
		var formatted *genai.Content
		if hasFunctionResponse && dropToolContent {
			// Deep copy with empty function responses
			contentCopy := &genai.Content{
				Role:  content.Role,
				Parts: make([]*genai.Part, len(consolidatedParts)),
			}
			for j, part := range consolidatedParts {
				if part.FunctionResponse != nil {
					contentCopy.Parts[j] = &genai.Part{
						FunctionResponse: &genai.FunctionResponse{
							Name:     part.FunctionResponse.Name,
							Response: map[string]any{}, // Empty response to save tokens
						},
					}
				} else {
					contentCopy.Parts[j] = part
				}
			}
			formatted = contentCopy
		} else {
			formatted = &genai.Content{
				Role:  content.Role,
				Parts: consolidatedParts,
			}
		}

		// Marshal to JSON (compact, no indent for JSONL)
		line, err := json.Marshal(formatted)
		if err != nil {
			Warnf("failed to serialize message: %v", err)
			continue
		}

		// Write all lines as JSONL (one message per line)
		data = append(data, line...)
		data = append(data, '\n')
	}
	return data
}

// PushMessages adds multiple content items to the history (high performance)
// Uses append-mode for incremental saves using JSONL format (one message per line)
func (s *GeminiSession) Push(messages ...interface{}) error {
	if len(messages) == 0 {
		return nil
	}

	newmsgs := []*genai.Content{}
	for _, msg := range messages {
		switch v := msg.(type) {
		case *genai.Content:
			newmsgs = append(newmsgs, v)
		case []*genai.Content:
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

// Save persists the Gemini session to disk using JSONL format (one message per line).
func (s *GeminiSession) Save() error {
	if s.Name == "" || len(s.Messages) == 0 {
		return nil
	}

	// Write all messages to file
	data := s.MarshalMessages(s.Messages, false)
	return s.writeFile(data)
}

// Load retrieves the Gemini session from disk (JSONL format).
func (s *GeminiSession) Load() error {
	if s.Name == "" {
		return nil
	}

	lines, err := s.readFile()
	if err != nil {
		return err
	}

	// Handle empty or non-existent files
	if len(lines) == 0 {
		s.Messages = []*genai.Content{}
		return nil
	}

	// Parse each JSONL line as a message
	s.Messages = make([]*genai.Content, 0, len(lines))
	for i, line := range lines {
		var msg genai.Content
		if err := json.Unmarshal(line, &msg); err != nil {
			return fmt.Errorf("failed to parse message at line %d: %w", i+1, err)
		}
		s.Messages = append(s.Messages, &msg)
	}

	// Validate format
	if len(s.Messages) > 0 {
		msg := s.Messages[0]
		if len(msg.Parts) <= 0 || msg.Role == "" {
			return fmt.Errorf("invalid session format: isn't a compatible format. '%s'", s.Path)
		}
	}

	return nil
}

// Clear removes all messages from the session
func (s *GeminiSession) Clear() error {
	s.Messages = []*genai.Content{}
	return s.BaseSession.Clear()
}
