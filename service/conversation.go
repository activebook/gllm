package service

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
)

var (
	cinstance *Conversation
)

type Conversation struct {
	Messages   []*model.ChatCompletionMessage
	Name       string
	Path       string
	ShouldLoad bool
}

func NewConversation(title string, shouldLoad bool) *Conversation {
	if cinstance != nil {
		return cinstance
	}
	cinstance = &Conversation{
		Messages:   []*model.ChatCompletionMessage{},
		Name:       GetDefaultConvoName(),
		ShouldLoad: shouldLoad,
	}
	if shouldLoad {
		// Set default path
		if title == "" {
			title = GetDefaultConvoName()
		}
		cinstance.Name = title
		sanitzed := GetSanitizeTitle(cinstance.Name)
		cinstance.SetPath(sanitzed)
	}
	return cinstance
}

func GetConversion() *Conversation {
	if cinstance == nil {
		cinstance = NewConversation("", false)
	}
	return cinstance
}

func (c *Conversation) SetPath(title string) {
	dir := MakeUserSubDir("gllm", "convo")
	c.Path = GetFilePath(dir, title+".json")
}

func (c *Conversation) PushMessage(message *model.ChatCompletionMessage) {
	c.Messages = append(c.Messages, message)
}

func (c *Conversation) PushMessages(messages []*model.ChatCompletionMessage) {
	c.Messages = append(c.Messages, messages...)

}

func (c *Conversation) Save() error {
	// Save conversation to file
	if !c.ShouldLoad {
		// don't save anything
		return nil
	}
	if len(c.Messages) == 0 {
		return nil
	}
	data, err := json.MarshalIndent(c.Messages, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.Path, data, 0644)
}

func (c *Conversation) Load() error {
	if !c.ShouldLoad {
		// If convoPath is not set, don't load anything
		return nil
	}
	Infof("Loading previous conversation: %s\n", c.Name)

	data, err := os.ReadFile(c.Path)
	if err != nil {
		// Handle file not found specifically if needed (e.g., return empty history)
		if os.IsNotExist(err) {
			return nil // Return empty slice if file doesn't exist
		}
		return fmt.Errorf("failed to read conversation file '%s': %w", c.Path, err)
	}

	// First try to validate the JSON format before unmarshaling
	if !json.Valid(data) {
		return fmt.Errorf("invalid JSON format in conversation file '%s'", c.Path)
	}

	err = json.Unmarshal(data, &c.Messages)
	if err != nil {
		return fmt.Errorf("failed to deserialize conversation: %w", err)
	}
	if len(c.Messages) > 0 {
		msg := c.Messages[0]
		if msg.Content == nil {
			return fmt.Errorf("invalid conversation format: isn't a compatible format. '%s'", c.Path)
		}
	}
	return nil
}
