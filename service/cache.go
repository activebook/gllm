package service

import (
	"encoding/json"
	"fmt"
	"sync"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	openai "github.com/sashabaranov/go-openai"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"google.golang.org/genai"
)

// =============================================================================
// TokenCache - Thread-safe cache for message token counts
// =============================================================================

// TokenCache provides a thread-safe cache for storing token counts of LLM messages.
// It uses JSON-marshaled message content as keys to ensure correct uniqueness.
type TokenCache struct {
	mu      sync.RWMutex
	cache   map[string]int
	maxSize int
	hits    int64 // Cache hit counter for metrics
	misses  int64 // Cache miss counter for metrics
}

// DefaultMaxCacheSize is the default maximum number of entries in the cache
const DefaultMaxCacheSize = 10000

// NewTokenCache creates a new TokenCache with the specified maximum size
func NewTokenCache(maxSize int) *TokenCache {
	if maxSize <= 0 {
		maxSize = DefaultMaxCacheSize
	}
	return &TokenCache{
		cache:   make(map[string]int),
		maxSize: maxSize,
	}
}

// Get retrieves a cached token count for the given key.
// Returns the count and true if found, or 0 and false if not found.
func (tc *TokenCache) Get(key string) (int, bool) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	if count, ok := tc.cache[key]; ok {
		tc.hits++
		return count, true
	}
	tc.misses++
	return 0, false
}

// Set stores a token count for the given key.
// If the cache is full, it evicts approximately half of the entries.
func (tc *TokenCache) Set(key string, count int) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	// Evict if cache is full
	if len(tc.cache) >= tc.maxSize {
		tc.evictLocked()
	}

	tc.cache[key] = count
}

// evictLocked removes approximately half of the cache entries.
// Must be called with write lock held.
func (tc *TokenCache) evictLocked() {
	toRemove := tc.maxSize / 2
	removed := 0
	for key := range tc.cache {
		delete(tc.cache, key)
		removed++
		if removed >= toRemove {
			break
		}
	}
}

// Size returns the current number of entries in the cache
func (tc *TokenCache) Size() int {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return len(tc.cache)
}

// Clear removes all entries from the cache
func (tc *TokenCache) Clear() {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.cache = make(map[string]int)
	tc.hits = 0
	tc.misses = 0
}

// Stats returns cache statistics (hits, misses, size)
func (tc *TokenCache) Stats() (hits, misses int64, size int) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.hits, tc.misses, len(tc.cache)
}

// =============================================================================
// Message Key Generation - JSON-based for collision-free caching
// =============================================================================

// GetOpenAIMessageKey generates a cache key for an OpenAI message by JSON marshaling.
// This captures ALL fields (Content, ReasoningContent, ToolCalls, MultiContent, etc.)
// ensuring different messages never produce the same key.
func GetOpenAIMessageKey(msg openai.ChatCompletionMessage) string {
	data, err := json.Marshal(msg)
	if err != nil {
		// Fallback: role + content (unlikely to fail)
		return string(msg.Role) + "|" + msg.Content
	}
	return string(data)
}

// GetOpenChatMessageKey generates a cache key for an OpenChat (Volcengine) message.
func GetOpenChatMessageKey(msg *model.ChatCompletionMessage) string {
	if msg == nil {
		return ""
	}
	data, err := json.Marshal(msg)
	if err != nil {
		// Fallback
		content := ""
		if msg.Content != nil && msg.Content.StringValue != nil {
			content = *msg.Content.StringValue
		}
		return string(msg.Role) + "|" + content
	}
	return string(data)
}

// GetGeminiMessageKey generates a cache key for a Gemini message.
func GetGeminiMessageKey(msg *genai.Content) string {
	if msg == nil {
		return ""
	}
	data, err := json.Marshal(msg)
	if err != nil {
		content := ""
		// concatenate all parts.text
		for _, part := range msg.Parts {
			content += part.Text
		}
		return string(msg.Role) + "|" + content
	}
	return string(data)
}

// GetAnthropicMessageKey generates a cache key for an Anthropic message.
func GetAnthropicMessageKey(msg anthropic.MessageParam) string {
	data, err := json.Marshal(msg)
	if err != nil {
		// Fallback: role + content length/summary?
		// Since content is complex blocks, marshalling failure is rare unless interface{}
		// If it fails, we fall back to a simple string rep or just don't cache effectively (misses)
		// But let's try to make something unique.
		return string(msg.Role) + fmt.Sprintf("%v", msg.Content)
	}
	return string(data)
}

// =============================================================================
// Global Token Cache Instance
// =============================================================================

// globalTokenCache is the singleton instance used by ContextManager
var globalTokenCache = NewTokenCache(DefaultMaxCacheSize)

// GetGlobalTokenCache returns the global token cache instance
func GetGlobalTokenCache() *TokenCache {
	return globalTokenCache
}

// ClearTokenCache clears the global token cache (useful for testing)
func ClearTokenCache() {
	globalTokenCache.Clear()
}

// =============================================================================
// Convenience Methods for OpenAI/OpenChat Messages
// =============================================================================

// GetOrComputeOpenAITokens retrieves cached tokens or computes and caches them.
func (tc *TokenCache) GetOrComputeOpenAITokens(msg openai.ChatCompletionMessage) int {
	key := GetOpenAIMessageKey(msg)
	if count, found := tc.Get(key); found {
		return count
	}
	count := EstimateOpenAIMessageTokens(msg)
	tc.Set(key, count)
	return count
}

// GetOrComputeOpenChatTokens retrieves cached tokens or computes and caches them.
func (tc *TokenCache) GetOrComputeOpenChatTokens(msg *model.ChatCompletionMessage) int {
	if msg == nil {
		return 0
	}
	key := GetOpenChatMessageKey(msg)
	if count, found := tc.Get(key); found {
		return count
	}
	count := EstimateOpenChatMessageTokens(msg)
	tc.Set(key, count)
	return count
}

// GetOrComputeGeminiTokens retrieves cached tokens or computes and caches them for Gemini.
func (tc *TokenCache) GetOrComputeGeminiTokens(msg *genai.Content) int {
	if msg == nil {
		return 0
	}
	key := GetGeminiMessageKey(msg)
	if count, found := tc.Get(key); found {
		return count
	}
	count := EstimateGeminiMessageTokens(msg)
	tc.Set(key, count)
	return count
}

// GetOrComputeAnthropicTokens retrieves cached tokens or computes and caches them.
func (tc *TokenCache) GetOrComputeAnthropicTokens(msg anthropic.MessageParam) int {
	key := GetAnthropicMessageKey(msg)
	if count, found := tc.Get(key); found {
		return count
	}
	count := EstimateAnthropicMessageTokens(msg)
	tc.Set(key, count)
	return count
}
