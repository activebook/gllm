package service

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
