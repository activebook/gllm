package service

import (
	"encoding/json"

	openai "github.com/sashabaranov/go-openai"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
)

// Token estimation constants
// These are refined based on modern tokenizer behavior (cl100k_base, qwen, etc.):
//   - English: ~4 chars/token (ASCII)
//   - Chinese: ~0.6-2.0 tokens/char (Qwen is efficient, OpenAI is 2.0).
//     We use 2.5 bytes/token => ~1.2 tokens/char as a balanced estimate.
//   - Japanese/Korean: ~1.5 tokens/char. 3 bytes/char / 2.0 => 1.5 tokens/char.
//   - Tool Calls: JSON structure overhead is small (~20 tokens), not 100.
const (
	CharsPerTokenEnglish  = 4.0 // Average for English text
	CharsPerTokenChinese  = 2.5 // Tuned: 3 bytes/char / 2.5 = 1.2 tokens/char (balanced)
	CharsPerTokenJapanese = 2.0 // 3 bytes / 2.0 = 1.5 tokens/char
	CharsPerTokenKorean   = 2.0 // 3 bytes / 2.0 = 1.5 tokens/char
	CharsPerTokenCode     = 3.5 // Tuned: Code is dense. 3.5 chars/token.
	CharsPerTokenDefault  = 4.0 // Default fallback
	MessageOverheadTokens = 3   // Standard overhead per message (<|start|>role and <|end|>)
	ToolCallOverhead      = 24  // Reduced from 100 to 24 (closer to reality for JSON overhead)
)

// EstimateTokens provides fast character-based estimation for text.
// This is approximately 90% accurate compared to tiktoken.
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}

	// Detect content type and use appropriate ratio
	charsPerToken := detectCharsPerToken(text)
	return int(float64(len(text))/charsPerToken) + 1
}

// detectCharsPerToken determines the appropriate ratio based on content.
// Supports detection of Chinese, Japanese, Korean, and code.
func detectCharsPerToken(text string) float64 {
	runes := []rune(text)
	total := len(runes)
	if total == 0 {
		return CharsPerTokenDefault
	}

	// Count different character types
	var cjkIdeographs int // Chinese characters (also used in Japanese)
	var hiragana int      // Japanese hiragana
	var katakana int      // Japanese katakana
	var hangul int        // Korean hangul

	for _, r := range runes {
		switch {
		// CJK Unified Ideographs (Chinese, Japanese kanji)
		case r >= '\u4e00' && r <= '\u9fff':
			cjkIdeographs++
		// CJK Extension A
		case r >= '\u3400' && r <= '\u4dbf':
			cjkIdeographs++
		// Hiragana (Japanese)
		case r >= '\u3040' && r <= '\u309f':
			hiragana++
		// Katakana (Japanese)
		case r >= '\u30a0' && r <= '\u30ff':
			katakana++
		// Hangul Syllables (Korean)
		case r >= '\uac00' && r <= '\ud7a3':
			hangul++
		// Hangul Jamo (Korean letters)
		case r >= '\u1100' && r <= '\u11ff':
			hangul++
		// Hangul Compatibility Jamo
		case r >= '\u3130' && r <= '\u318f':
			hangul++
		}
	}

	// Calculate percentages
	cjkPercent := float64(cjkIdeographs) / float64(total)
	japaneseKanaPercent := float64(hiragana+katakana) / float64(total)
	hangulPercent := float64(hangul) / float64(total)

	// Detect Korean (has hangul but little/no CJK)
	if hangulPercent > 0.2 {
		return CharsPerTokenKorean
	}

	// Detect Japanese (has kana + possibly some kanji)
	if japaneseKanaPercent > 0.1 {
		// Japanese typically mixes kana with kanji
		return CharsPerTokenJapanese
	}

	// Detect Chinese (high CJK but no kana)
	if cjkPercent > 0.3 {
		return CharsPerTokenChinese
	}

	// Check for code indicators
	codeIndicators := []string{
		"func ", "function ", "def ", "class ",
		"import ", "package ", "const ", "var ",
		"if (", "for (", "while (", "switch (",
		"```", "{", "}", "=>", "->",
	}
	codeScore := 0
	for _, indicator := range codeIndicators {
		if contains(text, indicator) {
			codeScore++
		}
	}
	if codeScore >= 3 {
		return CharsPerTokenCode
	}

	return CharsPerTokenDefault
}

