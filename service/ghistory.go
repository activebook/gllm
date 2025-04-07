package service

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"

	// Import filepath
	"github.com/google/generative-ai-go/genai"
)

var (
	ginstance *GHistory
)

type GHistory struct {
	History    []*genai.Content
	Name       string
	Path       string
	ShouldLoad bool
}

func NewGHistory(title string, shouldLoad bool) *GHistory {
	if ginstance != nil {
		return ginstance
	}
	ginstance = &GHistory{
		History:    []*genai.Content{},
		Name:       GetDefaultConvoName(),
		ShouldLoad: shouldLoad,
	}
	if shouldLoad {
		// Set default path
		if title == "" {
			title = GetDefaultConvoName()
		}
		ginstance.Name = title
		sanitzed := GetSanitizeTitle(ginstance.Name)
		ginstance.SetPath(sanitzed)
	}
	return ginstance
}

func GetGHistory() *GHistory {
	if ginstance == nil {
		ginstance = NewGHistory("", false)
	}
	return ginstance
}

func (g *GHistory) SetPath(title string) {
	dir := MakeUserSubDir("gllm", "convo")
	g.Path = GetFilePath(dir, title+".json")
}

// Custom types for marshaling/unmarshaling
type SerializablePart struct {
	Type     string                 `json:"type"`
	Text     string                 `json:"text,omitempty"`
	MIMEType string                 `json:"mime_type,omitempty"` // For blobs
	Data     string                 `json:"data,omitempty"`      // Base64 encoded for blobs
	Name     string                 `json:"name,omitempty"`      // For function calls
	Args     map[string]interface{} `json:"args,omitempty"`      // For function calls
	Language int32                  `json:"language,omitempty"`  // For executable code
	Code     string                 `json:"code,omitempty"`      // For executable code
	Outcome  int32                  `json:"outcome,omitempty"`   // For code execution result
	Output   string                 `json:"output,omitempty"`
}

type SerializableContent struct {
	Role  string             `json:"role"`
	Parts []SerializablePart `json:"parts"`
}

func (g *GHistory) Save() error {
	// Save conversation to file
	if !g.ShouldLoad {
		// don't save anything
		return nil
	}
	if len(g.History) == 0 {
		return nil
	}
	data, err := g.serializeHistory(g.History)
	if err != nil {
		return fmt.Errorf("failed to serialize conversation: %w", err)
	}
	return os.WriteFile(g.Path, data, 0644)
}

func (g *GHistory) Load() error {
	if !g.ShouldLoad {
		// If convoPath is not set, don't load anything
		return nil
	}
	Infof("Loading previous conversation: %s\n", g.Name)

	data, err := os.ReadFile(g.Path)
	if err != nil {
		// Handle file not found specifically if needed (e.g., return empty history)
		if os.IsNotExist(err) {
			return nil // Return empty slice if file doesn't exist
		}
		return fmt.Errorf("failed to read conversation file '%s': %w", g.Path, err)
	}
	// If the file is empty, return an empty history
	if len(data) == 0 {
		return nil
	}
	// First try to validate the JSON format before unmarshaling
	if !json.Valid(data) {
		return fmt.Errorf("invalid JSON format in conversation file '%s'", g.Path)
	}

	// Deserialize the JSON data
	// DeserializeHistory is a function that converts the JSON data back into the original structure
	history, err := g.deserializeHistory(data)
	if err != nil {
		return fmt.Errorf("failed to deserialize conversation: %w", err)
	}
	g.History = history
	return nil
}

// When marshaling:
func (g *GHistory) serializeHistory(history []*genai.Content) ([]byte, error) {
	var serializableHistory []SerializableContent
	for _, content := range history {
		sc := SerializableContent{
			Role: content.Role,
		}

		for _, part := range content.Parts {
			var sp SerializablePart

			switch v := part.(type) {
			case genai.Text:
				sp.Type = "text"
				sp.Text = string(v)

			case genai.Blob:
				sp.Type = "blob"
				sp.MIMEType = v.MIMEType
				sp.Data = base64.StdEncoding.EncodeToString(v.Data)

			case *genai.Blob:
				sp.Type = "blob"
				sp.MIMEType = v.MIMEType
				sp.Data = base64.StdEncoding.EncodeToString(v.Data)

			case genai.FunctionCall:
				sp.Type = "function_call"
				sp.Name = v.Name
				sp.Args = v.Args

			case *genai.FunctionCall:
				sp.Type = "function_call"
				sp.Name = v.Name
				sp.Args = v.Args

			case genai.FunctionResponse:
				sp.Type = "function_response"
				sp.Name = v.Name
				sp.Args = v.Response

			case *genai.FunctionResponse:
				sp.Type = "function_response"
				sp.Name = v.Name
				sp.Args = v.Response

			case genai.ExecutableCode:
				sp.Type = "executable_code"
				sp.Code = v.Code
				sp.Language = int32(v.Language)

			case genai.CodeExecutionResult:
				sp.Type = "code_execution_result"
				sp.Outcome = int32(v.Outcome)
				sp.Output = v.Output

			// Add cases for other types as needed
			default:
				return nil, fmt.Errorf("unsupported part type: %T", part)
			}

			sc.Parts = append(sc.Parts, sp)
		}

		serializableHistory = append(serializableHistory, sc)
	}

	// You can use json.Marshal(serializableHistory) for compact JSON
	// return json.Marshal(serializableHistory)
	// Or use json.MarshalIndent(serializableHistory, "", "  ") for pretty-printed JSON
	return json.MarshalIndent(serializableHistory, "", "  ")
}

// When unmarshaling:
func (g *GHistory) deserializeHistory(data []byte) ([]*genai.Content, error) {
	var serializableHistory []SerializableContent
	if err := json.Unmarshal(data, &serializableHistory); err != nil {
		return nil, err
	}
	if len(serializableHistory) > 0 {
		if serializableHistory[0].Parts == nil {
			return nil, fmt.Errorf("invalid conversation format: isn't a compatible format. '%s'", g.Path)
		}
	}

	var history []*genai.Content
	for _, sc := range serializableHistory {
		content := &genai.Content{
			Role: sc.Role,
		}

		for _, part := range sc.Parts {
			switch part.Type {
			case "text":
				content.Parts = append(content.Parts, genai.Text(part.Text))

			case "blob":
				data, err := base64.StdEncoding.DecodeString(part.Data)
				if err != nil {
					return nil, err
				}
				content.Parts = append(content.Parts, &genai.Blob{
					MIMEType: part.MIMEType,
					Data:     data,
				})

			case "function_call":
				content.Parts = append(content.Parts, &genai.FunctionCall{
					Name: part.Name,
					Args: part.Args,
				})

			case "function_response":
				content.Parts = append(content.Parts, &genai.FunctionResponse{
					Name:     part.Name,
					Response: part.Args,
				})

			case "executable_code":
				content.Parts = append(content.Parts, &genai.ExecutableCode{
					Code:     part.Code,
					Language: genai.ExecutableCodeLanguage(part.Language),
				})

			case "code_execution_result":
				content.Parts = append(content.Parts, &genai.CodeExecutionResult{
					Outcome: genai.CodeExecutionResultOutcome(part.Outcome),
					Output:  part.Output,
				})

			// Add cases for other types as needed
			default:
				return nil, fmt.Errorf("unsupported part type: %s", part.Type)
			}
		}

		history = append(history, content)
	}

	return history, nil
}
