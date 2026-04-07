package service

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/activebook/gllm/data"
	"github.com/anthropics/anthropic-sdk-go"
	openai "github.com/openai/openai-go/v3"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
	"google.golang.org/genai"
)

const RenameSystemPrompt = `You are a session title generator.
Analyze the conversation and produce a concise, descriptive title.

Rules:
1. Output ONLY the title — no punctuation, no quotes, no explanation
2. Use 4–8 words, all lowercase, joined by hyphens
3. Name the specific artifact, concept, or mechanism at the center of the session
4. Include proper nouns — tool names, file names, API names, commands — when central
5. Titles should be scannable and self-explanatory when read in a list days later

Good titles (specific, scannable, topic-focused):
- gemini-context-window-injection-strategy
- uncached-token-cost-fields-breakdown
- vscodeignore-packaging-and-marketplace-copy
- unix-socket-ipc-daemon-architecture
- ask-user-tool-four-mode-schema

Bad titles — too vague to identify the topic later:
- add-feature-to-system        (no specifics, could be anything)
- implement-support-for-files  (which files? what system?)
- fix-prompt-formatting-bug    (what prompt? what bug?)
- update-existing-configuration (tells you nothing)`

const RenamePromptFormat = `Generate a title for this conversation following your system instructions.
Output ONLY the hyphenated title, nothing else.`

// illegalNameChars matches any character not suitable for a session directory name.
var illegalNameChars = regexp.MustCompile(`[^a-zA-Z0-9\-_]`)

// sanitizeGeneratedName cleans up a model-produced name:
// strips surrounding whitespace/quotes, collapses runs of
// sanitizeGeneratedName normalizes a model-produced title into a filesystem-safe,
// lowercase hyphenated slug suitable for use as a directory name.
// It trims surrounding whitespace and wrapping quotes, converts to lowercase,
// replaces spaces and underscores with hyphens, removes characters outside
// [a-z0-9-_], collapses consecutive hyphens into one, and trims leading/trailing
// hyphens. The resulting string may be empty.
func sanitizeGeneratedName(raw string) string {
	name := strings.TrimSpace(raw)
	// Strip wrapping quotes the model occasionally emits
	name = strings.Trim(name, `"'`+"`")
	name = strings.ToLower(name)
	// Replace spaces and underscores with hyphens before stripping other chars
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")
	// Remove any character that is not alphanumeric, hyphen
	name = illegalNameChars.ReplaceAllString(name, "")
	// Collapse consecutive hyphens
	name = regexp.MustCompile(`[-]{2,}`).ReplaceAllString(name, "-")
	// Trim leading/trailing hyphens
	name = strings.Trim(name, "-")
	return name
}

// GenerateSessionName invokes the active provider's synchronous completion API
// to produce a concise, hyphen-separated title derived from the session history.
//
// It mirrors CompressSession's architecture: a minimal Agent is constructed
// from the config, and each provider appends the rename prompt to the existing
// message slice before making a single non-streaming call.
//
// Returns ("", err) on any failure so the caller can fall back gracefully.
func GenerateSessionName(modelConfig *data.AgentConfig, sessionData []byte) (string, error) {
	ag := &Agent{
		Model: constructModelInfo(&modelConfig.Model),
	}
	ag.Context = NewContextManager(ag, StrategyNone)

	var raw string
	var err error

	switch modelConfig.Model.Provider {

	case ModelProviderOpenAI:
		var messages []openai.ChatCompletionMessageParamUnion
		if err = parseJSONL(sessionData, &messages); err != nil {
			return "", fmt.Errorf("failed to parse OpenAI session for rename: %w", err)
		}
		send := append(make([]openai.ChatCompletionMessageParamUnion, 0, len(messages)+1), messages...)
		send = append(send, openai.UserMessage(RenamePromptFormat))
		raw, err = ag.GenerateOpenAISync(send, RenameSystemPrompt)

	case ModelProviderAnthropic:
		var messages []anthropic.MessageParam
		if err = parseJSONL(sessionData, &messages); err != nil {
			return "", fmt.Errorf("failed to parse Anthropic session for rename: %w", err)
		}
		send := append(make([]anthropic.MessageParam, 0, len(messages)+1), messages...)
		send = append(send, anthropic.NewUserMessage(anthropic.NewTextBlock(RenamePromptFormat)))
		raw, err = ag.GenerateAnthropicSync(send, RenameSystemPrompt)

	case ModelProviderGemini:
		var messages []*genai.Content
		if err = parseJSONL(sessionData, &messages); err != nil {
			return "", fmt.Errorf("failed to parse Gemini session for rename: %w", err)
		}
		send := append(make([]*genai.Content, 0, len(messages)+1), messages...)
		send = append(send, &genai.Content{
			Role:  genai.RoleUser,
			Parts: []*genai.Part{{Text: RenamePromptFormat}},
		})
		raw, err = ag.GenerateGeminiSync(send, RenameSystemPrompt)

	case ModelProviderOpenAICompatible:
		var messages []*model.ChatCompletionMessage
		if err = parseJSONL(sessionData, &messages); err != nil {
			return "", fmt.Errorf("failed to parse OpenChat session for rename: %w", err)
		}
		send := append(make([]*model.ChatCompletionMessage, 0, len(messages)+1), messages...)
		send = append(send, &model.ChatCompletionMessage{
			Role: model.ChatMessageRoleUser,
			Content: &model.ChatCompletionMessageContent{
				StringValue: volcengine.String(RenamePromptFormat),
			},
			Name: Ptr(""),
		})
		raw, err = ag.GenerateOpenChatSync(send, RenameSystemPrompt)

	default:
		return "", fmt.Errorf("unsupported provider for session rename: %s", modelConfig.Model.Provider)
	}

	if err != nil {
		return "", fmt.Errorf("model call failed during session rename: %w", err)
	}

	name := sanitizeGeneratedName(raw)
	if name == "" {
		return "", fmt.Errorf("model returned an empty or unusable name: %q", raw)
	}
	return name, nil
}
