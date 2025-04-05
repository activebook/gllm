package service

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	// Import filepath
	"github.com/google/generative-ai-go/genai"
)

// Custom types for marshaling/unmarshaling
type SerializablePart struct {
	Type     string                 `json:"type"`
	Text     string                 `json:"text,omitempty"`
	MIMEType string                 `json:"mime_type,omitempty"`
	Data     string                 `json:"data,omitempty"` // Base64 encoded for blobs
	Name     string                 `json:"name,omitempty"` // For function calls
	Args     map[string]interface{} `json:"args,omitempty"` // For function calls
}

type SerializableContent struct {
	Role  string             `json:"role"`
	Parts []SerializablePart `json:"parts"`
}

// When marshaling:
func SerializeHistory(history []*genai.Content) ([]byte, error) {
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

			case *genai.Blob:
				sp.Type = "blob"
				sp.MIMEType = v.MIMEType
				sp.Data = base64.StdEncoding.EncodeToString(v.Data)

			case *genai.FunctionCall:
				sp.Type = "function_call"
				sp.Name = v.Name
				sp.Args = v.Args

			// Add cases for other types as needed
			default:
				return nil, fmt.Errorf("unsupported part type: %T", part)
			}

			sc.Parts = append(sc.Parts, sp)
		}

		serializableHistory = append(serializableHistory, sc)
	}

	return json.Marshal(serializableHistory)
}

// When unmarshaling:
func DeserializeHistory(data []byte) ([]*genai.Content, error) {
	var serializableHistory []SerializableContent
	if err := json.Unmarshal(data, &serializableHistory); err != nil {
		return nil, err
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

			// Add cases for other types as needed
			default:
				return nil, fmt.Errorf("unsupported part type: %s", part.Type)
			}
		}

		history = append(history, content)
	}

	return history, nil
}
