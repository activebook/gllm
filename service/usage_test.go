package service

import (
	"strings"
	"testing"
	"time"

	"github.com/activebook/gllm/data"
)

func TestTokenUsageRecordingAndSnapshot(t *testing.T) {
	tu := NewTokenUsage()
	tu.RecordTokenUsage(1000, 200, 300, 50, 1250)

	if tu.InputTokens != 1000 {
		t.Errorf("expected InputTokens 1000, got %d", tu.InputTokens)
	}
	if tu.OutputTokens != 200 {
		t.Errorf("expected OutputTokens 200, got %d", tu.OutputTokens)
	}
	if tu.CachedTokens != 300 {
		t.Errorf("expected CachedTokens 300, got %d", tu.CachedTokens)
	}
	if tu.ThoughtTokens != 50 {
		t.Errorf("expected ThoughtTokens 50, got %d", tu.ThoughtTokens)
	}
	if tu.TotalTokens != 1250 {
		t.Errorf("expected TotalTokens 1250, got %d", tu.TotalTokens)
	}

	// Persist to singleton
	SetLatestTokenUsage(tu, "gpt-4o", 128000, 4096)

	snap, ok := GetLatestTokenUsage()
	if !ok || snap == nil {
		t.Fatalf("expected snapshot to be retrieved, got nil")
	}

	if snap.ModelName != "gpt-4o" {
		t.Errorf("expected ModelName 'gpt-4o', got %s", snap.ModelName)
	}
	if snap.TotalTokens != 1250 {
		t.Errorf("expected TotalTokens 1250, got %d", snap.TotalTokens)
	}
	if snap.ContextLength != 128000 {
		t.Errorf("expected ContextLength 128000, got %d", snap.ContextLength)
	}
	if snap.RecordedAt.IsZero() {
		t.Errorf("expected non-zero RecordedAt time")
	}
}

func TestRenderStatusCard(t *testing.T) {
	agent := &data.AgentConfig{
		Name: "test-agent",
		Model: data.Model{
			Name:     "gpt-4o",
			Model:    "gpt-4o-2024-08-06",
			Provider: "openai",
		},
		Think: "medium",
	}

	// 1. Nil snapshot case
	emptyCard := RenderStatusCard(nil, agent, "session-test")
	if !strings.Contains(emptyCard, "test-agent") {
		t.Errorf("expected empty card to contain agent name 'test-agent'")
	}
	if !strings.Contains(emptyCard, "No turn token usage recorded") {
		t.Errorf("expected empty card to contain 'No turn token usage recorded'")
	}

	// 2. Active snapshot case with context length
	snap := &TokenUsageSnapshot{
		InputTokens:          32000,
		OutputTokens:         1500,
		CachedTokens:         20000,
		ThoughtTokens:        400,
		TotalTokens:          33900,
		CachedTokensInPrompt: true,
		ModelName:            "gpt-4o-2024-08-06",
		ContextLength:        128000,
		MaxOutputTokens:      4096,
		RecordedAt:           time.Now(),
	}

	card := RenderStatusCard(snap, agent, "session-test")
	if !strings.Contains(card, "Context Utilization") {
		t.Errorf("expected card to display Context Utilization section")
	}
	if !strings.Contains(card, "32000") {
		t.Errorf("expected card to display total input tokens")
	}
	if !strings.Contains(card, "33900") {
		t.Errorf("expected card to display total tokens")
	}
	if !strings.Contains(card, "Cached:") {
		t.Errorf("expected card to display cached tokens")
	}
}

func TestGetCachedPercentageColor(t *testing.T) {
	// Initialize default theme colors if needed
	data.LoadTheme("Dracula")

	highColor := GetCachedPercentageColor(85.0)
	if string(highColor) != data.HighCachedHex {
		t.Errorf("expected %s for >80%%, got %s", data.HighCachedHex, string(highColor))
	}

	medColor := GetCachedPercentageColor(65.0)
	if string(medColor) != data.MedCachedHex {
		t.Errorf("expected %s for >50%%, got %s", data.MedCachedHex, string(medColor))
	}

	lowColor := GetCachedPercentageColor(25.0)
	if string(lowColor) != data.LowCachedHex {
		t.Errorf("expected %s for >20%%, got %s", data.LowCachedHex, string(lowColor))
	}

	offColor := GetCachedPercentageColor(10.0)
	if string(offColor) != data.OffCachedHex {
		t.Errorf("expected %s for <=20%%, got %s", data.OffCachedHex, string(offColor))
	}
}
