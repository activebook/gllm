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

// Provider domains mapping for better performance and maintainability
var providerDomains = map[string]ModelProvider{
	// Google domains
	"googleapis.com": ModelGemini,
	"google.com":     ModelGemini,
	"ai.google.dev":  ModelGemini,

	// Mistral domains
	"mistral.ai":  ModelMistral,
	"mistral.com": ModelMistral,
	"codestral":   ModelMistral,
	"magistral":   ModelMistral,

	// Chinese/Other domains
	".cn":              ModelOpenChat,
	"aliyuncs.com":     ModelOpenChat,
	"volces.com":       ModelOpenChat,
	"tencentcloud.com": ModelOpenChat,
	"longcat.chat":     ModelOpenChat,
	"moonshot.cn":      ModelOpenChat,
	"moonshot.ai":      ModelOpenChat,
	"bigmodel.cn":      ModelOpenChat,
	"z.ai":             ModelOpenChat,
	"minimax.io":       ModelOpenChat,
	"minimax.com":      ModelOpenChat,
	"baidu.com":        ModelOpenChat,
	"deepseek.com":     ModelOpenChat,
}

func DetectModelProvider(endPoint string) ModelProvider {
	if endPoint == "" {
		return ModelUnknown
	}

	// Normalize endpoint to lowercase for case-insensitive matching
	endPoint = strings.ToLower(endPoint)

	// Check for exact matches first (more specific)
	for domain, provider := range providerDomains {
		if strings.Contains(endPoint, domain) {
			return provider
		}
	}

	return ModelOpenAICompatible
}
