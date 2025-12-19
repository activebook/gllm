package service

import (
	"testing"
)

func TestDetectModelProvider(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		expected ModelProvider
	}{
		{
			name:     "empty endpoint",
			endpoint: "",
			expected: ModelUnknown,
		},
		{
			name:     "Google Gemini API",
			endpoint: "https://generativelanguage.googleapis.com/v1beta/models/gemini-pro:generateContent",
			expected: ModelGemini,
		},
		{
			name:     "Google AI Studio",
			endpoint: "https://ai.google.dev/api/generate",
			expected: ModelGemini,
		},
		{
			name:     "Mistral AI",
			endpoint: "https://api.mistral.ai/v1/chat/completions",
			expected: ModelMistral,
		},
		{
			name:     "Codestral",
			endpoint: "https://codestral.mistral.ai/v1/completions",
			expected: ModelMistral,
		},
		{
			name:     "Chinese model - Alibaba",
			endpoint: "https://dashscope.aliyuncs.com/api/v1/models",
			expected: ModelOpenChat,
		},
		{
			name:     "Chinese model - Volcengine",
			endpoint: "https://ark.cn-beijing.volces.com/api/v3/models",
			expected: ModelOpenChat,
		},
		{
			name:     "Chinese model - Moonshot",
			endpoint: "https://api.moonshot.cn/v1/models",
			expected: ModelOpenChat,
		},
		{
			name:     "DeepSeek",
			endpoint: "https://api.deepseek.com/v1/chat/completions",
			expected: ModelOpenChat,
		},
		{
			name:     "OpenAI compatible - Groq",
			endpoint: "https://api.groq.com/openai/v1/chat/completions",
			expected: ModelOpenAICompatible,
		},
		{
			name:     "OpenAI compatible - Together AI",
			endpoint: "https://api.together.xyz/v1/chat/completions",
			expected: ModelOpenAICompatible,
		},
		{
			name:     "Unknown domain",
			endpoint: "https://api.unknown-provider.com/v1/chat",
			expected: ModelOpenAICompatible,
		},
		{
			name:     "Case insensitive matching",
			endpoint: "https://API.MISTRAL.AI/v1/chat",
			expected: ModelMistral,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectModelProvider(tt.endpoint, "")
			if result != tt.expected {
				t.Errorf("DetectModelProvider(%q) = %v, want %v", tt.endpoint, result, tt.expected)
			}
		})
	}
}

func TestModelProviderConstants(t *testing.T) {
	// Test that all constants are defined and unique
	providers := []ModelProvider{
		ModelGemini,
		ModelOpenAI,
		ModelOpenChat,
		ModelOpenAICompatible,
		ModelMistral,
		ModelUnknown,
	}

	// Check for uniqueness
	seen := make(map[ModelProvider]bool)
	for _, provider := range providers {
		if seen[provider] {
			t.Errorf("Duplicate provider constant: %v", provider)
		}
		seen[provider] = true
	}

	// Check that constants are not empty
	for _, provider := range providers {
		if provider == "" {
			t.Error("Provider constant should not be empty")
		}
	}
}

func BenchmarkDetectModelProvider(b *testing.B) {
	endpoints := []string{
		"https://generativelanguage.googleapis.com/v1beta/models/gemini-pro:generateContent",
		"https://api.mistral.ai/v1/chat/completions",
		"https://dashscope.aliyuncs.com/api/v1/models",
		"https://api.groq.com/openai/v1/chat/completions",
		"https://api.unknown-provider.com/v1/chat",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, endpoint := range endpoints {
			DetectModelProvider(endpoint, "")
		}
	}
}
