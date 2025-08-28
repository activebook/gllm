package service

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	openai "github.com/sashabaranov/go-openai"
	//"github.com/google/generative-ai-go/genai"
	"github.com/spf13/viper"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
)

func shouldSaveSearchResults() bool {
	return viper.GetBool("search_engines.results.save")
}

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

	// Check if file exists, if not, create an empty one
	if _, err := os.Stat(c.Path); os.IsNotExist(err) {
		empty := []byte("[]")
		_ = os.WriteFile(c.Path, empty, 0644)
	}
}

func (c *BaseConversation) GetPath() string {
	return c.Path
}

// Common validation and file operations
func (c *BaseConversation) readFile() ([]byte, error) {
	if c.Name == "" {
		return nil, nil
	}

	data, err := os.ReadFile(c.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Return empty slice if file doesn't exist
		}
		return nil, fmt.Errorf("failed to read conversation file '%s': %w", c.Path, err)
	}

	// If the file is empty, return an empty slice
	if len(data) == 0 {
		return nil, nil
	}

	// Validate the JSON format
	if !json.Valid(data) {
		return nil, fmt.Errorf("invalid JSON format in conversation file '%s'", c.Path)
	}

	return data, nil
}

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

func (c *BaseConversation) Open(title string) error {
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
	// Clear the content of the file by writing an empty string to it
	empty := []byte("[]")
	err := os.WriteFile(c.Path, empty, 0644)
	if err != nil {
		return fmt.Errorf("failed to clear file: %w", err) // Return error if the file write fails
	}
	return nil
}

/*
 * OpenChat Conversation
 */

// OpenChatConversation manages conversations for Volcengine model
type OpenChatConversation struct {
	BaseConversation
	Messages []*model.ChatCompletionMessage
}

// Open initializes an OpenChatConversation with the provided title, resolving
// an index to the actual conversation name if necessary. It resets the messages,
// sanitizes the conversation name for the path, and sets the internal path accordingly.
// Returns an error if the title cannot be resolved.
func (c *OpenChatConversation) Open(title string) error {
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
	c.BaseConversation = BaseConversation{
		Name: title,
	}
	c.Messages = []*model.ChatCompletionMessage{}
	sanitized := GetSanitizeTitle(c.Name)
	c.SetPath(sanitized)
	return nil
}

// PushMessages adds multiple messages to the conversation
func (c *OpenChatConversation) Push(messages ...interface{}) {
	for _, msg := range messages {
		switch v := msg.(type) {
		case *model.ChatCompletionMessage:
			c.Messages = append(c.Messages, v)
		case []*model.ChatCompletionMessage:
			c.Messages = append(c.Messages, v...)
		}
	}
}

func (c *OpenChatConversation) GetMessages() interface{} {
	return c.Messages
}

func (c *OpenChatConversation) SetMessages(messages interface{}) {
	if msgs, ok := messages.([]*model.ChatCompletionMessage); ok {
		c.Messages = msgs
	}
}

// Save persists the conversation to disk
func (c *OpenChatConversation) Save() error {
	if c.Name == "" || len(c.Messages) == 0 {
		return nil
	}

	if !shouldSaveSearchResults() {
		// Most major systems (including ChatGPT and Google's Gemini) indeed discard search results between turns
		// Clear content for tool messages before saving
		empty := ""
		for _, msg := range c.Messages {
			if msg.Role == model.ChatMessageRoleTool {
				//msg.Content = nil // or "" if Content is a string
				msg.Content = &model.ChatCompletionMessageContent{StringValue: &empty}
			}
		}
	}

	data, err := json.MarshalIndent(c.Messages, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize conversation: %w", err)
	}

	return c.writeFile(data)
}

// Load retrieves the conversation from disk
func (c *OpenChatConversation) Load() error {
	if c.Name == "" {
		return nil
	}

	data, err := c.readFile()
	if err != nil || data == nil {
		return err
	}

	err = json.Unmarshal(data, &c.Messages)
	if err != nil {
		return fmt.Errorf("failed to deserialize conversation: %w", err)
	}

	if len(c.Messages) > 0 {
		msg := c.Messages[0]
		if msg.Content == nil {
			return fmt.Errorf("invalid conversation format: isn't a compatible format. '%s'", c.Path)
		}
	}

	return nil
}

