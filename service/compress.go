package service

import (
	"fmt"

	"github.com/activebook/gllm/data"
	anthropic "github.com/anthropics/anthropic-sdk-go"
	openai "github.com/sashabaranov/go-openai"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
	"google.golang.org/genai"
)

const CompressionSystemPrompt = `You are a conversation compression assistant. Your task is to compress the following conversation into a thorough, information-dense summary that preserves ALL key information.

Rules:
1. Preserve ALL factual information, decisions, conclusions, and action items
2. Preserve ALL code snippets, file paths, configuration details, and technical specifications
3. Preserve the chronological flow of the conversation
4. Preserve any unresolved questions or pending tasks
5. Use structured format (headers, bullet points) for clarity
6. Do NOT add any information that was not in the original conversation
7. Do NOT lose any details that would be needed to continue the conversation seamlessly

The compressed output should allow someone to read it and have full context to continue the conversation as if they had read the entire history.`

const CompressionPromptFormat = `Please compress the conversation history above according to your system instructions.`

const CompressedContextPrefix = "Here is the compressed context of our conversation:\n\n"
const CompressedContextAck = "Context compressed successfully. I have read the summary and am ready to continue."

// CompressConversation takes the raw conversation JSONL bytes and the active agent,
// and returns a compressed summary string using the active provider's non-streaming API.
// No need to preserve the latest user message, because it's coming from /compress command.
func CompressConversation(modelConfig *data.AgentConfig, convoData []byte) (string, error) {
	// Reconstruct a lightweight Agent instance just for sync generation
	ag := &Agent{
		Model: constructModelInfo(&modelConfig.Model),
	}

	switch modelConfig.Model.Provider {

	case ModelProviderOpenAI:
		var messages []openai.ChatCompletionMessage
		if err := parseJSONL(convoData, &messages); err != nil {
			return "", fmt.Errorf("failed to parse OpenAI conversation: %w", err)
		}
		send := append(make([]openai.ChatCompletionMessage, 0, len(messages)+1), messages...)
		send = append(send, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: CompressionPromptFormat,
		})
		return ag.GenerateOpenAISync(send, CompressionSystemPrompt)

	case ModelProviderAnthropic:
		var messages []anthropic.MessageParam
		if err := parseJSONL(convoData, &messages); err != nil {
			return "", fmt.Errorf("failed to parse Anthropic conversation: %w", err)
		}
		send := append(make([]anthropic.MessageParam, 0, len(messages)+1), messages...)
		send = append(send, anthropic.NewUserMessage(anthropic.NewTextBlock(CompressionPromptFormat)))
		return ag.GenerateAnthropicSync(send, CompressionSystemPrompt)

	case ModelProviderGemini:
		var messages []*genai.Content
		if err := parseJSONL(convoData, &messages); err != nil {
			return "", fmt.Errorf("failed to parse Gemini conversation: %w", err)
		}
		send := append(make([]*genai.Content, 0, len(messages)+1), messages...)
		send = append(send, &genai.Content{
			Role:  genai.RoleUser,
			Parts: []*genai.Part{{Text: CompressionPromptFormat}},
		})
		return ag.GenerateGeminiSync(send, CompressionSystemPrompt)

	case ModelProviderOpenAICompatible: // OpenChat / Volcengine
		var messages []*model.ChatCompletionMessage
		if err := parseJSONL(convoData, &messages); err != nil {
			return "", fmt.Errorf("failed to parse OpenChat conversation: %w", err)
		}
		send := append(make([]*model.ChatCompletionMessage, 0, len(messages)+1), messages...)
		send = append(send, &model.ChatCompletionMessage{
			Role: model.ChatMessageRoleUser,
			Content: &model.ChatCompletionMessageContent{
				StringValue: volcengine.String(CompressionPromptFormat),
			},
			Name: Ptr(""),
		})
		return ag.GenerateOpenChatSync(send, CompressionSystemPrompt)

	default:
		return "", fmt.Errorf("unsupported provider for compression: %s", modelConfig.Model.Provider)
	}
}

// BuildCompressedConvo constructs a new 2-message JSONL conversation from the summary,
// formatted for the specified provider. User provides the summary, assistant acknowledges.
func BuildCompressedConvo(summary string, provider string) ([]byte, error) {
	switch provider {
	case ModelProviderOpenAI:
		messages := []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: CompressedContextPrefix + summary},
			{Role: openai.ChatMessageRoleAssistant, Content: CompressedContextAck},
		}
		return marshalJSONL(messages)

	case ModelProviderAnthropic:
		messages := []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(CompressedContextPrefix + summary)),
			anthropic.NewAssistantMessage(anthropic.NewTextBlock(CompressedContextAck)),
		}
		return marshalJSONL(messages)

	case ModelProviderGemini:
		messages := []*genai.Content{
			{Role: genai.RoleUser, Parts: []*genai.Part{{Text: CompressedContextPrefix + summary}}},
			{Role: genai.RoleModel, Parts: []*genai.Part{{Text: CompressedContextAck}}},
		}
		return marshalJSONL(messages)

	case ModelProviderOpenAICompatible: // OpenChat / Volcengine
		messages := []*model.ChatCompletionMessage{
			{
				Role: model.ChatMessageRoleUser,
				Content: &model.ChatCompletionMessageContent{
					StringValue: volcengine.String(CompressedContextPrefix + summary),
				},
				Name: Ptr(""),
			},
			{
				Role: model.ChatMessageRoleAssistant,
				Content: &model.ChatCompletionMessageContent{
					StringValue: volcengine.String(CompressedContextAck),
				},
				Name: Ptr(""),
			},
		}
		return marshalJSONL(messages)

	default:
		return nil, fmt.Errorf("unsupported provider for building compressed convo: %s", provider)
	}
}

