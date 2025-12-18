package service

import (
	openai "github.com/sashabaranov/go-openai"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
)

// TruncationStrategy defines how to handle context overflow
type TruncationStrategy string

const (
	// StrategyTruncateOldest removes oldest messages first, preserving system prompt
	StrategyTruncateOldest TruncationStrategy = "truncate_oldest"

	// StrategySummarize replaces old context with a summary (future implementation)
	StrategySummarize TruncationStrategy = "summarize"

	// StrategyNone disables truncation - will fail if context exceeds limit
	StrategyNone TruncationStrategy = "none"

	// DefaultBufferPercent is the default safety buffer (80% of available space)
	DefaultBufferPercent = 0.80
)

// ContextManager handles context window limits for LLM conversations
type ContextManager struct {
	MaxInputTokens int                // Maximum input tokens allowed
	Strategy       TruncationStrategy // Strategy for handling overflow
	BufferPercent  float64            // Safety buffer (0.0-1.0)
}

// NewContextManager creates a context manager with the given model limits
func NewContextManager(limits ModelLimits, strategy TruncationStrategy) *ContextManager {
	bufferPercent := DefaultBufferPercent
	return &ContextManager{
		MaxInputTokens: limits.MaxInputTokens(bufferPercent),
		Strategy:       strategy,
		BufferPercent:  bufferPercent,
	}
}

// NewContextManagerForModel creates a context manager by looking up the model name
func NewContextManagerForModel(modelName string, strategy TruncationStrategy) *ContextManager {
	limits := GetModelLimits(modelName)
	return NewContextManager(limits, strategy)
}

// =============================================================================
// OpenAI Message Handling
// =============================================================================

// PrepareOpenAIMessages processes messages to fit within context window limits.
// Returns the processed messages and a boolean indicating if truncation occurred.
func (cm *ContextManager) PrepareOpenAIMessages(messages []openai.ChatCompletionMessage) ([]openai.ChatCompletionMessage, bool) {
	if cm.Strategy == StrategyNone || len(messages) == 0 {
		return messages, false
	}

	// Calculate current token usage using cache
	currentTokens := cm.estimateOpenAIMessagesWithCache(messages)
	// Test Only
	// fmt.Println("Token count: ", currentTokens, " MaxInputTokens: ", cm.MaxInputTokens)
	if currentTokens <= cm.MaxInputTokens {
		return messages, false
	}

	// Need to truncate
	return cm.truncateOpenAIMessages(messages)
}

// estimateOpenAIMessagesWithCache uses global cache for token estimation
func (cm *ContextManager) estimateOpenAIMessagesWithCache(messages []openai.ChatCompletionMessage) int {
	cache := GetGlobalTokenCache()
	total := 0
	for _, msg := range messages {
		total += cache.GetOrComputeOpenAITokens(msg)
	}
	return total + 3 // priming tokens
}

