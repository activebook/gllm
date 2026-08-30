package sse

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

// writeDeltaPayload formats and flushes a standard OpenAI delta SSE packet.
// The 'key' is typically 'content' or 'reasoning_content'.
// Packet format: data: {"choices":[{"delta":{"<key>":"<content>"}}]}
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
	b, err := json.Marshal(payload)
	if err == nil {
		fmt.Fprintf(s.writer, "data: %s\n\n", string(b))
		s.flusher.Flush()
	}
}

// writeSSEEvent is the private helper that builds and flushes a unified GLLM event packet.
// All GLLM-proprietary events share the same invariant envelope:
//
//	data: {"type":"<eventType>","data":{...dataPayload}}
func (s *SSEOutput) writeSSEEvent(eventType string, dataPayload map[string]interface{}) {
	packet := map[string]interface{}{
		"type": eventType,
		"data": dataPayload,
	}
	b, err := json.Marshal(packet)
	if err == nil {
		fmt.Fprintf(s.writer, "data: %s\n\n", string(b))
		s.flusher.Flush()
	}
}

// --- OpenAI-compatible output methods (Track 1) ---

// Writeln outputs the formatted string followed by a newline as a content delta.
func (s *SSEOutput) Writeln(args ...interface{}) {
	s.writeDeltaPayload("content", fmt.Sprintln(args...))
}

// Writef outputs the formatted string as a content delta.
func (s *SSEOutput) Writef(format string, args ...interface{}) {
	s.writeDeltaPayload("content", fmt.Sprintf(format, args...))
}

// Write outputs the unformatted arguments as a content delta.
func (s *SSEOutput) Write(args ...interface{}) {
	s.writeDeltaPayload("content", fmt.Sprint(args...))
}

// WriteReasoningPayload sends chain-of-thought chunks using the 'reasoning_content'
// delta key, compatible with DeepSeek/extended OpenAI streaming schemas.
func (s *SSEOutput) WriteReasoningPayload(content string) {
	s.writeDeltaPayload("reasoning_content", content)
}

// Close gracefully terminates the SSE stream with the standard [DONE] sentinel.
func (s *SSEOutput) Close() {
	fmt.Fprintf(s.writer, "data: [DONE]\n\n")
	s.flusher.Flush()
}

// --- GLLM-proprietary typed event methods (Track 2) ---

// WriteStatusEvent emits a status lifecycle notification.
//
//	data: {"type":"status","data":{"content":"<status>"}}
func (s *SSEOutput) WriteStatusEvent(status string) {
	s.writeSSEEvent("status", map[string]interface{}{
		"content": status,
	})
}

// WriteToolCallEvent emits a tool invocation event. The content is a structured
// object containing the function name and its description.
// it shouldn't expose the arguments of the function to the web client
//
//	data: {"type":"tool_call","data":{"content":{"function":"...","description":"..."}}}
func (s *SSEOutput) WriteToolCallEvent(function string, description string) {
	s.writeSSEEvent("tool_call", map[string]interface{}{
		"content": map[string]interface{}{
			"function":    function,
			"description": description,
		},
	})
}

// WriteCommandEvent emits REPL command output. The error field is omitted on success.
//
//	Success: data: {"type":"command","data":{"content":"..."}}
//	Error:   data: {"type":"command","data":{"content":"","error":"..."}}
func (s *SSEOutput) WriteCommandEvent(content string, errMsg string) {
	payload := map[string]interface{}{
		"content": content,
	}
	if errMsg != "" {
		payload["error"] = errMsg
	}
	s.writeSSEEvent("command", payload)
}

// WriteErrorEvent emits a system or agent-level error with a machine-readable code.
//
//	data: {"type":"error","data":{"content":"...","code":"..."}}
func (s *SSEOutput) WriteErrorEvent(content string, code string) {
	s.writeSSEEvent("error", map[string]interface{}{
		"content": content,
		"code":    code,
	})
}

// WriteDiffEvent emits a raw diff to the client so it can render the changes interactively.
//
//	data: {"type":"diff","data":{"before":"...","after":"..."}}
func (s *SSEOutput) WriteDiffEvent(before, after string) {
	s.writeSSEEvent("diff", map[string]interface{}{
		"before": before,
		"after":  after,
	})
}

// WriteRequestEvent emits a request that the server has paused and needs user input.
// This is typically used for tool confirmations or "ask user" prompts.
//
//	data: {"type":"request","data":{"id":"...","type":"...","purpose":"..."}}
func (s *SSEOutput) WriteRequestEvent(id string, reqType string, purpose string) {
	payload := map[string]interface{}{
		"id":      id,
		"type":    reqType,
		"purpose": purpose,
	}
	s.writeSSEEvent("request", payload)
}
