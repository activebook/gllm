package data

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SessionStore provides file operations for session history files.
type SessionStore struct {
	dir string
}

// NewSessionStore creates a new sessionStore with the default directory.
func NewSessionStore() *SessionStore {
	return &SessionStore{
		dir: GetSessionDirPath(),
	}
}

// GetDir returns the session directory path.
func (ss *SessionStore) GetDir() string {
	return ss.dir
}

// EnsureDir creates the session directory if it doesn't exist.
func (ss *SessionStore) EnsureDir() error {
	return os.MkdirAll(ss.dir, 0755)
}

// List returns all session names (without extension), sorted by modification time (newest first).
func (ss *SessionStore) List() ([]string, error) {
	entries, err := os.ReadDir(ss.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read session directory: %w", err)
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
		if !strings.HasSuffix(name, ".jsonl") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		files = append(files, fileInfo{
			name:    strings.TrimSuffix(name, ".jsonl"),
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

// Load reads a session file by name and returns raw bytes.
// The service layer is responsible for parsing the JSON based on provider type.
func (ss *SessionStore) Load(name string) ([]byte, error) {
	path := ss.getPath(name)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session '%s' not found", name)
		}
		return nil, fmt.Errorf("failed to read session: %w", err)
	}
	return data, nil
}

// Save writes a session file.
func (ss *SessionStore) Save(name string, data []byte) error {
	if err := ss.EnsureDir(); err != nil {
		return err
	}

	path := ss.getPath(name)
	return os.WriteFile(path, data, 0644)
}

// Delete removes a session file.
func (ss *SessionStore) Delete(name string) error {
	path := ss.getPath(name)
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	return nil
}

// DeleteAll removes all session files.
func (ss *SessionStore) DeleteAll() error {
	entries, err := os.ReadDir(ss.dir)
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
		if strings.HasSuffix(entry.Name(), ".jsonl") {
			path := filepath.Join(ss.dir, entry.Name())
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("failed to delete %s: %w", entry.Name(), err)
			}
		}
	}

	return nil
}

// Exists checks if a session exists.
func (ss *SessionStore) Exists(name string) bool {
	path := ss.getPath(name)
	_, err := os.Stat(path)
	return err == nil
}

// GetPath returns the full path for a session name.
func (ss *SessionStore) GetPath(name string) string {
	return ss.getPath(name)
}

func (ss *SessionStore) getPath(name string) string {
	// Sanitize the name
	safeName := sanitizeFileName(name)
	if !strings.HasSuffix(safeName, ".jsonl") {
		safeName = safeName + ".jsonl"
	}
	return filepath.Join(ss.dir, safeName)
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
