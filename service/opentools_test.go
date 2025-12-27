package service

import (
	"testing"
)

func TestGetOpenEmbeddingToolsFiltered(t *testing.T) {
	t.Run("empty filter returns nil (no tools)", func(t *testing.T) {
		result := GetOpenEmbeddingToolsFiltered(nil)
		if result != nil {
			t.Errorf("Expected nil for nil filter, got %d tools", len(result))
		}
	})

	t.Run("empty slice returns nil (no tools)", func(t *testing.T) {
		result := GetOpenEmbeddingToolsFiltered([]string{})
		if result != nil {
			t.Errorf("Expected nil for empty filter, got %d tools", len(result))
		}
	})

	t.Run("valid tool names filter correctly", func(t *testing.T) {
		allowed := []string{"shell", "read_file"}
		result := GetOpenEmbeddingToolsFiltered(allowed)
		if len(result) != 2 {
			t.Errorf("Expected 2 tools, got %d", len(result))
		}

		// Verify the returned tools match the filter
		for _, tool := range result {
			found := false
			for _, name := range allowed {
				if tool.Function.Name == name {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Unexpected tool in result: %s", tool.Function.Name)
			}
		}
	})

	t.Run("unknown tool names are ignored gracefully", func(t *testing.T) {
		allowed := []string{"shell", "unknown_tool", "nonexistent"}
		result := GetOpenEmbeddingToolsFiltered(allowed)
		if len(result) != 1 {
			t.Errorf("Expected 1 tool (only 'shell'), got %d", len(result))
		}
		if result[0].Function.Name != "shell" {
			t.Errorf("Expected 'shell', got '%s'", result[0].Function.Name)
		}
	})

	t.Run("all unknown names returns empty", func(t *testing.T) {
		allowed := []string{"foo", "bar", "baz"}
		result := GetOpenEmbeddingToolsFiltered(allowed)
		if len(result) != 0 {
			t.Errorf("Expected 0 tools, got %d", len(result))
		}
	})

	t.Run("mixed valid and invalid names", func(t *testing.T) {
		allowed := []string{"shell", "invalid1", "write_file", "invalid2", "edit_file"}
		result := GetOpenEmbeddingToolsFiltered(allowed)
		if len(result) != 3 {
			t.Errorf("Expected 3 tools, got %d", len(result))
		}
	})
}

func TestIsValidEmbeddingTool(t *testing.T) {
	t.Run("valid tool returns true", func(t *testing.T) {
		if !IsValidEmbeddingTool("shell") {
			t.Error("Expected 'shell' to be valid")
		}
	})

	t.Run("invalid tool returns false", func(t *testing.T) {
		if IsValidEmbeddingTool("nonexistent") {
			t.Error("Expected 'nonexistent' to be invalid")
		}
	})
}
