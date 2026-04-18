package sse

import (
	stdio "io"

	"github.com/charmbracelet/x/ansi"
)

// sseWriter adapts SSEOutput to the stdio.Writer contract.
// It strips ANSI escape codes before emitting a command SSE event.
type sseWriter struct {
	sse     *SSEOutput
	errMode bool // true → emit command with error field instead of content
}

// NewSSEWriter creates an stdio.Writer that emits "command" SSE events as output.
func NewSSEWriter(sse *SSEOutput) stdio.Writer {
	return &sseWriter{sse: sse}
}

// NewSSEErrWriter creates an stdio.Writer that emits "command" SSE events as errors.
func NewSSEErrWriter(sse *SSEOutput) stdio.Writer {
	return &sseWriter{sse: sse, errMode: true}
}

func (w *sseWriter) Write(p []byte) (int, error) {
	// Strip terminal ANSI escape sequences to produce clean plain-text for web clients.
	text := ansi.Strip(string(p))
	if w.errMode {
		w.sse.WriteCommandEvent("", text)
	} else {
		w.sse.WriteCommandEvent(text, "")
	}
	return len(p), nil
}
