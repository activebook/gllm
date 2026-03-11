package service

import (
	"testing"

	openai "github.com/sashabaranov/go-openai"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"google.golang.org/genai"
)

func TestNewContextManager(t *testing.T) {
	limits := ModelLimits{ContextWindow: 128000, MaxOutputTokens: 16384}
	expectedMax := limits.MaxInputTokens(DefaultBufferPercent)
	ag := &Agent{Model: &ModelInfo{ModelName: "gpt-4o", Provider: ModelProviderOpenAI}}
	cm := NewContextManager(ag, StrategyTruncateOldest)

	if cm.GetStrategy() != StrategyTruncateOldest {
		t.Errorf("Strategy = %v, want %v", cm.GetStrategy(), StrategyTruncateOldest)
	}

	// Internal details check requires type assertion
	oc := cm.(*openAIContext)
	if oc.maxInputTokens != expectedMax {
		t.Errorf("MaxInputTokens = %d, want %d", oc.maxInputTokens, expectedMax)
	}
}

func TestPruneOpenAIMessagesNoTruncation(t *testing.T) {
	cm := &openAIContext{
		commonContext: commonContext{
			maxInputTokens: 100000,
			strategy:       StrategyTruncateOldest,
		},
	}

	systemPrompt := "You are a helpful assistant."
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "Hello"},
		{Role: openai.ChatMessageRoleAssistant, Content: "Hi there!"},
	}

	resultAny, truncated, _ := cm.PruneMessages(messages, systemPrompt, nil)
	result := resultAny.([]openai.ChatCompletionMessage)

	if truncated {
		t.Error("Expected no truncation for small messages")
	}

	if len(result) != len(messages) {
		t.Errorf("Message count = %d, want %d", len(result), len(messages))
	}
}

func TestPruneOpenAIMessagesWithTruncation(t *testing.T) {
	cm := &openAIContext{
		commonContext: commonContext{
			maxInputTokens: 60,
			strategy:       StrategyTruncateOldest,
		},
	}

	systemPrompt := "You are a helpful assistant."
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "This is message one with some content."},
		{Role: openai.ChatMessageRoleAssistant, Content: "This is response one with some content."},
		{Role: openai.ChatMessageRoleUser, Content: "This is message two with some content."},
		{Role: openai.ChatMessageRoleAssistant, Content: "This is response two with some content."},
	}

	resultAny, truncated, _ := cm.PruneMessages(messages, systemPrompt, nil)
	result := resultAny.([]openai.ChatCompletionMessage)

	if !truncated {
		t.Error("Expected truncation for messages exceeding limit")
	}

	if len(result) >= len(messages) {
		t.Errorf("Expected fewer messages after truncation, got %d (original %d)",
			len(result), len(messages))
	}

	for _, msg := range result {
		if msg.Role == openai.ChatMessageRoleSystem {
			t.Error("System message should not be in the pruned dialogue result")
		}
	}
}

func TestPruneOpenAIMessagesWithStrategyNone(t *testing.T) {
	cm := &openAIContext{
		commonContext: commonContext{
			maxInputTokens: 10,
			strategy:       StrategyNone,
		},
	}

	systemPrompt := "You are a helpful assistant with a long description."
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "Hello, this is a relatively long message."},
	}

	resultAny, truncated, _ := cm.PruneMessages(messages, systemPrompt, nil)
	result := resultAny.([]openai.ChatCompletionMessage)

	if truncated {
		t.Error("StrategyNone should not truncate")
	}

	if len(result) != len(messages) {
		t.Errorf("Message count = %d, want %d", len(result), len(messages))
	}
}

