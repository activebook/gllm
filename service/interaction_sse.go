package service

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/internal/event"
	"github.com/google/uuid"
)

// InteractionKind tags the kind of pending interaction request.
type InteractionKind string

const (
	InteractionKindConfirm InteractionKind = "tool_confirm"
	InteractionKindAskUser InteractionKind = "ask_user"
)

// pendingConfirm is the suspended state for a single RequestConfirm call.
type pendingConfirm struct {
	toolsUse *data.ToolsUse
	done     chan struct{}
}

// pendingAskUser is the suspended state for a single RequestAskUser call.
type pendingAskUser struct {
	resp chan event.AskUserResponse
}

// InteractionRegistry is a thread-safe store for pending interactive requests.
// Each pending item is keyed by a UUID that the frontend must echo back on /v1/interact.
var InteractionRegistry = &interactionRegistry{items: sync.Map{}}

type interactionRegistryEntry struct {
	kind    InteractionKind
	confirm *pendingConfirm
	askUser *pendingAskUser
}

type interactionRegistry struct {
	items sync.Map
}

func (r *interactionRegistry) store(id string, entry interactionRegistryEntry) {
	r.items.Store(id, entry)
}

func (r *interactionRegistry) load(id string) (interactionRegistryEntry, bool) {
	v, ok := r.items.Load(id)
	if !ok {
		return interactionRegistryEntry{}, false
	}
	return v.(interactionRegistryEntry), true
}

func (r *interactionRegistry) delete(id string) {
	r.items.Delete(id)
}

// ResolveConfirm is called from the /v1/interact endpoint with the user's decision.
func (r *interactionRegistry) ResolveConfirm(id string, approve string) error {
	entry, ok := r.load(id)
	if !ok || entry.kind != InteractionKindConfirm {
		return fmt.Errorf("interaction %q not found or not a confirm request", id)
	}
	r.delete(id)

	approve = strings.ToLower(approve)
	switch approve {
	case "always", "all":
		entry.confirm.toolsUse.ConfirmAlways()
		planModeInSession, yoloModeInSession := data.GetSessionMode()
		if !planModeInSession && !yoloModeInSession {
			data.SetYoloModeInSession(true)
		}
	case "once", "yes":
		entry.confirm.toolsUse.ConfirmOnce()
	default:
		entry.confirm.toolsUse.ConfirmCancel()
	}

	close(entry.confirm.done)
	return nil
}

// ResolveAskUser is called from the /v1/interact endpoint with the user's text response.
func (r *interactionRegistry) ResolveAskUser(id string, answer string, cancelled bool) error {
	entry, ok := r.load(id)
	if !ok || entry.kind != InteractionKindAskUser {
		return fmt.Errorf("interaction %q not found or not an ask_user request", id)
	}
	r.delete(id)
	entry.askUser.resp <- event.AskUserResponse{Answer: answer, Cancelled: cancelled}
	return nil
}

// SSEInteractionHandler implements InteractionHandler for the SSE server.
// It suspends the caller goroutine via a channel and emits an SSE event to the client,
// allowing the frontend to render a dialog and POST back the user's decision.
type SSEInteractionHandler struct {
	// emitFunc sends any SSE event to the open stream. It must be safe to call
	// from a different goroutine than the one that owns the http.ResponseWriter.
	emitFunc func(id string, kind InteractionKind, purpose string)

	diffFunc func(before, after string)

	// timeout how long we wait for firmware before auto-cancelling.
	// 0 means wait forever.
	timeout time.Duration
}

// NewSSEInteractionHandler creates the headless handler bound to an SSE emit function.
func NewSSEInteractionHandler(
	emitFunc func(id string, kind InteractionKind, purpose string),
	diffFunc func(before, after string),
	timeout time.Duration,
) *SSEInteractionHandler {
	return &SSEInteractionHandler{emitFunc: emitFunc, diffFunc: diffFunc, timeout: timeout}
}

func (h *SSEInteractionHandler) RequestConfirm(description string, toolsUse *data.ToolsUse) {
	if toolsUse.AutoApprove {
		toolsUse.Confirm = data.ToolConfirmYes
		return
	}

	id := uuid.New().String()
	done := make(chan struct{})

	InteractionRegistry.store(id, interactionRegistryEntry{
		kind:    InteractionKindConfirm,
		confirm: &pendingConfirm{toolsUse: toolsUse, done: done},
	})

	h.emitFunc(id, InteractionKindConfirm, description)

	// Block until the frontend resolves or the timeout expires.
	if h.timeout > 0 {
		select {
		case <-done:
		case <-time.After(h.timeout):
			InteractionRegistry.delete(id)
			toolsUse.Confirm = data.ToolConfirmCancel // auto-cancel on timeout
		}
	} else {
		<-done
	}
}

func (h *SSEInteractionHandler) RequestAskUser(req event.AskUserRequest) (event.AskUserResponse, error) {
	id := uuid.New().String()
	respCh := make(chan event.AskUserResponse, 1)

	InteractionRegistry.store(id, interactionRegistryEntry{
		kind:    InteractionKindAskUser,
		askUser: &pendingAskUser{resp: respCh},
	})

	h.emitFunc(id, InteractionKindAskUser, req.Question)

	var resp event.AskUserResponse
	if h.timeout > 0 {
		select {
		case resp = <-respCh:
		case <-time.After(h.timeout):
			InteractionRegistry.delete(id)
			return event.AskUserResponse{Cancelled: true}, fmt.Errorf("ask_user timed out")
		}
	} else {
		resp = <-respCh
	}

	if resp.Cancelled {
		return resp, fmt.Errorf("user cancelled")
	}
	return resp, nil
}

func (h *SSEInteractionHandler) RequestDiff(before, after string, contextLines int) string {
	// In headless mode we emit the raw diff to the client via SSE.
	// The frontend renders it interactively.
	if h.diffFunc != nil {
		h.diffFunc(before, after)
	}
	// Return empty string to suppress backend console ANSI diff rendering.
	return ""
}
