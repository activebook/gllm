package service

import (
	"testing"
)

func TestFindBestModelMatch(t *testing.T) {
	t.Parallel()

	index := []remoteModelIndexEntry{
		{Name: "gpt-4o", File: "gpt-4o.json"},
		{Name: "gpt-4o-mini", File: "gpt-4o-mini.json"},
		{Name: "gemini-2.5-pro", File: "gemini-2.5-pro.json"},
		{Name: "gemini-3-flash-preview", File: "gemini-3-flash-preview.json"},
		{Name: "free", File: "free.json"},
		{Name: "auto", File: "auto.json"},
	}

	tests := []struct {
		name     string
		input    string // NOTE: caller (SyncModelLimits) always lowercases before passing in
		wantName string // empty string means expect nil (no match)
	}{
		// Phase 1: Exact match
		{"exact match", "gpt-4o", "gpt-4o"},
		{"exact match with :free suffix", "free", "free"},

		// Phase 2: Date stamp stripping
		{"date strip YYYY-MM-DD", "gpt-4o-2024-08-06", "gpt-4o"},
		{"date strip MM-YYYY", "command-r-08-2024", ""},   // not in index, ensure no crash
		{"date strip short -0528", "gpt-4o-0528", "gpt-4o"},

		// Phase 3: Version-suffix stripping (new dedicated phase)
		{"version suffix -preview on input", "gemini-2.5-pro-preview", "gemini-2.5-pro"},
		{"version suffix -preview on entry", "gemini-2.5-pro", "gemini-2.5-pro"},

		// Phase 4: Input is a more-specific variant of an index entry
		{"prefix: specific input vs generic entry", "gpt-4o-mini-2024-07-18", "gpt-4o-mini"},

		// Phase 5: Reverse-prefix — canonical input matches specific index entry
		{"reverse-prefix: gemini-3-flash -> gemini-3-flash-preview", "gemini-3-flash", "gemini-3-flash-preview"},

		// Safety guards
		{"guard: short input not matched by reverse-prefix", "free", "free"},   // exact match takes priority
		{"guard: 'auto' not reverse-matched to a longer entry", "auto", "auto"}, // exact match, length < 10 guards reverse
		{"guard: no match for unknown model", "unknown-model-xyz", ""},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := findBestModelMatch(tt.input, index)
			if tt.wantName == "" {
				if got != nil {
					t.Errorf("expected no match, got %q", got.Name)
				}
			} else {
				if got == nil {
					t.Errorf("expected match %q, got nil", tt.wantName)
				} else if got.Name != tt.wantName {
					t.Errorf("expected %q, got %q", tt.wantName, got.Name)
				}
			}
		})
	}
}

func TestStripVersionSuffix(t *testing.T) {
	t.Parallel()

	cases := []struct{ in, want string }{
		{"gemini-3-flash-preview", "gemini-3-flash"},
		{"gemini-2.5-pro-latest", "gemini-2.5-pro"},
		{"model-v1-beta2", "model-v1"},
		{"model-v1-rc", "model-v1"},
		{"model-stable", "model"},
		{"gpt-4o", "gpt-4o"}, // no suffix, unchanged
	}

	for _, c := range cases {
		got := stripVersionSuffix(c.in)
		if got != c.want {
			t.Errorf("stripVersionSuffix(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
