package service

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/activebook/gllm/util"
)

// Session is an interface for handling session history
type Session interface {
	SetPath(title string)
	GetPath() string
	GetName() string
	GetTopSessionName() string
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

	// Format is expected to be "SessionName" or "SessionName::TaskKey"
	parts := strings.SplitN(title, "::", 2)
	sessionID := util.GetSanitizeTitle(parts[0])

	// Default session name is main
	sessionName := MainSessionName
	if len(parts) == 2 && parts[1] != "" {
		sessionName = util.GetSanitizeTitle(parts[1])
	}

	s.Path = filepath.Join(dir, sessionID, sessionName+SessionFileExtension)
}

func (s *BaseSession) GetPath() string {
	return s.Path
}

func (s *BaseSession) GetName() string {
	return s.Name
}

// GetTopSessionName returns the top session name, which is the first part of the session name.
// The session name is in the format of "SessionName::TaskKey"
// For example, if the session name is "Main::Task1", the top session name is "Main"
// if the session name is "Main", the top session name is "Main"
func (s *BaseSession) GetTopSessionName() string {
	parts := strings.SplitN(s.Name, "::", 2)
	return parts[0]
}

// readFile reads the JSONL file and returns each line as a separate byte slice.
// Each line in a JSONL file is a complete JSON object representing one message.
//
// Uses bufio.Reader.ReadBytes instead of bufio.Scanner so that lines of
// arbitrary length (e.g. base64-encoded images) are handled without hitting
// the scanner's fixed token-size ceiling.
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

	// 64 KB I/O read buffer — controls disk read granularity, NOT max line size.
	reader := bufio.NewReaderSize(file, 64*1024)
	var lines [][]byte

	for {
		// ReadBytes reads until '\n' regardless of how large the line is.
		// It always returns a fresh allocation, so no copy is needed.
		line, err := reader.ReadBytes('\n')
		line = bytes.TrimSpace(line)

		if len(line) > 0 {
			if !json.Valid(line) {
				return nil, fmt.Errorf("invalid JSON in session file '%s'", s.Path)
			}
			lines = append(lines, line)
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading session file '%s': %w", s.Path, err)
		}
	}

	return lines, nil
}

// appendFile appends data to the JSONL file.
// This is the primary write method for efficient incremental saves.
func (s *BaseSession) appendFile(data []byte) error {
	if s.Name == "" {
		return nil
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(s.Path), 0750); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
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

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(s.Path), 0750); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
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

// Open initializes a session with the provided title, resolving
// an index to the actual session name if necessary. It sanitizes the
// session name for the path, and sets the internal path accordingly.
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
	// The name remains the original display name (or "name:task_key")
	s.Name = title
	// SetPath will handle splitting and sanitizing the components
	s.SetPath(title)
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
	// Delete the entire session folder
	// s.Path is sessions/<session_id>/<handler>.jsonl, so filepath.Dir(s.Path) is the folder
	sessionDir := filepath.Dir(s.Path)
	err := os.RemoveAll(sessionDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to clear session: %w", err)
	}
	return nil
}
