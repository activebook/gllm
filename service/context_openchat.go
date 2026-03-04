package service

import (
	"strings"

	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
)

// openChatContext implements ContextManager for the OpenChat (Volcengine) provider.
type openChatContext struct {
	commonContext
}

// PruneMessages trims the OpenChat message history to fit within the context window.
// extra[0] may optionally carry []*model.Tool for tool-token accounting.
func (c *openChatContext) PruneMessages(messages any, extra ...any) (any, bool, error) {
	msgs := messages.([]*model.ChatCompletionMessage)
	var tools []*model.Tool
	if len(extra) > 0 {
		if t, ok := extra[0].([]*model.Tool); ok {
			tools = t
		}
	}
	return c.pruneOpenChatMessages(msgs, tools)
}

func (c *openChatContext) pruneOpenChatMessages(messages []*model.ChatCompletionMessage, tools []*model.Tool) ([]*model.ChatCompletionMessage, bool, error) {
	if c.strategy == StrategyNone || len(messages) == 0 {
		return messages, false, nil
	}

	toolTokens := EstimateOpenChatToolTokens(tools)
	currentTokens := c.estimateTokens(messages) + toolTokens
	Debugf("Token count: %d MaxInputTokens[80%%]: %d", currentTokens, c.maxInputTokens)
	if currentTokens <= c.maxInputTokens {
		return messages, false, nil
	}

	switch c.strategy {
	case StrategySummarize:
		compressed, err := compressOpenChatMessages(c.agent, messages)
		return compressed, true, err
	case StrategyTruncateOldest:
		compressed, truncated := c.truncate(messages, toolTokens)
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

func (c *openChatContext) truncate(messages []*model.ChatCompletionMessage, toolTokens int) ([]*model.ChatCompletionMessage, bool) {
	if len(messages) == 0 {
		return messages, false
	}

	var systemMsgs, nonSystemMsgs []*model.ChatCompletionMessage
	for _, msg := range messages {
		if msg.Role == model.ChatMessageRoleSystem {
			systemMsgs = append(systemMsgs, msg)
		} else {
			nonSystemMsgs = append(nonSystemMsgs, msg)
		}
	}

	// Consolidate multiple system messages into one
	if len(systemMsgs) > 1 {
		var combinedContent string
		if systemMsgs[0].Content != nil && systemMsgs[0].Content.StringValue != nil {
			combinedContent = *systemMsgs[0].Content.StringValue
		}
		for i := 1; i < len(systemMsgs); i++ {
			if systemMsgs[i].Content != nil && systemMsgs[i].Content.StringValue != nil {
				newSys := *systemMsgs[i].Content.StringValue
				if !strings.Contains(combinedContent, newSys) {
					combinedContent += "\n" + newSys
				}
			}
		}
		if systemMsgs[0].Content == nil {
			systemMsgs[0].Content = &model.ChatCompletionMessageContent{}
		}
		systemMsgs[0].Content.StringValue = &combinedContent
		systemMsgs = systemMsgs[:1]
	}

	systemTokens := c.estimateTokens(systemMsgs)
	availableTokens := c.maxInputTokens - systemTokens - toolTokens

	cache := GetGlobalTokenCache()
	tokenCounts := make([]int, len(nonSystemMsgs))
	nonSystemTokens := 0
	for i, msg := range nonSystemMsgs {
		tokenCounts[i] = cache.GetOrComputeOpenChatTokens(msg)
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
			if msg.Role != model.ChatMessageRoleTool && len(msg.ToolCalls) == 0 {
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

	result := make([]*model.ChatCompletionMessage, 0, len(systemMsgs)+len(nonSystemMsgs))
	result = append(result, systemMsgs...)
	result = append(result, nonSystemMsgs...)
	return result, truncated
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
	return indices
}
