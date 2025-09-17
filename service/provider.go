package service

import (
	"strings"
)

type ModelProvider string

const (
	// Model types
	ModelGemini           ModelProvider = "gemini" // for google gemini models
	ModelOpenAI           ModelProvider = "openai"
	ModelOpenChat         ModelProvider = "openchat" // for chinese models
	ModelOpenAICompatible ModelProvider = "openai-compatible"
	ModelMistral          ModelProvider = "mistral" // for mistral models
	ModelUnknown          ModelProvider = "unknown"
)

func DetectModelProvider(endPoint string) ModelProvider {
	goo_domains := []string{"googleapis.com", "google.com"}
	for _, domain := range goo_domains {
		if strings.Contains(endPoint, domain) {
			return ModelGemini
		}
	}
	mistral_domains := []string{"mistral.ai", "mistral.com", "codestral", "magistral"}
	for _, domain := range mistral_domains {
		if strings.Contains(endPoint, domain) {
			return ModelMistral
		}
	}

	// Chinese models and others
	chn_domains := []string{".cn", "aliyuncs.com", "volces.com", "tencentcloud.com", "longcat.chat", "moonshot.cn", "moonshot.ai", "bigmodel.cn", "z.ai", "minimax.io", "minimax.com", "baidu.com", "deepseek.com"}
	for _, domain := range chn_domains {
		if strings.Contains(endPoint, domain) {
			return ModelOpenChat
		}
	}

	return ModelOpenAICompatible
}
