package service

import "testing"

func TestExtractThinkTags(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		expectedThink   string
		expectedCleaned string
	}{
		{
			name:            "no think tags",
			input:           "Hello, this is a normal response.",
			expectedThink:   "",
			expectedCleaned: "Hello, this is a normal response.",
		},
		{
			name:            "single think tag",
			input:           "<think>Let me think about this...</think>Here is the answer.",
			expectedThink:   "Let me think about this...",
			expectedCleaned: "Here is the answer.",
		},
		{
			name:            "multiline think content",
			input:           "<think>First, I need to consider...\nSecond, the implications are...</think>The conclusion is clear.",
			expectedThink:   "First, I need to consider...\nSecond, the implications are...",
			expectedCleaned: "The conclusion is clear.",
		},
		{
			name:            "multiple think tags",
			input:           "<think>Step 1</think>Result 1<think>Step 2</think>Result 2",
			expectedThink:   "Step 1\nStep 2",
			expectedCleaned: "Result 1Result 2",
		},
		{
			name:            "think tag only",
			input:           "<think>Just thinking...</think>",
			expectedThink:   "Just thinking...",
			expectedCleaned: "",
		},
		{
			name:            "empty think tag",
			input:           "<think></think>Some content",
			expectedThink:   "",
			expectedCleaned: "Some content",
		},
		{
			name:            "think tag with whitespace",
			input:           "<think>  Thinking with spaces  </think>  Content  ",
			expectedThink:   "Thinking with spaces",
			expectedCleaned: "Content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotThink, gotCleaned := ExtractThinkTags(tt.input)
			if gotThink != tt.expectedThink {
				t.Errorf("ExtractThinkTags() thinking = %q, want %q", gotThink, tt.expectedThink)
			}
			if gotCleaned != tt.expectedCleaned {
				t.Errorf("ExtractThinkTags() cleaned = %q, want %q", gotCleaned, tt.expectedCleaned)
			}
		})
	}
}
