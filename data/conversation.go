package data

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ConversationStore provides file operations for conversation history files.
type ConversationStore struct {
	dir string
}

// NewConversationStore creates a new ConversationStore with the default directory.
func NewConversationStore() *ConversationStore {
	return &ConversationStore{
		dir: GetConvoDirPath(),
	}
}

// GetDir returns the conversation directory path.
func (c *ConversationStore) GetDir() string {
	return c.dir
}

// EnsureDir creates the conversation directory if it doesn't exist.
func (c *ConversationStore) EnsureDir() error {
	return os.MkdirAll(c.dir, 0755)
}

// List returns all conversation names (without extension), sorted by modification time (newest first).
func (c *ConversationStore) List() ([]string, error) {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read conversation directory: %w", err)
	}

	type fileInfo struct {
		name    string
		modTime int64
	}

	var files []fileInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		files = append(files, fileInfo{
			name:    strings.TrimSuffix(name, ".json"),
			modTime: info.ModTime().Unix(),
		})
	}

	// Sort by modification time (newest first)
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime > files[j].modTime
	})

	result := make([]string, len(files))
	for i, f := range files {
		result[i] = f.name
	}

	return result, nil
}

// Load reads a conversation file by name and returns raw bytes.
// The service layer is responsible for parsing the JSON based on provider type.
func (c *ConversationStore) Load(name string) ([]byte, error) {
	path := c.getPath(name)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("conversation '%s' not found", name)
		}
		return nil, fmt.Errorf("failed to read conversation: %w", err)
	}
	return data, nil
}

// Save writes a conversation file.
func (c *ConversationStore) Save(name string, data []byte) error {
	if err := c.EnsureDir(); err != nil {
		return err
	}

	path := c.getPath(name)
	return os.WriteFile(path, data, 0644)
}

// Delete removes a conversation file.
func (c *ConversationStore) Delete(name string) error {
	path := c.getPath(name)
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete conversation: %w", err)
	}
	return nil
}

// DeleteAll removes all conversation files.
func (c *ConversationStore) DeleteAll() error {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".json") {
			path := filepath.Join(c.dir, entry.Name())
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("failed to delete %s: %w", entry.Name(), err)
			}
		}
	}

	return nil
}

// Exists checks if a conversation exists.
func (c *ConversationStore) Exists(name string) bool {
	path := c.getPath(name)
	_, err := os.Stat(path)
	return err == nil
}

// GetPath returns the full path for a conversation name.
func (c *ConversationStore) GetPath(name string) string {
	return c.getPath(name)
}

func (c *ConversationStore) getPath(name string) string {
	// Sanitize the name
	safeName := sanitizeFileName(name)
	if !strings.HasSuffix(safeName, ".json") {
		safeName = safeName + ".json"
	}
	return filepath.Join(c.dir, safeName)
}

// sanitizeFileName removes or replaces characters that are not safe for file names.
func sanitizeFileName(name string) string {
	// Replace problematic characters with underscores
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)
	return replacer.Replace(name)
}
