package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	openai "github.com/openai/openai-go/v3"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	gemini "google.golang.org/genai"
)

/*
 * UniversalMessage is a provider-agnostic representation of a chat message.
 * It extracts only the essential semantic content for cross-provider conversion.
 *
 * Key Design Decisions:
 * 1. Text content and images are preserved via Parts.
 * 2. Reasoning is preserved.
 * 3. Tool calls and tool responses are discarded.
 * 4. Role normalization: "model" (Gemini) → "assistant"
 */
type UniversalMessage struct {
	Role      UniversalRole // "system", "user", "assistant"
	Parts     []UniversalPart
	Reasoning string // Thinking/reasoning content (if any)

	// Tool interactions
	ToolCalls  []UniversalToolCall
	ToolResult *UniversalToolResult
}

// UniversalToolCall carries the semantic intent of a single tool invocation.
type UniversalToolCall struct {
	ID   string
	Name string
	Args map[string]interface{}
}

// UniversalToolResult carries the output of a single tool execution.
type UniversalToolResult struct {
	CallID  string
	Name    string
	Output  string
	IsError bool
}

type UniversalPartType string

const (
	PartTypeText  UniversalPartType = "text"
	PartTypeImage UniversalPartType = "image"
)

type UniversalPart struct {
	Type     UniversalPartType
	MIMEType string
	Text     string // Populated if Type is PartTypeText
	Data     []byte // Raw bytes, populated if Type is PartTypeImage
}

// GetTextContent returns all text parts concatenated.
func (um *UniversalMessage) GetTextContent() string {
	var builder strings.Builder
	for _, part := range um.Parts {
		if part.Type == PartTypeText {
			if builder.Len() > 0 {
				builder.WriteByte('\n')
			}
			builder.WriteString(part.Text)
		}
	}
	return builder.String()
}

// HasContent returns true if the message has any text, reasoning, media parts, or tool interactions.
func (um *UniversalMessage) HasContent() bool {
	return len(um.Parts) > 0 || um.Reasoning != "" || len(um.ToolCalls) > 0 || um.ToolResult != nil
}

type UniversalRole string

const (
	UniversalRoleSystem    UniversalRole = "system"
	UniversalRoleUser      UniversalRole = "user"
	UniversalRoleAssistant UniversalRole = "assistant"
	UniversalRoleTool      UniversalRole = "tool"
)

func (r UniversalRole) String() string {
	return string(r)
}

func ConvertToUniversalRole(role string) UniversalRole {
	switch role {
	case gemini.RoleModel: // model
		return UniversalRoleAssistant
	case model.ChatMessageRoleAssistant: // assistant
		return UniversalRoleAssistant
	case model.ChatMessageRoleTool, "function": // tool and function
		return UniversalRoleTool
	case model.ChatMessageRoleUser: // user
		return UniversalRoleUser
	case model.ChatMessageRoleSystem: // system
		return UniversalRoleSystem
	default:
		return UniversalRole(role)
	}
}

// ==============================
// High-Level Conversion Function
// ==============================

// parseJSONL parses strictly JSONL format (one JSON object per line)
func parseJSONL[T any](data []byte, target *[]T) error {
	lines := bytes.Split(data, []byte("\n"))
	for i, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var item T
		if err := json.Unmarshal(line, &item); err != nil {
			return fmt.Errorf("line %d: %w", i+1, err)
		}
		*target = append(*target, item)
	}
	return nil
}

// marshalJSONL encodes a slice as JSONL (one JSON object per line)
func marshalJSONL[T any](messages []T) ([]byte, error) {
	var buf bytes.Buffer
	for _, msg := range messages {
		line, err := json.Marshal(msg)
		if err != nil {
			return nil, err
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	return buf.Bytes(), nil
}

// correlateToolNames scans all tool calls in the message history to build an ID-to-Name index,
// then backfills the Name on any ToolResult messages that are missing it (e.g. from OpenAI).
func correlateToolNames(msgs []UniversalMessage) {
	idToName := map[string]string{}
	for _, msg := range msgs {
		for _, tc := range msg.ToolCalls {
			if tc.ID != "" && tc.Name != "" {
				idToName[tc.ID] = tc.Name
			}
		}
	}
	for i := range msgs {
		if msgs[i].ToolResult != nil && msgs[i].ToolResult.Name == "" {
			msgs[i].ToolResult.Name = idToName[msgs[i].ToolResult.CallID]
		}
	}
}

// ConvertMessages parses source provider data and builds target provider messages.
// Returns the converted data encoded as JSON.
//
// Supported source/target providers:
// - ModelProviderOpenAI
// - ModelProviderOpenAICompatible (OpenChat)
// - ModelProviderAnthropic
// - ModelProviderGemini
func ConvertMessages(data []byte, sourceProvider, targetProvider string) ([]byte, error) {
	if sourceProvider == targetProvider {
		// No conversion needed
		return data, nil
	}

	// Check compatible providers (OpenAI and OpenChat use same format for text)
	if (sourceProvider == ModelProviderOpenAI || sourceProvider == ModelProviderOpenAICompatible) &&
		(targetProvider == ModelProviderOpenAI || targetProvider == ModelProviderOpenAICompatible) {
		// Direct copy for compatible providers
		return data, nil
	}

	// Step 1: Parse source data to universal format
	var uniMsgs []UniversalMessage

	switch sourceProvider {
	case ModelProviderOpenAI:
		var msgs []openai.ChatCompletionMessageParamUnion
		if err := parseJSONL(data, &msgs); err != nil {
			return nil, fmt.Errorf("failed to parse OpenAI messages: %w", err)
		}
		uniMsgs = ParseOpenAIMessages(msgs)

	case ModelProviderOpenAICompatible:
		var msgs []model.ChatCompletionMessage
		if err := parseJSONL(data, &msgs); err != nil {
			return nil, fmt.Errorf("failed to parse OpenChat messages: %w", err)
		}
		uniMsgs = ParseOpenChatMessages(msgs)

	case ModelProviderAnthropic:
		var msgs []anthropic.MessageParam
		if err := parseJSONL(data, &msgs); err != nil {
			return nil, fmt.Errorf("failed to parse Anthropic messages: %w", err)
		}
		uniMsgs = ParseAnthropicMessages(msgs)

	case ModelProviderGemini:
		var msgs []*gemini.Content
		if err := parseJSONL(data, &msgs); err != nil {
			return nil, fmt.Errorf("failed to parse Gemini messages: %w", err)
		}
		uniMsgs = ParseGeminiMessages(msgs)

	default:
		return nil, fmt.Errorf("unsupported source provider: %s", sourceProvider)
	}

	// Step 1.5: Backfill tool names for ToolResults (needed for OpenAI -> Gemini)
	correlateToolNames(uniMsgs)

	// Step 2: Build target format and marshal as JSONL
	switch targetProvider {
	case ModelProviderOpenAI:
		newmsgs := BuildOpenAIMessages(uniMsgs)
		return marshalJSONL(newmsgs)

	case ModelProviderOpenAICompatible:
		newmsgs := BuildOpenChatMessages(uniMsgs)
		return marshalJSONL(newmsgs)

	case ModelProviderAnthropic:
		newmsgs := BuildAnthropicMessages(uniMsgs)
		return marshalJSONL(newmsgs)

	case ModelProviderGemini:
		newmsgs := BuildGeminiMessages(uniMsgs)
		return marshalJSONL(newmsgs)

	default:
		return nil, fmt.Errorf("unsupported target provider: %s", targetProvider)
	}
}
