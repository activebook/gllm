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

const CompressionPromptFormat = `Please compress the entire conversation history above according to your system instructions.`

const CompressedContextPrefix = "Here is the compressed context of our conversation:\n\n"
const CompressedContextAck = "Context compressed successfully. I have read the summary and am ready to continue."

// CompressConversation takes the raw conversation JSONL bytes and the active agent,
// and returns a compressed summary string using the active provider's non-streaming API.
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
		// Add compression prompt
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: CompressionPromptFormat,
		})
		return ag.GenerateOpenAISync(messages, CompressionSystemPrompt)

	case ModelProviderAnthropic:
		var messages []anthropic.MessageParam
		if err := parseJSONL(convoData, &messages); err != nil {
			return "", fmt.Errorf("failed to parse Anthropic conversation: %w", err)
		}
		// Add compression prompt request
		messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(CompressionPromptFormat)))
		return ag.GenerateAnthropicSync(messages, CompressionSystemPrompt)

	case ModelProviderGemini:
		var messages []*genai.Content
		if err := parseJSONL(convoData, &messages); err != nil {
			return "", fmt.Errorf("failed to parse Gemini conversation: %w", err)
		}
		messages = append(messages, &genai.Content{
			Role:  genai.RoleUser,
			Parts: []*genai.Part{{Text: CompressionPromptFormat}},
		})
		return ag.GenerateGeminiSync(messages, CompressionSystemPrompt)

	case ModelProviderOpenAICompatible: // OpenChat / Volcengine
		var messages []*model.ChatCompletionMessage
		if err := parseJSONL(convoData, &messages); err != nil {
			return "", fmt.Errorf("failed to parse OpenChat conversation: %w", err)
		}
		// Add compression prompt
		messages = append(messages, &model.ChatCompletionMessage{
			Role: model.ChatMessageRoleUser,
			Content: &model.ChatCompletionMessageContent{
				StringValue: volcengine.String(CompressionPromptFormat),
			},
			Name: Ptr(""),
		})
		return ag.GenerateOpenChatSync(messages, CompressionSystemPrompt)

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
func compressOpenAIMessages(ag *Agent, messages []openai.ChatCompletionMessage) ([]openai.ChatCompletionMessage, error) {
	// Copy into a fresh slice to avoid mutating the caller's backing array (slice aliasing)
	send := append(make([]openai.ChatCompletionMessage, 0, len(messages)+1), messages...)
	send = append(send, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: CompressionPromptFormat,
	})
	summary, err := ag.GenerateOpenAISync(send, CompressionSystemPrompt)
	if err != nil {
		return nil, err
	}
	return []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: CompressedContextPrefix + summary},
		{Role: openai.ChatMessageRoleAssistant, Content: CompressedContextAck},
	}, nil
}

// compressAnthropicMessages compresses Anthropic messages using the active provider's non-streaming API.
func compressAnthropicMessages(ag *Agent, messages []anthropic.MessageParam) ([]anthropic.MessageParam, error) {
	// Copy into a fresh slice to avoid mutating the caller's backing array (slice aliasing)
	send := append(make([]anthropic.MessageParam, 0, len(messages)+1), messages...)
	send = append(send, anthropic.NewUserMessage(anthropic.NewTextBlock(CompressionPromptFormat)))
	summary, err := ag.GenerateAnthropicSync(send, CompressionSystemPrompt)
	if err != nil {
		return nil, err
	}
	return []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(CompressedContextPrefix + summary)),
		anthropic.NewAssistantMessage(anthropic.NewTextBlock(CompressedContextAck)),
	}, nil
}

// compressGeminiMessages compresses Gemini messages using the active provider's non-streaming API.
func compressGeminiMessages(ag *Agent, messages []*genai.Content) ([]*genai.Content, error) {
	// Copy into a fresh slice to avoid mutating the caller's backing array (slice aliasing)
	send := append(make([]*genai.Content, 0, len(messages)+1), messages...)
	send = append(send, &genai.Content{
		Role:  genai.RoleUser,
		Parts: []*genai.Part{{Text: CompressionPromptFormat}},
	})
	summary, err := ag.GenerateGeminiSync(send, CompressionSystemPrompt)
	if err != nil {
		return nil, err
	}
	return []*genai.Content{
		{Role: genai.RoleUser, Parts: []*genai.Part{{Text: CompressedContextPrefix + summary}}},
		{Role: genai.RoleModel, Parts: []*genai.Part{{Text: CompressedContextAck}}},
	}, nil
}

// compressOpenChatMessages compresses OpenChat messages using the active provider's non-streaming API.
func compressOpenChatMessages(ag *Agent, messages []*model.ChatCompletionMessage) ([]*model.ChatCompletionMessage, error) {
	// Copy into a fresh slice to avoid mutating the caller's backing array (slice aliasing)
	send := append(make([]*model.ChatCompletionMessage, 0, len(messages)+1), messages...)
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
	return []*model.ChatCompletionMessage{
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
	}, nil
}
