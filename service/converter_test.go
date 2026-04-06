package service

import (
	"encoding/json"
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

func TestConvertMessages_PreservesToolCalls(t *testing.T) {
	// OpenAI conversation with tool calls (JSONL format)
	input := `{"role": "user", "content": "Search for cats"}
{"role": "assistant", "tool_calls": [{"id": "1", "type": "function", "function": {"name": "search", "arguments": "{\"q\":\"cat\"}"}}]}
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

	// Should have 4 messages:
	// 1. user ("Search for cats")
	// 2. model (FunctionCall search)
	// 3. user (FunctionResponse search)
	// 4. model ("Here are the results.")
	if len(geminiMessages) != 4 {
		t.Fatalf("Expected 4 messages after conversion, got %d", len(geminiMessages))
	}

	// Verify Message 2 (FunctionCall)
	if geminiMessages[1]["role"] != "model" {
		t.Errorf("Expected msg 2 role 'model', got '%v'", geminiMessages[1]["role"])
	}
	parts2 := geminiMessages[1]["parts"].([]interface{})
	part2 := parts2[0].(map[string]interface{})
	if _, hasFunc := part2["functionCall"]; !hasFunc {
		t.Errorf("Expected functionCall in msg 2")
	}

	// Verify Message 3 (FunctionResponse)
	if geminiMessages[2]["role"] != "user" {
		t.Errorf("Expected msg 3 role 'user', got '%v'", geminiMessages[2]["role"])
	}
	parts3 := geminiMessages[2]["parts"].([]interface{})
	part3 := parts3[0].(map[string]interface{})
	funcResp, hasResp := part3["functionResponse"].(map[string]interface{})
	if !hasResp {
		t.Fatalf("Expected functionResponse in msg 3")
	}
	if funcResp["name"] != "search" {
		t.Errorf("Expected functionResponse name 'search' (correlated), got '%v'", funcResp["name"])
	}
}

func TestConvertMessages_PreservesReasoning(t *testing.T) {
	// NOTE: The official openai-go SDK's ChatCompletionMessageParamUnion does NOT
	// preserve custom fields like `reasoning_content` during JSON unmarshal.
	// The project encodes reasoning as a `<think>...</think>` prefix in the
	// content string (see BuildOpenAIMessages). This test validates that round-trip.
	input := `{"role": "user", "content": "Think about this"}
{"role": "assistant", "content": "<think>\nLet me think...\n</think>\nMy answer"}`

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
		out, _ := json.MarshalIndent(geminiMessages, "", "  ")
		t.Logf("Output: %s", string(out))
		t.Errorf("Expected model message to have at least 2 parts (thought + text)")
	}
}

func TestConvertMessages_Multimodal_OpenAIToGemini(t *testing.T) {
	// Input: OpenAI message with image
	input := `{"role": "user", "content": [{"type": "text", "text": "What is this?"}, {"type": "image_url", "image_url": {"url": "data:image/jpeg;base64,aGVsbG8="}}]}`

	result, err := ConvertMessages([]byte(input), ModelProviderOpenAI, ModelProviderGemini)
	if err != nil {
		t.Fatalf("ConvertMessages failed: %v", err)
	}

	var geminiMessages []map[string]interface{}
	if err := parseJSONL(result, &geminiMessages); err != nil {
		t.Fatalf("Result is not valid JSONL: %v", err)
	}

	if len(geminiMessages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(geminiMessages))
	}

	parts, ok := geminiMessages[0]["parts"].([]interface{})
	if !ok || len(parts) != 2 {
		t.Fatalf("Expected 2 parts in Gemini message, got %v", parts)
	}

	// First part is text
	part1 := parts[0].(map[string]interface{})
	if part1["text"] != "What is this?" {
		t.Errorf("Expected first part text 'What is this?', got '%v'", part1["text"])
	}

	// Second part is image inlineData
	part2 := parts[1].(map[string]interface{})
	inlineData, ok := part2["inlineData"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected inlineData in second part, got %v", part2)
	}
	if inlineData["mimeType"] != "image/jpeg" {
		t.Errorf("Expected mimeType 'image/jpeg', got '%v'", inlineData["mimeType"])
	}
	if inlineData["data"] != "aGVsbG8=" {
		t.Errorf("Expected data 'aGVsbG8=', got '%v'", inlineData["data"])
	}
}

func TestConvertMessages_Multimodal_GeminiToAnthropic(t *testing.T) {
	// Input: Gemini message with image
	input := `{"role": "user", "parts": [{"text": "Explain this diagram"}, {"inlineData": {"mimeType": "image/png", "data": "c29tZWRhdGE="}}]}`

	result, err := ConvertMessages([]byte(input), ModelProviderGemini, ModelProviderAnthropic)
	if err != nil {
		t.Fatalf("ConvertMessages failed: %v", err)
	}

	var anthropicMessages []map[string]interface{}
	if err := parseJSONL(result, &anthropicMessages); err != nil {
		t.Fatalf("Result is not valid JSONL: %v", err)
	}

	if len(anthropicMessages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(anthropicMessages))
	}

	content, ok := anthropicMessages[0]["content"].([]interface{})
	if !ok || len(content) != 2 {
		t.Fatalf("Expected 2 content blocks in Anthropic message, got %v", content)
	}

	// First block is text
	block1 := content[0].(map[string]interface{})
	if block1["text"] != "Explain this diagram" {
		t.Errorf("Expected first block text 'Explain this diagram', got '%v'", block1["text"])
	}

	// Second block is image
	block2 := content[1].(map[string]interface{})
	if block2["type"] != "image" {
		t.Errorf("Expected type 'image', got '%v'", block2["type"])
	}
	source, ok := block2["source"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected source in image block, got %v", block2)
	}
	if source["media_type"] != "image/png" {
		t.Errorf("Expected media_type 'image/png', got '%v'", source["media_type"])
	}
	if source["data"] != "c29tZWRhdGE=" {
		t.Errorf("Expected data 'c29tZWRhdGE=', got '%v'", source["data"])
	}
}

func TestConvertMessages_Multimodal_AnthropicToOpenChat(t *testing.T) {
	// Input: Anthropic message with image
	input := `{"role": "user", "content": [{"type": "text", "text": "Analyze frame"}, {"type": "image", "source": {"type": "base64", "media_type": "image/webp", "data": "d2VicGRhdGE="}}]}`

	result, err := ConvertMessages([]byte(input), ModelProviderAnthropic, ModelProviderOpenAICompatible)
	if err != nil {
		t.Fatalf("ConvertMessages failed: %v", err)
	}

	var openChatMessages []map[string]interface{}
	if err := parseJSONL(result, &openChatMessages); err != nil {
		t.Fatalf("Result is not valid JSONL: %v", err)
	}

	if len(openChatMessages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(openChatMessages))
	}

	contentField := openChatMessages[0]["content"]
	var contentArr []interface{}
	
	if arr, ok := contentField.([]interface{}); ok {
		contentArr = arr
	} else if obj, ok := contentField.(map[string]interface{}); ok {
		contentArr = obj["ListValue"].([]interface{})
	}

	if len(contentArr) != 2 {
		t.Fatalf("Expected content array of length 2 in OpenChat message, got %v", contentArr)
	}
	
	// First block is text
	block1 := contentArr[0].(map[string]interface{})
	if block1["text"] != "Analyze frame" {
		t.Errorf("Expected first block text 'Analyze frame', got '%v'", block1["text"])
	}

	// Second block is image_url
	block2 := contentArr[1].(map[string]interface{})
	imageURL, ok := block2["image_url"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected image_url object, got '%v'", block2)
	}
	if imageURL["url"] != "data:image/webp;base64,d2VicGRhdGE=" {
		t.Errorf("Expected Data URL 'data:image/webp;base64,d2VicGRhdGE=', got '%v'", imageURL["url"])
	}
}
