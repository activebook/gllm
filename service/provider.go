package service

import (
	"context"
	"strings"
)

const (
	// Model types
	ModelGemini           = "gemini" // for google gemini models
	ModelOpenAI           = "openai"
	ModelOpenChat         = "openchat" // for chinese models
	ModelOpenAICompatible = "openai-compatible"
	ModelMistral          = "mistral" // for mistral models
	ModelUnknown          = "unknown"
)

func DetectModelProvider(endPoint string) string {
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
	chn_domains := []string{".cn", "aliyuncs.com", "volces.com", "tencentcloud.com", "moonshot.cn", "moonshot.ai", "bigmodel.cn", "z.ai", "minimax.io", "minimax.com", "baidu.com", "deepseek.com"}
	for _, domain := range chn_domains {
		if strings.Contains(endPoint, domain) {
			return ModelOpenChat
		}
	}

	return ModelOpenAICompatible
}

// OpenProcessor is the main processor for OpenAI-like models
// For tools implementation
// - It manages the context, notifications, data streaming, and tool usage
// - It handles queries and references, and maintains the status stack
type OpenProcessor struct {
	ctx        context.Context
	notify     chan<- StreamNotify      // Sub Channel to send notifications
	data       chan<- StreamData        // Sub Channel to send data
	proceed    <-chan bool              // Main Channel to receive proceed signal
	search     *SearchEngine            // Search engine
	toolsUse   *ToolsUse                // Use tools
	queries    []string                 // List of queries to be sent to the AI assistant
	references []map[string]interface{} // keep track of the references
	status     *StatusStack             // Stack to manage streaming status
}
