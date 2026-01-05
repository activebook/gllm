package service

import (
	"testing"
)

func TestDetectMessageProvider(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// --- OpenAI Scenarios ---
		{
			name:     "OpenAI Simple Text",
			input:    `[{"role": "user", "content": "hello"}]`,
			expected: ModelProviderOpenAICompatible,
		},
		{
			name:     "OpenAI Multimodal Text",
			input:    `[{"role": "user", "content": [{"type": "text", "text": "hello"}]}]`,
			expected: ModelProviderOpenAICompatible,
		},
		{
			name:     "OpenAI Multimodal Image",
			input:    `[{"role": "user", "content": [{"type": "image_url", "image_url": {"url": "http://example.com/i.jpg"}}]}]`,
			expected: ModelProviderOpenAI,
		},
		{
			name:     "OpenAI Tool Calls",
			input:    `[{"role": "assistant", "tool_calls": [{"id": "1", "type": "function", "function": {"name": "test", "arguments": "{}"}}]}]`,
			expected: ModelProviderOpenAI,
		},
		{
			name:     "OpenAI Tool Response",
			input:    `[{"role": "tool", "tool_call_id": "1", "content": "done"}]`,
			expected: ModelProviderOpenAI,
		},
		{
			name:     "OpenAI Reasoning",
			input:    `[{"role": "assistant", "reasoning_content": "let me think"}]`,
			expected: ModelProviderOpenAI,
		},

		// --- Anthropic Scenarios ---
		{
			name:     "Anthropic Multimodal Image",
			input:    `[{"role": "user", "content": [{"type": "image", "source": {"type": "base64", "media_type": "image/jpeg", "data": "..."}}]}]`,
			expected: ModelProviderAnthropic,
		},
		{
			name:     "Anthropic Tool Use",
			input:    `[{"role": "assistant", "content": [{"type": "tool_use", "id": "1", "name": "test", "input": {}}]}]`,
			expected: ModelProviderAnthropic,
		},
		{
			name:     "Anthropic Tool Result",
			input:    `[{"role": "user", "content": [{"type": "tool_result", "tool_use_id": "1", "content": "res"}]}]`,
			expected: ModelProviderAnthropic,
		},
		{
			name:     "Anthropic Thinking",
			input:    `[{"role": "assistant", "content": [{"type": "thinking", "thinking": "hmmm"}]}]`,
			expected: ModelProviderAnthropic,
		},

		// --- Gemini Scenarios ---
		{
			name: "Gemini Simple Text",
			input: `[
				{"role": "user", "parts": [{"text": "hello"}]},
				{"role": "model", "parts": [{"text": "hi"}]}
			]`,
			expected: ModelProviderGemini,
		},
		{
			name:     "Gemini Inline Data",
			input:    `[{"role": "user", "parts": [{"inline_data": {"mime_type": "image/jpeg", "data": "..."}}]}]`,
			expected: ModelProviderGemini,
		},
		{
			name:     "Gemini Function Call",
			input:    `[{"role": "model", "parts": [{"function_call": {"name": "test", "args": {}}}]}]`,
			expected: ModelProviderGemini,
		},

		// --- Edge Cases ---
		{
			name:     "Empty Array",
			input:    `[]`,
			expected: ModelProviderUnknown,
		},
		{
			name:     "Invalid JSON",
			input:    `{not-json}`,
			expected: ModelProviderUnknown,
		},
		{
			name:     "Unknown Format",
			input:    `[{"foo": "bar"}]`,
			expected: ModelProviderUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectMessageProvider([]byte(tt.input))
			if got != tt.expected {
				t.Errorf("DetectMessageProvider() = %v, want %v", got, tt.expected)
			}
		})
	}
}