func TestToolPairRemoval(t *testing.T) {
	ClearTokenCache()

	cm := &openAIContext{
		commonContext: commonContext{
			maxInputTokens: 40,
			strategy:       StrategyTruncateOldest,
		},
	}

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "What is the weather?"},
		{
			Role: openai.ChatMessageRoleAssistant,
			ToolCalls: []openai.ToolCall{
				{ID: "call_123", Function: openai.FunctionCall{Name: "get_weather", Arguments: `{"city":"Tokyo"}`}},
			},
		},
		{
			Role:       openai.ChatMessageRoleTool,
			ToolCallID: "call_123",
			Content:    "The weather in Tokyo is sunny.",
		},
		{Role: openai.ChatMessageRoleAssistant, Content: "The weather in Tokyo is sunny!"},
		{Role: openai.ChatMessageRoleUser, Content: "Thanks! What about tomorrow?"},
	}

	resultAny, truncated, _ := cm.PruneMessages(messages, "", nil)
	result := resultAny.([]openai.ChatCompletionMessage)

	if !truncated {
		t.Error("Expected truncation to occur")
	}

	toolCallIDs := make(map[string]bool)
	toolResponseIDs := make(map[string]bool)

	for _, msg := range result {
		for _, call := range msg.ToolCalls {
			toolCallIDs[call.ID] = true
		}
		if msg.ToolCallID != "" {
			toolResponseIDs[msg.ToolCallID] = true
		}
	}

	for id := range toolCallIDs {
		if !toolResponseIDs[id] {
			t.Errorf("Tool call %s exists but its response was removed", id)
		}
	}
	for id := range toolResponseIDs {
		if !toolCallIDs[id] {
			t.Errorf("Tool response %s exists but its call was removed", id)
		}
	}
}

func TestOpenAISystemPromptAccounting(t *testing.T) {
	ClearTokenCache()

	// Set a very tight limit
	cm := &openAIContext{
		commonContext: commonContext{
			maxInputTokens: 50,
			strategy:       StrategyTruncateOldest,
		},
	}

	// Long system prompt should consume most of the budget
	systemPrompt := "This is a very long system prompt that specifically designed to take up a significant portion of the token budget. We need to make sure that it is long enough so that when combined with the dialogue messages, it exceeds the tight limit of fifty tokens that we have set for this test case. By doing this, we can verify that the context manager correctly accounts for the system prompt overhead even when it is not part of the dialogue history slice itself. This is critical for our new architectural design where system prompts are handled out-of-band."
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "Some dialogue message"},
		{Role: openai.ChatMessageRoleAssistant, Content: "Some dialogue response"},
	}

	resultAny, truncated, _ := cm.PruneMessages(messages, systemPrompt, nil)
	result := resultAny.([]openai.ChatCompletionMessage)

	if !truncated {
		t.Error("Expected truncation due to system prompt overhead")
	}

	if len(result) >= len(messages) {
		t.Errorf("Expected fewer messages, got %d", len(result))
	}
}

// OpenChat Context Tests

func strPtr(s string) *string {
	return &s
}

func TestPruneOpenChatMessagesNoTruncation(t *testing.T) {
	ClearTokenCache()

	cm := &openChatContext{
		commonContext: commonContext{
			maxInputTokens: 100000,
			strategy:       StrategyTruncateOldest,
		},
	}

	systemPrompt := "You are a helpful assistant."
	messages := []*model.ChatCompletionMessage{
		{
			Role: model.ChatMessageRoleUser,
			Content: &model.ChatCompletionMessageContent{
				StringValue: strPtr("Hello"),
			},
		},
		{
			Role: model.ChatMessageRoleAssistant,
			Content: &model.ChatCompletionMessageContent{
				StringValue: strPtr("Hi there!"),
			},
		},
	}

	resultAny, truncated, _ := cm.PruneMessages(messages, systemPrompt, nil)
	result := resultAny.([]*model.ChatCompletionMessage)

	if truncated {
		t.Error("Expected no truncation for small messages")
	}

	if len(result) != len(messages) {
		t.Errorf("Message count = %d, want %d", len(result), len(messages))
	}
}

