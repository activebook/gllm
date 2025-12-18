package service

import (
	"testing"

	openai "github.com/sashabaranov/go-openai"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
)

func TestNewContextManager(t *testing.T) {
	limits := ModelLimits{ContextWindow: 128000, MaxOutputTokens: 16384}
	cm := NewContextManager(limits, StrategyTruncateOldest)

	if cm.Strategy != StrategyTruncateOldest {
		t.Errorf("Strategy = %v, want %v", cm.Strategy, StrategyTruncateOldest)
	}

	if cm.BufferPercent != DefaultBufferPercent {
		t.Errorf("BufferPercent = %v, want %v", cm.BufferPercent, DefaultBufferPercent)
	}

	// MaxInputTokens should match what MaxInputTokens method returns
	expectedMax := limits.MaxInputTokens(DefaultBufferPercent)
	if cm.MaxInputTokens != expectedMax {
		t.Errorf("MaxInputTokens = %d, want %d", cm.MaxInputTokens, expectedMax)
	}
}

func TestNewContextManagerForModel(t *testing.T) {
	cm := NewContextManagerForModel("gpt-4o", StrategyTruncateOldest)

	// gpt-4o has 128000 context, 16384 output
	limits := GetModelLimits("gpt-4o")
	expectedMax := limits.MaxInputTokens(DefaultBufferPercent)
	if cm.MaxInputTokens != expectedMax {
		t.Errorf("MaxInputTokens for gpt-4o = %d, want %d", cm.MaxInputTokens, expectedMax)
	}
}

func TestPrepareOpenAIMessagesNoTruncation(t *testing.T) {
	// Create a context manager with large limit
	cm := &ContextManager{
		MaxInputTokens: 100000,
		Strategy:       StrategyTruncateOldest,
		BufferPercent:  0.8,
	}

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "You are a helpful assistant."},
		{Role: openai.ChatMessageRoleUser, Content: "Hello"},
		{Role: openai.ChatMessageRoleAssistant, Content: "Hi there!"},
	}

	result, truncated := cm.PrepareOpenAIMessages(messages)

	if truncated {
		t.Error("Expected no truncation for small messages")
	}

	if len(result) != len(messages) {
		t.Errorf("Message count = %d, want %d", len(result), len(messages))
	}
}

func TestPrepareOpenAIMessagesWithTruncation(t *testing.T) {
	// Create a context manager with small limit
	cm := &ContextManager{
		MaxInputTokens: 50, // Very small to force truncation
		Strategy:       StrategyTruncateOldest,
		BufferPercent:  0.8,
	}

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "You are a helpful assistant."},
		{Role: openai.ChatMessageRoleUser, Content: "This is message one with some content."},
		{Role: openai.ChatMessageRoleAssistant, Content: "This is response one with some content."},
		{Role: openai.ChatMessageRoleUser, Content: "This is message two with some content."},
		{Role: openai.ChatMessageRoleAssistant, Content: "This is response two with some content."},
	}

	result, truncated := cm.PrepareOpenAIMessages(messages)

	if !truncated {
		t.Error("Expected truncation for messages exceeding limit")
	} else {
		t.Log("Truncation occurred")
		t.Log(result)
	}

	// Should have fewer messages than original
	if len(result) >= len(messages) {
		t.Errorf("Expected fewer messages after truncation, got %d (original %d)",
			len(result), len(messages))
	}

	// System message should be preserved
	if len(result) > 0 && result[0].Role != openai.ChatMessageRoleSystem {
		t.Error("System message should be preserved at index 0")
	}
}

func TestPrepareOpenAIMessagesWithStrategyNone(t *testing.T) {
	cm := &ContextManager{
		MaxInputTokens: 10, // Very small
		Strategy:       StrategyNone,
		BufferPercent:  0.8,
	}

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "You are a helpful assistant with a long description."},
		{Role: openai.ChatMessageRoleUser, Content: "Hello, this is a relatively long message."},
	}

	result, truncated := cm.PrepareOpenAIMessages(messages)

	// With StrategyNone, no truncation should occur
	if truncated {
		t.Error("StrategyNone should not truncate")
	}

	if len(result) != len(messages) {
		t.Errorf("Message count = %d, want %d", len(result), len(messages))
	}
}

func TestToolPairRemoval(t *testing.T) {
	// Clear cache before test
	ClearTokenCache()

	// Create a context manager with small limit to force truncation
	cm := &ContextManager{
		MaxInputTokens: 100, // Very small to force removal
		Strategy:       StrategyTruncateOldest,
		BufferPercent:  0.8,
	}

	// Create messages with a tool call/response pair
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

	result, truncated := cm.PrepareOpenAIMessages(messages)

	if !truncated {
		t.Error("Expected truncation to occur")
	} else {
		t.Log("Truncation occurred")
		t.Log(result)
	}

	// Verify tool pair consistency: if tool call exists, its response must exist too
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

	// Every tool call must have its response, and vice versa
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

func TestPreserveSystemMessage(t *testing.T) {
	cm := &ContextManager{
		MaxInputTokens: 30, // Force truncation
		Strategy:       StrategyTruncateOldest,
		BufferPercent:  0.8,
	}

	systemContent := "Important system instructions"
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: systemContent},
		{Role: openai.ChatMessageRoleUser, Content: "Message 1"},
		{Role: openai.ChatMessageRoleUser, Content: "Message 2"},
		{Role: openai.ChatMessageRoleUser, Content: "Message 3"},
	}

	result, _ := cm.PrepareOpenAIMessages(messages)

	// System message should always be first
	if len(result) == 0 {
		t.Fatal("Result should not be empty")
	}

	if result[0].Role != openai.ChatMessageRoleSystem {
		t.Error("First message should be system message")
	}

	if result[0].Content != systemContent {
		t.Errorf("System content = %q, want %q", result[0].Content, systemContent)
	}
}