// Clear removes all messages from the conversation
func (c *OpenChatConversation) Clear() error {
	c.Messages = []*model.ChatCompletionMessage{}
	return c.BaseConversation.Clear()
}

/*
 * OpenAI Conversation
 */

// OpenAIConversation represents a conversation using OpenAI format
type OpenAIConversation struct {
	BaseConversation
	Messages []openai.ChatCompletionMessage
}

func (c *OpenAIConversation) Open(title string) error {
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
	c.BaseConversation = BaseConversation{
		Name: title,
	}
	c.Messages = []openai.ChatCompletionMessage{}
	sanitized := GetSanitizeTitle(c.Name)
	c.SetPath(sanitized)
	return nil
}

// PushMessages adds multiple messages to the conversation
func (c *OpenAIConversation) Push(messages ...interface{}) {
	for _, msg := range messages {
		switch v := msg.(type) {
		case openai.ChatCompletionMessage:
			c.Messages = append(c.Messages, v)
		case []openai.ChatCompletionMessage:
			c.Messages = append(c.Messages, v...)
		}
	}
}

func (c *OpenAIConversation) GetMessages() interface{} {
	return c.Messages
}

func (c *OpenAIConversation) SetMessages(messages interface{}) {
	if msgs, ok := messages.([]openai.ChatCompletionMessage); ok {
		c.Messages = msgs
	}
}

// Save persists the conversation to disk
func (c *OpenAIConversation) Save() error {
	if c.Name == "" || len(c.Messages) == 0 {
		return nil
	}

	if !shouldSaveSearchResults() {
		// Most major systems (including ChatGPT and Google's Gemini) indeed discard search results between turns
		// Clear content for tool messages before saving
		empty := ""
		for i := range c.Messages {
			msg := c.Messages[i]
			if msg.Role == openai.ChatMessageRoleTool {
				msg.Content = empty
			}
		}
	}

	data, err := json.MarshalIndent(c.Messages, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize conversation: %w", err)
	}

	return c.writeFile(data)
}

// Load retrieves the conversation from disk
func (c *OpenAIConversation) Load() error {
	if c.Name == "" {
		return nil
	}

	// read file
	data, err := c.readFile()
	if err != nil || data == nil {
		return err
	}

	// Parse messages
	var messages []openai.ChatCompletionMessage
	if err := json.Unmarshal(data, &messages); err != nil {
		// If there's an error unmarshaling, it might be an old format
		return fmt.Errorf("failed to parse conversation file: %v", err)
	}
	c.Messages = messages
	return nil
}

// Clear removes all messages from the conversation
func (c *OpenAIConversation) Clear() error {
	c.Messages = []openai.ChatCompletionMessage{}
	return c.BaseConversation.Clear()
}

/*
 * Google Gemini Conversation
 */
// GeminiConversation manages conversations for Google's Gemini model
// type GeminiConversation struct {
// 	BaseConversation
// 	History []*genai.Content
// }

// var geminiInstance *GeminiConversation
// var geminiOnce sync.Once

// // NewGeminiConversation creates or returns the singleton instance
// func NewGeminiConversation(title string, shouldLoad bool) *GeminiConversation {
// 	geminiOnce.Do(func() {
// 		geminiInstance = &GeminiConversation{
// 			BaseConversation: BaseConversation{
// 				Name:       GetDefaultConvoName(),
// 				ShouldLoad: shouldLoad,
// 			},
// 			History: []*genai.Content{},
// 		}
// 		if shouldLoad {
// 			if title == "" {
// 				title = GetDefaultConvoName()
// 			} else {
// 				// check if it's an index
// 				index, err := strconv.Atoi(title)
// 				if err == nil {
// 					// It's an index, resolve to conversation name using your sorted list logic
// 					convos, err := ListSortedConvos(GetConvoDir())
// 					if err != nil {
// 						// handle error
// 						Warnf("Failed to resolve conversation index: %v", err)
// 						Warnf("Using default conversation")
// 						title = GetDefaultConvoName()
// 					}
// 					if index < 1 || index > len(convos) {
// 						// handle out of range
// 						Warnf("Conversation index out of range: %d", index)
// 						Warnf("Using default conversation")
// 						title = GetDefaultConvoName()
// 					} else {
// 						title = convos[index-1].Name
// 					}
// 				}
// 			}
// 			geminiInstance.Name = title
// 			sanitized := GetSanitizeTitle(geminiInstance.Name)
// 			geminiInstance.SetPath(sanitized)
// 		}
// 	})
// 	return geminiInstance
// }