/**
 * Utility Compression functions for different providers
 * These functions are used by the ContextManager to compress messages.
 * Only inside service/ directory
 */

// compressOpenAIMessages compresses OpenAI messages using the active provider's non-streaming API.
// If the last message is a user message, it is excluded from the summary and re-appended verbatim
// afterward, preserving the user's exact current intent.
func compressOpenAIMessages(ag *Agent, messages []openai.ChatCompletionMessage) ([]openai.ChatCompletionMessage, error) {
	if len(messages) == 0 {
		return messages, nil
	}
	latest := messages[len(messages)-1]
	isUserMsg := latest.Role == openai.ChatMessageRoleUser
	history := messages
	if isUserMsg {
		history = messages[:len(messages)-1]
	}

	send := append(make([]openai.ChatCompletionMessage, 0, len(history)+1), history...)
	send = append(send, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: CompressionPromptFormat,
	})
	summary, err := ag.GenerateOpenAISync(send, CompressionSystemPrompt)
	if err != nil {
		return nil, err
	}
	// [compressed history] → [ack] → [latest user message verbatim (if applicable)]
	result := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: CompressedContextPrefix + summary},
		{Role: openai.ChatMessageRoleAssistant, Content: CompressedContextAck},
	}
	if isUserMsg {
		result = append(result, latest)
	}
	return result, nil
}

// compressAnthropicMessages compresses Anthropic messages using the active provider's non-streaming API.
// If the last message is a user message, it is excluded from the summary and re-appended verbatim.
func compressAnthropicMessages(ag *Agent, messages []anthropic.MessageParam) ([]anthropic.MessageParam, error) {
	if len(messages) == 0 {
		return messages, nil
	}
	latest := messages[len(messages)-1]
	isUserMsg := latest.Role == anthropic.MessageParamRoleUser
	history := messages
	if isUserMsg {
		history = messages[:len(messages)-1]
	}

	send := append(make([]anthropic.MessageParam, 0, len(history)+1), history...)
	send = append(send, anthropic.NewUserMessage(anthropic.NewTextBlock(CompressionPromptFormat)))
	summary, err := ag.GenerateAnthropicSync(send, CompressionSystemPrompt)
	if err != nil {
		return nil, err
	}
	result := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(CompressedContextPrefix + summary)),
		anthropic.NewAssistantMessage(anthropic.NewTextBlock(CompressedContextAck)),
	}
	if isUserMsg {
		result = append(result, latest)
	}
	return result, nil
}

// compressGeminiMessages compresses Gemini messages using the active provider's non-streaming API.
// If the last message is a user message, it is excluded from the summary and re-appended verbatim.
func compressGeminiMessages(ag *Agent, messages []*genai.Content) ([]*genai.Content, error) {
	if len(messages) == 0 {
		return messages, nil
	}
	latest := messages[len(messages)-1]
	isUserMsg := latest != nil && latest.Role == genai.RoleUser
	history := messages
	if isUserMsg {
		history = messages[:len(messages)-1]
	}

	send := append(make([]*genai.Content, 0, len(history)+1), history...)
	send = append(send, &genai.Content{
		Role:  genai.RoleUser,
		Parts: []*genai.Part{{Text: CompressionPromptFormat}},
	})
	summary, err := ag.GenerateGeminiSync(send, CompressionSystemPrompt)
	if err != nil {
		return nil, err
	}
	result := []*genai.Content{
		{Role: genai.RoleUser, Parts: []*genai.Part{{Text: CompressedContextPrefix + summary}}},
		{Role: genai.RoleModel, Parts: []*genai.Part{{Text: CompressedContextAck}}},
	}
	if isUserMsg {
		result = append(result, latest)
	}
	return result, nil
}

// compressOpenChatMessages compresses OpenChat messages using the active provider's non-streaming API.
// If the last message is a user message, it is excluded from the summary and re-appended verbatim.
func compressOpenChatMessages(ag *Agent, messages []*model.ChatCompletionMessage) ([]*model.ChatCompletionMessage, error) {
	if len(messages) == 0 {
		return messages, nil
	}
	latest := messages[len(messages)-1]
	isUserMsg := latest != nil && latest.Role == model.ChatMessageRoleUser
	history := messages
	if isUserMsg {
		history = messages[:len(messages)-1]
	}

	send := append(make([]*model.ChatCompletionMessage, 0, len(history)+1), history...)
	send = append(send, &model.ChatCompletionMessage{
		Role: model.ChatMessageRoleUser,
		Content: &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(CompressionPromptFormat),
		},
		Name: Ptr(""),
	})
	summary, err := ag.GenerateOpenChatSync(send, CompressionSystemPrompt)
	if err != nil {
		return nil, err
	}
	result := []*model.ChatCompletionMessage{
		{
			Role: model.ChatMessageRoleUser,
			Content: &model.ChatCompletionMessageContent{
				StringValue: volcengine.String(CompressedContextPrefix + summary),
			},
			Name: Ptr(""),
		},
		{
			Role: model.ChatMessageRoleAssistant,
			Content: &model.ChatCompletionMessageContent{
				StringValue: volcengine.String(CompressedContextAck),
			},
			Name: Ptr(""),
		},
	}
	if isUserMsg {
		result = append(result, latest)
	}
	return result, nil
}
