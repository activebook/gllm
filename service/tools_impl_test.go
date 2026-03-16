package service

import (
	"strings"
	"testing"
)

func TestProcessFileContentRange(t *testing.T) {
	content := []byte("line1\nline2\nline3\nline4\nline5")
	path := "test.txt"

	tests := []struct {
		name               string
		includeLineNumbers bool
		offset             int
		limit              int
		wantContains       []string
		wantNotContains    []string
	}{
		{
			name:               "Full file without line numbers",
			includeLineNumbers: false,
			offset:             0,
			limit:              -1,
			wantContains:       []string{"Content of test.txt (5 lines):", "line1", "line5"},
			wantNotContains:    []string{"1 |"},
		},
		{
			name:               "Full file with line numbers",
			includeLineNumbers: true,
			offset:             0,
			limit:              -1,
			wantContains:       []string{"Content of test.txt (5 lines, with line numbers):", "   1 | line1", "   5 | line5"},
		},
		{
			name:               "Range without line numbers",
			includeLineNumbers: false,
			offset:             1, // 2nd line
			limit:              2,
			wantContains:       []string{"Content of test.txt (lines 2-3 of 5):", "line2", "line3"},
			wantNotContains:    []string{"line1", "line4", "line5"},
		},
		{
			name:               "Range with line numbers",
			includeLineNumbers: true,
			offset:             2, // 3rd line
			limit:              2,
			wantContains:       []string{"Content of test.txt (lines 3-4 of 5):", "   3 | line3", "   4 | line4"},
			wantNotContains:    []string{"line1", "line2", "line5"},
		},
		{
			name:               "Offset exceeds total lines",
			includeLineNumbers: false,
			offset:             10,
			limit:              -1,
			wantContains:       []string{"Error: Offset 11 exceeds total lines (5)"},
		},
		{
			name:               "Empty file",
			includeLineNumbers: false,
			offset:             0,
			limit:              -1,
			wantContains:       []string{"0 lines", "<empty file>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var testContent []byte
			if tt.name == "Empty file" {
				testContent = []byte("")
			} else {
				testContent = content
			}
			got := processFileContentRange(path, testContent, tt.includeLineNumbers, tt.offset, tt.limit)
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("processFileContentRange() got:\n%s\nwant to contain: %s", got, want)
				}
			}
			for _, wantNot := range tt.wantNotContains {
				if strings.Contains(got, wantNot) {
					t.Errorf("processFileContentRange() got:\n%s\nwant NOT to contain: %s", got, wantNot)
				}
			}
		})
	}
}