func TestPruneOpenChatMessagesWithTruncation(t *testing.T) {
	ClearTokenCache()

	cm := &openChatContext{
		commonContext: commonContext{
			maxInputTokens: 50,
			strategy:       StrategyTruncateOldest,
		},
	}

	systemPrompt := "You are a helpful assistant."
	messages := []*model.ChatCompletionMessage{
		{
			Role: model.ChatMessageRoleUser,
			Content: &model.ChatCompletionMessageContent{
				StringValue: strPtr("This is message one with some content."),
			},
		},
		{
			Role: model.ChatMessageRoleAssistant,
			Content: &model.ChatCompletionMessageContent{
				StringValue: strPtr("This is response one with some content."),
			},
		},
		{
			Role: model.ChatMessageRoleUser,
			Content: &model.ChatCompletionMessageContent{
				StringValue: strPtr("This is message two with some content."),
			},
		},
		{
			Role: model.ChatMessageRoleAssistant,
			Content: &model.ChatCompletionMessageContent{
				StringValue: strPtr("This is response two with some content."),
			},
		},
	}

	resultAny, truncated, _ := cm.PruneMessages(messages, systemPrompt, nil)
	result := resultAny.([]*model.ChatCompletionMessage)

	if !truncated {
		t.Error("Expected truncation for messages exceeding limit")
	}

	for _, msg := range result {
		if msg.Role == model.ChatMessageRoleSystem {
			t.Error("System message should not be in the pruned dialogue result")
		}
	}
}

func TestOpenChatToolPairRemoval(t *testing.T) {
	ClearTokenCache()

	cm := &openChatContext{
		commonContext: commonContext{
			maxInputTokens: 40,
			strategy:       StrategyTruncateOldest,
		},
	}

	messages := []*model.ChatCompletionMessage{
		{
			Role: model.ChatMessageRoleUser,
			Content: &model.ChatCompletionMessageContent{
				StringValue: strPtr("What is the weather?"),
			},
		},
		{
			Role: model.ChatMessageRoleAssistant,
			ToolCalls: []*model.ToolCall{
				{
					ID:   "call_456",
					Type: "function",
					Function: model.FunctionCall{
						Name:      "get_weather",
						Arguments: `{"city":"Tokyo"}`,
					},
				},
			},
		},
		{
			Role:       model.ChatMessageRoleTool,
			ToolCallID: "call_456",
			Content: &model.ChatCompletionMessageContent{
				StringValue: strPtr("The weather in Tokyo is sunny."),
			},
		},
		{
			Role: model.ChatMessageRoleAssistant,
			Content: &model.ChatCompletionMessageContent{
				StringValue: strPtr("The weather in Tokyo is sunny!"),
			},
		},
		{
			Role: model.ChatMessageRoleUser,
			Content: &model.ChatCompletionMessageContent{
				StringValue: strPtr("Thanks! What about tomorrow?"),
			},
		},
	}

	resultAny, truncated, _ := cm.PruneMessages(messages, nil)
	result := resultAny.([]*model.ChatCompletionMessage)

	if !truncated {
		t.Error("Expected truncation to occur")
	}

	toolCallIDs := make(map[string]bool)
	toolResponseIDs := make(map[string]bool)

	for _, msg := range result {
		for _, call := range msg.ToolCalls {
			toolCallIDs[call.ID] = true
		}
		if msg.ToolCallID != "" {
			toolResponseIDs[msg.ToolCallID] = true
		}
	}

	for id := range toolCallIDs {
		if !toolResponseIDs[id] {
			t.Errorf("OpenChat: Tool call %s exists but its response was removed", id)
		}
	}
	for id := range toolResponseIDs {
		if !toolCallIDs[id] {
			t.Errorf("OpenChat: Tool response %s exists but its call was removed", id)
		}
	}
}