// contains is a simple string contains check (avoids strings import in hot path)
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// EstimateOpenAIMessageTokens estimates tokens for an OpenAI chat message.
// This accounts for role tokens, content, and tool calls.
func EstimateOpenAIMessageTokens(msg openai.ChatCompletionMessage) int {
	tokens := MessageOverheadTokens

	// Content tokens
	tokens += EstimateTokens(msg.Content)

	// Reasoning content (for models that support it)
	if msg.ReasoningContent != "" {
		tokens += EstimateTokens(msg.ReasoningContent)
	}

	// Multi-part content (images, etc.)
	if len(msg.MultiContent) > 0 {
		for _, part := range msg.MultiContent {
			switch part.Type {
			case openai.ChatMessagePartTypeText:
				tokens += EstimateTokens(part.Text)
			case openai.ChatMessagePartTypeImageURL:
				// Images typically cost 765-1105 tokens depending on size
				// Use a conservative estimate
				tokens += 1000
			}
		}
	}

	// Tool calls
	if len(msg.ToolCalls) > 0 {
		for _, call := range msg.ToolCalls {
			tokens += ToolCallOverhead
			tokens += EstimateTokens(call.Function.Name)
			tokens += EstimateTokens(call.Function.Arguments)
		}
	}

	// Tool call ID (for tool responses)
	if msg.ToolCallID != "" {
		tokens += EstimateTokens(msg.ToolCallID)
	}

	return tokens
}

// EstimateOpenChatMessageTokens estimates tokens for an OpenChat (Volcengine) message.
func EstimateOpenChatMessageTokens(msg *model.ChatCompletionMessage) int {
	if msg == nil {
		return 0
	}

	tokens := MessageOverheadTokens

	// Content tokens
	if msg.Content != nil {
		if msg.Content.StringValue != nil {
			tokens += EstimateTokens(*msg.Content.StringValue)
		}
		if len(msg.Content.ListValue) > 0 {
			for _, part := range msg.Content.ListValue {
				if part.Text != "" {
					tokens += EstimateTokens(part.Text)
				}
				if part.ImageURL != nil {
					tokens += 1000 // Image token estimate
				}
				if part.VideoURL != nil {
					tokens += 1000 // Video token estimate
				}
			}
		}
	}

	// Reasoning content
	if msg.ReasoningContent != nil && *msg.ReasoningContent != "" {
		tokens += EstimateTokens(*msg.ReasoningContent)
	}

	// Tool calls
	if len(msg.ToolCalls) > 0 {
		for _, call := range msg.ToolCalls {
			tokens += ToolCallOverhead
			if call.Function.Name != "" {
				tokens += EstimateTokens(call.Function.Name)
			}
			tokens += EstimateTokens(call.Function.Arguments)
		}
	}

	// Tool call ID
	if msg.ToolCallID != "" {
		tokens += EstimateTokens(msg.ToolCallID)
	}

	return tokens
}

// EstimateOpenAIMessagesTokens estimates total tokens for a slice of OpenAI messages.
func EstimateOpenAIMessagesTokens(messages []openai.ChatCompletionMessage) int {
	total := 0
	for _, msg := range messages {
		total += EstimateOpenAIMessageTokens(msg)
	}
	// Add base overhead for the conversation
	return total + 3 // Every conversation has 3 extra tokens for priming
}

// EstimateOpenChatMessagesTokens estimates total tokens for a slice of OpenChat messages.
func EstimateOpenChatMessagesTokens(messages []*model.ChatCompletionMessage) int {
	total := 0
	for _, msg := range messages {
		total += EstimateOpenChatMessageTokens(msg)
	}
	return total + 3
}

// EstimateJSONTokens estimates tokens for arbitrary JSON data.
// Useful for estimating tool results or complex structured content.
func EstimateJSONTokens(data interface{}) int {
	bytes, err := json.Marshal(data)
	if err != nil {
		return 0
	}
	// JSON typically has slightly higher token density due to punctuation
	return int(float64(len(bytes)) / 3.5)
}
