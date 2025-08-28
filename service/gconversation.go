package service

import (
	"encoding/json"
	"fmt"

	"google.golang.org/genai"
)

/*
 * Google Gemini2.0 Conversation
 */
// GeminiConversation manages conversations for Google's Gemini model
type Gemini2Conversation struct {
	BaseConversation
	Messages []*genai.Content
}

// NewGeminiConversation creates or returns the singleton instance
func (g *Gemini2Conversation) Open(title string) error {
	// check if it's an index
	title, err := FindConvosByIndex(title)
	if err != nil {
		return err
	}
	// If title is still empty, no convo found
	if title == "" {
		return nil
	}
	// Set the name and path
	g.BaseConversation = BaseConversation{
		Name: title,
	}
	g.Messages = []*genai.Content{}
	sanitized := GetSanitizeTitle(g.Name)
	g.SetPath(sanitized)
	return nil
}

// PushContents adds multiple content items to the history
func (g *Gemini2Conversation) Push(messages ...interface{}) {
	for _, msg := range messages {
		switch v := msg.(type) {
		case *genai.Content:
			g.Messages = append(g.Messages, v)
		case []*genai.Content:
			g.Messages = append(g.Messages, v...)
		}
	}
}

func (g *Gemini2Conversation) GetMessages() interface{} {
	return g.Messages
}

func (g *Gemini2Conversation) SetMessages(messages interface{}) {
	if msgs, ok := messages.([]*genai.Content); ok {
		g.Messages = msgs
	}
}

// Save persists the Gemini conversation to disk
func (g *Gemini2Conversation) Save() error {
	if g.Name == "" || len(g.Messages) == 0 {
		return nil
	}

	// Remove any model messages with nil Parts before saving
	filtered := make([]*genai.Content, 0, len(g.Messages))
	for _, content := range g.Messages {
		if content.Role == genai.RoleModel && content.Parts == nil {
			continue // skip invalid model messages
		}
		filtered = append(filtered, content)
	}

	data, err := json.MarshalIndent(filtered, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize conversation: %w", err)
	}

	return g.writeFile(data)
}

// Load retrieves the Gemini conversation from disk
func (g *Gemini2Conversation) Load() error {
	if g.Name == "" {
		return nil
	}

	data, err := g.readFile()
	if err != nil || data == nil {
		return err
	}

	err = json.Unmarshal(data, &g.Messages)
	if err != nil {
		return fmt.Errorf("failed to deserialize conversation: %w", err)
	}

	if len(g.Messages) > 0 {
		msg := g.Messages[0]
		if msg.Parts == nil {
			return fmt.Errorf("invalid conversation format: isn't a compatible format. '%s'", g.Path)
		}
	}

	return nil
}

// Clear removes all messages from the conversation
func (g *Gemini2Conversation) Clear() error {
	g.Messages = []*genai.Content{}
	return g.BaseConversation.Clear()
}
