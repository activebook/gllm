package service

import (
	"strings"
	"testing"
)

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		minToken int // minimum expected tokens
		maxToken int // maximum expected tokens (allows some variance)
	}{
		{
			name:     "empty string",
			input:    "",
			minToken: 0,
			maxToken: 0,
		},
		{
			name:     "single word",
			input:    "hello",
			minToken: 1,
			maxToken: 3,
		},
		{
			name:     "short English sentence",
			input:    "The quick brown fox jumps over the lazy dog.",
			minToken: 8,
			maxToken: 15,
		},
		{
			name:     "Chinese text",
			input:    "你好世界，这是一个测试",
			minToken: 5,
			maxToken: 30, // Uses default ratio if Chinese char ratio < 30%
		},
		{
			name:     "code snippet",
			input:    "func main() {\n\tfmt.Println(\"Hello, World!\")\n}",
			minToken: 10,
			maxToken: 25,
		},
		{
			name:     "mixed content",
			input:    "Hello 你好 func test() { return 42 }",
			minToken: 8,
			maxToken: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EstimateTokens(tt.input)
			if result < tt.minToken || result > tt.maxToken {
				t.Errorf("EstimateTokens(%q) = %d, want between %d and %d",
					tt.input, result, tt.minToken, tt.maxToken)
			}
		})
	}
}

func TestDetectCharsPerToken(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected float64
	}{
		{
			name:     "English text",
			input:    "This is a normal English sentence without any code.",
			expected: CharsPerTokenDefault,
		},
		{
			name:     "Chinese text",
			input:    "这是一段纯中文文本，用于测试中文的字符比例",
			expected: CharsPerTokenChinese,
		},
		{
			name:     "Japanese text with hiragana",
			input:    "これは日本語のテストです。ひらがなとカタカナを含みます。",
			expected: CharsPerTokenJapanese,
		},
		{
			name:     "Korean text",
			input:    "안녕하세요. 한국어 테스트입니다. 오늘 날씨가 좋습니다.",
			expected: CharsPerTokenKorean,
		},
		{
			name:     "code with multiple indicators",
			input:    "func main() {\n\tif (x > 0) {\n\t\tfmt.Println(x)\n\t}\n}",
			expected: CharsPerTokenCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectCharsPerToken(tt.input)
			if result != tt.expected {
				t.Errorf("detectCharsPerToken(%q) = %v, want %v",
					tt.input, result, tt.expected)
			}
		})
	}
}

func TestEstimateTokensLargeText(t *testing.T) {
	// Create a large text (approximately 10000 characters)
	largeText := strings.Repeat("Lorem ipsum dolor sit amet, consectetur adipiscing elit. ", 200)

	result := EstimateTokens(largeText)

	// Should be approximately len/4 = 2500 tokens
	if result < 2000 || result > 3000 {
		t.Errorf("EstimateTokens for large text = %d, expected around 2500", result)
	}
}
