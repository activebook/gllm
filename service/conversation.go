package service

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
)

var (
	instance *Conversation
)

type Conversation struct {
	Messages   []*model.ChatCompletionMessage
	Name       string
	Path       string
	ShouldLoad bool
}

func GetDefaultConvoName() string {
	// Get the default conversation name from the config
	// This is a placeholder function. Replace with actual logic to get the default name.
	return "default"
}

func NewConversation(title string, shouldLoad bool) *Conversation {
	if instance != nil {
		return instance
	}
	instance = &Conversation{
		Messages:   []*model.ChatCompletionMessage{},
		Name:       GetDefaultConvoName(),
		ShouldLoad: shouldLoad,
	}
	if shouldLoad {
		// Set default path
		if title == "" {
			title = GetDefaultConvoName()
		}
		instance.Name = title
		sanitzed := GetSanitizeTitle(instance.Name)
		instance.SetPath(sanitzed)
	}
	return instance
}

func GetConversion() *Conversation {
	if instance == nil {
		instance = NewConversation("", false)
	}
	return instance
}

func (c *Conversation) SetPath(title string) {
	// Prefer os.UserConfigDir()
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		Warnf("Warning: Could not find user dir, falling back to home directory.%v\n", err)
		userConfigDir, _ = os.UserHomeDir()
	}
	convoDir := filepath.Join(userConfigDir, "gllm", "convo")
	if err := os.MkdirAll(convoDir, 0750); err != nil { // 0750 permissions: user rwx, group rx, others none
		Errorf("Error creating conversation directory '%s': %v\n", convoDir, err)
		convoDir = ""
	}
	c.Path = filepath.Join(convoDir, title+".json")
}

func (c *Conversation) PushMessage(message *model.ChatCompletionMessage) {
	c.Messages = append(c.Messages, message)
}

func (c *Conversation) PushMessages(messages []*model.ChatCompletionMessage) {
	c.Messages = append(c.Messages, messages...)

}

func (c *Conversation) Save() error {
	// Save conversation to file
	// ...
	/*
		data := []byte("")
		// Implement saving logic here
		for _, msg := range messages {
			// Serialize each message and save to the file
			json, err := msg.Content.MarshalJSON()
			if err != nil {
				return err
			}
			data = append(data, json...)
		}
	*/
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
		return err
	}
	err = json.Unmarshal(data, &c.Messages)
	if err != nil {
		return err
	}
	return nil
}
