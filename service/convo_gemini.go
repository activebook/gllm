package service

import (
	"encoding/json"
	"fmt"

	"google.golang.org/genai"
)

/*
 * Google Gemini Conversation
 */
// GeminiConversation manages conversations for Google's Gemini model
type GeminiConversation struct {
	BaseConversation
	Messages []*genai.Content
}

// consolidateTextParts merges consecutive text parts to reduce fragmentation
// from streaming responses while preserving non-text parts (function calls, etc.)
func (g *GeminiConversation) consolidateTextParts(parts []*genai.Part) []*genai.Part {
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

func (g *GeminiConversation) GetMessages() interface{} {
	return g.Messages
}

func (g *GeminiConversation) SetMessages(messages interface{}) {
	if msgs, ok := messages.([]*genai.Content); ok {
		g.Messages = msgs
	}
}

func (g *GeminiConversation) MarshalMessages(messages []*genai.Content) []byte {
	// Build all formatted messages
	var data []byte
	for _, content := range messages {
		// First, consolidate consecutive text parts from streaming
		consolidatedParts := g.consolidateTextParts(content.Parts)

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
		if hasFunctionResponse {
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
func (g *GeminiConversation) Push(messages ...interface{}) error {
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
	g.Messages = append(g.Messages, newmsgs...)

	// Only persist to file if conversation has a name
	if g.Name == "" {
		return nil
	}
	data := g.MarshalMessages(newmsgs)
	return g.appendFile(data)
}

// Save persists the Gemini conversation to disk using JSONL format (one message per line).
func (g *GeminiConversation) Save() error {
	if g.Name == "" || len(g.Messages) == 0 {
		return nil
	}

	// Write all messages to file
	data := g.MarshalMessages(g.Messages)
	return g.writeFile(data)
}

// Load retrieves the Gemini conversation from disk (JSONL format).
func (g *GeminiConversation) Load() error {
	if g.Name == "" {
		return nil
	}

	lines, err := g.readFile()
	if err != nil {
		return err
	}

	// Handle empty or non-existent files
	if len(lines) == 0 {
		g.Messages = []*genai.Content{}
		return nil
	}

	// Parse each JSONL line as a message
	g.Messages = make([]*genai.Content, 0, len(lines))
	for i, line := range lines {
		var msg genai.Content
		if err := json.Unmarshal(line, &msg); err != nil {
			return fmt.Errorf("failed to parse message at line %d: %w", i+1, err)
		}
		g.Messages = append(g.Messages, &msg)
	}

	// Validate format
	if len(g.Messages) > 0 {
		msg := g.Messages[0]
		if len(msg.Parts) <= 0 || msg.Role == "" {
			return fmt.Errorf("invalid conversation format: isn't a compatible format. '%s'", g.Path)
		}
	}

	return nil
}

// Clear removes all messages from the conversation
func (g *GeminiConversation) Clear() error {
	g.Messages = []*genai.Content{}
	return g.BaseConversation.Clear()
}
