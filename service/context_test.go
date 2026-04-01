package service

import (
	"encoding/json"
	"testing"

	openai "github.com/openai/openai-go/v3"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"google.golang.org/genai"
)

func TestNewContextManager(t *testing.T) {
	limits := ModelLimits{ContextWindow: 128000, MaxOutputTokens: 16384}
	expectedMax := limits.MaxInputTokens(DefaultBufferPercent)
	ag := &Agent{Model: &ModelInfo{Model: "gpt-4o", Provider: ModelProviderOpenAI}}
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
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Hello"),
		openai.AssistantMessage("Hi there!"),
	}

	resultAny, truncated, _ := cm.PruneMessages(messages, systemPrompt, nil)
	result := resultAny.([]openai.ChatCompletionMessageParamUnion)

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
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("This is message one with some content."),
		openai.AssistantMessage("This is response one with some content."),
		openai.UserMessage("This is message two with some content."),
		openai.AssistantMessage("This is response two with some content."),
	}

	resultAny, truncated, _ := cm.PruneMessages(messages, systemPrompt, nil)
	result := resultAny.([]openai.ChatCompletionMessageParamUnion)

	if !truncated {
		t.Error("Expected truncation for messages exceeding limit")
	}

	if len(result) >= len(messages) {
		t.Errorf("Expected fewer messages after truncation, got %d (original %d)",
			len(result), len(messages))
	}

	for _, msg := range result {
		rolePtr := msg.GetRole()
		if rolePtr != nil && *rolePtr == "system" {
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
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Hello, this is a relatively long message."),
	}

	resultAny, truncated, _ := cm.PruneMessages(messages, systemPrompt, nil)
	result := resultAny.([]openai.ChatCompletionMessageParamUnion)

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

	var asstMsg, toolMsg openai.ChatCompletionMessageParamUnion
	json.Unmarshal([]byte(`{"role":"assistant","tool_calls":[{"id":"call_123","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"Tokyo\"}"}}]}`), &asstMsg)
	json.Unmarshal([]byte(`{"role":"tool","tool_call_id":"call_123","content":"The weather in Tokyo is sunny."}`), &toolMsg)

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("What is the weather?"),
		asstMsg,
		toolMsg,
		openai.AssistantMessage("The weather in Tokyo is sunny!"),
		openai.UserMessage("Thanks! What about tomorrow?"),
	}

	resultAny, truncated, _ := cm.PruneMessages(messages, "", nil)
	result := resultAny.([]openai.ChatCompletionMessageParamUnion)

	if !truncated {
		t.Error("Expected truncation to occur")
	}

	for _, msg := range result {
		// Log the result types for manual verification
		if msg.OfAssistant != nil {
			t.Log("Assistant message present")
		}
		if msg.OfTool != nil {
			t.Logf("Tool message present: %v", msg.OfTool.ToolCallID)
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
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("Some dialogue message"),
		openai.AssistantMessage("Some dialogue response"),
	}

	resultAny, truncated, _ := cm.PruneMessages(messages, systemPrompt, nil)
	result := resultAny.([]openai.ChatCompletionMessageParamUnion)

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

// TestPruneGeminiMessagesFunctionCallAfterUser reproduces the exact Gemini 400 error:
// "function call turn comes immediately after a user turn or after a function response turn".
// Scenario: after truncation the first remaining message is a Model/FunctionCall turn,
// which is illegal because it is not preceded by a User or FunctionResponse turn.
func TestPruneGeminiMessagesFunctionCallAfterUser(t *testing.T) {
	ClearTokenCache()
	cm := &geminiContext{
		commonContext: commonContext{
			maxInputTokens: 20, // tight limit forces truncation
			strategy:       StrategyTruncateOldest,
		},
	}

	// Sequence:
	//   [0] User  "old question"
	//   [1] Model FunctionCall(search)
	//   [2] User  FunctionResponse(search)
	//   [3] Model "old answer"
	//   [4] User  "new question"
	//   [5] Model FunctionCall(search2)  <-- if [0-3] are dropped individually,
	//                                        this can end up at index 0 (illegal)
	//   [6] User  FunctionResponse(search2)
	//   [7] Model "new answer"
	messages := []*genai.Content{
		{Role: genai.RoleUser, Parts: []*genai.Part{{Text: "old question"}}},
		{
			Role:  genai.RoleModel,
			Parts: []*genai.Part{{FunctionCall: &genai.FunctionCall{Name: "search", Args: map[string]interface{}{"q": "x"}}}},
		},
		{
			Role:  genai.RoleUser,
			Parts: []*genai.Part{{FunctionResponse: &genai.FunctionResponse{Name: "search", Response: map[string]interface{}{"r": "y"}}}},
		},
		{Role: genai.RoleModel, Parts: []*genai.Part{{Text: "old answer"}}},
		{Role: genai.RoleUser, Parts: []*genai.Part{{Text: "new question"}}},
		{
			Role:  genai.RoleModel,
			Parts: []*genai.Part{{FunctionCall: &genai.FunctionCall{Name: "search2", Args: map[string]interface{}{"q": "z"}}}},
		},
		{
			Role:  genai.RoleUser,
			Parts: []*genai.Part{{FunctionResponse: &genai.FunctionResponse{Name: "search2", Response: map[string]interface{}{"r": "w"}}}},
		},
		{Role: genai.RoleModel, Parts: []*genai.Part{{Text: "new answer"}}},
	}

	resultAny, truncated, _ := cm.PruneMessages(messages, "", nil)
	result := resultAny.([]*genai.Content)

	if !truncated {
		t.Fatal("Expected truncation")
	}

	// Validate the full sequence satisfies Gemini's constraints:
	// 1. First message must be User (not Model, not FunctionResponse).
	// 2. A FunctionCall (Model turn) must be preceded by User or FunctionResponse.
	if len(result) == 0 {
		return // empty is technically valid (nothing to send)
	}
	if result[0].Role != genai.RoleUser {
		t.Errorf("first message role = %q, want %q", result[0].Role, genai.RoleUser)
	}
	for _, part := range result[0].Parts {
		if part.FunctionResponse != nil {
			t.Error("first message must not be a FunctionResponse")
		}
	}
	for i := 1; i < len(result); i++ {
		if result[i].Role != genai.RoleModel {
			continue
		}
		for _, part := range result[i].Parts {
			if part.FunctionCall == nil {
				continue
			}
			// FunctionCall must be preceded by User or FunctionResponse
			prev := result[i-1]
			prevIsUser := prev.Role == genai.RoleUser
			prevIsFuncResp := false
			for _, pp := range prev.Parts {
				if pp.FunctionResponse != nil {
					prevIsFuncResp = true
				}
			}
			if !prevIsUser && !prevIsFuncResp {
				t.Errorf("message[%d] is a FunctionCall but message[%d] (role=%q) is neither User nor FunctionResponse", i, i-1, prev.Role)
			}
		}
	}
}

func TestPruneGeminiMessagesRoleSequence(t *testing.T) {
	ClearTokenCache()
	cm := &geminiContext{
		commonContext: commonContext{
			maxInputTokens: 20, // Smaller limit to trigger truncation
			strategy:       StrategyTruncateOldest,
		},
	}

	messages := []*genai.Content{
		{Role: genai.RoleUser, Parts: []*genai.Part{{Text: "Message 1 (User)"}}},
		{Role: genai.RoleModel, Parts: []*genai.Part{{Text: "Message 2 (Model)"}}},
		{Role: genai.RoleUser, Parts: []*genai.Part{{Text: "Message 3 (User)"}}},
		{Role: genai.RoleModel, Parts: []*genai.Part{{Text: "Message 4 (Model)"}}},
	}

	// Truncation will remove "Message 1 (User)" first.
	// We want to ensure it doesn't leave "Message 2 (Model)" as the first message.
	resultAny, truncated, _ := cm.PruneMessages(messages, "", nil)
	result := resultAny.([]*genai.Content)

	if !truncated {
		t.Error("Expected truncation")
	}

	if len(result) > 0 {
		if result[0].Role != genai.RoleUser {
			t.Errorf("First message role = %v, want %v", result[0].Role, genai.RoleUser)
		}
	}
}

func TestPruneGeminiMessagesToolResponseStart(t *testing.T) {
	ClearTokenCache()
	cm := &geminiContext{
		commonContext: commonContext{
			maxInputTokens: 20,
			strategy:       StrategyTruncateOldest,
		},
	}

	messages := []*genai.Content{
		{Role: genai.RoleUser, Parts: []*genai.Part{{Text: "User question"}}},
		{
			Role: genai.RoleModel,
			Parts: []*genai.Part{
				{FunctionCall: &genai.FunctionCall{Name: "search", Args: map[string]interface{}{"q": "foo"}}},
			},
		},
		{
			Role: genai.RoleUser,
			Parts: []*genai.Part{
				{FunctionResponse: &genai.FunctionResponse{Name: "search", Response: map[string]interface{}{"res": "bar"}}},
			},
		},
		{Role: genai.RoleModel, Parts: []*genai.Part{{Text: "Final answer"}}},
	}

	// If truncation removes first message, it should also remove the tool pair if the start is invalid.
	// Actually, if it removes User, it must remove Model (Call).
	// Then it's left with User (Response). But Response must follow a Call.
	// So it must remove User (Response) too.
	// And then it's left with Model (Final answer). So it must remove that too.
	resultAny, truncated, _ := cm.PruneMessages(messages, "", nil)
	result := resultAny.([]*genai.Content)

	if !truncated {
		t.Error("Expected truncation")
	}

	if len(result) > 0 {
		if result[0].Role != genai.RoleUser {
			t.Errorf("First message role = %v, want %v", result[0].Role, genai.RoleUser)
		}
		for _, part := range result[0].Parts {
			if part.FunctionResponse != nil {
				t.Error("First message should not be a tool response")
			}
		}
	}
}
