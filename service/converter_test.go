package service

import (
	"testing"
)

/*
The `-run` flag in `go test` accepts a __regular expression__ (not just wildcards), so:

```bash
go test ./service -run TestConvertMessages
```

This matches all test function names that contain "TestConvertMessages"
*/

func TestConvertMessages_OpenAIToGemini(t *testing.T) {
	// Input: Simple OpenAI conversation (JSONL format)
	input := `{"role": "system", "content": "You are a helpful assistant."}
{"role": "user", "content": "Hello!"}
{"role": "assistant", "content": "Hi there!"}`

	result, err := ConvertMessages([]byte(input), ModelProviderOpenAI, ModelProviderGemini)
	if err != nil {
		t.Fatalf("ConvertMessages failed: %v", err)
	}

	// Verify result is valid JSONL
	var geminiMessages []map[string]interface{}
	if err := parseJSONL(result, &geminiMessages); err != nil {
		t.Fatalf("Result is not valid JSONL: %v", err)
	}

	// Should have 2 messages (system is inlined into first user message)
	if len(geminiMessages) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(geminiMessages))
	}

	// First message should be user with inlined system content
	if geminiMessages[0]["role"] != "user" {
		t.Errorf("Expected first message role 'user', got '%v'", geminiMessages[0]["role"])
	}

	// Second message should be user
	if geminiMessages[1]["role"] != "user" {
		t.Errorf("Expected second message role 'user', got '%v'", geminiMessages[1]["role"])
	}

	// Third message should be model
	if geminiMessages[2]["role"] != "model" {
		t.Errorf("Expected third message role 'model', got '%v'", geminiMessages[2]["role"])
	}
}

func TestConvertMessages_GeminiToOpenAI(t *testing.T) {
	// Input: Simple Gemini conversation (JSONL format)
	input := `{"role": "user", "parts": [{"text": "Hello from Gemini!"}]}
{"role": "model", "parts": [{"text": "Greetings!"}]}`

	result, err := ConvertMessages([]byte(input), ModelProviderGemini, ModelProviderOpenAI)
	if err != nil {
		t.Fatalf("ConvertMessages failed: %v", err)
	}

	// Verify result is valid JSONL
	var openaiMessages []map[string]interface{}
	if err := parseJSONL(result, &openaiMessages); err != nil {
		t.Fatalf("Result is not valid JSONL: %v", err)
	}

	// Should have 2 messages
	if len(openaiMessages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(openaiMessages))
	}

	// First message should be user
	if openaiMessages[0]["role"] != "user" {
		t.Errorf("Expected first message role 'user', got '%v'", openaiMessages[0]["role"])
	}

	// Second message should be assistant (not model)
	if openaiMessages[1]["role"] != "assistant" {
		t.Errorf("Expected second message role 'assistant', got '%v'", openaiMessages[1]["role"])
	}
}

func TestConvertMessages_AnthropicToOpenAI(t *testing.T) {
	// Input: Simple Anthropic conversation (JSONL format)
	input := `{"role": "user", "content": [{"type": "text", "text": "Hello from Anthropic!"}]}
{"role": "assistant", "content": [{"type": "text", "text": "Hi!"}]}`

	result, err := ConvertMessages([]byte(input), ModelProviderAnthropic, ModelProviderOpenAI)
	if err != nil {
		t.Fatalf("ConvertMessages failed: %v", err)
	}

	// Verify result is valid JSONL
	var openaiMessages []map[string]interface{}
	if err := parseJSONL(result, &openaiMessages); err != nil {
		t.Fatalf("Result is not valid JSONL: %v", err)
	}

	// Should have 2 messages
	if len(openaiMessages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(openaiMessages))
	}
}

func TestConvertMessages_SameProvider(t *testing.T) {
	// Should return input unchanged when source == target
	input := `{"role": "user", "content": "hello"}`

	result, err := ConvertMessages([]byte(input), ModelProviderOpenAI, ModelProviderOpenAI)
	if err != nil {
		t.Fatalf("ConvertMessages failed: %v", err)
	}

	if string(result) != input {
		t.Errorf("Expected unchanged input for same provider")
	}
}

func TestConvertMessages_InlinesToolResults(t *testing.T) {
	// OpenAI conversation with tool calls (JSONL format)
	// The tool response content should be preserved, but the call itself ignored.
	input := `{"role": "user", "content": "Search for cats"}
{"role": "assistant", "tool_calls": [{"id": "1", "type": "function", "function": {"name": "search", "arguments": "{}"}}]}
{"role": "tool", "tool_call_id": "1", "content": "Search results"}
{"role": "assistant", "content": "Here are the results."}`

	result, err := ConvertMessages([]byte(input), ModelProviderOpenAI, ModelProviderGemini)
	if err != nil {
		t.Fatalf("ConvertMessages failed: %v", err)
	}

	// Verify result is valid JSONL
	var geminiMessages []map[string]interface{}
	if err := parseJSONL(result, &geminiMessages); err != nil {
		t.Fatalf("Result is not valid JSONL: %v", err)
	}

	// Should have 3 messages:
	// 1. user ("Search for cats")
	// 2. user ("[Tool Result]:\nSearch results")
	// 3. model ("Here are the results.")
	if len(geminiMessages) != 3 {
		t.Errorf("Expected 3 messages after conversion, got %d", len(geminiMessages))
	}
}

func TestConvertMessages_PreservesReasoning(t *testing.T) {
	// OpenAI conversation with reasoning (JSONL format)
	input := `{"role": "user", "content": "Think about this"}
{"role": "assistant", "content": "My answer", "reasoning_content": "Let me think..."}`

	result, err := ConvertMessages([]byte(input), ModelProviderOpenAI, ModelProviderGemini)
	if err != nil {
		t.Fatalf("ConvertMessages failed: %v", err)
	}

	// Verify result is valid JSONL
	var geminiMessages []map[string]interface{}
	if err := parseJSONL(result, &geminiMessages); err != nil {
		t.Fatalf("Result is not valid JSONL: %v", err)
	}

	// Check that reasoning is preserved as a thought part
	if len(geminiMessages) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(geminiMessages))
	}

	// Model response should have parts with thought
	parts, ok := geminiMessages[1]["parts"].([]interface{})
	if !ok || len(parts) < 2 {
		t.Errorf("Expected model message to have at least 2 parts (thought + text)")
	}
}
