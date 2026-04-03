package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReadFileWithLargeImageLine verifies that readFile can handle JSONL session
// files where a single line exceeds 1 MB (e.g., a base64-encoded image payload).
// This is a regression test for the "bufio.Scanner: token too long" crash that
// occurred when loading sessions created from image-bearing turns.
func TestReadFileWithLargeImageLine(t *testing.T) {
	t.Parallel()

	// Build a synthetic Gemini content message with a large inline-data blob.
	// The base64 string is 2 MB, which is well beyond bufio.MaxScanTokenSize (64 KB)
	// and the previously configured 1 MB ceiling.
	largeBase64 := strings.Repeat("A", 2*1024*1024) // 2 MB of 'A's

	type inlineData struct {
		MIMEType string `json:"mimeType"`
		Data     string `json:"data"`
	}
	type part struct {
		InlineData *inlineData `json:"inlineData,omitempty"`
		Text       string      `json:"text,omitempty"`
	}
	type geminiContent struct {
		Role  string `json:"role"`
		Parts []part `json:"parts"`
	}

	messages := []geminiContent{
		{
			Role: "user",
			Parts: []part{
				{InlineData: &inlineData{MIMEType: "image/png", Data: largeBase64}},
				{Text: "describe this image"},
			},
		},
		{
			Role:  "model",
			Parts: []part{{Text: "It looks like a large test image."}},
		},
	}

	// Write messages as JSONL to a temp file.
	dir := t.TempDir()
	sessionDir := filepath.Join(dir, "sessions", "test-session")
	if err := os.MkdirAll(sessionDir, 0750); err != nil {
		t.Fatalf("failed to create session dir: %v", err)
	}
	sessionFile := filepath.Join(sessionDir, "main.jsonl")

	f, err := os.Create(sessionFile)
	if err != nil {
		t.Fatalf("failed to create session file: %v", err)
	}
	enc := json.NewEncoder(f)
	for _, msg := range messages {
		if err := enc.Encode(msg); err != nil {
			f.Close()
			t.Fatalf("failed to encode message: %v", err)
		}
	}
	f.Close()

	// Use BaseSession directly to invoke readFile.
	session := &BaseSession{
		Name: "test-session",
		Path: sessionFile,
	}

	lines, err := session.readFile()
	if err != nil {
		t.Fatalf("readFile() returned error for large image line: %v", err)
	}

	if got, want := len(lines), len(messages); got != want {
		t.Errorf("readFile() returned %d lines, want %d", got, want)
	}
}
