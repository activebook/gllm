package service

import (
	"encoding/json"

	openai "github.com/sashabaranov/go-openai"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
)

// Token estimation constants
// These are based on empirical analysis of various tokenizers:
// - OpenAI's tiktoken: ~4 characters per token for English
// - Chinese text: ~1.5 characters per token
// - Japanese/Korean: ~2 characters per token
// - Code: ~3 characters per token (more symbols/keywords)
const (
	CharsPerTokenEnglish  = 4.0 // Average for English text
	CharsPerTokenChinese  = 1.5 // Average for Chinese text (CJK ideographs)
	CharsPerTokenJapanese = 2.0 // Average for Japanese (mix of kanji + kana)
	CharsPerTokenKorean   = 2.0 // Average for Korean (Hangul syllables)
	CharsPerTokenCode     = 3.0 // Average for code
	CharsPerTokenDefault  = 4.0 // Default fallback
	MessageOverheadTokens = 4   // Overhead per message (role, formatting)
	ToolCallOverhead      = 100 // Approximate tokens for tool call metadata
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

	// Role token
	tokens += 1

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

	// Role token
	tokens += 1

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