// =============================================================================
// OpenChat Context Tests
// =============================================================================

// Helper to create OpenChat string pointer
func strPtr(s string) *string {
	return &s
}

func TestPrepareOpenChatMessagesNoTruncation(t *testing.T) {
	ClearTokenCache()

	cm := &ContextManager{
		MaxInputTokens: 100000,
		Strategy:       StrategyTruncateOldest,
		BufferPercent:  0.8,
	}

	messages := []*model.ChatCompletionMessage{
		{
			Role: model.ChatMessageRoleSystem,
			Content: &model.ChatCompletionMessageContent{
				StringValue: strPtr("You are a helpful assistant."),
			},
		},
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

	result, truncated := cm.PrepareOpenChatMessages(messages)

	if truncated {
		t.Error("Expected no truncation for small messages")
	}

	if len(result) != len(messages) {
		t.Errorf("Message count = %d, want %d", len(result), len(messages))
	}
}

func TestPrepareOpenChatMessagesWithTruncation(t *testing.T) {
	ClearTokenCache()

	cm := &ContextManager{
		MaxInputTokens: 50, // Very small to force truncation
		Strategy:       StrategyTruncateOldest,
		BufferPercent:  0.8,
	}

	messages := []*model.ChatCompletionMessage{
		{
			Role: model.ChatMessageRoleSystem,
			Content: &model.ChatCompletionMessageContent{
				StringValue: strPtr("You are a helpful assistant."),
			},
		},
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

	result, truncated := cm.PrepareOpenChatMessages(messages)

	if !truncated {
		t.Error("Expected truncation for messages exceeding limit")
	} else {
		t.Log("OpenChat truncation occurred")
		t.Logf("Result has %d messages (original %d)", len(result), len(messages))
	}

	// Should have fewer messages than original
	if len(result) >= len(messages) {
		t.Errorf("Expected fewer messages after truncation, got %d (original %d)",
			len(result), len(messages))
	}

	// System message should be preserved
	if len(result) > 0 && result[0].Role != model.ChatMessageRoleSystem {
		t.Error("System message should be preserved at index 0")
	}
}

func TestOpenChatToolPairRemoval(t *testing.T) {
	ClearTokenCache()

	cm := &ContextManager{
		MaxInputTokens: 100, // Very small to force removal
		Strategy:       StrategyTruncateOldest,
		BufferPercent:  0.8,
	}

	// Create messages with a tool call/response pair
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

	result, truncated := cm.PrepareOpenChatMessages(messages)

	if !truncated {
		t.Error("Expected truncation to occur")
	} else {
		t.Log("OpenChat tool pair truncation occurred")
		t.Logf("Result has %d messages", len(result))
	}

	// Verify tool pair consistency
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

	// Every tool call must have its response, and vice versa
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

func TestOpenChatPreserveSystemMessage(t *testing.T) {
	ClearTokenCache()

	cm := &ContextManager{
		MaxInputTokens: 30, // Force truncation
		Strategy:       StrategyTruncateOldest,
		BufferPercent:  0.8,
	}

	systemContent := "Important system instructions"
	messages := []*model.ChatCompletionMessage{
		{
			Role: model.ChatMessageRoleSystem,
			Content: &model.ChatCompletionMessageContent{
				StringValue: strPtr(systemContent),
			},
		},
		{
			Role: model.ChatMessageRoleUser,
			Content: &model.ChatCompletionMessageContent{
				StringValue: strPtr("Message 1"),
			},
		},
		{
			Role: model.ChatMessageRoleUser,
			Content: &model.ChatCompletionMessageContent{
				StringValue: strPtr("Message 2"),
			},
		},
		{
			Role: model.ChatMessageRoleUser,
			Content: &model.ChatCompletionMessageContent{
				StringValue: strPtr("Message 3"),
			},
		},
	}

	result, _ := cm.PrepareOpenChatMessages(messages)

	// System message should always be first
	if len(result) == 0 {
		t.Fatal("Result should not be empty")
	}

	if result[0].Role != model.ChatMessageRoleSystem {
		t.Error("First message should be system message")
	}

	if result[0].Content != nil && result[0].Content.StringValue != nil {
		if *result[0].Content.StringValue != systemContent {
			t.Errorf("System content = %q, want %q", *result[0].Content.StringValue, systemContent)
		}
	} else {
		t.Error("System message content should not be nil")
	}
}
