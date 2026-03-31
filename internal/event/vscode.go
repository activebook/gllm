package event

import (
	"context"
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
func (b *VSCodeConfirmBus) RegisterConfirmCancel(cancel context.CancelFunc) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.accepted = false
	b.cancelDiff = cancel
}

// ClearConfirmCancel is called by the UI when the prompt finishes normally to remove the hook.
func (b *VSCodeConfirmBus) ClearConfirmCancel() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cancelDiff = nil
}

// GetAccepted returns the remote decision, or nil if no remote event has superseded the prompt.
func (b *VSCodeConfirmBus) GetAccepted() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.accepted
}

func (b *VSCodeConfirmBus) Confirm() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cancelDiff != nil {
		b.accepted = true
		b.cancelDiff()
		b.cancelDiff = nil
	}
}

func (b *VSCodeConfirmBus) Reject() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cancelDiff != nil {
		b.accepted = false
		b.cancelDiff()
		b.cancelDiff = nil
	}
}
