package service

import (
	"encoding/json"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	openai "github.com/sashabaranov/go-openai"
	openchat "github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"google.golang.org/genai"
)

// Token estimation constants
// These are refined based on modern tokenizer behavior (cl100k_base, qwen, etc.):
//   - English: ~4 chars/token (ASCII)
//   - Chinese: ~0.6-2.0 tokens/char (Qwen is efficient, OpenAI is 2.0).
//     due to the different tokenization methods used by different models, the conversion ratios can vary
//     We use 2.5 bytes/token => ~1.2 tokens/char as a balanced estimate.
//   - Japanese/Korean: ~1.5 tokens/char. 3 bytes/char / 2.0 => 1.5 tokens/char.
//   - Tool Calls: JSON structure overhead is small (~20 tokens), not 100.
const (
	CharsPerTokenEnglish  = 4.0 // Average for English text
	CharsPerTokenChinese  = 2.5 // Tuned: 3 bytes/char / 2.5 = 1.2 tokens/char (balanced)
	CharsPerTokenJapanese = 2.0 // 3 bytes / 2.0 = 1.5 tokens/char
	CharsPerTokenKorean   = 2.0 // 3 bytes / 2.0 = 1.5 tokens/char
	CharsPerTokenCode     = 3.5 // Tuned: Code is dense. 3.5 chars/token.
	CharsPerTokenJSON     = 3.7 // JSON: Typically 3.5-4 characters per token. Tuned: 3.7 chars/token.
	CharsPerTokenDefault  = 4.0 // Default fallback
	MessageOverheadTokens = 3   // Standard overhead per message (<|start|>role and <|end|>)
	ToolCallOverhead      = 24  // Reduced from 100 to 24 (closer to reality for JSON overhead)

	// Media Token Costs (Heuristics)
	// 1MB = 1000 tokens
	TokenCostImageDefault = 1000 // Safe upper bound average for high-res images (OpenAI high detail is ~1105, low is 85)
	TokenCostImageGemini  = 1000 // Fixed cost for Gemini images <= 384px (often tiled, but 258 is the base unit)

	// Video/Audio Heuristics (Tokens per MB - heavily estimated as we don't have duration)
	// Assumptions:
	// - Video: 2Mbps (.25MB/s). 1MB = 4s. Gemini Video: 263 tokens/s. 4s * 263 = 1052 tokens.
	// - Audio: 128kbps (16KB/s). 1MB = 64s. Gemini Audio: 32 tokens/s. 64s * 32 = 2048 tokens.
	TokenCostVideoPerMBGemini   = 1000
	TokenCostVideoPerMBOpenChat = 1000 // For base64 encoded video
	TokenCostAudioPerMBGemini   = 2000
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
	var ascii int

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
		case r < 128:
			ascii++
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

// isDataURL checks if a string starts with "data:"
func isDataURL(s string) bool {
	return len(s) > 5 && s[:5] == "data:"
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
				// If it's a data URL, usage fixed cost
				if part.ImageURL != nil {
					if isDataURL(part.ImageURL.URL) {
						tokens += TokenCostImageDefault
					} else {
						tokens += EstimateTokens(part.ImageURL.URL)
					}
					// Check detail if available, but for now we rely on the heuristic
					// tokens += EstimateTokens(string(part.ImageURL.Detail))
				}
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
func EstimateOpenChatMessageTokens(msg *openchat.ChatCompletionMessage) int {
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
					if isDataURL(part.ImageURL.URL) {
						tokens += TokenCostImageDefault
					} else {
						tokens += EstimateTokens(part.ImageURL.URL)
					}
				}
				if part.VideoURL != nil {
					if isDataURL(part.VideoURL.URL) {
						// Video cost heuristic (1MB ~ 10k tokens)
						// Since we can't easily get size from base64 string without decoding or overhead math
						// base64 length = 4/3 * original size.
						// So len(base64) / 1.33 = size in bytes.
						// tokens = (len / 1.33 / 1024 / 1024) * TokenCostVideoPerMB
						// simplified: len / 1400  (approx)
						sizeInMB := (float64(len(part.VideoURL.URL)) / 1.33) / 1024 / 1024
						tokens += int(sizeInMB * TokenCostVideoPerMBOpenChat)
					} else {
						tokens += EstimateTokens(part.VideoURL.URL)
					}
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

// EstimateGeminiMessageTokens estimates tokens for a Gemini content message.
func EstimateGeminiMessageTokens(msg *genai.Content) int {
	if msg == nil {
		return 0
	}

	tokens := MessageOverheadTokens

	for _, part := range msg.Parts {
		// Text content
		if part.Text != "" {
			tokens += EstimateTokens(part.Text)
		}

		// Function call
		if part.FunctionCall != nil {
			tokens += ToolCallOverhead
			tokens += EstimateTokens(part.FunctionCall.Name)
			// Arguments are a map[string]interface{}, convert to JSON string to estimate
			tokens += EstimateJSONTokens(part.FunctionCall.Args)
		}

		// Function response
		if part.FunctionResponse != nil {
			tokens += ToolCallOverhead
			tokens += EstimateTokens(part.FunctionResponse.Name)
			tokens += EstimateJSONTokens(part.FunctionResponse.Response)
		}

		// Inline data (images/files)
		if part.InlineData != nil {
			// tokens += int(float64(len(part.InlineData.Data)) / 3.5) // OLD INCORRECT LOGIC
			sizeInMB := float64(len(part.InlineData.Data)) / 1024 / 1024
			if IsImageMIMEType(part.InlineData.MIMEType) {
				tokens += TokenCostImageGemini
			} else if IsVideoMIMEType(part.InlineData.MIMEType) {
				tokens += int(sizeInMB * TokenCostVideoPerMBGemini)
			} else if IsAudioMIMEType(part.InlineData.MIMEType) {
				tokens += int(sizeInMB * TokenCostAudioPerMBGemini)
			} else {
				// Fallback for other files (PDFs etc) - treat as dense text?
				// PDFs are often text, so size based might be okay, or extracting text would be better.
				// For now, let's assume safe text estimate: 1 char = 1 byte = 0.25 tokens?
				// Actually text is usually compressed.
				// Let's stick with the text heuristic for unknown types if it's text-based,
				// but PDF is binary.
				// Gemini PDF tokenization is based on extracted text.
				// Heuristic: 1MB PDF ~= 10k tokens?
				tokens += int(sizeInMB * 10000)
			}
			tokens += EstimateTokens(part.InlineData.MIMEType)
		}

		// File data (images/files via URI)
		if part.FileData != nil {
			// We can't know the size of remote user files easily here without fetching.
			// But usually fileUri means it's already uploaded.
			// The API response will have the usage.
			// For estimation, we might have to just guess or ignore.
			// Let's guess based on typical file?
			if IsImageMIMEType(part.FileData.MIMEType) {
				tokens += TokenCostImageGemini
			} else {
				// Can't estimate well.
				tokens += 100 // placeholder
			}
			tokens += EstimateTokens(part.FileData.FileURI)
			tokens += EstimateTokens(part.FileData.MIMEType)
		}
	}

	return tokens
}

// EstimateAnthropicMessageTokens estimates tokens for an Anthropic message.
func EstimateAnthropicMessageTokens(msg anthropic.MessageParam) int {
	tokens := MessageOverheadTokens

	for _, block := range msg.Content {
		// Text Content
		if block.OfText != nil {
			tokens += EstimateTokens(block.OfText.Text)
		}

		// Thinking Content
		if block.OfThinking != nil {
			tokens += EstimateTokens(block.OfThinking.Thinking)
		}
		if block.OfRedactedThinking != nil {
			tokens += EstimateTokens(block.OfRedactedThinking.Data)
		}

		// Tool Use
		if block.OfToolUse != nil {
			tokens += ToolCallOverhead
			tokens += EstimateTokens(block.OfToolUse.Name)
			// Input is interface{}, convert to JSON string to estimate
			tokens += EstimateJSONTokens(block.OfToolUse.Input)
		}

		// Tool Result
		if block.OfToolResult != nil {
			tokens += ToolCallOverhead
			tokens += EstimateTokens(block.OfToolResult.ToolUseID)
			// Use generic JSON estimation for robustness as Content structure varies
			tokens += EstimateJSONTokens(block.OfToolResult.Content)
		}

		// Image
		if block.OfImage != nil {
			// ImageBlockParam { Source: ImageBlockParamSourceUnion ... }
			// Try to access Base64 via OfBase64 if available, otherwise fallback to JSON size
			if block.OfImage.Source.OfBase64 != nil {
				tokens += TokenCostImageDefault
				tokens += EstimateTokens(string(block.OfImage.Source.OfBase64.MediaType))
			} else {
				// Fallback
				tokens += EstimateJSONTokens(block.OfImage.Source)
			}
		}

		// Document
		if block.OfDocument != nil {
			// DocumentBlockParam { Source: ... }
			// Usually base64 PDFs.
			// Heuristic: 1MB ~= 1000 tokens? Or stick to TokenCostImageDefault for now?
			// Let's estimate based on source data length if string.
			tokens += EstimateJSONTokens(block.OfDocument.Source)
		}
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
	return total + MessageOverheadTokens // Every conversation has 3 extra tokens for priming
}

// EstimateOpenChatMessagesTokens estimates total tokens for a slice of OpenChat messages.
func EstimateOpenChatMessagesTokens(messages []*openchat.ChatCompletionMessage) int {
	total := 0
	for _, msg := range messages {
		total += EstimateOpenChatMessageTokens(msg)
	}
	return total + MessageOverheadTokens
}

// EstimateGeminiMessagesTokens estimates total tokens for a slice of Gemini messages.
func EstimateGeminiMessagesTokens(messages []*genai.Content) int {
	total := 0
	for _, msg := range messages {
		total += EstimateGeminiMessageTokens(msg)
	}
	// Add base overhead
	return total + MessageOverheadTokens
}

// EstimateAnthropicMessagesTokens estimates total tokens for a slice of Anthropic messages.
func EstimateAnthropicMessagesTokens(messages []anthropic.MessageParam) int {
	total := 0
	for _, msg := range messages {
		total += EstimateAnthropicMessageTokens(msg)
	}
	// Add base overhead for the conversation
	return total + MessageOverheadTokens
}

// EstimateJSONTokens estimates tokens for arbitrary JSON data.
// Useful for estimating tool results or complex structured content.
func EstimateJSONTokens(data interface{}) int {
	bytes, err := json.Marshal(data)
	if err != nil {
		return 0
	}
	// JSON typically has slightly higher token density due to punctuation
	return int(float64(len(bytes)) / CharsPerTokenJSON)
}

// EstimateGeminiToolTokens estimates tokens for a slice of Gemini tools
func EstimateGeminiToolTokens(tools []*genai.Tool) int {
	if len(tools) == 0 {
		return 0
	}
	return EstimateJSONTokens(tools)
}

// EstimateOpenAIToolTokens estimates tokens for a slice of OpenAI tools
func EstimateOpenAIToolTokens(tools []openai.Tool) int {
	if len(tools) == 0 {
		return 0
	}
	return EstimateJSONTokens(tools)
}

// EstimateOpenChatToolTokens estimates tokens for a slice of OpenChat tools
func EstimateOpenChatToolTokens(tools []*openchat.Tool) int {
	if len(tools) == 0 {
		return 0
	}
	return EstimateJSONTokens(tools)
}

// EstimateAnthropicToolTokens estimates tokens for a slice of Anthropic tools.
func EstimateAnthropicToolTokens(tools []anthropic.ToolUnionParam) int {
	if len(tools) == 0 {
		return 0
	}
	// Tools are defined as JSON schemas
	return EstimateJSONTokens(tools)
}
