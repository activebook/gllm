package service

import (
	"fmt"
	"strings"

	"github.com/activebook/gllm/data"
)

// GetMemoryPath returns the path to the memory file.
func GetMemoryPath() string {
	store := data.NewMemoryStore()
	return store.GetPath()
}

// LoadMemory reads the memory file and returns a slice of memory items.
func LoadMemory() ([]string, error) {
	store := data.NewMemoryStore()
	return store.Load()
}

// SaveMemory writes the memory items to the memory file.
func SaveMemory(memories []string) error {
	store := data.NewMemoryStore()
	return store.Save(memories)
}

// AddMemory appends a single memory item to the memory file.
func AddMemory(memory string) error {
	store := data.NewMemoryStore()
	return store.Add(memory)
}

// RemoveMemory removes a specific memory item from the memory file.
func RemoveMemory(memory string) error {
	store := data.NewMemoryStore()
	return store.Remove(memory)
}

// ClearMemory removes all memories.
func ClearMemory() error {
	store := data.NewMemoryStore()
	return store.Clear()
}

// GetMemoryContent returns the memory content formatted for system prompt injection.
func GetMemoryContent() string {
	store := data.NewMemoryStore()
	return store.GetFormatted()
}

// GetMemoryList returns a formatted string of all memories for display.
func GetMemoryList() string {
	memories, err := LoadMemory()
	if err != nil {
		return fmt.Sprintf("Error loading memories: %v", err)
	}

	if len(memories) == 0 {
		return "No memories saved."
	}

	var content strings.Builder
	for i, memory := range memories {
		content.WriteString(fmt.Sprintf("%d. %s\n", i+1, memory))
	}

	return content.String()
}

// ReplaceAllMemories replaces all memories with the provided content.
func ReplaceAllMemories(content string) error {
	lines := strings.Split(content, "\n")
	var memories []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- ") {
			memory := strings.TrimPrefix(line, "- ")
			if memory != "" {
				memories = append(memories, memory)
			}
		} else if line != "" && !strings.HasPrefix(line, "#") {
			memories = append(memories, line)
		}
	}

	return SaveMemory(memories)
}
