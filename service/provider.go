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

	// Explicit OpenAI domains
	"openai.com":       ModelOpenAI,
	"api.openai.com":   ModelOpenAI,
	"azure.com":        ModelOpenAI,
	"azure-api.net":    ModelOpenAI,
	"openai.azure.com": ModelOpenAI,
	"modelscope.ai":    ModelOpenAI,
	"zenmux.ai":        ModelOpenAI,

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
	"modelscope.cn":    ModelOpenChat,
}

// Model name patterns for Chinese models (used when endpoint doesn't match)
var modelPatterns = map[string]ModelProvider{
	"qwen":     ModelOpenChat, // Alibaba Qwen models
	"deepseek": ModelOpenChat, // DeepSeek models
	"xiaomi":   ModelOpenChat, // Xiaom models
	"mimo":     ModelOpenChat, // Xiaom models
	"meituan":  ModelOpenChat, // Meituan Longcat models
	"longcat":  ModelOpenChat, // Meituan Longcat models
	"minimax":  ModelOpenChat, // MiniMax models
	"kimi":     ModelOpenChat, // Moonshot Kimi models
	"moonshot": ModelOpenChat, // Moonshot models
	"glm":      ModelOpenChat, // Zhipu GLM models
	"chatglm":  ModelOpenChat, // Zhipu ChatGLM models
	"ernie":    ModelOpenChat, // Baidu ERNIE models
	"hunyuan":  ModelOpenChat, // Tencent Hunyuan models
	"doubao":   ModelOpenChat, // ByteDance Doubao models
	"skylark":  ModelOpenChat, // ByteDance Skylark models
	"kat-":     ModelOpenChat, // Kuaishou Kat models (with hyphen to avoid false matches)
	"kat_":     ModelOpenChat, // Kuaishou Kat models (with underscore)
	"abab":     ModelOpenChat, // MiniMax ABAB models
	"yi-":      ModelOpenChat, // 01.AI Yi models (with hyphen to avoid false matches)
	"yi_":      ModelOpenChat, // 01.AI Yi models (with underscore)
}

// DetectModelProvider detects the model provider based on endpoint and model name.
// It first checks the endpoint domain, then falls back to model name patterns.
// This dual detection handles Chinese models hosted on US platforms (AWS, CoreWeave, etc.)
func DetectModelProvider(endPoint string, modelName ...string) ModelProvider {
	if endPoint == "" && len(modelName) == 0 {
		return ModelUnknown
	}

	// Normalize endpoint to lowercase for case-insensitive matching
	endPointLower := strings.ToLower(endPoint)

	// Check endpoint domain first (more specific)
	for domain, provider := range providerDomains {
		if strings.Contains(endPointLower, domain) {
			return provider
		}
	}

	// If endpoint didn't match, try model name patterns
	// This handles Chinese models hosted on non-Chinese platforms
	if len(modelName) > 0 && modelName[0] != "" {
		modelNameLower := strings.ToLower(modelName[0])
		for pattern, provider := range modelPatterns {
			if strings.Contains(modelNameLower, pattern) {
				return provider
			}
		}
	}

	return ModelOpenAICompatible
}