// truncateOpenAIMessages removes oldest messages while preserving critical ones.
// Removes tool call/response pairs together.
func (cm *ContextManager) truncateOpenAIMessages(messages []openai.ChatCompletionMessage) ([]openai.ChatCompletionMessage, bool) {
	if len(messages) == 0 {
		return messages, false
	}

	// Identify system messages to preserve (First and Last)
	var systemMsgs []openai.ChatCompletionMessage
	var nonSystemMsgs []openai.ChatCompletionMessage

	// Triage messages to system and non-system
	for _, msg := range messages {
		if msg.Role == openai.ChatMessageRoleSystem {
			systemMsgs = append(systemMsgs, msg)
		} else {
			nonSystemMsgs = append(nonSystemMsgs, msg)
		}
	}

	// Keep the first system message (Identity/Base)
	// AND keep the last system message (Current Instruction/Update)
	if len(systemMsgs) > 1 {
		systemMsgs = []openai.ChatCompletionMessage{systemMsgs[0], systemMsgs[len(systemMsgs)-1]}
	}

	// Calculate system message tokens (these are always kept)
	systemTokens := cm.estimateOpenAIMessagesWithCache(systemMsgs)

	// Available tokens for non-system messages
	availableTokens := cm.MaxInputTokens - systemTokens

	// Build token cache for non-system messages (array indexed)
	cache := GetGlobalTokenCache()
	tokenCounts := make([]int, len(nonSystemMsgs))
	nonSystemTokens := 0
	for i, msg := range nonSystemMsgs {
		tokenCounts[i] = cache.GetOrComputeOpenAITokens(msg)
		nonSystemTokens += tokenCounts[i]
	}

	// Remove oldest messages until we fit, handling tool pairs together
	truncated := false
	for nonSystemTokens > availableTokens && len(nonSystemMsgs) > 0 {
		// Try to find and remove the oldest removable item (could be a tool pair)
		removed := false

		for i := 0; i < len(nonSystemMsgs); i++ {
			msg := nonSystemMsgs[i]

			// Check if this is part of a tool call/response pair
			pairIndices := cm.findToolPairOpenAI(nonSystemMsgs, i)
			if len(pairIndices) > 0 {
				// Remove entire pair together (from highest index to lowest to maintain indices)
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

			// Regular message (not part of tool pair)
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
			// Can't remove any more messages safely
			break
		}
	}

	// Reassemble messages
	result := make([]openai.ChatCompletionMessage, 0, len(systemMsgs)+len(nonSystemMsgs))
	result = append(result, systemMsgs...)
	result = append(result, nonSystemMsgs...)

	return result, truncated
}

// findToolPairOpenAI finds all indices that form a tool call/response pair with the message at index
// Returns empty slice if the message is not part of a pair
func (cm *ContextManager) findToolPairOpenAI(messages []openai.ChatCompletionMessage, index int) []int {
	msg := messages[index]

	// If this is a tool response, find its call
	if msg.Role == openai.ChatMessageRoleTool && msg.ToolCallID != "" {
		for i := 0; i < index; i++ {
			for _, call := range messages[i].ToolCalls {
				if call.ID == msg.ToolCallID {
					// Found the pair - gather all responses to this call's parent
					return cm.gatherToolPairOpenAI(messages, i)
				}
			}
		}
	}

	// If this is a message with tool calls, gather all its responses
	if len(msg.ToolCalls) > 0 {
		return cm.gatherToolPairOpenAI(messages, index)
	}

	return nil
}

// gatherToolPairOpenAI gathers the tool call message and all its responses
func (cm *ContextManager) gatherToolPairOpenAI(messages []openai.ChatCompletionMessage, callIndex int) []int {
	indices := []int{callIndex}

	callMsg := messages[callIndex]
	for _, call := range callMsg.ToolCalls {
		// Find all responses to this call
		for j := callIndex + 1; j < len(messages); j++ {
			if messages[j].ToolCallID == call.ID {
				indices = append(indices, j)
			}
		}
	}

	return indices
}

// =============================================================================
// OpenChat Message Handling (same logic, different types)
// =============================================================================

// PrepareOpenChatMessages processes messages to fit within context window limits for OpenChat format.
func (cm *ContextManager) PrepareOpenChatMessages(messages []*model.ChatCompletionMessage) ([]*model.ChatCompletionMessage, bool) {
	if cm.Strategy == StrategyNone || len(messages) == 0 {
		return messages, false
	}

	// Calculate current token usage using cache
	currentTokens := cm.estimateOpenChatMessagesWithCache(messages)
	// Test Only
	// fmt.Println("Token count: ", currentTokens, " MaxInputTokens: ", cm.MaxInputTokens)
	if currentTokens <= cm.MaxInputTokens {
		return messages, false
	}

	// Need to truncate
	return cm.truncateOpenChatMessages(messages)
}

// estimateOpenChatMessagesWithCache uses global cache for token estimation
func (cm *ContextManager) estimateOpenChatMessagesWithCache(messages []*model.ChatCompletionMessage) int {
	cache := GetGlobalTokenCache()
	total := 0
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		total += cache.GetOrComputeOpenChatTokens(msg)
	}
	return total + 3 // priming tokens
}

