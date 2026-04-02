package service

import (
	"testing"
)

func TestSanitizeGeneratedName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "basic case with quotes and newlines",
			input:    "\"This is a test session\"\n",
			expected: "This-is-a-test-session",
		},
		{
			name:     "already hyphens but weird casing",
			input:    "SOME-random-TALK",
			expected: "SOME-random-TALK",
		},
		{
			name:     "illegal characters and extra spaces",
			input:    "  `Hello! World? How are you?`  ",
			expected: "Hello-World-How-are-you",
		},
		{
			name:     "multiple dash replacement",
			input:    "lots -- of --- dashes - here",
			expected: "lots-of-dashes-here",
		},
		{
			name:     "surrounding underscores",
			input:    "___my_test_session___",
			expected: "my-test-session",
		},
		{
			name:     "mixed symbols",
			input:    "\"We're fixing bug #123 (critical)!\"",
			expected: "Were-fixing-bug-123-critical",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeGeneratedName(tt.input)
			if got != tt.expected {
				t.Errorf("sanitizeGeneratedName(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}