func TestOpenChatSystemPromptAccounting(t *testing.T) {
	ClearTokenCache()

	cm := &openChatContext{
		commonContext: commonContext{
			maxInputTokens: 50,
			strategy:       StrategyTruncateOldest,
		},
	}

	systemPrompt := "This is a very long system prompt specifically designed to take up a significant portion of the token budget. We need to make sure that it is long enough so that when combined with the dialogue messages, it exceeds the tight limit of fifty tokens that we have set for this test case. By doing this, we can verify that the context manager correctly accounts for the system prompt overhead even when it is not part of the dialogue history slice itself."
	messages := []*model.ChatCompletionMessage{
		{
			Role: model.ChatMessageRoleUser,
			Content: &model.ChatCompletionMessageContent{
				StringValue: strPtr("User message"),
			},
		},
		{
			Role: model.ChatMessageRoleAssistant,
			Content: &model.ChatCompletionMessageContent{
				StringValue: strPtr("Assistant response"),
			},
		},
	}

	resultAny, truncated, _ := cm.PruneMessages(messages, systemPrompt, nil)
	result := resultAny.([]*model.ChatCompletionMessage)

	if !truncated {
		t.Error("Expected truncation due to system prompt overhead")
	}

	if len(result) >= len(messages) {
		t.Errorf("Expected fewer than %d messages, got %d", len(messages), len(result))
	}
}

// Gemini Context Tests

func TestPruneGeminiMessages(t *testing.T) {
	ClearTokenCache()
	cm := &geminiContext{
		commonContext: commonContext{
			maxInputTokens: 200,
			strategy:       StrategyTruncateOldest,
		},
	}

	messages := []*genai.Content{
		{Parts: []*genai.Part{{Text: "This is message 1"}}},
		{Parts: []*genai.Part{{Text: "This is message 2"}}},
		{Parts: []*genai.Part{{Text: "This is message 3"}}},
		{Parts: []*genai.Part{{Text: "This is message 4"}}},
		{Parts: []*genai.Part{{Text: "This is message 5"}}},
	}

	resultAny, truncated, _ := cm.PruneMessages(messages, "System Prompt", nil)
	result := resultAny.([]*genai.Content)
	if truncated {
		t.Errorf("Expected no truncation, but got truncated")
	}
	if len(result) != len(messages) {
		t.Errorf("Expected all messages, got %d", len(result))
	}

	cm.maxInputTokens = 30
	resultAny, truncated, _ = cm.PruneMessages(messages, "System Prompt", nil)
	result = resultAny.([]*genai.Content)
	if !truncated {
		t.Error("Expected truncation")
	}
	if len(result) >= len(messages) {
		t.Errorf("Expected fewer than %d messages, got %d", len(messages), len(result))
	}
}

func TestGeminiToolPairRemoval(t *testing.T) {
	ClearTokenCache()
	cm := &geminiContext{
		commonContext: commonContext{
			maxInputTokens: 30,
			strategy:       StrategyTruncateOldest,
		},
	}

	messages := []*genai.Content{
		{Parts: []*genai.Part{{Text: "User question"}}},
		{
			Parts: []*genai.Part{
				{FunctionCall: &genai.FunctionCall{Name: "search", Args: map[string]interface{}{"q": "foo"}}},
			},
		},
		{
			Parts: []*genai.Part{
				{FunctionResponse: &genai.FunctionResponse{Name: "search", Response: map[string]interface{}{"res": "bar"}}},
			},
		},
		{Parts: []*genai.Part{{Text: "Final answer"}}},
	}

	resultAny, truncated, _ := cm.PruneMessages(messages, "", nil)
	result := resultAny.([]*genai.Content)

	if !truncated {
		t.Error("Expected truncation")
	}

	hasCall := false
	hasResp := false
	for _, msg := range result {
		for _, p := range msg.Parts {
			if p.FunctionCall != nil {
				hasCall = true
			}
			if p.FunctionResponse != nil {
				hasResp = true
			}
		}
	}

	if hasCall != hasResp {
		t.Error("Tool pair broken: hasCall =", hasCall, " hasResp =", hasResp)
	}
}
