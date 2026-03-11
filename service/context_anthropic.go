package service

import (
	"sort"

	anthropic "github.com/anthropics/anthropic-sdk-go"
)

// anthropicContext implements ContextManager for the Anthropic provider.
type anthropicContext struct {
	commonContext
}

// PruneMessages trims the Anthropic message history to fit within the context window.
// extra[0] may be a string systemPrompt; extra[1] may be []anthropic.ToolUnionParam.
func (c *anthropicContext) PruneMessages(messages any, extra ...any) (any, bool, error) {
	msgs := messages.([]anthropic.MessageParam)
	var systemPrompt string
	var tools []anthropic.ToolUnionParam
	if len(extra) > 0 {
		if s, ok := extra[0].(string); ok {
			systemPrompt = s
		}
	}
	if len(extra) > 1 {
		if t, ok := extra[1].([]anthropic.ToolUnionParam); ok {
			tools = t
		}
	}
	return c.pruneAnthropicMessages(msgs, systemPrompt, tools)
}

func (c *anthropicContext) pruneAnthropicMessages(messages []anthropic.MessageParam, systemPrompt string, tools []anthropic.ToolUnionParam) ([]anthropic.MessageParam, bool, error) {
	if c.strategy == StrategyNone || len(messages) == 0 {
		return messages, false, nil
	}

	sysTokens := 0
	if systemPrompt != "" {
		sysTokens = EstimateTokens(systemPrompt) + MessageOverheadTokens
	}
	toolTokens := EstimateAnthropicToolTokens(tools)
	totalOverhead := sysTokens + toolTokens

	currentTokens := c.estimateTokens(messages) + totalOverhead
	Debugf("Token count: %d MaxInputTokens[80%%]: %d", currentTokens, c.maxInputTokens)
	if currentTokens <= c.maxInputTokens {
		return messages, false, nil
	}

	switch c.strategy {
	case StrategySummarize:
		compressed, err := compressAnthropicMessages(c.agent, messages)
		return compressed, true, err
	case StrategyTruncateOldest:
		compressed, truncated := c.truncate(messages, totalOverhead)
		return compressed, truncated, nil
	default:
		return messages, false, nil
	}
}

func (c *anthropicContext) estimateTokens(messages []anthropic.MessageParam) int {
	cache := GetGlobalTokenCache()
	total := 0
	for _, msg := range messages {
		total += cache.GetOrComputeAnthropicTokens(msg)
	}
	return total + MessageOverheadTokens
}

func (c *anthropicContext) truncate(messages []anthropic.MessageParam, totalOverhead int) ([]anthropic.MessageParam, bool) {
	if len(messages) == 0 {
		return messages, false
	}

	availableTokens := c.maxInputTokens - totalOverhead

	cache := GetGlobalTokenCache()
	tokenCounts := make([]int, len(messages))
	historyTokens := 0
	for i, msg := range messages {
		tokenCounts[i] = cache.GetOrComputeAnthropicTokens(msg)
		historyTokens += tokenCounts[i]
	}

	truncated := false
	for historyTokens > availableTokens && len(messages) > 0 {
		removed := false
		for i := 0; i < len(messages); i++ {
			if pairIndices := c.findToolPair(messages, i); len(pairIndices) > 0 {
				tokensRemoved := 0
				for j := len(pairIndices) - 1; j >= 0; j-- {
					idx := pairIndices[j]
					if idx < len(tokenCounts) {
						tokensRemoved += tokenCounts[idx]
						messages = append(messages[:idx], messages[idx+1:]...)
						tokenCounts = append(tokenCounts[:idx], tokenCounts[idx+1:]...)
					}
				}
				historyTokens -= tokensRemoved
				truncated = true
				removed = true
				break
			}
			// Regular message — safe to remove
			historyTokens -= tokenCounts[i]
			messages = append(messages[:i], messages[i+1:]...)
			tokenCounts = append(tokenCounts[:i], tokenCounts[i+1:]...)
			truncated = true
			removed = true
			break
		}
		if !removed {
			break
		}
	}

	return messages, truncated
}

// findToolPair returns the indices of the assistant tool-use message and its
// accompanying user tool-result messages at the given index.
func (c *anthropicContext) findToolPair(messages []anthropic.MessageParam, index int) []int {
	if index >= len(messages) {
		return nil
	}
	msg := messages[index]

	toolUseIDs := make(map[string]bool)
	var toolResultIDs []string
	for _, block := range msg.Content {
		if block.OfToolUse != nil {
			toolUseIDs[block.OfToolUse.ID] = true
		}
		if block.OfToolResult != nil {
			toolResultIDs = append(toolResultIDs, block.OfToolResult.ToolUseID)
		}
	}

	// Case 1: This is a User message containing tool results — find the preceding assistant call
	if len(toolResultIDs) > 0 && msg.Role == anthropic.MessageParamRoleUser {
		for i := index - 1; i >= 0; i-- {
			prev := messages[i]
			if prev.Role != anthropic.MessageParamRoleAssistant {
				continue
			}
			candidateIDs := make(map[string]bool)
			for _, b := range prev.Content {
				if b.OfToolUse != nil {
					candidateIDs[b.OfToolUse.ID] = true
				}
			}
			matched := false
			for _, rid := range toolResultIDs {
				if candidateIDs[rid] {
					matched = true
					break
				}
			}
			if matched {
				return c.gatherToolPair(messages, i, candidateIDs)
			}
		}
		// Orphan result — remove alone
		return []int{index}
	}

	// Case 2: This is an Assistant message with tool calls — gather its results
	if len(toolUseIDs) > 0 && msg.Role == anthropic.MessageParamRoleAssistant {
		return c.gatherToolPair(messages, index, toolUseIDs)
	}

	return nil
}

func (c *anthropicContext) gatherToolPair(messages []anthropic.MessageParam, callIndex int, callIDs map[string]bool) []int {
	indices := []int{callIndex}
	for j := callIndex + 1; j < len(messages); j++ {
		nextMsg := messages[j]
		if nextMsg.Role != anthropic.MessageParamRoleUser {
			break // Tool results immediately follow the assistant message
		}
		for _, b := range nextMsg.Content {
			if b.OfToolResult != nil && callIDs[b.OfToolResult.ToolUseID] {
				indices = append(indices, j)
				break
			}
		}
	}
	sort.Ints(indices)
	return indices
}
