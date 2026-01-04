package service

import (
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"google.golang.org/genai"
)

// ThinkingLevel represents the unified thinking/reasoning level across providers.
// Maps to provider-specific configurations:
// - OpenAI: reasoning_effort ("low"/"medium"/"high")
// - OpenChat: model.Thinking + ReasoningEffort
// - Gemini 2.5: ThinkingBudget (token count, -1 for dynamic)
// - Gemini 3: ThinkingLevel ("LOW"/"MEDIUM"/"HIGH")
// - Anthropic: thinking.budget_tokens
type ThinkingLevel string

const (
	ThinkingLevelOff    ThinkingLevel = "off"
	ThinkingLevelLow    ThinkingLevel = "low"
	ThinkingLevelMedium ThinkingLevel = "medium"
	ThinkingLevelHigh   ThinkingLevel = "high"
)

// AllThinkingLevels returns all valid thinking levels in order
func AllThinkingLevels() []ThinkingLevel {
	return []ThinkingLevel{
		ThinkingLevelOff,
		ThinkingLevelLow,
		ThinkingLevelMedium,
		ThinkingLevelHigh,
	}
}

// ParseThinkingLevel normalizes user input to a valid ThinkingLevel.
// Supports backward compatibility with boolean values.
func ParseThinkingLevel(s string) ThinkingLevel {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "off", "disabled", "false", "0", "":
		return ThinkingLevelOff
	case "low":
		return ThinkingLevelLow
	case "medium", "med":
		return ThinkingLevelMedium
	case "high", "max", "true", "on":
		return ThinkingLevelHigh
	default:
		return ThinkingLevelOff
	}
}

// IsEnabled returns true if thinking is enabled (not off)
func (t ThinkingLevel) IsEnabled() bool {
	return t != ThinkingLevelOff
}

// String returns the string representation
func (t ThinkingLevel) String() string {
	return string(t)
}

// Display returns a colorized display string for CLI output
func (t ThinkingLevel) Display() string {
	switch t {
	case ThinkingLevelOff:
		return reasoningColorOff + "off" + resetColor
	case ThinkingLevelLow:
		return reasoningColorLow + "low" + resetColor
	case ThinkingLevelMedium:
		return reasoningColorMed + "medium" + resetColor
	case ThinkingLevelHigh:
		return reasoningColorHigh + "high" + resetColor
	default:
		return reasoningColorOff + "off" + resetColor
	}
}

// ToOpenAIReasoningEffort returns the OpenAI reasoning_effort parameter value.
// Returns empty string for ThinkingLevelOff (no param should be set).
func (t ThinkingLevel) ToOpenAIReasoningEffort() string {
	switch t {
	case ThinkingLevelLow:
		return "low"
	case ThinkingLevelMedium:
		return "medium"
	case ThinkingLevelHigh:
		return "high"
	default:
		return ""
	}
}

// ToOpenChatParams returns the OpenChat model.Thinking and ReasoningEffort params.
func (t ThinkingLevel) ToOpenChatParams() (*model.Thinking, *model.ReasoningEffort) {
	if t == ThinkingLevelOff {
		return &model.Thinking{Type: model.ThinkingTypeDisabled}, nil
	}

	thinking := &model.Thinking{Type: model.ThinkingTypeEnabled}
	var effort model.ReasoningEffort

	switch t {
	case ThinkingLevelLow:
		effort = model.ReasoningEffortLow
	case ThinkingLevelMedium:
		effort = model.ReasoningEffortMedium
	case ThinkingLevelHigh:
		effort = model.ReasoningEffortHigh
	}

	return thinking, &effort
}

// ToAnthropicParams returns the thinking budget tokens for Anthropic.
// Returns 0 for ThinkingLevelOff.
func (t ThinkingLevel) ToAnthropicParams() anthropic.ThinkingConfigParamUnion {
	switch t {
	case ThinkingLevelOff:
		disable := anthropic.NewThinkingConfigDisabledParam()
		return anthropic.ThinkingConfigParamUnion{OfDisabled: &disable}
	case ThinkingLevelLow:
		return anthropic.ThinkingConfigParamOfEnabled(4096)
	case ThinkingLevelMedium:
		return anthropic.ThinkingConfigParamOfEnabled(16384)
	case ThinkingLevelHigh:
		// Large budgets: For thinking budgets above 32k,
		// we recommend using batch processing to avoid networking issues.
		// Requests pushing the model to think above 32k tokens
		// causes long running requests that might run up against system timeouts and open connection limits.
		return anthropic.ThinkingConfigParamOfEnabled(31999)
	default:
		disable := anthropic.NewThinkingConfigDisabledParam()
		return anthropic.ThinkingConfigParamUnion{OfDisabled: &disable}
	}
}

// ToGeminiConfig returns the Gemini ThinkingConfig based on model version.
// Gemini 3 uses ThinkingLevel, Gemini 2.5 uses ThinkingBudget.
func (t ThinkingLevel) ToGeminiConfig(modelName string) *genai.ThinkingConfig {
	if t == ThinkingLevelOff {
		// For Gemini 3, we cannot fully disable, so use minimal
		if IsModelGemini3(modelName) {
			return &genai.ThinkingConfig{
				IncludeThoughts: false,
				ThinkingLevel:   genai.ThinkingLevelUnspecified,
			}
		}
		// Gemini 2.5: budget 0 disables thinking
		budget := int32(0)
		return &genai.ThinkingConfig{
			IncludeThoughts: false,
			ThinkingBudget:  &budget,
		}
	}

	config := &genai.ThinkingConfig{IncludeThoughts: true}

	if IsModelGemini3(modelName) {
		// Gemini 3 uses ThinkingLevel enum
		switch t {
		case ThinkingLevelLow:
			config.ThinkingLevel = genai.ThinkingLevelLow
		case ThinkingLevelMedium:
			config.ThinkingLevel = genai.ThinkingLevelMedium
		case ThinkingLevelHigh:
			config.ThinkingLevel = genai.ThinkingLevelHigh
		}
	} else {
		// Gemini 2.5 uses ThinkingBudget
		var budget int32
		switch t {
		case ThinkingLevelLow:
			budget = 1024
		case ThinkingLevelMedium, ThinkingLevelHigh:
			budget = -1 // dynamic allocation
		}
		config.ThinkingBudget = &budget
	}

	return config
}
