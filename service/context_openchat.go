package service

import (
	"sort"

	"github.com/activebook/gllm/util"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
)

// openChatContext implements ContextManager for the OpenChat (Volcengine) provider.
type openChatContext struct {
	commonContext
}

// PruneMessages trims the OpenChat message history to fit within the context window.
// extra[0] may be a string systemPrompt for token overhead accounting.
// extra[1] may optionally carry []*model.Tool for tool-token accounting.
func (c *openChatContext) PruneMessages(messages any, extra ...any) (any, bool, error) {
	msgs := messages.([]*model.ChatCompletionMessage)
	var systemPrompt string
	var tools []*model.Tool
	if len(extra) > 0 {
		if s, ok := extra[0].(string); ok {
			systemPrompt = s
		}
	}
	if len(extra) > 1 {
		if t, ok := extra[1].([]*model.Tool); ok {
			tools = t
		}
	}
	return c.pruneOpenChatMessages(msgs, systemPrompt, tools)
}

func (c *openChatContext) pruneOpenChatMessages(messages []*model.ChatCompletionMessage, systemPrompt string, tools []*model.Tool) ([]*model.ChatCompletionMessage, bool, error) {
	if c.strategy == StrategyNone || len(messages) == 0 {
		return messages, false, nil
	}

	// System prompt and tools are fixed overhead, not part of the prunable message history.
	sysTokens := 0
	if systemPrompt != "" {
		sysTokens = EstimateTokens(systemPrompt) + MessageOverheadTokens
	}
	toolTokens := EstimateOpenChatToolTokens(tools)
	totalOverhead := sysTokens + toolTokens

	currentTokens := c.estimateTokens(messages) + totalOverhead
	util.Debugf("Token count: %d MaxInputTokens[80%%]: %d\n", currentTokens, c.maxInputTokens)
	if currentTokens <= c.maxInputTokens {
		return messages, false, nil
	}

	switch c.strategy {
	case StrategySummarize:
		compressed, err := compressOpenChatMessages(c.agent, messages)
		return compressed, true, err
	case StrategyTruncateOldest:
		compressed, truncated := c.truncate(messages, totalOverhead)
		return compressed, truncated, nil
	default:
		return messages, false, nil
	}
}

func (c *openChatContext) estimateTokens(messages []*model.ChatCompletionMessage) int {
	cache := GetGlobalTokenCache()
	total := 0
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		total += cache.GetOrComputeOpenChatTokens(msg)
	}
	return total + MessageOverheadTokens
}

func (c *openChatContext) truncate(messages []*model.ChatCompletionMessage, totalOverhead int) ([]*model.ChatCompletionMessage, bool) {
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
		tokenCounts[i] = cache.GetOrComputeOpenChatTokens(msg)
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
			if msg.Role != model.ChatMessageRoleTool && len(msg.ToolCalls) == 0 {
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

func (c *openChatContext) findToolPair(messages []*model.ChatCompletionMessage, index int) []int {
	msg := messages[index]
	if msg.Role == model.ChatMessageRoleTool && msg.ToolCallID != "" {
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

func (c *openChatContext) gatherToolPair(messages []*model.ChatCompletionMessage, callIndex int) []int {
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
