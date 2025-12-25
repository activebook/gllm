package service

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mitchellh/go-homedir"
)

const (
	// MemoryFileName is the name of the memory file
	MemoryFileName = "context.md"
	// MemoryHeader is the header for the memory file
	MemoryHeader = "## gllm Added Memories"
)

// GetMemoryPath returns the path to the memory file
// Uses os.UserConfigDir() for cross-platform support
func GetMemoryPath() string {
	// Prefer os.UserConfigDir()
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		// Fallback to home directory if UserConfigDir fails
		userConfigDir, _ = homedir.Dir()
	}

	// App specific directory: e.g., ~/Library/Application Support/gllm on macOS
	appConfigDir := filepath.Join(userConfigDir, "gllm")

	return filepath.Join(appConfigDir, MemoryFileName)
}

// LoadMemory reads the memory file and returns a slice of memory items
// Returns empty slice if file doesn't exist or is empty
func LoadMemory() ([]string, error) {
	memoryPath := GetMemoryPath()

	// Check if file exists
	if _, err := os.Stat(memoryPath); os.IsNotExist(err) {
		return []string{}, nil
	}

	// Read file
	file, err := os.Open(memoryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open memory file: %w", err)
	}
	defer file.Close()

	var memories []string
	scanner := bufio.NewScanner(file)
	inMemorySection := false

	for scanner.Scan() {
		line := scanner.Text()

		// Check for header
		if strings.TrimSpace(line) == MemoryHeader {
			inMemorySection = true
			continue
		}

		// Parse memory items (lines starting with "- ")
		if inMemorySection && strings.HasPrefix(strings.TrimSpace(line), "- ") {
			// Extract the memory content (remove "- " prefix)
			memory := strings.TrimPrefix(strings.TrimSpace(line), "- ")
			if memory != "" {
				memories = append(memories, memory)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading memory file: %w", err)
	}

	return memories, nil
}

// SaveMemory writes the memory items to the memory file
func SaveMemory(memories []string) error {
	memoryPath := GetMemoryPath()

	// Ensure the directory exists
	dir := filepath.Dir(memoryPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create memory directory: %w", err)
	}

	// Build content
	var content strings.Builder
	content.WriteString(MemoryHeader)
	content.WriteString("\n\n")

	for _, memory := range memories {
		content.WriteString("- ")
		content.WriteString(memory)
		content.WriteString("\n")
	}

	// Write to file
	if err := os.WriteFile(memoryPath, []byte(content.String()), 0644); err != nil {
		return fmt.Errorf("failed to write memory file: %w", err)
	}

	return nil
}

// AddMemory appends a single memory item to the memory file
func AddMemory(memory string) error {
	if memory == "" {
		return fmt.Errorf("memory content cannot be empty")
	}

	memories, err := LoadMemory()
	if err != nil {
		return err
	}

	// Check for duplicate
	for _, m := range memories {
		if m == memory {
			return nil // Already exists, no need to add
		}
	}

	memories = append(memories, memory)
	return SaveMemory(memories)
}

// RemoveMemory removes a specific memory item from the memory file
func RemoveMemory(memory string) error {
	if memory == "" {
		return fmt.Errorf("memory content cannot be empty")
	}

	memories, err := LoadMemory()
	if err != nil {
		return err
	}

	// Find and remove the memory
	found := false
	var newMemories []string
	for _, m := range memories {
		if m == memory {
			found = true
			continue
		}
		newMemories = append(newMemories, m)
	}

	if !found {
		return fmt.Errorf("memory not found: %s", memory)
	}

	return SaveMemory(newMemories)
}

// ClearMemory removes all memories
func ClearMemory() error {
	memoryPath := GetMemoryPath()

	// Check if file exists
	if _, err := os.Stat(memoryPath); os.IsNotExist(err) {
		return nil // Nothing to clear
	}

	// Write empty file with header only
	content := MemoryHeader + "\n\n"
	if err := os.WriteFile(memoryPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to clear memory file: %w", err)
	}

	return nil
}

// GetMemoryContent returns the memory content formatted for system prompt injection
// Returns empty string if no memories exist
func GetMemoryContent() string {
	memories, err := LoadMemory()
	if err != nil || len(memories) == 0 {
		return ""
	}

	var content strings.Builder
	content.WriteString("## User Memories\n\n")
	content.WriteString("The following are important facts about the user that you should remember:\n\n")

	for _, memory := range memories {
		content.WriteString("- ")
		content.WriteString(memory)
		content.WriteString("\n")
	}

	return content.String()
}

// GetMemoryList returns a formatted string of all memories for display
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

// ReplaceAllMemories replaces all memories with the provided content
func ReplaceAllMemories(content string) error {
	// Parse the content - split by newlines and filter for bullet points
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
			// Allow plain text as well
			memories = append(memories, line)
		}
	}

	return SaveMemory(memories)
}
