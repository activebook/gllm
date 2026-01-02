package service

import (
	"strings"
)

const (
	// Model types
	ModelProviderGemini           string = "gemini" // for google gemini models
	ModelProviderOpenAI           string = "openai"
	ModelProviderOpenAICompatible string = "openchat"
	ModelProviderAnthropic        string = "anthropic" // for anthropic models (official sdk)
	ModelProviderUnknown          string = "unknown"
)

// Provider domains mapping for better performance and maintainability
var providerDomains = map[string]string{
	// Google domains
	"googleapis.com": ModelProviderGemini,
	"google.com":     ModelProviderGemini,
	"ai.google.dev":  ModelProviderGemini,

	// Explicit OpenAI domains
	"openai.com":       ModelProviderOpenAI,
	"api.openai.com":   ModelProviderOpenAI,
	"azure.com":        ModelProviderOpenAI,
	"azure-api.net":    ModelProviderOpenAI,
	"openai.azure.com": ModelProviderOpenAI,
	"modelscope.ai":    ModelProviderOpenAI,
	"zenmux.ai":        ModelProviderOpenAI,

	// Anthropic domains
	"anthropic.com":     ModelProviderAnthropic,
	"api.anthropic.com": ModelProviderAnthropic,

	// Chinese/Other domains
	".cn":              ModelProviderOpenAICompatible,
	"aliyuncs.com":     ModelProviderOpenAICompatible,
	"volces.com":       ModelProviderOpenAICompatible,
	"tencentcloud.com": ModelProviderOpenAICompatible,
	"longcat.chat":     ModelProviderOpenAICompatible,
	"moonshot.cn":      ModelProviderOpenAICompatible,
	"moonshot.ai":      ModelProviderOpenAICompatible,
	"bigmodel.cn":      ModelProviderOpenAICompatible,
	"z.ai":             ModelProviderOpenAICompatible,
	"minimax.io":       ModelProviderOpenAICompatible,
	"minimax.com":      ModelProviderOpenAICompatible,
	"baidu.com":        ModelProviderOpenAICompatible,
	"deepseek.com":     ModelProviderOpenAICompatible,
	"modelscope.cn":    ModelProviderOpenAICompatible,
}

// Model name patterns for Chinese models (used when endpoint doesn't match)
var modelPatterns = map[string]string{
	"qwen":     ModelProviderOpenAICompatible, // Alibaba Qwen models
	"deepseek": ModelProviderOpenAICompatible, // DeepSeek models
	"xiaomi":   ModelProviderOpenAICompatible, // Xiaom models
	"mimo":     ModelProviderOpenAICompatible, // Xiaom models
	"meituan":  ModelProviderOpenAICompatible, // Meituan Longcat models
	"longcat":  ModelProviderOpenAICompatible, // Meituan Longcat models
	"minimax":  ModelProviderOpenAICompatible, // MiniMax models
	"kimi":     ModelProviderOpenAICompatible, // Moonshot Kimi models
	"moonshot": ModelProviderOpenAICompatible, // Moonshot models
	"glm":      ModelProviderOpenAICompatible, // Zhipu GLM models
	"chatglm":  ModelProviderOpenAICompatible, // Zhipu ChatGLM models
	"ernie":    ModelProviderOpenAICompatible, // Baidu ERNIE models
	"hunyuan":  ModelProviderOpenAICompatible, // Tencent Hunyuan models
	"doubao":   ModelProviderOpenAICompatible, // ByteDance Doubao models
	"skylark":  ModelProviderOpenAICompatible, // ByteDance Skylark models
	"kat-":     ModelProviderOpenAICompatible, // Kuaishou Kat models (with hyphen to avoid false matches)
	"kat_":     ModelProviderOpenAICompatible, // Kuaishou Kat models (with underscore)
	"abab":     ModelProviderOpenAICompatible, // MiniMax ABAB models
	"yi-":      ModelProviderOpenAICompatible, // 01.AI Yi models (with hyphen to avoid false matches)
	"yi_":      ModelProviderOpenAICompatible, // 01.AI Yi models (with underscore)

	"claude": ModelProviderAnthropic, // Anthropic Claude models
}

// DetectModelProvider detects the model provider based on endpoint and model name.
// It first checks the endpoint domain, then falls back to model name patterns.
// This dual detection handles Chinese models hosted on US platforms (AWS, CoreWeave, etc.)
func DetectModelProvider(endPoint string, modelName string) string {
	if endPoint == "" && len(modelName) == 0 {
		Debugf("Model Provider[%s] - Model[%s]\n", ModelProviderOpenAICompatible, modelName)
		return ModelProviderOpenAICompatible
	}

	// Normalize endpoint to lowercase for case-insensitive matching
	endPointLower := strings.ToLower(endPoint)

	// Check endpoint domain first (more specific)
	for domain, provider := range providerDomains {
		if strings.Contains(endPointLower, domain) {
			Debugf("Model Provider[%s] - Model[%s]\n", provider, modelName)
			return provider
		}
	}

	// If endpoint didn't match, try model name patterns
	// This handles Chinese models hosted on non-Chinese platforms
	if modelName != "" {
		modelNameLower := strings.ToLower(modelName)
		for pattern, provider := range modelPatterns {
			if strings.Contains(modelNameLower, pattern) {
				Debugf("Model Provider[%s] - Model[%s]\n", provider, modelName)
				return provider
			}
		}
	}

	Debugf("Model Provider[%s] - Model[%s]\n", ModelProviderOpenAICompatible, modelName)
	return ModelProviderOpenAICompatible
}
