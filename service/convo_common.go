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
)

// ConversationManager is an interface for handling conversation history
type ConversationManager interface {
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

// BaseConversation holds common fields and methods for all conversation types
type BaseConversation struct {
	Name string
	Path string
}

// SetPath sets the file path for saving the conversation
func (c *BaseConversation) SetPath(title string) {
	if title == "" {
		c.Path = ""
		return
	}
	dir := MakeUserSubDir("gllm", "convo")
	c.Path = GetFilePath(dir, title+".jsonl")
}

func (c *BaseConversation) GetPath() string {
	return c.Path
}

// readFile reads the JSONL file and returns each line as a separate byte slice.
// Each line in a JSONL file is a complete JSON object representing one message.
func (c *BaseConversation) readFile() ([][]byte, error) {
	if c.Name == "" {
		return nil, nil
	}

	file, err := os.Open(c.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to open conversation file '%s': %w", c.Path, err)
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
			return nil, fmt.Errorf("invalid JSON in conversation file '%s'", c.Path)
		}
		// Make a copy since scanner reuses the buffer
		lineCopy := make([]byte, len(line))
		copy(lineCopy, line)
		lines = append(lines, lineCopy)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading conversation file '%s': %w", c.Path, err)
	}

	return lines, nil
}

// appendFile appends data to the JSONL file.
// This is the primary write method for efficient incremental saves.
func (c *BaseConversation) appendFile(data []byte) error {
	if c.Name == "" {
		return nil
	}
	file, err := os.OpenFile(c.Path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
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
func (c *BaseConversation) writeFile(data []byte) error {
	if c.Name == "" {
		return nil
	}
	return os.WriteFile(c.Path, data, 0644)
}

func (c *BaseConversation) Push(messages ...interface{}) {
}

func (c *BaseConversation) GetMessages() interface{} {
	return nil
}

func (c *BaseConversation) SetMessages(messages interface{}) {
}

// Open initializes an OpenChatConversation with the provided title, resolving
// an index to the actual conversation name if necessary. It resets the messages,
// sanitizes the conversation name for the path, and sets the internal path accordingly.
// Returns an error if the title cannot be resolved.
func (c *BaseConversation) Open(title string) error {
	// If title is still empty, no convo found
	if title == "" {
		return nil
	}
	// check if it's an index
	title, err := FindConvosByIndex(title)
	if err != nil {
		return err
	}
	// Set the name and path
	c.Name = title
	sanitized := GetSanitizeTitle(c.Name)
	c.SetPath(sanitized)
	return nil
}

func (c *BaseConversation) Save() error {
	return nil
}

func (c *BaseConversation) Load() error {
	return nil
}

func (c *BaseConversation) Clear() error {
	if c.Name == "" {
		return nil
	}
	// Delete file instead of clearing content
	err := os.Remove(c.Path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to clear file: %w", err)
	}
	return nil
}

/*
 * Get the sorted list of conversations in the given directory
 * sorted by modTime descending
 */

type ConvoMeta struct {
	Name     string
	Provider string
	ModTime  int64
	Empty    bool
}

func GetConvoDir() string {
	dir := data.GetConvoDirPath()
	os.MkdirAll(dir, 0750)
	return dir
}

// ClearEmptyConvosAsync clears all empty conversations in background
func ClearEmptyConvosAsync() {
	go func() {
		files, err := os.ReadDir(GetConvoDir())
		if err != nil {
			return
		}
		for _, file := range files {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".jsonl") {
				fullPath := GetFilePath(GetConvoDir(), file.Name())
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

// FindConvosByIndex finds a conversation by index
// If the index is out of range, it returns an error
// If the index is valid, it returns the conversation name
func FindConvosByIndex(idx string) (string, error) {
	if strings.TrimSpace(idx) == "" {
		return "", nil
	}
	// check if it's an index
	index, err := strconv.Atoi(idx)
	if err == nil {
		// It's an index, resolve to conversation name using your sorted list logic
		convos, err := ListSortedConvos(GetConvoDir(), false, false)
		if err != nil {
			return "", err
		}
		if index < 1 || index > len(convos) {
			// handle out of range
			return "", fmt.Errorf("conversation index out of range: %d", index)
		} else {
			title := convos[index-1].Name
			return title, nil
		}
	} else {
		// idx is not a index
		return idx, nil
	}
}

// ListSortedConvos returns a slice of convoMeta sorted by modTime descending
// ListSortedConvos(dir, false, false)  // Fast - no file reads
// ListSortedConvos(dir, true, false)   // Fast - only metadata
// ListSortedConvos(dir, false, true)   // Slow - reads all files for provider
func ListSortedConvos(convoDir string, onlyNonEmpty bool, detectProvider bool) ([]ConvoMeta, error) {
	files, err := os.ReadDir(convoDir)
	if err != nil {
		return nil, fmt.Errorf("fail to read conversation directory: %v", err)
	}

	var convos []ConvoMeta
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".jsonl") {
			title := strings.TrimSuffix(file.Name(), ".jsonl")
			fullPath := GetFilePath(convoDir, file.Name())

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
			convos = append(convos, ConvoMeta{
				Name:     title,
				Provider: provider,
				ModTime:  info.ModTime().Unix(),
				Empty:    info.Size() == 0,
			})
		}
	}

	sort.Slice(convos, func(i, j int) bool {
		return convos[i].ModTime > convos[j].ModTime
	})
	return convos, nil
}
