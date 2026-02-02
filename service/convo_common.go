package service

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/activebook/gllm/data"
	//"github.com/google/generative-ai-go/genai"
)

// ConversationManager is an interface for handling conversation history
type ConversationManager interface {
	SetPath(title string)
	GetPath() string
	Load() error
	Save() error
	Open(title string) error
	Clear() error
	Push(messages ...interface{})
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
	c.Path = GetFilePath(dir, title+".json")
}

func (c *BaseConversation) GetPath() string {
	return c.Path
}

// readFile reads the file content and validates JSON format
func (c *BaseConversation) readFile() ([]byte, error) {
	if c.Name == "" {
		return nil, nil
	}

	data, err := os.ReadFile(c.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read conversation file '%s': %w", c.Path, err)
	}

	if len(data) > 0 && !json.Valid(data) {
		return nil, fmt.Errorf("invalid JSON format in conversation file '%s'", c.Path)
	}

	return data, nil
}

// writeFile writes the file content
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
	// check if it's an index
	title, err := FindConvosByIndex(title)
	if err != nil {
		return err
	}
	// If title is still empty, no convo found
	if title == "" {
		return nil
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
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
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
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
			title := strings.TrimSuffix(file.Name(), ".json")
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
				data, err := os.ReadFile(fullPath)
				if err == nil {
					provider = DetectMessageProvider(data)
				}
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
