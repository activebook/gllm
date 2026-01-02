package service

import (
	"strings"
)

// ModelLimits contains context window configuration for a model
type ModelLimits struct {
	ContextWindow   int // Total context window in tokens
	MaxOutputTokens int // Maximum output tokens allowed
}

// DefaultModelLimits is the registry of known model limits.
// Context window values must be from official documentation or verified by tests
var DefaultModelLimits = map[string]ModelLimits{

	/*
	 * Aliyun Models
	 */
	"qwen3-235b-a22b":                {ContextWindow: 128000, MaxOutputTokens: 8192},
	"qwen3-235b-a22b-instruct-2507":  {ContextWindow: 262144, MaxOutputTokens: 8192},
	"qwen3-235b-a22b-thinking-2507":  {ContextWindow: 262144, MaxOutputTokens: 8192},
	"qwen3-30b-a3b":                  {ContextWindow: 40000, MaxOutputTokens: 8192},
	"qwen3-32b":                      {ContextWindow: 40000, MaxOutputTokens: 8192},
	"qwen3-coder-480b-a35b-instruct": {ContextWindow: 262000, MaxOutputTokens: 8192},
	"qwen3-coder-plus":               {ContextWindow: 128000, MaxOutputTokens: 8192},
	"qwen3-max":                      {ContextWindow: 256000, MaxOutputTokens: 65536},
	"qwen3-max-preview":              {ContextWindow: 256000, MaxOutputTokens: 65536},
	"qwen3-next-80b-a3b-instruct":    {ContextWindow: 131072, MaxOutputTokens: 8192},
	"qwen3-next-80b-a3b-thinking":    {ContextWindow: 131072, MaxOutputTokens: 8192},
	"qwen3-vl-plus":                  {ContextWindow: 256000, MaxOutputTokens: 32768},
	"qwen3-vl-flash":                 {ContextWindow: 256000, MaxOutputTokens: 32768},
	"qwen3-omni-flash":               {ContextWindow: 64000, MaxOutputTokens: 16384},
	"qwen-max-2025-01-25":            {ContextWindow: 128000, MaxOutputTokens: 8192},
	"qwen-turbo":                     {ContextWindow: 1000000, MaxOutputTokens: 8192},
	"qwen-vl-max-2025-01-25":         {ContextWindow: 128000, MaxOutputTokens: 8192},

	/*
	 * ByteDance Models
	 */
	"doubao-seed-1-8":          {ContextWindow: 256000, MaxOutputTokens: 65536},
	"doubao-seed-1-6":          {ContextWindow: 256000, MaxOutputTokens: 32768},
	"doubao-seed-1.6":          {ContextWindow: 256000, MaxOutputTokens: 32768},
	"doubao-seed-1.6-flash":    {ContextWindow: 256000, MaxOutputTokens: 32768},
	"doubao-seed-1.6-thinking": {ContextWindow: 256000, MaxOutputTokens: 32768},
	"doubao-1.5-pro-32k":       {ContextWindow: 128000, MaxOutputTokens: 8192},
	"doubao-1.5-thinking-pro":  {ContextWindow: 128000, MaxOutputTokens: 8192},
	"doubao-1.5-vision-pro":    {ContextWindow: 128000, MaxOutputTokens: 8192},
	"doubao-1-5":               {ContextWindow: 128000, MaxOutputTokens: 16384},

	/*
	 * DeepSeek Models
	 */
	"deepseek-math-v2":                {ContextWindow: 160000, MaxOutputTokens: 8192},
	"deepseek-v3.1-terminus":          {ContextWindow: 128000, MaxOutputTokens: 8192},
	"deepseek-v3.1-terminus-thinking": {ContextWindow: 128000, MaxOutputTokens: 8192},
	"deepseek-v3.2-251201":            {ContextWindow: 128000, MaxOutputTokens: 8192},
	"deepseek-v3.2-exp":               {ContextWindow: 128000, MaxOutputTokens: 8192},
	"deepseek-v3.2-exp-thinking":      {ContextWindow: 128000, MaxOutputTokens: 8192},
	"deepseek-v3-2":                   {ContextWindow: 128000, MaxOutputTokens: 32768},
	"deepseek-v3-1":                   {ContextWindow: 128000, MaxOutputTokens: 32768},
	"deepseek-r1":                     {ContextWindow: 80000, MaxOutputTokens: 8192},
	"deepseek-r1-0528":                {ContextWindow: 80000, MaxOutputTokens: 8192},
	"deepseek-v3":                     {ContextWindow: 128000, MaxOutputTokens: 16384},
	"deepseek-v3.1":                   {ContextWindow: 128000, MaxOutputTokens: 8192},
	"deepseek-v3-0324":                {ContextWindow: 128000, MaxOutputTokens: 8192},

	/*
	 * Meituan Models
	 */
	"longcat-flash-chat":     {ContextWindow: 131072, MaxOutputTokens: 8192},
	"longcat-flash-thinking": {ContextWindow: 131072, MaxOutputTokens: 8192},

	/*
	 * Minimax Models
	 */
	"minimax-m2": {ContextWindow: 200000, MaxOutputTokens: 8192},
	"minimax-m1": {ContextWindow: 1000000, MaxOutputTokens: 8192},

	/*
	 * Moonshot-Kimi Models
	 */
	"kimi-k2-0905":     {ContextWindow: 256000, MaxOutputTokens: 8192},
	"kimi-k2-thinking": {ContextWindow: 256000, MaxOutputTokens: 8192},
	"kimi-k2-turbo":    {ContextWindow: 256000, MaxOutputTokens: 8192},
	"kimi-k2":          {ContextWindow: 128000, MaxOutputTokens: 8192},

	/*
	 * OpenAI Models
	 */
	"gpt-oss-120b": {ContextWindow: 128000, MaxOutputTokens: 8192},
	"gpt-oss-20b":  {ContextWindow: 128000, MaxOutputTokens: 8192},

	/*
	 * xAI Models
	 */
	"grok-code-fast-1": {ContextWindow: 256000, MaxOutputTokens: 8192},
	"grok-4-1-fast":    {ContextWindow: 2000000, MaxOutputTokens: 8192},
	"grok-4-fast":      {ContextWindow: 2000000, MaxOutputTokens: 8192},
	"grok-4":           {ContextWindow: 256000, MaxOutputTokens: 8192},

	/*
	 * Xiaomi Models
	 */
	"mimo-v2-flash": {ContextWindow: 256000, MaxOutputTokens: 8192},

	/*
	 * Kuaishou Models
	 */
	"kat-coder": {ContextWindow: 256000, MaxOutputTokens: 32768},

	/*
	 * zAI Models
	 */
	"glm-4.6":     {ContextWindow: 200000, MaxOutputTokens: 16384},
	"glm-4.5":     {ContextWindow: 131000, MaxOutputTokens: 16384},
	"glm-4.5-air": {ContextWindow: 131000, MaxOutputTokens: 16384},

	// OpenAI Models
	"gpt-5.2":       {ContextWindow: 400000, MaxOutputTokens: 128000},
	"gpt-5.2-pro":   {ContextWindow: 400000, MaxOutputTokens: 128000},
	"gpt-5.2-chat":  {ContextWindow: 128000, MaxOutputTokens: 16384},
	"gpt-5.1-codex": {ContextWindow: 400000, MaxOutputTokens: 128000},
	"gpt-5.1-chat":  {ContextWindow: 128000, MaxOutputTokens: 16384},
	"gpt-5.1":       {ContextWindow: 400000, MaxOutputTokens: 128000},
	"gpt-5":         {ContextWindow: 400000, MaxOutputTokens: 128000},
	"gpt-4.1":       {ContextWindow: 1000000, MaxOutputTokens: 32768},
	"gpt-4.1-mini":  {ContextWindow: 1000000, MaxOutputTokens: 32768},
	"gpt-4o":        {ContextWindow: 128000, MaxOutputTokens: 16384},
	"gpt-4o-mini":   {ContextWindow: 128000, MaxOutputTokens: 16384},
	"gpt-4-turbo":   {ContextWindow: 128000, MaxOutputTokens: 4096},
	"gpt-4":         {ContextWindow: 8192, MaxOutputTokens: 8192},
	"gpt-3.5-turbo": {ContextWindow: 16385, MaxOutputTokens: 4096},
	"o1":            {ContextWindow: 200000, MaxOutputTokens: 100000},
	"o1-mini":       {ContextWindow: 128000, MaxOutputTokens: 65536},
	"o1-pro":        {ContextWindow: 200000, MaxOutputTokens: 100000},
	"o3":            {ContextWindow: 200000, MaxOutputTokens: 100000},
	"o3-mini":       {ContextWindow: 200000, MaxOutputTokens: 100000},
	"o3-mini-high":  {ContextWindow: 200000, MaxOutputTokens: 100000},
	"o4":            {ContextWindow: 200000, MaxOutputTokens: 100000},
	"o4-mini":       {ContextWindow: 200000, MaxOutputTokens: 100000},
	"o4-mini-high":  {ContextWindow: 200000, MaxOutputTokens: 100000},

	// Anthropic (verified)
	"claude-opus-4.5":   {ContextWindow: 200000, MaxOutputTokens: 64000},
	"claude-sonnet-4.5": {ContextWindow: 1000000, MaxOutputTokens: 64000},
	"claude-haiku-4.5":  {ContextWindow: 200000, MaxOutputTokens: 64000},
	"claude-opus-4.1":   {ContextWindow: 200000, MaxOutputTokens: 64000},
	"claude-sonnet-4":   {ContextWindow: 1000000, MaxOutputTokens: 64000},
	"claude-3.7-sonnet": {ContextWindow: 200000, MaxOutputTokens: 8192},
	"claude-3-5-sonnet": {ContextWindow: 200000, MaxOutputTokens: 8192},
	"claude-3-5-haiku":  {ContextWindow: 200000, MaxOutputTokens: 8192},

	// Google Gemini Models
	"gemini-3-pro-preview":       {ContextWindow: 1048576, MaxOutputTokens: 65536},
	"gemini-3-flash-preview":     {ContextWindow: 1048576, MaxOutputTokens: 65536},
	"gemini-3-pro-image-preview": {ContextWindow: 65536, MaxOutputTokens: 32768},
	"gemini-flash-latest":        {ContextWindow: 1048576, MaxOutputTokens: 65536},
	"gemini-flash-lite-latest":   {ContextWindow: 1048576, MaxOutputTokens: 65536},
	"gemini-2.5-pro":             {ContextWindow: 1048576, MaxOutputTokens: 65536},
	"gemini-2.5-flash":           {ContextWindow: 1048576, MaxOutputTokens: 65536},
	"gemini-2.5-flash-lite":      {ContextWindow: 1048576, MaxOutputTokens: 65536},
	"gemini-2.0-flash":           {ContextWindow: 1048576, MaxOutputTokens: 8192},
	"gemini-pro":                 {ContextWindow: 32760, MaxOutputTokens: 8192},

	// Mistral Models
	"mistral-large-latest": {ContextWindow: 131072, MaxOutputTokens: 65536}, // context :contentReference[oaicite:14]{index=14}
	"mistral-small-latest": {ContextWindow: 131072, MaxOutputTokens: 65536}, // context :contentReference[oaicite:15]{index=15}
	"codestral-latest":     {ContextWindow: 262144, MaxOutputTokens: 65536},
	"mistral-large":        {ContextWindow: 128000, MaxOutputTokens: 8192},
	"mistral-medium":       {ContextWindow: 32000, MaxOutputTokens: 8192},
	"mistral-small":        {ContextWindow: 32000, MaxOutputTokens: 8192},
	"codestral":            {ContextWindow: 32000, MaxOutputTokens: 8192},
	"magistral":            {ContextWindow: 128000, MaxOutputTokens: 40000},
	"pixtral-large":        {ContextWindow: 128000, MaxOutputTokens: 8192},

	// Chinese Models - DeepSeek
	"deepseek-chat":     {ContextWindow: 128000, MaxOutputTokens: 8192},
	"deepseek-reasoner": {ContextWindow: 128000, MaxOutputTokens: 32768},

	// Chinese Models - Alibaba Qwen
	"qwen2.5-vl-72b-instruct": {ContextWindow: 128000, MaxOutputTokens: 32768},
	"qwen2.5-vl-7b-instruct":  {ContextWindow: 128000, MaxOutputTokens: 32768},
	"qwen-plus":               {ContextWindow: 1000000, MaxOutputTokens: 32768},
	"qwen-plus-latest":        {ContextWindow: 1000000, MaxOutputTokens: 32768},
	"qwen-flash":              {ContextWindow: 1000000, MaxOutputTokens: 32768},
	"qwen-max":                {ContextWindow: 32768, MaxOutputTokens: 8192},
	"qwen-max-latest":         {ContextWindow: 131072, MaxOutputTokens: 8192},
	"qwen-coder-plus":         {ContextWindow: 131072, MaxOutputTokens: 8192},
	"qwen-vl-plus":            {ContextWindow: 131072, MaxOutputTokens: 8192},

	// Chinese Models - Moonshot Kimi

	// Chinese Models - ByteDance Doubao
	"doubao-pro":       {ContextWindow: 128000, MaxOutputTokens: 4096},
	"doubao-lite":      {ContextWindow: 32000, MaxOutputTokens: 4096},
	"doubao-vision":    {ContextWindow: 128000, MaxOutputTokens: 4096},
	"doubao-embedding": {ContextWindow: 32000, MaxOutputTokens: 4096},

	// Chinese Models - Zhipu GLM

	// Chinese Models - MiniMax

	// Chinese Models - Tencent (Added)
	"hunyuan-2.0-thinking": {ContextWindow: 128000, MaxOutputTokens: 64000},
	"hunyuan-2.0":          {ContextWindow: 100000, MaxOutputTokens: 8000},
	"hunyuan":              {ContextWindow: 128000, MaxOutputTokens: 4096},

	// Other Models(legacy)
	// I don't think we need support for these models
	// model improvements are so fast that legacy models are not useful
}

