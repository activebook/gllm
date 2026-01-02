package service

import (
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	openai "github.com/sashabaranov/go-openai"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"google.golang.org/genai"
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
	MaxInputTokens  int                // Maximum input tokens allowed
	MaxOutputTokens int                // Maximum output tokens allowed (new field for Anthropic)
	Strategy        TruncationStrategy // Strategy for handling overflow
	BufferPercent   float64            // Safety buffer (0.0-1.0)
}

// NewContextManager creates a context manager with the given model limits
func NewContextManager(limits ModelLimits, strategy TruncationStrategy) *ContextManager {
	bufferPercent := DefaultBufferPercent
	return &ContextManager{
		MaxInputTokens:  limits.MaxInputTokens(bufferPercent),
		MaxOutputTokens: limits.MaxOutputTokens,
		Strategy:        strategy,
		BufferPercent:   bufferPercent,
	}
}

// NewContextManagerForModel creates a context manager by looking up the model name
func NewContextManagerForModel(modelName string, strategy TruncationStrategy) *ContextManager {
	limits := GetModelLimits(modelName)
	Debugf("Context Quota: modelName=%s, limits=%v, strategy=%s", modelName, limits, strategy)
	return NewContextManager(limits, strategy)
}

// =============================================================================
// OpenAI Message Handling
// =============================================================================

// PrepareOpenAIMessages processes messages to fit within context window limits.
// Returns the processed messages and a boolean indicating if truncation occurred.
// PrepareOpenAIMessages processes messages to fit within context window limits.
// Returns the processed messages and a boolean indicating if truncation occurred.
func (cm *ContextManager) PrepareOpenAIMessages(messages []openai.ChatCompletionMessage, tools []openai.Tool) ([]openai.ChatCompletionMessage, bool) {
	if cm.Strategy == StrategyNone || len(messages) == 0 {
		return messages, false
	}

	// Estimate tool tokens
	toolTokens := EstimateOpenAIToolTokens(tools)

	// Calculate current token usage using cache
	currentTokens := cm.estimateOpenAIMessagesWithCache(messages) + toolTokens
	// Debug logging (uses nil-safe wrapper)
	Debugf("Token count: %d MaxInputTokens[80%%]: %d", currentTokens, cm.MaxInputTokens)
	if currentTokens <= cm.MaxInputTokens {
		return messages, false
	}

	// Need to truncate
	return cm.truncateOpenAIMessages(messages, toolTokens)
}

// estimateOpenAIMessagesWithCache uses global cache for token estimation
func (cm *ContextManager) estimateOpenAIMessagesWithCache(messages []openai.ChatCompletionMessage) int {
	cache := GetGlobalTokenCache()
	total := 0
	for _, msg := range messages {
		total += cache.GetOrComputeOpenAITokens(msg)
	}
	return total + MessageOverheadTokens // priming tokens
}

