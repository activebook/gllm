package service

import (
	"sort"

	"github.com/activebook/gllm/util"
	"google.golang.org/genai"
)

// geminiContext implements ContextManager for the Gemini provider.
type geminiContext struct {
	commonContext
}

// PruneMessages trims the Gemini content history to fit within the context window.
// extra[0] may be a string systemPrompt; extra[1] may be []*genai.Tool.
func (c *geminiContext) PruneMessages(messages any, extra ...any) (any, bool, error) {
	msgs := messages.([]*genai.Content)
	var systemPrompt string
	var tools []*genai.Tool
	if len(extra) > 0 {
		if s, ok := extra[0].(string); ok {
			systemPrompt = s
		}
	}
	if len(extra) > 1 {
		if t, ok := extra[1].([]*genai.Tool); ok {
			tools = t
		}
	}
	return c.pruneGeminiMessages(msgs, systemPrompt, tools)
}

func (c *geminiContext) pruneGeminiMessages(messages []*genai.Content, systemPrompt string, tools []*genai.Tool) ([]*genai.Content, bool, error) {
	if c.strategy == StrategyNone {
		return messages, false, nil
	}

	// System prompt and tools are passed separately in Gemini API
	sysTokens := 0
	if systemPrompt != "" {
		sysTokens = EstimateTokens(systemPrompt) + MessageOverheadTokens
	}
	toolTokens := EstimateGeminiToolTokens(tools)
	totalOverhead := sysTokens + toolTokens

	currentTokens := c.estimateTokens(messages) + totalOverhead
	util.Debugf("Token count: %d MaxInputTokens[80%%]: %d\n", currentTokens, c.maxInputTokens)
	if currentTokens <= c.maxInputTokens {
		return messages, false, nil
	}

	switch c.strategy {
	case StrategySummarize:
		compressed, err := compressGeminiMessages(c.agent, messages)
		return compressed, true, err
	case StrategyTruncateOldest:
		compressed, truncated := c.truncate(messages, totalOverhead)
		return compressed, truncated, nil
	default:
		return messages, false, nil
	}
}

func (c *geminiContext) estimateTokens(messages []*genai.Content) int {
	cache := GetGlobalTokenCache()
	total := 0
	for _, msg := range messages {
		total += cache.GetOrComputeGeminiTokens(msg)
	}
	return total + MessageOverheadTokens
}

func (c *geminiContext) truncate(messages []*genai.Content, totalOverhead int) ([]*genai.Content, bool) {
	if len(messages) == 0 {
		return messages, false
	}

	availableTokens := c.maxInputTokens - totalOverhead

	cache := GetGlobalTokenCache()
	tokenCounts := make([]int, len(messages))
	historyTokens := 0
	for i, msg := range messages {
		tokenCounts[i] = cache.GetOrComputeGeminiTokens(msg)
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
			if !c.isToolMessage(messages[i]) {
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

	// Cleanup: Ensure the truncated history starts with a valid user turn.
	// Gemini requires that a FunctionCall must come immediately after a user turn or a function response turn.
	for len(messages) > 0 {
		msg := messages[0]

		// If it's a model message, it's not a valid start.
		// If it's a FunctionCall, its corresponding FunctionResponse (if any)
		// will be removed in subsequent iterations.
		if msg.Role == "model" || msg.Role == genai.RoleModel {
			messages = messages[1:]
			truncated = true
			continue
		}

		// If it's a user message, check if it's an orphaned FunctionResponse.
		// A FunctionResponse cannot be the first message (must follow a FunctionCall).
		isFuncResp := false
		for _, part := range msg.Parts {
			if part.FunctionResponse != nil {
				isFuncResp = true
				break
			}
		}
		if isFuncResp {
			messages = messages[1:]
			truncated = true
			continue
		}

		// Valid starting user message found.
		break
	}
	return messages, truncated
}


func (c *geminiContext) isToolMessage(msg *genai.Content) bool {
	if msg == nil {
		return false
	}
	for _, part := range msg.Parts {
		if part.FunctionCall != nil || part.FunctionResponse != nil {
			return true
		}
	}
	return false
}

func (c *geminiContext) findToolPair(messages []*genai.Content, index int) []int {
	msg := messages[index]
	if msg == nil {
		return nil
	}

	// Look for function response
	var responseName string
	for _, part := range msg.Parts {
		if part.FunctionResponse != nil {
			responseName = part.FunctionResponse.Name
			break
		}
	}
	if responseName != "" {
		for i := index - 1; i >= 0; i-- {
			for _, part := range messages[i].Parts {
				if part.FunctionCall != nil && part.FunctionCall.Name == responseName {
					return c.gatherToolPair(messages, i)
				}
			}
		}
	}

	// Look for function call
	for _, part := range msg.Parts {
		if part.FunctionCall != nil {
			return c.gatherToolPair(messages, index)
		}
	}

	return nil
}

func (c *geminiContext) gatherToolPair(messages []*genai.Content, callIndex int) []int {
	indices := []int{callIndex}
	callMsg := messages[callIndex]
	if callMsg == nil {
		return indices
	}

	callNames := make(map[string]bool)
	for _, part := range callMsg.Parts {
		if part.FunctionCall != nil {
			callNames[part.FunctionCall.Name] = true
		}
	}

	for j := callIndex + 1; j < len(messages); j++ {
		msg := messages[j]
		if msg == nil {
			continue
		}
		// Stop at the next model turn — responses always live in the
		// immediately-following user turn, never further.
		if msg.Role == "model" || msg.Role == genai.RoleModel {
			break
		}
		for _, part := range msg.Parts {
			if part.FunctionResponse != nil && callNames[part.FunctionResponse.Name] {
				indices = append(indices, j)
			}
		}
	}
	sort.Ints(indices)
	return indices
}
