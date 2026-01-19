package data

import (
	"sync"
	"testing"
	"time"
)

func TestSharedState_SetAndGet(t *testing.T) {
	ss := NewSharedState()

	// Test basic set and get
	err := ss.Set("key1", "value1", "agent1")
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	value, exists := ss.Get("key1")
	if !exists {
		t.Fatal("Expected key1 to exist")
	}
	if value != "value1" {
		t.Fatalf("Expected value1, got %v", value)
	}

	// Test non-existent key
	_, exists = ss.Get("nonexistent")
	if exists {
		t.Fatal("Expected nonexistent key to not exist")
	}
}

func TestSharedState_SetEmptyKey(t *testing.T) {
	ss := NewSharedState()

	err := ss.Set("", "value", "agent1")
	if err == nil {
		t.Fatal("Expected error for empty key")
	}
}

func TestSharedState_GetString(t *testing.T) {
	ss := NewSharedState()

	// Test string value
	ss.Set("str", "hello", "agent1")
	if ss.GetString("str") != "hello" {
		t.Fatal("Expected 'hello'")
	}

	// Test non-existent key
	if ss.GetString("nonexistent") != "" {
		t.Fatal("Expected empty string for non-existent key")
	}

	// Test non-string value (should marshal to JSON)
	ss.Set("map", map[string]string{"a": "b"}, "agent1")
	result := ss.GetString("map")
	if result != `{"a":"b"}` {
		t.Fatalf("Expected JSON string, got: %s", result)
	}
}

func TestSharedState_Metadata(t *testing.T) {
	ss := NewSharedState()

	before := time.Now()
	ss.Set("key1", "value1", "agent1")
	after := time.Now()

	meta := ss.GetMetadata("key1")
	if meta == nil {
		t.Fatal("Expected metadata to exist")
	}

	if meta.CreatedBy != "agent1" {
		t.Fatalf("Expected CreatedBy=agent1, got %s", meta.CreatedBy)
	}

	if meta.CreatedAt.Before(before) || meta.CreatedAt.After(after) {
		t.Fatal("CreatedAt timestamp out of range")
	}

	if meta.ContentType != ContentTypeText {
		t.Fatalf("Expected ContentType=text, got %s", meta.ContentType)
	}

	// Test update preserves CreatedBy and CreatedAt
	time.Sleep(10 * time.Millisecond)
	ss.Set("key1", "value2", "agent2")

	updatedMeta := ss.GetMetadata("key1")
	if updatedMeta.CreatedBy != "agent1" {
		t.Fatal("CreatedBy should not change on update")
	}
	if updatedMeta.CreatedAt != meta.CreatedAt {
		t.Fatal("CreatedAt should not change on update")
	}
	if !updatedMeta.UpdatedAt.After(meta.UpdatedAt) {
		t.Fatal("UpdatedAt should be updated")
	}
}

func TestSharedState_Delete(t *testing.T) {
	ss := NewSharedState()

	ss.Set("key1", "value1", "agent1")

	// Delete existing key
	deleted := ss.Delete("key1")
	if !deleted {
		t.Fatal("Expected delete to return true")
	}

	// Verify deletion
	if ss.Has("key1") {
		t.Fatal("Key should not exist after deletion")
	}

	// Delete non-existent key
	deleted = ss.Delete("nonexistent")
	if deleted {
		t.Fatal("Expected delete of non-existent key to return false")
	}
}

func TestSharedState_List(t *testing.T) {
	ss := NewSharedState()

	ss.Set("key1", "value1", "agent1")
	ss.Set("key2", "value2", "agent2")

	list := ss.List()
	if len(list) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(list))
	}

	if list["key1"].CreatedBy != "agent1" {
		t.Fatal("Wrong CreatedBy for key1")
	}
	if list["key2"].CreatedBy != "agent2" {
		t.Fatal("Wrong CreatedBy for key2")
	}
}

func TestSharedState_Keys(t *testing.T) {
	ss := NewSharedState()

	ss.Set("key1", "value1", "agent1")
	ss.Set("key2", "value2", "agent2")

	keys := ss.Keys()
	if len(keys) != 2 {
		t.Fatalf("Expected 2 keys, got %d", len(keys))
	}

	// Check both keys exist (order not guaranteed)
	hasKey1, hasKey2 := false, false
	for _, k := range keys {
		if k == "key1" {
			hasKey1 = true
		}
		if k == "key2" {
			hasKey2 = true
		}
	}
	if !hasKey1 || !hasKey2 {
		t.Fatal("Missing expected keys")
	}
}