// truncateOpenAIMessages removes oldest messages while preserving critical ones.
// Removes tool call/response pairs together.
func (cm *ContextManager) truncateOpenAIMessages(messages []openai.ChatCompletionMessage, toolTokens int) ([]openai.ChatCompletionMessage, bool) {
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

	// Consolidate all current system-level instructions into ONE system message
	if len(systemMsgs) > 1 {
		for i := 1; i < len(systemMsgs); i++ {
			// Bugfixed: Don't include duplicate system messages
			if !strings.Contains(systemMsgs[0].Content, systemMsgs[i].Content) {
				systemMsgs[0].Content += "\n" + systemMsgs[i].Content
			}
		}
		// Place it at the start (models pay most attention here)
		systemMsgs = systemMsgs[:1]
	}

	// Calculate system message tokens (these are always kept)
	systemTokens := cm.estimateOpenAIMessagesWithCache(systemMsgs)

	// Available tokens for non-system messages
	availableTokens := cm.MaxInputTokens - systemTokens - toolTokens

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
func (cm *ContextManager) PrepareOpenChatMessages(messages []*model.ChatCompletionMessage, tools []*model.Tool) ([]*model.ChatCompletionMessage, bool) {
	if cm.Strategy == StrategyNone || len(messages) == 0 {
		return messages, false
	}

	// Estimate tool tokens
	toolTokens := EstimateOpenChatToolTokens(tools)

	// Calculate current token usage using cache
	currentTokens := cm.estimateOpenChatMessagesWithCache(messages) + toolTokens
	// Debug logging (uses nil-safe wrapper)
	Debugf("Token count: %d MaxInputTokens[80%%]: %d", currentTokens, cm.MaxInputTokens)
	if currentTokens <= cm.MaxInputTokens {
		return messages, false
	}

	// Need to truncate
	return cm.truncateOpenChatMessages(messages, toolTokens)
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
	return total + MessageOverheadTokens // priming tokens
}

// truncateOpenChatMessages removes oldest messages while preserving critical ones.
// Removes tool call/response pairs together.
func (cm *ContextManager) truncateOpenChatMessages(messages []*model.ChatCompletionMessage, toolTokens int) ([]*model.ChatCompletionMessage, bool) {
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

	// Consolidate all current system-level instructions into ONE system message
	if len(systemMsgs) > 1 {
		var combinedContent string
		if systemMsgs[0].Content != nil && systemMsgs[0].Content.StringValue != nil {
			combinedContent = *systemMsgs[0].Content.StringValue
		}

		for i := 1; i < len(systemMsgs); i++ {
			if systemMsgs[i].Content != nil && systemMsgs[i].Content.StringValue != nil {
				newSys := *systemMsgs[i].Content.StringValue
				// Bugfixed: Don't include duplicate system messages
				if !strings.Contains(combinedContent, newSys) {
					combinedContent += "\n" + newSys
				}
			}
		}

		// Update the first message with consolidated content
		if systemMsgs[0].Content == nil {
			systemMsgs[0].Content = &model.ChatCompletionMessageContent{}
		}
		systemMsgs[0].Content.StringValue = &combinedContent

		// Place it at the start (models pay most attention here)
		systemMsgs = systemMsgs[:1]
	}

	// Calculate system message tokens
	systemTokens := cm.estimateOpenChatMessagesWithCache(systemMsgs)
	availableTokens := cm.MaxInputTokens - systemTokens - toolTokens

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

// =============================================================================
// Gemini Message Handling
// =============================================================================

// PrepareGeminiMessages processes messages to fit within context window limits.
func (cm *ContextManager) PrepareGeminiMessages(messages []*genai.Content, systemPrompt string, tools []*genai.Tool) ([]*genai.Content, bool) {
	if cm.Strategy == StrategyNone {
		return messages, false
	}

	// Calculate tokens for system prompt (passed separately)
	sysTokens := 0
	if systemPrompt != "" {
		sysTokens = EstimateTokens(systemPrompt) + MessageOverheadTokens
	}

	// Calculate tool tokens to reserve space
	toolTokens := EstimateGeminiToolTokens(tools)

	// Add tool tokens to the total overhead
	totalOverhead := sysTokens + toolTokens

	currentTokens := cm.estimateGeminiMessagesWithCache(messages) + totalOverhead
	// Debug logging (uses nil-safe wrapper)
	Debugf("Token count: %d MaxInputTokens[80%%]: %d", currentTokens, cm.MaxInputTokens)
	if currentTokens <= cm.MaxInputTokens {
		return messages, false
	}

	return cm.truncateGeminiMessages(messages, totalOverhead)
}

func (cm *ContextManager) estimateGeminiMessagesWithCache(messages []*genai.Content) int {
	cache := GetGlobalTokenCache()
	total := 0
	for _, msg := range messages {
		total += cache.GetOrComputeGeminiTokens(msg)
	}
	return total + MessageOverheadTokens // priming
}

func (cm *ContextManager) truncateGeminiMessages(messages []*genai.Content, totalOverhead int) ([]*genai.Content, bool) {
	if len(messages) == 0 {
		return messages, false
	}

	availableTokens := cm.MaxInputTokens - totalOverhead

	// Build token cache
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
			// Check tool pair
			pairIndices := cm.findToolPairGemini(messages, i)
			if len(pairIndices) > 0 {
				// Remove pair (highest index first)
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

			// Regular message
			if !cm.isGeminiToolMessage(messages[i]) {
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

func (cm *ContextManager) isGeminiToolMessage(msg *genai.Content) bool {
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

func (cm *ContextManager) findToolPairGemini(messages []*genai.Content, index int) []int {
	msg := messages[index]
	if msg == nil {
		return nil
	}

	// Check if message contains function response
	var responseName string
	for _, part := range msg.Parts {
		if part.FunctionResponse != nil {
			responseName = part.FunctionResponse.Name
			break
		}
	}

	// If it's a response, find its call (search backwards)
	if responseName != "" {
		for i := index - 1; i >= 0; i-- {
			for _, part := range messages[i].Parts {
				if part.FunctionCall != nil && part.FunctionCall.Name == responseName {
					return cm.gatherToolPairGemini(messages, i)
				}
			}
		}
	}

	// Check if message contains function call
	hasCall := false
	for _, part := range msg.Parts {
		if part.FunctionCall != nil {
			hasCall = true
			break
		}
	}

	if hasCall {
		return cm.gatherToolPairGemini(messages, index)
	}

	return nil
}

func (cm *ContextManager) gatherToolPairGemini(messages []*genai.Content, callIndex int) []int {
	indices := []int{callIndex}
	callMsg := messages[callIndex]
	if callMsg == nil {
		return indices
	}

	// Identify call names
	callNames := make(map[string]bool)
	for _, part := range callMsg.Parts {
		if part.FunctionCall != nil {
			callNames[part.FunctionCall.Name] = true
		}
	}

	// Find responses
	for j := callIndex + 1; j < len(messages); j++ {
		msg := messages[j]
		if msg == nil {
			continue
		}
		for _, part := range msg.Parts {
			if part.FunctionResponse != nil && callNames[part.FunctionResponse.Name] {
				indices = append(indices, j)
			}
		}
	}
	return indices
}

// GetCurrentTokenCountGemini returns the current token count for Gemini messages
func GetCurrentTokenCountGemini(messages []*genai.Content) int {
	return EstimateGeminiMessagesTokens(messages)
}

// =============================================================================
// Anthropic Message Handling
// =============================================================================

// PrepareAnthropicMessages processes messages to fit within context window limits.
func (cm *ContextManager) PrepareAnthropicMessages(messages []anthropic.MessageParam, systemPrompt string, tools []anthropic.ToolUnionParam) ([]anthropic.MessageParam, bool) {
	if cm.Strategy == StrategyNone || len(messages) == 0 {
		return messages, false
	}

	// Calculate tokens for system prompt (passed separately)
	sysTokens := 0
	if systemPrompt != "" {
		sysTokens = EstimateTokens(systemPrompt) + MessageOverheadTokens
	}

	// Estimate tool tokens
	toolTokens := EstimateAnthropicToolTokens(tools)

	// Add tool tokens to the total overhead
	totalOverhead := sysTokens + toolTokens

	// Calculate current token usage using cache
	currentTokens := cm.estimateAnthropicMessagesWithCache(messages) + totalOverhead
	// Debug logging (uses nil-safe wrapper)
	Debugf("Token count: %d MaxInputTokens[80%%]: %d", currentTokens, cm.MaxInputTokens)
	if currentTokens <= cm.MaxInputTokens {
		return messages, false
	}

	// Need to truncate
	return cm.truncateAnthropicMessages(messages, totalOverhead)
}

// estimateAnthropicMessagesWithCache uses global cache for token estimation
func (cm *ContextManager) estimateAnthropicMessagesWithCache(messages []anthropic.MessageParam) int {
	cache := GetGlobalTokenCache()
	total := 0
	for _, msg := range messages {
		total += cache.GetOrComputeAnthropicTokens(msg)
	}
	return total + MessageOverheadTokens // Add conversation overhead
}

// truncateAnthropicMessages removes oldest messages while preserving critical ones.
func (cm *ContextManager) truncateAnthropicMessages(messages []anthropic.MessageParam, totalOverhead int) ([]anthropic.MessageParam, bool) {
	if len(messages) == 0 {
		return messages, false
	}

	// Anthropic messages don't strictly have "system" role in MessageParam list usually
	// The "System" prompt is separate in MessageNewParams, but here we deal with "User" and "Assistant" history.
	// However, if we preserve the FIRST user message, that's often good practice.
	// But usually we just truncate oldest user/assistant pairs.

	availableTokens := cm.MaxInputTokens - totalOverhead

	// Build token counts
	tokenCounts := make([]int, len(messages))
	historyTokens := 0
	cache := GetGlobalTokenCache()
	for i, msg := range messages {
		tokenCounts[i] = cache.GetOrComputeAnthropicTokens(msg)
		historyTokens += tokenCounts[i]
	}

	truncated := false
	for historyTokens > availableTokens && len(messages) > 0 {
		removed := false
		// Remove items from start (index 0)
		// We should try to preserve tool pairs if possible, similar to other models.

		for i := 0; i < len(messages); i++ {
			// Check tool pair logic (assuming helper exists or we simplify)
			// Anthropic tool use: Assistant (tool_use) -> User (tool_result)
			// We should remove them as a pair.

			pairIndices := cm.findToolPairAnthropic(messages, i)
			if len(pairIndices) > 0 {
				// Remove pair
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

			// Regular message
			// Check if message is a Tool Use or Tool Result NOT captured by pair logic
			// (Should be captured, but if solitary, just remove)
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

func (cm *ContextManager) findToolPairAnthropic(messages []anthropic.MessageParam, index int) []int {
	if index >= len(messages) {
		return nil
	}
	msg := messages[index]

	// Helper to check for content blocks
	hasToolUse := false
	hasToolResult := false
	var toolUseID string

	for _, block := range msg.Content {
		if block.OfToolUse != nil {
			hasToolUse = true
			toolUseID = block.OfToolUse.ID
			break // Assuming one tool use per message for simplicity of pair finding, though parallel is possible
		}
		if block.OfToolResult != nil {
			hasToolResult = true
			toolUseID = block.OfToolResult.ToolUseID
			break
		}
	}

	// Case 1: Message is Tool Result (User) -> Find preceding Assistant Tool Use
	if hasToolResult && msg.Role == anthropic.MessageParamRoleUser {
		// Look backwards for the tool use
		for i := index - 1; i >= 0; i-- {
			prevMsg := messages[i]
			if prevMsg.Role == anthropic.MessageParamRoleAssistant {
				for _, b := range prevMsg.Content {
					if b.OfToolUse != nil && b.OfToolUse.ID == toolUseID {
						return []int{i, index} // Found unique pair (Assistant, User)
						// Note: This simplistic logic assumes 1:1. Parallel tool calls might be trickier.
						// But removing just this pair is a safe start.
					}
				}
			}
		}
		// If not found, it's an orphan result, safe to remove itself
		return []int{index}
	}

	// Case 2: Message is Tool Use (Assistant) -> Find following User Tool Result
	if hasToolUse && msg.Role == anthropic.MessageParamRoleAssistant {
		indices := []int{index}
		// Look forward for the tool result
		// A single Assistant message might have multiple tool uses.
		// We should gather ALL results.
		// But `findToolPair` contract usually returns a group to remove.
		// Let's gather all subsequent results.
		for j := index + 1; j < len(messages); j++ {
			nextMsg := messages[j]
			if nextMsg.Role == anthropic.MessageParamRoleUser {
				for _, b := range nextMsg.Content {
					if b.OfToolResult != nil {
						// Is this result for one of the tool uses in `msg`?
						// Need to check all tool uses in `msg`.
						// For now, simpler: if it's a tool result, assume it's related if usage pattern holds.
						// More strictly: check IDs.
						if b.OfToolResult.ToolUseID == toolUseID {
							indices = append(indices, j)
						}
					}
				}
			}
		}
		return indices
	}

	return nil
}
