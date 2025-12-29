package service

import (
	"encoding/base64"
	"fmt"
	"testing"

	openai "github.com/sashabaranov/go-openai"
	openchat "github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"google.golang.org/genai"
)

// Helper to create a dummy large base64 string
func createLargeBase64(sizeMB int) string {
	sizeBytes := sizeMB * 1024 * 1024
	data := make([]byte, sizeBytes)
	return base64.StdEncoding.EncodeToString(data)
}

// Helper to create a dummy data URL
func createDataURL(base64Data string, weirdType string) string {
	return fmt.Sprintf("data:%s;base64,%s", weirdType, base64Data)
}

func TestEstimateTokens_Text(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected int
	}{
		{"Empty", "", 0},
		{"English", "Hello world", 4}, // 11 chars / 4 + 1 = 3
		{"Chinese", "你好世界", 4},        // 4 chars / 1.2 + 1 = 4
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateTokens(tt.text)
			if got == 0 && tt.expected != 0 {
				t.Errorf("EstimateTokens() = %v, expected non-zero", got)
			}
		})
	}
}

func TestEstimateOpenAIMessageTokens_Image(t *testing.T) {
	// 1 MB image base64
	b64 := createLargeBase64(1)
	dataURL := createDataURL(b64, "image/png")

	msg := openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleUser,
		MultiContent: []openai.ChatMessagePart{
			{
				Type: openai.ChatMessagePartTypeImageURL,
				ImageURL: &openai.ChatMessageImageURL{
					URL: dataURL,
				},
			},
		},
	}

	tokens := EstimateOpenAIMessageTokens(msg)
	// Expect tokens to be around TokenCostImageDefault (1000) + overhead
	// Not 1MB worth of text tokens which would be ~250,000
	if tokens > 2000 {
		t.Errorf("EstimateOpenAIMessageTokens() = %v, expected < 2000 for image", tokens)
	}
	if tokens < 1000 {
		t.Errorf("EstimateOpenAIMessageTokens() = %v, expected >= 1000 for image", tokens)
	}
}

func TestEstimateOpenChatMessageTokens_Video(t *testing.T) {
	// 1 MB video base64
	// 1 MB bytes = 1.33MB base64 string.
	// Our heuristic for video is len(url) / 1400.
	// 1.33 * 1024 * 1024 / 1400 = 1,400,000 / 1400 = 1000 tokens approximately.
	b64 := createLargeBase64(1)
	dataURL := createDataURL(b64, "video/mp4")

	msg := &openchat.ChatCompletionMessage{
		Role: openchat.ChatMessageRoleUser,
		Content: &openchat.ChatCompletionMessageContent{
			ListValue: []*openchat.ChatCompletionMessageContentPart{
				{
					Type: openchat.ChatCompletionMessageContentPartTypeVideoURL,
					VideoURL: &openchat.ChatMessageVideoURL{
						URL: dataURL,
					},
				},
			},
		},
	}

	tokens := EstimateOpenChatMessageTokens(msg)

	// Expect ~1000 tokens
	if tokens > 2000 {
		t.Errorf("EstimateOpenChatMessageTokens() = %v, expected < 2000 for 1MB video", tokens)
	}
	if tokens < 800 {
		t.Errorf("EstimateOpenChatMessageTokens() = %v, expected > 800 for 1MB video", tokens)
	}
}

func TestEstimateGeminiMessageTokens_Media(t *testing.T) {
	// 1MB Image
	blob := make([]byte, 1024*1024)

	// Test Image
	msgImage := &genai.Content{
		Parts: []*genai.Part{
			{
				InlineData: &genai.Blob{
					MIMEType: "image/png",
					Data:     blob,
				},
			},
		},
	}
	tokensImage := EstimateGeminiMessageTokens(msgImage)
	// Expect TokenCostImageGemini (1000) + overhead (~6)
	if tokensImage < 1000 || tokensImage > 1100 {
		t.Errorf("EstimateGeminiMessageTokens(Image) = %v, expected ~1006", tokensImage)
	}

	// Test Audio (1MB)
	msgAudio := &genai.Content{
		Parts: []*genai.Part{
			{
				InlineData: &genai.Blob{
					MIMEType: "audio/mp3",
					Data:     blob,
				},
			},
		},
	}
	tokensAudio := EstimateGeminiMessageTokens(msgAudio)
	// Expect 1MB * TokenCostAudioPerMB (2000)
	if tokensAudio < 1900 || tokensAudio > 2100 {
		t.Errorf("EstimateGeminiMessageTokens(Audio) = %v, expected ~2000", tokensAudio)
	}
}
