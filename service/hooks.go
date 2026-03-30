package service

import "strings"

// FileHooks holds registered callbacks for file lifecycle events.
// All hook funcs are invoked asynchronously by their caller.
type FileHooks struct {
	OnPreview []func(path, content string)
	OnSaved   []func(path string)
	OnDiscard []func(path string)
}

// NewFileHooks builds a FileHooks populated from all currently-enabled plugins.
func NewFileHooks() FileHooks {
	var h FileHooks
	// VSCode Companion plugin
	h.OnPreview = append(h.OnPreview, func(path, content string) { SendVSCodePreview(path, content) })
	h.OnSaved = append(h.OnSaved, func(path string) { SendVSCodeSaved(path) })
	h.OnDiscard = append(h.OnDiscard, func(path string) { SendVSCodeDiscard(path) })
	// Future plugins: append more funcs here
	return h
}

// Preview dispatches the preview event to all registered hooks.
func (h *FileHooks) Preview(path, content string) {
	for _, fn := range h.OnPreview {
		fn(path, content)
	}
}

// Saved dispatches the saved event to all registered hooks.
func (h *FileHooks) Saved(path string) {
	for _, fn := range h.OnSaved {
		fn(path)
	}
}

// Discard dispatches the discard event to all registered hooks.
func (h *FileHooks) Discard(path string) {
	for _, fn := range h.OnDiscard {
		fn(path)
	}
}

// ContextHooks holds registered providers that contribute additional context
// to the LLM prompt at the start of every user turn.
// Each provider is a func() string that returns a formatted context block,
// or an empty string if it has nothing to contribute (e.g. plugin disabled).
type ContextHooks struct {
	providers []func() string
}

// NewContextHooks builds a ContextHooks populated from all currently-enabled context providers.
func NewContextHooks() ContextHooks {
	var h ContextHooks
	// VSCode Companion plugin: injects active file, cursor, and selected text
	h.providers = append(h.providers, GetVSCodeContext)
	// Future context providers: append more funcs here
	// e.g. h.providers = append(h.providers, GetGitContextString)
	return h
}

// Collect calls all registered providers synchronously and returns the joined
// non-empty contributions, ready for prepending to the LLM prompt.
func (h ContextHooks) Collect() string {
	var parts []string
	for _, fn := range h.providers {
		if s := fn(); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, "\n")
}
