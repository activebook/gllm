package io

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// SSEOutput is an implementation of Output that sends data via Server-Sent Events.
type SSEOutput struct {
	writer  http.ResponseWriter
	flusher http.Flusher
}

// NewSSEOutput creates a new SSEOutput from an http.ResponseWriter.
// It returns an error if the writer does not support the http.Flusher interface.
func NewSSEOutput(w http.ResponseWriter) (*SSEOutput, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming unsupported by client")
	}

	return &SSEOutput{
		writer:  w,
		flusher: flusher,
	}, nil
}

// writeDeltaPayload is the private helper that formats and flushes a standard
// OpenAI delta SSE packet. The 'key' is typically 'content' or 'reasoning_content'.
func (s *SSEOutput) writeDeltaPayload(key, content string) {
	if content == "" {
		return
	}
	payload := map[string]interface{}{
		"choices": []map[string]interface{}{
			{
				"delta": map[string]string{key: content},
			},
		},
	}
	bytes, err := json.Marshal(payload)
	if err == nil {
		fmt.Fprintf(s.writer, "data: %s\n\n", string(bytes))
		s.flusher.Flush()
	}
}

// Writeln outputs the formatted string followed by a newline.
func (s *SSEOutput) Writeln(args ...interface{}) {
	s.writeDeltaPayload("content", fmt.Sprintln(args...))
}

// Writef outputs the formatted string.
func (s *SSEOutput) Writef(format string, args ...interface{}) {
	s.writeDeltaPayload("content", fmt.Sprintf(format, args...))
}

// Write outputs the unformatted arguments.
func (s *SSEOutput) Write(args ...interface{}) {
	s.writeDeltaPayload("content", fmt.Sprint(args...))
}

// Close gracefully closes the SSE connection.
func (s *SSEOutput) Close() {
	// Usually for SSE stream ending we send a [DONE] payload or just close the connection.
	fmt.Fprintf(s.writer, "data: [DONE]\n\n")
	s.flusher.Flush()
}

// WriteSSEEvent sends a named SSE event with a JSON-encoded data payload.
func (s *SSEOutput) WriteSSEEvent(event string, data interface{}) {
	bytes, err := json.Marshal(data)
	if err == nil {
		fmt.Fprintf(s.writer, "event: %s\ndata: %s\n\n", event, string(bytes))
		s.flusher.Flush()
	}
}

// WriteReasoningPayload sends chain-of-thought chunks using the 'reasoning_content'
// delta key, compatible with DeepSeek/extended OpenAI streaming schemas.
func (s *SSEOutput) WriteReasoningPayload(content string) {
	s.writeDeltaPayload("reasoning_content", content)
}
