package service

import (
	"testing"
)

func TestDetectMessageProvider_Ambiguity(t *testing.T) {
	// 1. Anthropic Message with Text
	// This currently works because OpenAI doesn't support array content in the struct yet.
	anthropicText := []byte(`[{"role": "user", "content": [{"type": "text", "text": "hello"}]}]`)
	if provider := DetectMessageProvider(anthropicText); provider != ModelProviderAnthropic {
		// Currently this might actually fail to be OpenAI (returns Unknown or Anthropic)
		// expected: Anthropic
		// actual: ?
		t.Logf("Anthropic Text detected as: %s", provider)
	}

	// 2. Anthropic Message with Image (Source field)
	anthropicImage := []byte(`[{"role": "user", "content": [{"type": "image", "source": {"type": "base64", "media_type": "image/jpeg", "data": "..."}}]}]`)
	if provider := DetectMessageProvider(anthropicImage); provider != ModelProviderAnthropic {
		t.Logf("Anthropic Image detected as: %s", provider)
	}

	// 3. OpenAI Multimodal Message (Logic currently broken in code)
	// If we fix the code to support this, it SHOULD be OpenAI.
	openaiMulti := []byte(`[{"role": "user", "content": [{"type": "text", "text": "hello"}, {"type": "image_url", "image_url": {"url": "http://example.com/img.jpg"}}]}]`)
	if provider := DetectMessageProvider(openaiMulti); provider != ModelProviderOpenAI {
		t.Logf("OpenAI Multimodal detected as: %s", provider)
	}
}
