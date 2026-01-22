package data

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// MemoryFileName is the name of the memory file
	MemoryFileName = "context.md"
	// MemoryHeader is the header for the memory file
	MemoryHeader = "## gllm Added Memories"
)

// MemoryStore provides typed access to the memory/context file.
type MemoryStore struct {
	path string
}

// NewMemoryStore creates a new MemoryStore with the default path.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		path: GetMemoryFilePath(),
	}
}

// GetPath returns the path to the memory file.
func (m *MemoryStore) GetPath() string {
	return m.path
}

// Load reads and returns all memory items from the file.
// Returns empty slice if file doesn't exist.
func (m *MemoryStore) Load() ([]string, error) {
	if _, err := os.Stat(m.path); os.IsNotExist(err) {
		return []string{}, nil
	}

	file, err := os.Open(m.path)
	if err != nil {
		return nil, fmt.Errorf("failed to open memory file: %w", err)
	}
	defer file.Close()

	var memories []string
	scanner := bufio.NewScanner(file)
	inMemorySection := false

	for scanner.Scan() {
		line := scanner.Text()

		if strings.TrimSpace(line) == MemoryHeader {
			inMemorySection = true
			continue
		}

		if inMemorySection && strings.HasPrefix(strings.TrimSpace(line), "- ") {
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

// Save writes memory items to the file.
func (m *MemoryStore) Save(memories []string) error {
	if err := os.MkdirAll(filepath.Dir(m.path), 0755); err != nil {
		return fmt.Errorf("failed to create memory directory: %w", err)
	}

	var content strings.Builder
	content.WriteString(MemoryHeader)
	content.WriteString("\n\n")

	for _, memory := range memories {
		content.WriteString("- ")
		content.WriteString(memory)
		content.WriteString("\n")
	}

	if err := os.WriteFile(m.path, []byte(content.String()), 0644); err != nil {
		return fmt.Errorf("failed to write memory file: %w", err)
	}

	return nil
}

// Add appends a memory item. Returns error if duplicate.
func (m *MemoryStore) Add(memory string) error {
	if memory == "" {
		return fmt.Errorf("memory content cannot be empty")
	}

	memories, err := m.Load()
	if err != nil {
		return err
	}

	// Check for duplicate
	for _, existing := range memories {
		if existing == memory {
			return nil // Already exists
		}
	}

	memories = append(memories, memory)
	return m.Save(memories)
}

// Remove removes a specific memory item.
func (m *MemoryStore) Remove(memory string) error {
	if memory == "" {
		return fmt.Errorf("memory content cannot be empty")
	}

	memories, err := m.Load()
	if err != nil {
		return err
	}

	found := false
	var filtered []string
	for _, existing := range memories {
		if existing == memory {
			found = true
			continue
		}
		filtered = append(filtered, existing)
	}

	if !found {
		return fmt.Errorf("memory not found: %s", memory)
	}

	return m.Save(filtered)
}

// Clear removes all memories.
func (m *MemoryStore) Clear() error {
	if _, err := os.Stat(m.path); os.IsNotExist(err) {
		return nil
	}

	content := MemoryHeader + "\n\n"
	return os.WriteFile(m.path, []byte(content), 0644)
}

// GetFormattedXML returns memory content formatted for system prompt injection in XML format.
// This is useful for injecting memory into a system prompt.
// Bugfix: Using xml to replace markdown
// XML supports nested structures naturally, making it more suitable for system prompts and memory injection.
// Markdown: General content, documentation, when token efficiency matters more than parsing precision
func (m *MemoryStore) GetFormatted() string {
	memories, err := m.Load()
	if err != nil || len(memories) == 0 {
		return ""
	}

	var content strings.Builder
	content.WriteString("<user_memory>\n")
	content.WriteString("<description>Important facts about the user</description>\n")
	content.WriteString("<memories>\n")

	for _, memory := range memories {
		content.WriteString("  <memory>")
		content.WriteString(memory)
		content.WriteString("</memory>\n")
	}

	content.WriteString("</memories>\n")
	content.WriteString("</user_memory>")

	return content.String()
}
