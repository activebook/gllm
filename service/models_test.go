package service

import (
	"testing"
)

func TestGetModelLimits(t *testing.T) {
	tests := []struct {
		name          string
		modelName     string
		expectDefault bool
		minContext    int
	}{
		{
			name:          "exact match gpt-4o",
			modelName:     "gpt-4o",
			expectDefault: false,
			minContext:    128000,
		},
		{
			name:          "exact match gemini-2.5-pro",
			modelName:     "gemini-2.5-pro",
			expectDefault: false,
			minContext:    1000000,
		},
		{
			name:          "pattern match qwen model",
			modelName:     "qwen-turbo-latest",
			expectDefault: false,
			minContext:    100000,
		},
		{
			name:          "pattern match deepseek",
			modelName:     "deepseek-v3-latest",
			expectDefault: false,
			minContext:    60000,
		},
		{
			name:          "unknown model",
			modelName:     "unknown-model-xyz",
			expectDefault: true,
			minContext:    32000,
		},
		{
			name:          "empty model name",
			modelName:     "",
			expectDefault: true,
			minContext:    32000,
		},
		{
			name:          "case insensitive match",
			modelName:     "GPT-4O",
			expectDefault: false,
			minContext:    128000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limits := GetModelLimits(tt.modelName)

			if tt.expectDefault {
				if limits != DefaultLimits {
					t.Errorf("GetModelLimits(%q) should return DefaultLimits", tt.modelName)
				}
			} else {
				if limits.ContextWindow < tt.minContext {
					t.Errorf("GetModelLimits(%q) ContextWindow = %d, want >= %d",
						tt.modelName, limits.ContextWindow, tt.minContext)
				}
			}
		})
	}
}

func TestMaxInputTokens(t *testing.T) {
	tests := []struct {
		name          string
		limits        ModelLimits
		bufferPercent float64
		expected      int
	}{
		{
			name:          "80% buffer for gpt-4o",
			limits:        ModelLimits{ContextWindow: 128000, MaxOutputTokens: 16384},
			bufferPercent: 0.8,
			expected:      89292, // (128000 - 16384) * 0.8
		},
		{
			name:          "100% buffer (no safety margin)",
			limits:        ModelLimits{ContextWindow: 32000, MaxOutputTokens: 4096},
			bufferPercent: 1.0,
			expected:      27904, // (32000 - 4096) * 1.0
		},
		{
			name:          "invalid buffer defaults to 80%",
			limits:        ModelLimits{ContextWindow: 100000, MaxOutputTokens: 8192},
			bufferPercent: -1.0,
			expected:      73446, // (100000 - 8192) * 0.8
		},
		{
			name:          "buffer over 1.0 defaults to 80%",
			limits:        ModelLimits{ContextWindow: 100000, MaxOutputTokens: 8192},
			bufferPercent: 1.5,
			expected:      73446,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.limits.MaxInputTokens(tt.bufferPercent)
			// Allow small variance due to floating point
			if result < tt.expected-10 || result > tt.expected+10 {
				t.Errorf("MaxInputTokens(%v) = %d, want approximately %d",
					tt.bufferPercent, result, tt.expected)
			}
		})
	}
}

func TestModelRegistryCompleteness(t *testing.T) {
	// Ensure key model families are represented
	families := []string{
		"gpt-4",
		"claude",
		"gemini",
		"mistral",
		"qwen",
		"deepseek",
		"glm",
		"llama",
	}

	for _, family := range families {
		found := false
		for modelName := range DefaultModelLimits {
			if len(modelName) >= len(family) && modelName[:len(family)] == family {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Model family %q not found in DefaultModelLimits", family)
		}
	}
}
