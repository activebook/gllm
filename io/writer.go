package io

import (
	stdio "io"

	"github.com/charmbracelet/x/ansi"
)

// sseWriter adapts SSEOutput to the stdio.Writer contract.
// It strips ANSI escape codes before emitting a command_output SSE event.
type sseWriter struct {
	sse     *SSEOutput
	errMode bool // true → emit "command_error" event instead of "command_output"
}

// NewSSEWriter creates an stdio.Writer that emits "command_output" SSE events.
func NewSSEWriter(sse *SSEOutput) stdio.Writer {
	return &sseWriter{sse: sse}
}

// NewSSEErrWriter creates an stdio.Writer that emits "command_error" SSE events.
func NewSSEErrWriter(sse *SSEOutput) stdio.Writer {
	return &sseWriter{sse: sse, errMode: true}
}

func (w *sseWriter) Write(p []byte) (int, error) {
	// ANSI Stripping: The sseWriter dynamically filters out terminal color ANSI escape sequences using Charmbracelet's parser, ensuring clean, plain-text command outputs for web clients while keeping Rich UIs intact on terminal.
	text := ansi.Strip(string(p))
	if w.errMode {
		w.sse.WriteSSEEvent("command_error", text)
	} else {
		w.sse.WriteSSEEvent("command_output", text)
	}
	return len(p), nil
}
