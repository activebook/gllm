package event

import (
	"context"
	"path/filepath"
	"sync"
)

const ()

// --- VSCode Confirm Synchronization ---
// The VSCodeConfirmBus allows external components (e.g. VSCode companion) to override and cancel
// an ongoing terminal UI confirmation prompt when the external component has already determined the result.

type VSCodeConfirmBus struct {
	mu         sync.Mutex
	cancelDiff context.CancelFunc // currently active confirmation canceller
	accepted   bool               // result from remote (nil = no result yet)
	path       string             // file path relevant to the confirmation, if any
}

var (
	instanceVSCodeConfirmBus = &VSCodeConfirmBus{}
)

// GetVSCodeConfirmBus returns the global bus for remote confirmation overrides.
func GetVSCodeConfirmBus() *VSCodeConfirmBus {
	return instanceVSCodeConfirmBus
}

// RegisterConfirmCancel is called by the UI just before blocking on a user prompt.
// If a remote event arrives, it will call check this cancel function.
func (b *VSCodeConfirmBus) RegisterConfirmCancel(cancel context.CancelFunc, path string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.accepted = false
	// Always use an absolute path for matching, since in worktrees and other contexts the same file may be referenced by different relative paths.
	// Or related path can be the same in worktree, so we can not rely on relative path
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path // fallback to original path if Abs fails for some reason
	}
	b.path = absPath
	b.cancelDiff = cancel
}

// ClearConfirmCancel is called by the UI when the prompt finishes normally to remove the hook.
func (b *VSCodeConfirmBus) ClearConfirmCancel() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.accepted = false
	b.path = ""
	b.cancelDiff = nil
}

// GetAccepted returns the remote decision, or nil if no remote event has superseded the prompt.
func (b *VSCodeConfirmBus) GetAccepted() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.accepted
}

func (b *VSCodeConfirmBus) Confirm(path string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}
	if b.cancelDiff != nil && b.path == absPath {
		b.accepted = true
		b.path = ""
		b.cancelDiff()
		b.cancelDiff = nil
	}
}

func (b *VSCodeConfirmBus) Reject(path string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}
	if b.cancelDiff != nil && b.path == absPath {
		b.accepted = false
		b.path = ""
		b.cancelDiff()
		b.cancelDiff = nil
	}
}
