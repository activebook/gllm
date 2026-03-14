package service

import (
	"sort"

	"github.com/activebook/gllm/util"
	openai "github.com/sashabaranov/go-openai"
)

// openAIContext implements ContextManager for the OpenAI provider.
type openAIContext struct {
	commonContext
}

// PruneMessages trims the OpenAI message history to fit within the context window.
// extra[0] may be a string systemPrompt for token overhead accounting.
// extra[1] may optionally carry []openai.Tool for tool-token accounting.
func (c *openAIContext) PruneMessages(messages any, extra ...any) (any, bool, error) {
	msgs := messages.([]openai.ChatCompletionMessage)
	var systemPrompt string
	var tools []openai.Tool
	if len(extra) > 0 {
		if s, ok := extra[0].(string); ok {
			systemPrompt = s
		}
	}
	if len(extra) > 1 {
		if t, ok := extra[1].([]openai.Tool); ok {
			tools = t
		}
	}
	return c.pruneOpenAIMessages(msgs, systemPrompt, tools)
}

func (c *openAIContext) pruneOpenAIMessages(messages []openai.ChatCompletionMessage, systemPrompt string, tools []openai.Tool) ([]openai.ChatCompletionMessage, bool, error) {
	if c.strategy == StrategyNone || len(messages) == 0 {
		return messages, false, nil
	}

	// System prompt and tools are fixed overhead, not part of the prunable message history.
	sysTokens := 0
	if systemPrompt != "" {
		sysTokens = EstimateTokens(systemPrompt) + MessageOverheadTokens
	}
	toolTokens := EstimateOpenAIToolTokens(tools)
	totalOverhead := sysTokens + toolTokens

	currentTokens := c.estimateTokens(messages) + totalOverhead
	util.Debugf("Token count: %d MaxInputTokens[80%%]: %d\n", currentTokens, c.maxInputTokens)
	if currentTokens <= c.maxInputTokens {
		return messages, false, nil
	}

	switch c.strategy {
	case StrategySummarize:
		compressed, err := compressOpenAIMessages(c.agent, messages)
		return compressed, true, err
	case StrategyTruncateOldest:
		compressed, truncated := c.truncate(messages, totalOverhead)
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

func (c *openAIContext) truncate(messages []openai.ChatCompletionMessage, totalOverhead int) ([]openai.ChatCompletionMessage, bool) {
	if len(messages) == 0 {
		return messages, false
	}

	// Messages are pure dialogue (no system message) — the system cost is already
	// captured in totalOverhead passed by the caller.
	availableTokens := c.maxInputTokens - totalOverhead

	cache := GetGlobalTokenCache()
	tokenCounts := make([]int, len(messages))
	historyTokens := 0
	for i, msg := range messages {
		tokenCounts[i] = cache.GetOrComputeOpenAITokens(msg)
		historyTokens += tokenCounts[i]
	}

	truncated := false
	for historyTokens > availableTokens && len(messages) > 0 {
		removed := false
		for i := 0; i < len(messages); i++ {
			msg := messages[i]
			if pairIndices := c.findToolPair(messages, i); len(pairIndices) > 0 {
				tokensRemoved := 0
				for j := len(pairIndices) - 1; j >= 0; j-- {
					idx := pairIndices[j]
					tokensRemoved += tokenCounts[idx]
					messages = append(messages[:idx], messages[idx+1:]...)
					tokenCounts = append(tokenCounts[:idx], tokenCounts[idx+1:]...)
				}
				historyTokens -= tokensRemoved
				truncated = true
				removed = true
				break
			}
			if msg.Role != openai.ChatMessageRoleTool && len(msg.ToolCalls) == 0 {
				historyTokens -= tokenCounts[i]
				messages = append(messages[:i], messages[i+1:]...)
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

	return messages, truncated
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
	sort.Ints(indices)
	return indices
}
