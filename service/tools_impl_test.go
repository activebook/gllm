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

func TestReplaceFirstOccurrence(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		search    string
		replace   string
		wantOut   string
		wantCount int
	}{
		{
			name:      "exact unique match",
			content:   "foo bar baz",
			search:    "bar",
			replace:   "qux",
			wantOut:   "foo qux baz",
			wantCount: 1,
		},
		{
			name:      "not found returns original",
			content:   "foo bar baz",
			search:    "zzz",
			replace:   "qux",
			wantOut:   "foo bar baz",
			wantCount: 0,
		},
		{
			name:      "ambiguous returns original and count",
			content:   "a b a",
			search:    "a",
			replace:   "x",
			wantOut:   "a b a", // unchanged
			wantCount: 2,
		},
		{
			name:      "deletion via empty replace",
			content:   "hello world",
			search:    "hello ",
			replace:   "",
			wantOut:   "world",
			wantCount: 1,
		},
		{
			name:      "only first replaced when test has unique longer context",
			content:   "func foo() {}\nfunc bar() {}",
			search:    "func foo() {}",
			replace:   "func foo() { return }",
			wantOut:   "func foo() { return }\nfunc bar() {}",
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOut, gotCount := replaceFirstOccurrence(tt.content, tt.search, tt.replace)
			if gotCount != tt.wantCount {
				t.Errorf("count = %d, want %d", gotCount, tt.wantCount)
			}
			if gotOut != tt.wantOut {
				t.Errorf("output = %q, want %q", gotOut, tt.wantOut)
			}
		})
	}
}

func TestNormalizeLineWS(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no-op clean input", "foo\nbar", "foo\nbar"},
		{"strips trailing spaces", "foo   \nbar  ", "foo\nbar"},
		{"expands tabs", "\tfoo\n\tbar", "    foo\n    bar"},
		{"normalises CRLF", "foo\r\nbar\r\n", "foo\nbar\n"},
		{"mixed tab and trailing", "\tfoo   \n\tbar\t", "    foo\n    bar"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeLineWS(tt.input)
			if got != tt.want {
				t.Errorf("normalizeLineWS(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestApplyWSNormalizedReplace(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		search    string
		replace   string
		wantFound bool
		wantOut   string
	}{
		{
			name:      "exact match passes through",
			content:   "line1\nfunc foo() {}\nline3",
			search:    "func foo() {}",
			replace:   "func foo() { return }",
			wantFound: true,
			wantOut:   "line1\nfunc foo() { return }\nline3",
		},
		{
			name:      "trailing space in search forgiven",
			content:   "line1\nfunc foo() {}\nline3",
			search:    "func foo() {}   ", // trailing spaces — LLM hallucination
			replace:   "func foo() { return }",
			wantFound: true,
			wantOut:   "line1\nfunc foo() { return }\nline3",
		},
		{
			name:      "tab vs spaces forgiven",
			content:   "func foo() {\n    return nil\n}",
			search:    "func foo() {\n\treturn nil\n}", // LLM used tab
			replace:   "func foo() {\n    return 0\n}",
			wantFound: true,
			wantOut:   "func foo() {\n    return 0\n}",
		},
		{
			name:      "CRLF in search forgiven",
			content:   "foo\nbar",
			search:    "foo\r\nbar",
			replace:   "baz",
			wantFound: true,
			wantOut:   "baz",
		},
		{
			name:      "not found returns false",
			content:   "foo\nbar",
			search:    "zzz",
			replace:   "x",
			wantFound: false,
		},
		{
			name:      "ambiguous normalized match returns false",
			content:   "return nil\nreturn nil",
			search:    "return nil",
			replace:   "return err",
			wantFound: false, // 2 matches — ambiguous, must reject
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOut, gotFound := applyWSNormalizedReplace(tt.content, tt.search, tt.replace)
			if gotFound != tt.wantFound {
				t.Errorf("found = %v, want %v", gotFound, tt.wantFound)
			}
			if tt.wantFound && gotOut != tt.wantOut {
				t.Errorf("output = %q, want %q", gotOut, tt.wantOut)
			}
		})
	}
}