// DefaultLimits is the fallback for unknown models
var DefaultLimitsLegacy = ModelLimits{
	// Conservative default for older generation models
	ContextWindow:   32000,
	MaxOutputTokens: 4096,
}

var DefaultLimitsModern = ModelLimits{
	// Default for modern generation models
	ContextWindow:   128000,
	MaxOutputTokens: 8192,
}

// IsModelGemini3 checks if the model name is a Gemini 3 model
func IsModelGemini3(modelName string) bool {
	return strings.Contains(modelName, "gemini-3")
}

// GetModelLimits retrieves the limits for a given model name.
// It performs exact match first, then pattern matching, then returns defaults.
func GetModelLimits(modelName string) ModelLimits {
	if modelName == "" {
		return DefaultLimitsModern
	}

	modelNameLower := strings.ToLower(modelName)

	// Try exact match first
	if limits, ok := DefaultModelLimits[modelNameLower]; ok {
		return limits
	}

	// Try pattern matching (model name contains key)
	for pattern, limits := range DefaultModelLimits {
		if strings.Contains(modelNameLower, pattern) {
			return limits
		}
	}

	// Return defaults for unknown models
	return DefaultLimitsModern
}

// MaxInputTokens calculates the maximum input tokens with a safety buffer.
// The buffer ensures there's always room for the model's response.
func (ml ModelLimits) MaxInputTokens(bufferPercent float64) int {
	if bufferPercent <= 0 || bufferPercent > 1 {
		bufferPercent = 0.8 // Default to 80% if invalid
	}
	// If there's no strict output cap smaller than the context,
	// assume the maximum possible output is contextWindow itself.
	maxOutputCap := ml.MaxOutputTokens
	if ml.MaxOutputTokens >= ml.ContextWindow {
		maxOutputCap = 0
	}
	// Remaining tokens for input + generation = context window - strict output cap
	available := ml.ContextWindow - maxOutputCap
	// Safety margin
	return int(float64(available) * bufferPercent)
}
