package data

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// StateContentType represents the type of content stored in SharedState
type StateContentType string

const (
	ContentTypeText    StateContentType = "text"
	ContentTypeJSON    StateContentType = "json"
	ContentTypeFileRef StateContentType = "file_ref"
	ContentTypeBinary  StateContentType = "binary"
)

// StateMetadata contains provenance information for a SharedState entry
type StateMetadata struct {
	CreatedBy   string           `json:"created_by"`   // Agent name that wrote this
	CreatedAt   time.Time        `json:"created_at"`   // When the entry was created
	UpdatedAt   time.Time        `json:"updated_at"`   // When the entry was last updated
	ContentType StateContentType `json:"content_type"` // Type of content
	Size        int              `json:"size"`         // Size in bytes (approximate)
}

// SharedState provides a concurrent-safe memory space for agents to communicate.
// It acts as a key-value store with metadata tracking for provenance.
type SharedState struct {
	mu       sync.RWMutex
	data     map[string]interface{}
	metadata map[string]*StateMetadata
}

// NewSharedState creates a new SharedState instance
func NewSharedState() *SharedState {
	return &SharedState{
		data:     make(map[string]interface{}),
		metadata: make(map[string]*StateMetadata),
	}
}

// Set stores a value in the SharedState with the given key.
// If the key already exists, it updates the value and UpdatedAt timestamp.
func (s *SharedState) Set(key string, value interface{}, agentName string) error {
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	contentType := detectContentType(value)
	size := estimateSize(value)

	// Check if key exists to determine if this is create or update
	if existing, exists := s.metadata[key]; exists {
		existing.UpdatedAt = now
		existing.ContentType = contentType
		existing.Size = size
		// Keep original CreatedBy and CreatedAt
	} else {
		s.metadata[key] = &StateMetadata{
			CreatedBy:   agentName,
			CreatedAt:   now,
			UpdatedAt:   now,
			ContentType: contentType,
			Size:        size,
		}
	}

	s.data[key] = value
	return nil
}

// Get retrieves a value from the SharedState by key.
// Returns the value and true if found, nil and false otherwise.
func (s *SharedState) Get(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	value, exists := s.data[key]
	return value, exists
}

// GetString retrieves a string value from the SharedState.
// Returns empty string if key doesn't exist or value is not a string.
func (s *SharedState) GetString(key string) string {
	value, exists := s.Get(key)
	if !exists {
		return ""
	}
	if str, ok := value.(string); ok {
		return str
	}
	// Try to marshal non-string values to JSON
	if bytes, err := json.Marshal(value); err == nil {
		return string(bytes)
	}
	// If it's a byte array, return a string representation
	if bytes, ok := value.([]byte); ok {
		return fmt.Sprintf("[binary data, %d bytes]", len(bytes))
	}
	return fmt.Sprintf("%v", value)
}

// GetMetadata retrieves the metadata for a key.
// Returns nil if the key doesn't exist.
func (s *SharedState) GetMetadata(key string) *StateMetadata {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if meta, exists := s.metadata[key]; exists {
		// Return a copy to prevent external modification
		metaCopy := *meta
		return &metaCopy
	}
	return nil
}

// Delete removes a key from the SharedState.
// Returns true if the key was deleted, false if it didn't exist.
func (s *SharedState) Delete(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.data[key]; exists {
		delete(s.data, key)
		delete(s.metadata, key)
		return true
	}
	return false
}

// List returns metadata for all keys in the SharedState.
func (s *SharedState) List() map[string]*StateMetadata {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*StateMetadata, len(s.metadata))
	for k, v := range s.metadata {
		metaCopy := *v
		result[k] = &metaCopy
	}
	return result
}

// Keys returns all keys in the SharedState.
func (s *SharedState) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	return keys
}

// Clear removes all entries from the SharedState.
func (s *SharedState) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data = make(map[string]interface{})
	s.metadata = make(map[string]*StateMetadata)
}

// Len returns the number of entries in the SharedState.
func (s *SharedState) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.data)
}

// SetScoped stores a value with an agent-scoped key (agentName:key)
func (s *SharedState) SetScoped(agentName, key string, value interface{}) error {
	scopedKey := fmt.Sprintf("%s:%s", agentName, key)
	return s.Set(scopedKey, value, agentName)
}

// GetScoped retrieves a value with an agent-scoped key (agentName:key)
func (s *SharedState) GetScoped(agentName, key string) (interface{}, bool) {
	scopedKey := fmt.Sprintf("%s:%s", agentName, key)
	return s.Get(scopedKey)
}

// GetAgentScope returns all entries created by a specific agent.
func (s *SharedState) GetAgentScope(agentName string) map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]interface{})
	for key, meta := range s.metadata {
		if meta.CreatedBy == agentName {
			result[key] = s.data[key]
		}
	}
	return result
}

// GetAgentKeys returns all keys created by a specific agent.
func (s *SharedState) GetAgentKeys(agentName string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var keys []string
	for key, meta := range s.metadata {
		if meta.CreatedBy == agentName {
			keys = append(keys, key)
		}
	}
	return keys
}

// Has checks if a key exists in the SharedState.
func (s *SharedState) Has(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, exists := s.data[key]
	return exists
}

// detectContentType infers the content type from the value
func detectContentType(value interface{}) StateContentType {
	switch v := value.(type) {
	case string:
		// Check if it looks like JSON
		trimmed := v
		if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
			var js interface{}
			if json.Unmarshal([]byte(trimmed), &js) == nil {
				return ContentTypeJSON
			}
		}
		return ContentTypeText
	case []byte:
		return ContentTypeBinary
	case map[string]interface{}, []interface{}:
		return ContentTypeJSON
	default:
		return ContentTypeText
	}
}

// estimateSize estimates the size of a value in bytes
func estimateSize(value interface{}) int {
	switch v := value.(type) {
	case string:
		return len(v)
	case []byte:
		return len(v)
	default:
		// For other types, marshal to JSON to estimate
		if bytes, err := json.Marshal(v); err == nil {
			return len(bytes)
		}
		return 0
	}
}

// FormatList returns a formatted string representation of all entries for display
func (s *SharedState) FormatList() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.metadata) == 0 {
		return "SharedState is empty. No data has been stored yet."
	}

	var result string
	result = fmt.Sprintf("SharedState contains %d entries:\n\n", len(s.metadata))

	for key, meta := range s.metadata {
		result += fmt.Sprintf("Key: %s\n", key)
		result += fmt.Sprintf("  Created by: %s\n", meta.CreatedBy)
		result += fmt.Sprintf("  Created at: %s\n", meta.CreatedAt.Format(time.RFC3339))
		result += fmt.Sprintf("  Updated at: %s\n", meta.UpdatedAt.Format(time.RFC3339))
		result += fmt.Sprintf("  Type: %s\n", meta.ContentType)
		result += fmt.Sprintf("  Size: %d bytes\n", meta.Size)
		result += "\n"
	}

	return result
}
