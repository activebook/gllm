package service

import (
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

// openAIContext implements ContextManager for the OpenAI provider.
type openAIContext struct {
	commonContext
}

// PruneMessages trims the OpenAI message history to fit within the context window.
// extra[0] may optionally carry []openai.Tool for tool-token accounting.
func (c *openAIContext) PruneMessages(messages any, extra ...any) (any, bool, error) {
	msgs := messages.([]openai.ChatCompletionMessage)
	var tools []openai.Tool
	if len(extra) > 0 {
		if t, ok := extra[0].([]openai.Tool); ok {
			tools = t
		}
	}
	return c.pruneOpenAIMessages(msgs, tools)
}

func (c *openAIContext) pruneOpenAIMessages(messages []openai.ChatCompletionMessage, tools []openai.Tool) ([]openai.ChatCompletionMessage, bool, error) {
	if c.strategy == StrategyNone || len(messages) == 0 {
		return messages, false, nil
	}

	toolTokens := EstimateOpenAIToolTokens(tools)
	currentTokens := c.estimateTokens(messages) + toolTokens
	Debugf("Token count: %d MaxInputTokens[80%%]: %d", currentTokens, c.maxInputTokens)
	if currentTokens <= c.maxInputTokens {
		return messages, false, nil
	}

	switch c.strategy {
	case StrategySummarize:
		compressed, err := compressOpenAIMessages(c.agent, messages)
		return compressed, true, err
	case StrategyTruncateOldest:
		compressed, truncated := c.truncate(messages, toolTokens)
		return compressed, truncated, nil
	default:
		return messages, false, nil
	}
}

func (c *openAIContext) estimateTokens(messages []openai.ChatCompletionMessage) int {
	cache := GetGlobalTokenCache()
	total := 0
	for _, msg := range messages {
		total += cache.GetOrComputeOpenAITokens(msg)
	}
	return total + MessageOverheadTokens
}

func (c *openAIContext) truncate(messages []openai.ChatCompletionMessage, toolTokens int) ([]openai.ChatCompletionMessage, bool) {
	if len(messages) == 0 {
		return messages, false
	}

	// Segregate system vs non-system messages
	var systemMsgs, nonSystemMsgs []openai.ChatCompletionMessage
	for _, msg := range messages {
		if msg.Role == openai.ChatMessageRoleSystem {
			systemMsgs = append(systemMsgs, msg)
		} else {
			nonSystemMsgs = append(nonSystemMsgs, msg)
		}
	}

	// Consolidate multiple system messages into one
	if len(systemMsgs) > 1 {
		for i := 1; i < len(systemMsgs); i++ {
			if !strings.Contains(systemMsgs[0].Content, systemMsgs[i].Content) {
				systemMsgs[0].Content += "\n" + systemMsgs[i].Content
			}
		}
		systemMsgs = systemMsgs[:1]
	}

	systemTokens := c.estimateTokens(systemMsgs)
	availableTokens := c.maxInputTokens - systemTokens - toolTokens

	cache := GetGlobalTokenCache()
	tokenCounts := make([]int, len(nonSystemMsgs))
	nonSystemTokens := 0
	for i, msg := range nonSystemMsgs {
		tokenCounts[i] = cache.GetOrComputeOpenAITokens(msg)
		nonSystemTokens += tokenCounts[i]
	}

	truncated := false
	for nonSystemTokens > availableTokens && len(nonSystemMsgs) > 0 {
		removed := false
		for i := 0; i < len(nonSystemMsgs); i++ {
			msg := nonSystemMsgs[i]
			if pairIndices := c.findToolPair(nonSystemMsgs, i); len(pairIndices) > 0 {
				tokensRemoved := 0
				for j := len(pairIndices) - 1; j >= 0; j-- {
					idx := pairIndices[j]
					tokensRemoved += tokenCounts[idx]
					nonSystemMsgs = append(nonSystemMsgs[:idx], nonSystemMsgs[idx+1:]...)
					tokenCounts = append(tokenCounts[:idx], tokenCounts[idx+1:]...)
				}
				nonSystemTokens -= tokensRemoved
				truncated = true
				removed = true
				break
			}
			if msg.Role != openai.ChatMessageRoleTool && len(msg.ToolCalls) == 0 {
				nonSystemTokens -= tokenCounts[i]
				nonSystemMsgs = append(nonSystemMsgs[:i], nonSystemMsgs[i+1:]...)
				tokenCounts = append(tokenCounts[:i], tokenCounts[i+1:]...)
				truncated = true
				removed = true
				break
			}
		}
		if !removed {
			break
		}
	}

	result := make([]openai.ChatCompletionMessage, 0, len(systemMsgs)+len(nonSystemMsgs))
	result = append(result, systemMsgs...)
	result = append(result, nonSystemMsgs...)
	return result, truncated
}

func (c *openAIContext) findToolPair(messages []openai.ChatCompletionMessage, index int) []int {
	msg := messages[index]
	if msg.Role == openai.ChatMessageRoleTool && msg.ToolCallID != "" {
		for i := 0; i < index; i++ {
			for _, call := range messages[i].ToolCalls {
				if call.ID == msg.ToolCallID {
					return c.gatherToolPair(messages, i)
				}
			}
		}
	}
	if len(msg.ToolCalls) > 0 {
		return c.gatherToolPair(messages, index)
	}
	return nil
}

func (c *openAIContext) gatherToolPair(messages []openai.ChatCompletionMessage, callIndex int) []int {
	indices := []int{callIndex}
	callMsg := messages[callIndex]
	for _, call := range callMsg.ToolCalls {
		for j := callIndex + 1; j < len(messages); j++ {
			if messages[j].ToolCallID == call.ID {
				indices = append(indices, j)
			}
		}
	}
	return indices
}
