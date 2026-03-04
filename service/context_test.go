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

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "You are a helpful assistant."},
		{Role: openai.ChatMessageRoleUser, Content: "Hello"},
		{Role: openai.ChatMessageRoleAssistant, Content: "Hi there!"},
	}

	resultAny, truncated, _ := cm.PruneMessages(messages, nil)
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
			maxInputTokens: 50,
			strategy:       StrategyTruncateOldest,
		},
	}

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "You are a helpful assistant."},
		{Role: openai.ChatMessageRoleUser, Content: "This is message one with some content."},
		{Role: openai.ChatMessageRoleAssistant, Content: "This is response one with some content."},
		{Role: openai.ChatMessageRoleUser, Content: "This is message two with some content."},
		{Role: openai.ChatMessageRoleAssistant, Content: "This is response two with some content."},
	}

	resultAny, truncated, _ := cm.PruneMessages(messages, nil)
	result := resultAny.([]openai.ChatCompletionMessage)

	if !truncated {
		t.Error("Expected truncation for messages exceeding limit")
	}

	if len(result) >= len(messages) {
		t.Errorf("Expected fewer messages after truncation, got %d (original %d)",
			len(result), len(messages))
	}

	if len(result) > 0 && result[0].Role != openai.ChatMessageRoleSystem {
		t.Error("System message should be preserved at index 0")
	}
}

func TestPruneOpenAIMessagesWithStrategyNone(t *testing.T) {
	cm := &openAIContext{
		commonContext: commonContext{
			maxInputTokens: 10,
			strategy:       StrategyNone,
		},
	}

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "You are a helpful assistant with a long description."},
		{Role: openai.ChatMessageRoleUser, Content: "Hello, this is a relatively long message."},
	}

	resultAny, truncated, _ := cm.PruneMessages(messages, nil)
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

	resultAny, truncated, _ := cm.PruneMessages(messages, nil)
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

func TestPreserveMultiSystemMessages(t *testing.T) {
	ClearTokenCache()

	cm := &openAIContext{
		commonContext: commonContext{
			maxInputTokens: 40,
			strategy:       StrategyTruncateOldest,
		},
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

	resultAny, truncated, _ := cm.PruneMessages(messages, nil)
	result := resultAny.([]openai.ChatCompletionMessage)

	if !truncated {
		t.Error("Expected truncation")
	}

	systemCount := 0
	for _, msg := range result {
		if msg.Role == openai.ChatMessageRoleSystem {
			systemCount++
		}
	}
	if systemCount != 1 {
		t.Errorf("Expected exactly 1 system message, got %d", systemCount)
	}

	if result[0].Role != openai.ChatMessageRoleSystem {
		t.Error("First message should be system message")
	}

	expectedContent := sys1 + "\n" + sys2 + "\n" + sys3
	if result[0].Content != expectedContent {
		t.Errorf("System content mismatch.\nGot: %q\nWant: %q", result[0].Content, expectedContent)
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

	resultAny, truncated, _ := cm.PruneMessages(messages, nil)
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

	resultAny, truncated, _ := cm.PruneMessages(messages, nil)
	result := resultAny.([]*model.ChatCompletionMessage)

	if !truncated {
		t.Error("Expected truncation for messages exceeding limit")
	}

	if len(result) >= len(messages) {
		t.Errorf("Expected fewer messages after truncation, got %d (original %d)",
			len(result), len(messages))
	}

	if len(result) > 0 && result[0].Role != model.ChatMessageRoleSystem {
		t.Error("System message should be preserved at index 0")
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

func TestOpenChatPreserveMultiSystemMessages(t *testing.T) {
	ClearTokenCache()

	cm := &openChatContext{
		commonContext: commonContext{
			maxInputTokens: 20,
			strategy:       StrategyTruncateOldest,
		},
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

	resultAny, _, _ := cm.PruneMessages(messages, nil)
	result := resultAny.([]*model.ChatCompletionMessage)

	systemCount := 0
	for _, msg := range result {
		if msg.Role == model.ChatMessageRoleSystem {
			systemCount++
		}
	}
	if systemCount != 1 {
		t.Errorf("Expected exactly 1 system message, got %d", systemCount)
	}

	if result[0].Role != model.ChatMessageRoleSystem {
		t.Error("First message should be system message")
	}

	expectedContent := sys1 + "\n" + sys2 + "\n" + sys3
	if *result[0].Content.StringValue != expectedContent {
		t.Errorf("System content mismatch.\nGot: %q\nWant: %q", *result[0].Content.StringValue, expectedContent)
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
