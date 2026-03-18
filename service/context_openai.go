package service

import (
	"sort"

	"github.com/activebook/gllm/util"
	openai "github.com/openai/openai-go/v3"
)

// openAIContext implements ContextManager for the OpenAI provider.
type openAIContext struct {
	commonContext
}

// PruneMessages trims the OpenAI message history to fit within the context window.
// extra[0] may be a string systemPrompt for token overhead accounting.
// extra[1] may optionally carry []openai.ChatCompletionToolUnionParam for tool-token accounting.
func (c *openAIContext) PruneMessages(messages any, extra ...any) (any, bool, error) {
	msgs := messages.([]openai.ChatCompletionMessageParamUnion)
	var systemPrompt string
	var tools []openai.ChatCompletionToolUnionParam
	if len(extra) > 0 {
		if s, ok := extra[0].(string); ok {
			systemPrompt = s
		}
	}
	if len(extra) > 1 {
		if t, ok := extra[1].([]openai.ChatCompletionToolUnionParam); ok {
			tools = t
		}
	}
	return c.pruneOpenAIMessages(msgs, systemPrompt, tools)
}

func (c *openAIContext) pruneOpenAIMessages(messages []openai.ChatCompletionMessageParamUnion, systemPrompt string, tools []openai.ChatCompletionToolUnionParam) ([]openai.ChatCompletionMessageParamUnion, bool, error) {
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

func (c *openAIContext) estimateTokens(messages []openai.ChatCompletionMessageParamUnion) int {
	cache := GetGlobalTokenCache()
	total := 0
	for _, msg := range messages {
		total += cache.GetOrComputeOpenAITokens(msg)
	}
	return total + MessageOverheadTokens
}

func (c *openAIContext) truncate(messages []openai.ChatCompletionMessageParamUnion, totalOverhead int) ([]openai.ChatCompletionMessageParamUnion, bool) {
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
			role := msg.GetRole()
			isTool := role != nil && *role == "tool"
			if !isTool && len(msg.GetToolCalls()) == 0 {
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

func (c *openAIContext) findToolPair(messages []openai.ChatCompletionMessageParamUnion, index int) []int {
	msg := messages[index]
	role := msg.GetRole()
	isTool := role != nil && *role == "tool"
	toolCallID := msg.GetToolCallID()

	if isTool && toolCallID != nil && *toolCallID != "" {
		for i := 0; i < index; i++ {
			for _, call := range messages[i].GetToolCalls() {
				if call.OfFunction != nil && call.OfFunction.ID == *toolCallID {
					return c.gatherToolPair(messages, i)
				}
			}
		}
	}
	if len(msg.GetToolCalls()) > 0 {
		return c.gatherToolPair(messages, index)
	}
	return nil
}

func (c *openAIContext) gatherToolPair(messages []openai.ChatCompletionMessageParamUnion, callIndex int) []int {
	indices := []int{callIndex}
	callMsg := messages[callIndex]
	for _, call := range callMsg.GetToolCalls() {
		for j := callIndex + 1; j < len(messages); j++ {
			tID := messages[j].GetToolCallID()
			if call.OfFunction != nil && tID != nil && *tID == call.OfFunction.ID {
				indices = append(indices, j)
			}
		}
	}
	sort.Ints(indices)
	return indices
}