// truncateOpenChatMessages removes oldest messages while preserving critical ones.
// Removes tool call/response pairs together.
func (cm *ContextManager) truncateOpenChatMessages(messages []*model.ChatCompletionMessage) ([]*model.ChatCompletionMessage, bool) {
	if len(messages) == 0 {
		return messages, false
	}

	// Identify system messages to preserve (First and Last)
	var systemMsgs []*model.ChatCompletionMessage
	var nonSystemMsgs []*model.ChatCompletionMessage

	// Triage messages to system and non-system
	for _, msg := range messages {
		if msg.Role == model.ChatMessageRoleSystem {
			systemMsgs = append(systemMsgs, msg)
		} else {
			nonSystemMsgs = append(nonSystemMsgs, msg)
		}
	}

	// Keep the first system message (Identity/Base)
	// AND keep the last system message (Current Instruction/Update)
	if len(systemMsgs) > 1 {
		systemMsgs = []*model.ChatCompletionMessage{systemMsgs[0], systemMsgs[len(systemMsgs)-1]}
	}

	// Calculate system message tokens
	systemTokens := cm.estimateOpenChatMessagesWithCache(systemMsgs)
	availableTokens := cm.MaxInputTokens - systemTokens

	// Build token cache for non-system messages
	cache := GetGlobalTokenCache()
	tokenCounts := make([]int, len(nonSystemMsgs))
	nonSystemTokens := 0
	for i, msg := range nonSystemMsgs {
		tokenCounts[i] = cache.GetOrComputeOpenChatTokens(msg)
		nonSystemTokens += tokenCounts[i]
	}

	// Remove oldest messages until we fit, handling tool pairs together
	truncated := false
	for nonSystemTokens > availableTokens && len(nonSystemMsgs) > 0 {
		removed := false

		for i := 0; i < len(nonSystemMsgs); i++ {
			msg := nonSystemMsgs[i]

			// Check if this is part of a tool call/response pair
			pairIndices := cm.findToolPairOpenChat(nonSystemMsgs, i)
			if len(pairIndices) > 0 {
				// Remove entire pair together
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

			// Regular message (not part of tool pair)
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

	// Reassemble messages
	result := make([]*model.ChatCompletionMessage, 0, len(systemMsgs)+len(nonSystemMsgs))
	result = append(result, systemMsgs...)
	result = append(result, nonSystemMsgs...)

	return result, truncated
}

// findToolPairOpenChat finds all indices that form a tool call/response pair
func (cm *ContextManager) findToolPairOpenChat(messages []*model.ChatCompletionMessage, index int) []int {
	msg := messages[index]

	// If this is a tool response, find its call
	if msg.Role == model.ChatMessageRoleTool && msg.ToolCallID != "" {
		for i := 0; i < index; i++ {
			for _, call := range messages[i].ToolCalls {
				if call.ID == msg.ToolCallID {
					return cm.gatherToolPairOpenChat(messages, i)
				}
			}
		}
	}

	// If this has tool calls, gather all its responses
	if len(msg.ToolCalls) > 0 {
		return cm.gatherToolPairOpenChat(messages, index)
	}

	return nil
}

// gatherToolPairOpenChat gathers the tool call message and all its responses
func (cm *ContextManager) gatherToolPairOpenChat(messages []*model.ChatCompletionMessage, callIndex int) []int {
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

// =============================================================================
// Utility Functions
// =============================================================================

// GetCurrentTokenCount returns the current token count for OpenAI messages
func GetCurrentTokenCount(messages []openai.ChatCompletionMessage) int {
	return EstimateOpenAIMessagesTokens(messages)
}

// GetCurrentTokenCountOpenChat returns the current token count for OpenChat messages
func GetCurrentTokenCountOpenChat(messages []*model.ChatCompletionMessage) int {
	return EstimateOpenChatMessagesTokens(messages)
}
