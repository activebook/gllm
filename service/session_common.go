package service

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/util"
)

// SessionManager is an interface for handling session history
type SessionManager interface {
	SetPath(title string)
	GetPath() string
	Load() error
	Save() error
	Open(title string) error
	Clear() error
	Push(messages ...interface{}) error
	GetMessages() interface{}
	SetMessages(messages interface{})
}

// BaseSession holds common fields and methods for all session types
type BaseSession struct {
	Name string
	Path string
}

// SetPath sets the file path for saving the session
func (s *BaseSession) SetPath(title string) {
	if title == "" {
		s.Path = ""
		return
	}
	dir := GetSessionsDir()
	s.Path = util.GetFilePath(dir, title+".jsonl")
}

func (s *BaseSession) GetPath() string {
	return s.Path
}

// readFile reads the JSONL file and returns each line as a separate byte slice.
// Each line in a JSONL file is a complete JSON object representing one message.
func (s *BaseSession) readFile() ([][]byte, error) {
	if s.Name == "" {
		return nil, nil
	}

	file, err := os.Open(s.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to open session file '%s': %w", s.Path, err)
	}
	defer file.Close()

	var lines [][]byte
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 64*1024)   // Start with 64KB
	scanner.Buffer(buf, 1024*1024) // Can grow up to 1MB

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue // Skip empty lines
		}
		if !json.Valid(line) {
			return nil, fmt.Errorf("invalid JSON in session file '%s'", s.Path)
		}
		// Make a copy since scanner reuses the buffer
		lineCopy := make([]byte, len(line))
		copy(lineCopy, line)
		lines = append(lines, lineCopy)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading session file '%s': %w", s.Path, err)
	}

	return lines, nil
}

// appendFile appends data to the JSONL file.
// This is the primary write method for efficient incremental saves.
func (s *BaseSession) appendFile(data []byte) error {
	if s.Name == "" {
		return nil
	}
	file, err := os.OpenFile(s.Path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file for append: %w", err)
	}
	defer file.Close()

	// Write the JSON line followed by newline
	if _, err := file.Write(data); err != nil {
		return err
	}
	return err
}

// writeFile rewrites the entire file content (used for full saves when needed).
func (s *BaseSession) writeFile(data []byte) error {
	if s.Name == "" {
		return nil
	}
	return os.WriteFile(s.Path, data, 0644)
}

func (s *BaseSession) Push(messages ...interface{}) {
}

func (s *BaseSession) GetMessages() interface{} {
	return nil
}

func (s *BaseSession) SetMessages(messages interface{}) {
}

// Open initializes an OpenChatsession with the provided title, resolving
// an index to the actual session name if necessary. It resets the messages,
// sanitizes the session name for the path, and sets the internal path accordingly.
// Returns an error if the title cannot be resolved.
func (s *BaseSession) Open(title string) error {
	// If title is still empty, no session found
	if title == "" {
		return nil
	}
	// check if it's an index
	title, err := FindSessionByIndex(title)
	if err != nil {
		return err
	}
	// Set the name and path
	s.Name = title
	sanitized := util.GetSanitizeTitle(s.Name)
	s.SetPath(sanitized)
	return nil
}

func (s *BaseSession) Save() error {
	return nil
}

func (s *BaseSession) Load() error {
	return nil
}

func (s *BaseSession) Clear() error {
	if s.Name == "" {
		return nil
	}
	// Delete file instead of clearing content
	err := os.Remove(s.Path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to clear file: %w", err)
	}
	return nil
}

/*
 * Get the sorted list of sessions in the given directory
 * sorted by modTime descending
 */

type SessionMeta struct {
	Name     string
	Provider string
	ModTime  int64
	Empty    bool
}

func GetSessionsDir() string {
	dir := data.GetSessionsDirPath()
	os.MkdirAll(dir, 0750)
	return dir
}

// ClearEmptySessionsAsync clears all empty sessions in background
func ClearEmptySessionsAsync() {
	go func() {
		files, err := os.ReadDir(GetSessionsDir())
		if err != nil {
			return
		}
		for _, file := range files {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".jsonl") {
				fullPath := util.GetFilePath(GetSessionsDir(), file.Name())
				info, err := file.Info()
				if err != nil {
					continue
				}
				if info.Size() == 0 {
					os.Remove(fullPath)
				}
			}
		}
	}()
}

// FindSessionByIndex finds a session by index
// If the index is out of range, it returns an error
// If the index is valid, it returns the session name
func FindSessionByIndex(idx string) (string, error) {
	if strings.TrimSpace(idx) == "" {
		return "", nil
	}
	// check if it's an index
	index, err := strconv.Atoi(idx)
	if err == nil {
		// It's an index, resolve to session name using your sorted list logic
		sessions, err := ListSortedSessions(GetSessionsDir(), false, false)
		if err != nil {
			return "", err
		}
		if index < 1 || index > len(sessions) {
			// handle out of range
			return "", fmt.Errorf("session index out of range: %d", index)
		} else {
			title := sessions[index-1].Name
			return title, nil
		}
	} else {
		// idx is not a index
		return idx, nil
	}
}

// ListSortedSessions returns a slice of sessionMeta sorted by modTime descending
// ListSortedSessions(dir, false, false)  // Fast - no file reads
// ListSortedSessions(dir, true, false)   // Fast - only metadata
// ListSortedSessions(dir, false, true)   // Slow - reads all files for provider
func ListSortedSessions(sessionDir string, onlyNonEmpty bool, detectProvider bool) ([]SessionMeta, error) {
	files, err := os.ReadDir(sessionDir)
	if err != nil {
		return nil, fmt.Errorf("fail to read session directory: %v", err)
	}

	var sessions []SessionMeta
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".jsonl") {
			title := strings.TrimSuffix(file.Name(), ".jsonl")
			fullPath := util.GetFilePath(sessionDir, file.Name())

			// Use file.Info() instead of os.Stat()
			info, err := file.Info()
			if err != nil {
				continue
			}

			if onlyNonEmpty && info.Size() == 0 {
				continue
			}
			var provider string
			if detectProvider {
				provider = DetectMessageProvider(fullPath)
			}
			sessions = append(sessions, SessionMeta{
				Name:     title,
				Provider: provider,
				ModTime:  info.ModTime().Unix(),
				Empty:    info.Size() == 0,
			})
		}
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].ModTime > sessions[j].ModTime
	})
	return sessions, nil
}
