package service

// TruncationStrategy defines how to handle context overflow
type TruncationStrategy string

const (
	// StrategyTruncateOldest removes oldest messages first, preserving system prompt
	StrategyTruncateOldest TruncationStrategy = "truncate_oldest"

	// StrategySummarize replaces old context with a summary
	StrategySummarize TruncationStrategy = "summarize"

	// StrategyNone disables truncation - will fail if context exceeds limit
	StrategyNone TruncationStrategy = "none"

	// DefaultBufferPercent is the default safety buffer (80% of available space)
	/*
	 * Before context window fills up, you may run into "context rot,"
	 * where model performance degrades as input length increases even when there's technically room left
	 * — LLMs don't process all tokens equally, with attention concentrating on the beginning and end,
	 * so information in the middle gets less reliable processing.
	 *
	 * 80% leaves room for the model to "breathe" and maintain high-quality reasoning.
	 */
	DefaultBufferPercent = 0.80
)

// ContextManager is the public interface implemented by each provider-specific context manager.
// Callers type-assert the returned messages slice to the concrete provider type.
type ContextManager interface {
	// PruneMessages checks whether the message history exceeds the context limit and
	// applies the configured strategy (truncation or summarisation) if needed.
	// • messages — the typed provider slice (e.g. []openai.ChatCompletionMessage)
	// • extra    — optional additional args (tools, systemPrompt) required by the provider
	// Returns the (possibly pruned) slice, a truncated flag, and any error.
	PruneMessages(messages any, extra ...any) (any, bool, error)

	// GetStrategy returns the active truncation strategy.
	GetStrategy() TruncationStrategy

	// GetMaxOutputTokens returns the model's maximum output token budget.
	GetMaxOutputTokens() int
}

// commonContext holds the fields common to every provider and supplies the two
// accessor methods that implement the non-pruning parts of ContextManager.
type commonContext struct {
	agent           *Agent
	maxInputTokens  int
	maxOutputTokens int
	strategy        TruncationStrategy
	bufferPercent   float64
}

func (b *commonContext) GetStrategy() TruncationStrategy { return b.strategy }
func (b *commonContext) GetMaxOutputTokens() int         { return b.maxOutputTokens }

// NewContextManager constructs the correct provider-specific ContextManager for the agent.
func NewContextManager(ag *Agent, strategy TruncationStrategy) ContextManager {
	limits := GetModelLimits(ag.Model.ModelName)
	Debugf("Context Quota: modelName=%s, limits=%v, strategy=%s", ag.Model.ModelName, limits, strategy)
	base := commonContext{
		agent:           ag,
		maxInputTokens:  limits.MaxInputTokens(DefaultBufferPercent),
		maxOutputTokens: limits.MaxOutputTokens,
		strategy:        strategy,
	}
	switch ag.Model.Provider {
	case ModelProviderOpenAI:
		return &openAIContext{commonContext: base}
	case ModelProviderOpenAICompatible:
		return &openChatContext{commonContext: base}
	case ModelProviderGemini:
		return &geminiContext{commonContext: base}
	case ModelProviderAnthropic:
		return &anthropicContext{commonContext: base}
	default:
		// Fall back to OpenAI-compatible as the safest default
		return &openAIContext{commonContext: base}
	}
}