// // GetGeminiConversation returns the singleton instance
// func GetGeminiConversation() *GeminiConversation {
// 	if geminiInstance == nil {
// 		return NewGeminiConversation("", false)
// 	}
// 	return geminiInstance
// }

// // PushContent adds a content item to the history
// func (g *GeminiConversation) PushContent(content *genai.Content) {
// 	g.History = append(g.History, content)
// }

// // PushContents adds multiple content items to the history
// func (g *GeminiConversation) PushContents(contents []*genai.Content) {
// 	g.History = append(g.History, contents...)
// }

// // Custom types for marshaling/unmarshaling Gemini content
// type SerializablePart struct {
// 	Type     string                 `json:"type"`
// 	Text     string                 `json:"text,omitempty"`
// 	MIMEType string                 `json:"mime_type,omitempty"` // For blobs
// 	Data     string                 `json:"data,omitempty"`      // Base64 encoded for blobs
// 	Name     string                 `json:"name,omitempty"`      // For function calls
// 	Args     map[string]interface{} `json:"args,omitempty"`      // For function calls
// 	Language int32                  `json:"language,omitempty"`  // For executable code
// 	Code     string                 `json:"code,omitempty"`      // For executable code
// 	Outcome  int32                  `json:"outcome,omitempty"`   // For code execution result
// 	Output   string                 `json:"output,omitempty"`
// }

// type SerializableContent struct {
// 	Role  string             `json:"role"`
// 	Parts []SerializablePart `json:"parts"`
// }

// // serializeHistory converts Gemini content to JSON-serializable format
// func (g *GeminiConversation) serializeHistory(history []*genai.Content) ([]byte, error) {
// 	var serializableHistory []SerializableContent
// 	for _, content := range history {
// 		sc := SerializableContent{
// 			Role: content.Role,
// 		}

// 		for _, part := range content.Parts {
// 			var sp SerializablePart

// 			switch v := part.(type) {
// 			case genai.Text:
// 				sp.Type = "text"
// 				sp.Text = string(v)

// 			case genai.Blob:
// 				sp.Type = "blob"
// 				sp.MIMEType = v.MIMEType
// 				sp.Data = base64.StdEncoding.EncodeToString(v.Data)

// 			case *genai.Blob:
// 				sp.Type = "blob"
// 				sp.MIMEType = v.MIMEType
// 				sp.Data = base64.StdEncoding.EncodeToString(v.Data)

// 			case genai.FunctionCall:
// 				sp.Type = "function_call"
// 				sp.Name = v.Name
// 				sp.Args = v.Args

// 			case *genai.FunctionCall:
// 				sp.Type = "function_call"
// 				sp.Name = v.Name
// 				sp.Args = v.Args

// 			case genai.FunctionResponse:
// 				sp.Type = "function_response"
// 				sp.Name = v.Name
// 				if shouldSaveSearchResults() {
// 					// Most major systems (including ChatGPT and Google's Gemini) indeed discard search results between turns
// 					sp.Args = v.Response
// 				}

// 			case *genai.FunctionResponse:
// 				sp.Type = "function_response"
// 				sp.Name = v.Name
// 				if shouldSaveSearchResults() {
// 					// Most major systems (including ChatGPT and Google's Gemini) indeed discard search results between turns
// 					sp.Args = v.Response
// 				}

// 			case genai.ExecutableCode:
// 				sp.Type = "executable_code"
// 				sp.Code = v.Code
// 				sp.Language = int32(v.Language)

// 			case genai.CodeExecutionResult:
// 				sp.Type = "code_execution_result"
// 				sp.Outcome = int32(v.Outcome)
// 				sp.Output = v.Output

// 			default:
// 				return nil, fmt.Errorf("unsupported part type: %T", part)
// 			}

// 			sc.Parts = append(sc.Parts, sp)
// 		}

// 		serializableHistory = append(serializableHistory, sc)
// 	}

// 	return json.MarshalIndent(serializableHistory, "", "  ")
// }

// // deserializeHistory converts JSON data back to Gemini content
// func (g *GeminiConversation) deserializeHistory(data []byte) ([]*genai.Content, error) {
// 	var serializableHistory []SerializableContent
// 	if err := json.Unmarshal(data, &serializableHistory); err != nil {
// 		return nil, err
// 	}
// 	if len(serializableHistory) > 0 {
// 		if serializableHistory[0].Parts == nil {
// 			return nil, fmt.Errorf("invalid conversation format: isn't a compatible format. '%s'", g.Path)
// 		}
// 	}