func TestSharedState_Clear(t *testing.T) {
	ss := NewSharedState()

	ss.Set("key1", "value1", "agent1")
	ss.Set("key2", "value2", "agent2")

	ss.Clear()

	if ss.Len() != 0 {
		t.Fatal("Expected empty state after clear")
	}
}

func TestSharedState_Len(t *testing.T) {
	ss := NewSharedState()

	if ss.Len() != 0 {
		t.Fatal("Expected len=0 for new state")
	}

	ss.Set("key1", "value1", "agent1")
	if ss.Len() != 1 {
		t.Fatal("Expected len=1")
	}

	ss.Set("key2", "value2", "agent2")
	if ss.Len() != 2 {
		t.Fatal("Expected len=2")
	}
}

func TestSharedState_ScopedOperations(t *testing.T) {
	ss := NewSharedState()

	// Test SetScoped and GetScoped
	ss.SetScoped("agent1", "key1", "value1")

	value, exists := ss.GetScoped("agent1", "key1")
	if !exists {
		t.Fatal("Expected scoped key to exist")
	}
	if value != "value1" {
		t.Fatalf("Expected value1, got %v", value)
	}

	// Verify the actual key format
	value, exists = ss.Get("agent1:key1")
	if !exists {
		t.Fatal("Expected raw key agent1:key1 to exist")
	}
}

func TestSharedState_GetAgentScope(t *testing.T) {
	ss := NewSharedState()

	ss.Set("key1", "value1", "agent1")
	ss.Set("key2", "value2", "agent1")
	ss.Set("key3", "value3", "agent2")

	scope := ss.GetAgentScope("agent1")
	if len(scope) != 2 {
		t.Fatalf("Expected 2 entries for agent1, got %d", len(scope))
	}

	if scope["key1"] != "value1" || scope["key2"] != "value2" {
		t.Fatal("Wrong values in agent scope")
	}
}

func TestSharedState_GetAgentKeys(t *testing.T) {
	ss := NewSharedState()

	ss.Set("key1", "value1", "agent1")
	ss.Set("key2", "value2", "agent1")
	ss.Set("key3", "value3", "agent2")

	keys := ss.GetAgentKeys("agent1")
	if len(keys) != 2 {
		t.Fatalf("Expected 2 keys for agent1, got %d", len(keys))
	}
}

func TestSharedState_ContentTypeDetection(t *testing.T) {
	ss := NewSharedState()

	// Text content
	ss.Set("text", "hello world", "agent1")
	meta := ss.GetMetadata("text")
	if meta.ContentType != ContentTypeText {
		t.Fatalf("Expected text type, got %s", meta.ContentType)
	}

	// JSON string content
	ss.Set("json_str", `{"key": "value"}`, "agent1")
	meta = ss.GetMetadata("json_str")
	if meta.ContentType != ContentTypeJSON {
		t.Fatalf("Expected json type for JSON string, got %s", meta.ContentType)
	}

	// Map content
	ss.Set("map", map[string]interface{}{"key": "value"}, "agent1")
	meta = ss.GetMetadata("map")
	if meta.ContentType != ContentTypeJSON {
		t.Fatalf("Expected json type for map, got %s", meta.ContentType)
	}

	// Binary content
	ss.Set("binary", []byte{0x00, 0x01, 0x02}, "agent1")
	meta = ss.GetMetadata("binary")
	if meta.ContentType != ContentTypeBinary {
		t.Fatalf("Expected binary type, got %s", meta.ContentType)
	}
}

func TestSharedState_ConcurrentAccess(t *testing.T) {
	ss := NewSharedState()
	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := "key" + string(rune('0'+n%10))
			ss.Set(key, n, "agent"+string(rune('0'+n%5)))
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := "key" + string(rune('0'+n%10))
			ss.Get(key)
			ss.Has(key)
			ss.GetMetadata(key)
		}(i)
	}

	wg.Wait()

	// Verify no panic and state is consistent
	if ss.Len() > 10 {
		t.Fatalf("Expected at most 10 unique keys, got %d", ss.Len())
	}
}

func TestSharedState_FormatList(t *testing.T) {
	ss := NewSharedState()

	// Test empty state
	result := ss.FormatList()
	if result != "SharedState is empty. No data has been stored yet." {
		t.Fatalf("Unexpected empty state message: %s", result)
	}

	// Test with entries
	ss.Set("key1", "value1", "agent1")
	result = ss.FormatList()

	if len(result) == 0 {
		t.Fatal("Expected non-empty formatted list")
	}
}
