package service

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	"google.golang.org/genai"
)

/*
 * Google Gemini2.0 Conversation
 */
// GeminiConversation manages conversations for Google's Gemini model
type Gemini2Conversation struct {
	BaseConversation
	History []*genai.Content
}

var gemini2Instance *Gemini2Conversation
var gemini2Once sync.Once

// NewGeminiConversation creates or returns the singleton instance
func NewGemini2Conversation(title string, shouldLoad bool) *Gemini2Conversation {
	gemini2Once.Do(func() {
		gemini2Instance = &Gemini2Conversation{
			BaseConversation: BaseConversation{
				Name:       GetDefaultConvoName(),
				ShouldLoad: shouldLoad,
			},
			History: []*genai.Content{},
		}
		if shouldLoad {
			if title == "" {
				title = GetDefaultConvoName()
			} else {
				// check if it's an index
				index, err := strconv.Atoi(title)
				if err == nil {
					// It's an index, resolve to conversation name using your sorted list logic
					convos, err := ListSortedConvos(GetConvoDir())
					if err != nil {
						// handle error
						Warnf("Failed to resolve conversation index: %v", err)
						Warnf("Using default conversation")
						title = GetDefaultConvoName()
					}
					if index < 1 || index > len(convos) {
						// handle out of range
						Warnf("Conversation index out of range: %d", index)
						Warnf("Using default conversation")
						title = GetDefaultConvoName()
					} else {
						title = convos[index-1].Name
					}
				}
			}
			gemini2Instance.Name = title
			sanitized := GetSanitizeTitle(gemini2Instance.Name)
			gemini2Instance.SetPath(sanitized)
		}
	})
	return gemini2Instance
}

// GetGeminiConversation returns the singleton instance
func GetGemini2Conversation() *Gemini2Conversation {
	if gemini2Instance == nil {
		return NewGemini2Conversation("", false)
	}
	return gemini2Instance
}

// PushContent adds a content item to the history
func (g *Gemini2Conversation) PushContent(content *genai.Content) {
	g.History = append(g.History, content)
}

// PushContents adds multiple content items to the history
func (g *Gemini2Conversation) PushContents(contents []*genai.Content) {
	g.History = append(g.History, contents...)
}

// Save persists the Gemini conversation to disk
func (g *Gemini2Conversation) Save() error {
	if !g.ShouldLoad || len(g.History) == 0 {
		return nil
	}

	// Remove any model messages with nil Parts before saving
	filtered := make([]*genai.Content, 0, len(g.History))
	for _, content := range g.History {
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
	data, err := g.readFile()
	if err != nil || data == nil {
		return err
	}

	err = json.Unmarshal(data, &g.History)
	if err != nil {
		return fmt.Errorf("failed to deserialize conversation: %w", err)
	}

	if len(g.History) > 0 {
		msg := g.History[0]
		if msg.Parts == nil {
			return fmt.Errorf("invalid conversation format: isn't a compatible format. '%s'", g.Path)
		}
	}

	return nil
}