// 	var history []*genai.Content
// 	for _, sc := range serializableHistory {
// 		content := &genai.Content{
// 			Role: sc.Role,
// 		}

// 		for _, part := range sc.Parts {
// 			switch part.Type {
// 			case "text":
// 				content.Parts = append(content.Parts, genai.Text(part.Text))

// 			case "blob":
// 				data, err := base64.StdEncoding.DecodeString(part.Data)
// 				if err != nil {
// 					return nil, err
// 				}
// 				content.Parts = append(content.Parts, &genai.Blob{
// 					MIMEType: part.MIMEType,
// 					Data:     data,
// 				})

// 			case "function_call":
// 				content.Parts = append(content.Parts, &genai.FunctionCall{
// 					Name: part.Name,
// 					Args: part.Args,
// 				})

// 			case "function_response":
// 				content.Parts = append(content.Parts, &genai.FunctionResponse{
// 					Name:     part.Name,
// 					Response: part.Args,
// 				})

// 			case "executable_code":
// 				content.Parts = append(content.Parts, &genai.ExecutableCode{
// 					Code:     part.Code,
// 					Language: genai.ExecutableCodeLanguage(part.Language),
// 				})

// 			case "code_execution_result":
// 				content.Parts = append(content.Parts, &genai.CodeExecutionResult{
// 					Outcome: genai.CodeExecutionResultOutcome(part.Outcome),
// 					Output:  part.Output,
// 				})

// 			default:
// 				return nil, fmt.Errorf("unsupported part type: %s", part.Type)
// 			}
// 		}

// 		history = append(history, content)
// 	}

// 	return history, nil
// }

// // Save persists the Gemini conversation to disk
// func (g *GeminiConversation) Save() error {
// 	if !g.ShouldLoad || len(g.History) == 0 {
// 		return nil
// 	}

// 	data, err := g.serializeHistory(g.History)
// 	if err != nil {
// 		return fmt.Errorf("failed to serialize conversation: %w", err)
// 	}

// 	return g.writeFile(data)
// }

// // Load retrieves the Gemini conversation from disk
// func (g *GeminiConversation) Load() error {
// 	data, err := g.readFile()
// 	if err != nil || data == nil {
// 		return err
// 	}

// 	history, err := g.deserializeHistory(data)
// 	if err != nil {
// 		return fmt.Errorf("failed to deserialize conversation: %w", err)
// 	}

// 	g.History = history
// 	return nil
// }

/*
 * Get the sorted list of conversations in the given directory
 * sorted by modTime descending
 */

type ConvoMeta struct {
	Name     string
	Provider ModelProvider
	ModTime  int64
}

func GetConvoDir() string {
	dir := MakeUserSubDir("gllm", "convo")
	return dir
}

func FindConvosByIndex(idx string) (string, error) {
	if strings.TrimSpace(idx) == "" {
		return "", nil
	}
	// check if it's an index
	index, err := strconv.Atoi(idx)
	if err == nil {
		// It's an index, resolve to conversation name using your sorted list logic
		convos, err := ListSortedConvos(GetConvoDir())
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

// listSortedConvos returns a slice of convoMeta sorted by modTime descending
func ListSortedConvos(convoDir string) ([]ConvoMeta, error) {
	files, err := os.ReadDir(convoDir)
	if err != nil {
		return nil, fmt.Errorf("fail to read conversation directory: %v", err)
	}
	var convos []ConvoMeta
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
			title := strings.TrimSuffix(file.Name(), ".json")
			fullPath := GetFilePath(convoDir, file.Name())
			var provider ModelProvider
			data, err := os.ReadFile(fullPath)
			if err == nil {
				provider = DetectMessageProvider(data)
			}
			info, err := os.Stat(fullPath)
			var modTime int64
			if err == nil {
				modTime = info.ModTime().Unix()
			}
			convos = append(convos, ConvoMeta{
				Name:     title,
				Provider: provider,
				ModTime:  modTime,
			})
		}
	}
	sort.Slice(convos, func(i, j int) bool {
		return convos[i].ModTime > convos[j].ModTime
	})
	return convos, nil
}
