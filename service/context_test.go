package service

import (
	"testing"

	openai "github.com/sashabaranov/go-openai"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"google.golang.org/genai"
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
		MaxInputTokens: 40, // Reduced to force removal
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

func TestPreserveMultiSystemMessages(t *testing.T) {
	// Clear cache
	ClearTokenCache()

	cm := &ContextManager{
		MaxInputTokens: 40, // Force truncation (reduced from 60)
		Strategy:       StrategyTruncateOldest,
		BufferPercent:  0.8,
	}

	sys1 := "System message 1 (Identity)"
	sys2 := "System message 2 (Intermediate)"
	sys3 := "System message 3 (Current Task)"

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: sys1},
		{Role: openai.ChatMessageRoleUser, Content: "Msg A"},
		{Role: openai.ChatMessageRoleAssistant, Content: "Msg B"},
		{Role: openai.ChatMessageRoleSystem, Content: sys2},
		{Role: openai.ChatMessageRoleUser, Content: "Msg C"},
		{Role: openai.ChatMessageRoleAssistant, Content: "Msg D"},
		{Role: openai.ChatMessageRoleSystem, Content: sys3},
		{Role: openai.ChatMessageRoleUser, Content: "Msg E"},
	}

	// Should preserve all system messages by merging them into one
	// Msg A, B, C, D likely dropped to fit 40 tokens
	result, truncated := cm.PrepareOpenAIMessages(messages)

	if !truncated {
		t.Error("Expected truncation")
	}

	// Should have exactly 1 system message now (consolidated)
	systemCount := 0
	for _, msg := range result {
		if msg.Role == openai.ChatMessageRoleSystem {
			systemCount++
		}
	}
	if systemCount != 1 {
		t.Errorf("Expected exactly 1 system message, got %d", systemCount)
	}

	// First message must be the consolidated system message
	if result[0].Role != openai.ChatMessageRoleSystem {
		t.Error("First message should be system message")
	}

	// Content should contain all 3 system messages
	expectedContent := sys1 + "\n" + sys2 + "\n" + sys3
	if result[0].Content != expectedContent {
		t.Errorf("System content mismatch.\nGot: %q\nWant: %q", result[0].Content, expectedContent)
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
		MaxInputTokens: 40, // Reduced to force removal
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

func TestOpenChatPreserveMultiSystemMessages(t *testing.T) {
	ClearTokenCache()

	cm := &ContextManager{
		MaxInputTokens: 20, // Force truncation (reduced to ensure trigger)
		Strategy:       StrategyTruncateOldest,
		BufferPercent:  0.8,
	}

	sys1 := "System message 1 (Identity)"
	sys2 := "System message 2 (Intermediate)"
	sys3 := "System message 3 (Current Task)"

	messages := []*model.ChatCompletionMessage{
		{Role: model.ChatMessageRoleSystem, Content: &model.ChatCompletionMessageContent{StringValue: strPtr(sys1)}},
		{Role: model.ChatMessageRoleUser, Content: &model.ChatCompletionMessageContent{StringValue: strPtr("Msg A")}},
		{Role: model.ChatMessageRoleSystem, Content: &model.ChatCompletionMessageContent{StringValue: strPtr(sys2)}},
		{Role: model.ChatMessageRoleUser, Content: &model.ChatCompletionMessageContent{StringValue: strPtr("Msg B")}},
		{Role: model.ChatMessageRoleSystem, Content: &model.ChatCompletionMessageContent{StringValue: strPtr(sys3)}},
		{Role: model.ChatMessageRoleUser, Content: &model.ChatCompletionMessageContent{StringValue: strPtr("Msg C")}},
	}

	// Should preserve all system messages by merging them into one
	result, _ := cm.PrepareOpenChatMessages(messages)

	// Should have exactly 1 system message now (consolidated)
	systemCount := 0
	for _, msg := range result {
		if msg.Role == model.ChatMessageRoleSystem {
			systemCount++
		}
	}
	if systemCount != 1 {
		t.Errorf("Expected exactly 1 system message, got %d", systemCount)
	}

	// First message must be the consolidated system message
	if result[0].Role != model.ChatMessageRoleSystem {
		t.Error("First message should be system message")
	}

	// Content should contain all 3 system messages
	expectedContent := sys1 + "\n" + sys2 + "\n" + sys3
	if *result[0].Content.StringValue != expectedContent {
		t.Errorf("System content mismatch.\nGot: %q\nWant: %q", *result[0].Content.StringValue, expectedContent)
	}
}

// =============================================================================
// Gemini Context Tests
// =============================================================================

func TestPrepareGeminiMessages(t *testing.T) {
	ClearTokenCache()
	cm := &ContextManager{
		MaxInputTokens: 50,
		Strategy:       StrategyTruncateOldest,
		BufferPercent:  0.8,
	}

	messages := []*genai.Content{
		{Parts: []*genai.Part{{Text: "This is message 1 with enough content to consume tokens"}}},
		{Parts: []*genai.Part{{Text: "This is message 2 with enough content to consume tokens"}}},
		{Parts: []*genai.Part{{Text: "This is message 3 with enough content to consume tokens"}}},
		{Parts: []*genai.Part{{Text: "This is message 4 with enough content to consume tokens"}}},
		{Parts: []*genai.Part{{Text: "This is message 5 with enough content to consume tokens"}}},
	}

	result, truncated := cm.PrepareGeminiMessages(messages, "System Prompt")

	if !truncated {
		t.Error("Expected truncation")
	}
	if len(result) >= len(messages) {
		t.Errorf("Expected fewer messages, got %d", len(result))
	}
}

func TestGeminiToolPairRemoval(t *testing.T) {
	ClearTokenCache()
	cm := &ContextManager{
		MaxInputTokens: 30, // Force truncation
		Strategy:       StrategyTruncateOldest,
		BufferPercent:  0.8,
	}

	messages := []*genai.Content{
		{Parts: []*genai.Part{{Text: "User question"}}}, // Should be removed
		{ // Tool Call
			Parts: []*genai.Part{
				{FunctionCall: &genai.FunctionCall{Name: "search", Args: map[string]interface{}{"q": "foo"}}},
			},
		},
		{ // Tool Response
			Parts: []*genai.Part{
				{FunctionResponse: &genai.FunctionResponse{Name: "search", Response: map[string]interface{}{"res": "bar"}}},
			},
		},
		{Parts: []*genai.Part{{Text: "Final answer"}}},
	}

	result, truncated := cm.PrepareGeminiMessages(messages, "")

	if !truncated {
		t.Error("Expected truncation")
	}

	// Because tokens are low, "User question" should be removed.
	// The tool pair occupies tokens. If we remove tool pair, we lose context.
	// But "User question" is oldest.
	// Let's see if we retain the pair.
	// Assuming "Final answer" + Pair fits? Maybe not.
	// If User question removed -> fine.
	// If Pair removed -> Both must go.

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
