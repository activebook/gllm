package service

import (
	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/internal/event"
)

// InteractionHandler abstracts the interactive side effects (such as asking for confirmation 
// or prompting the user) away from the core Agent execution loop. 
// This decoupling allows the Agent to run seamlessly in headless environments (like SSE servers)
// without blocking on the global event bus.
type InteractionHandler interface {
	// RequestConfirm asks the environment for a decision regarding a tool action.
	// It assumes the implementing code updates `toolsUse.Confirm` and possibly displays the `description`.
	// For backward compatibility, we currently pass the full `data.ToolsUse` struct.
	RequestConfirm(description string, toolsUse *data.ToolsUse)

	// RequestAskUser sends a prompt to the environment and waits for a user response.
	RequestAskUser(req event.AskUserRequest) (event.AskUserResponse, error)

	// RequestDiff requests the environment to render a diff.
	RequestDiff(before, after string, contextLines int) string
}

// DefaultInteractionHandler provides the legacy behavior of routing interactions
// through the global event.Bus for the CLI UI.
type DefaultInteractionHandler struct{}

func (d DefaultInteractionHandler) RequestConfirm(description string, toolsUse *data.ToolsUse) {
	event.RequestConfirm(description, toolsUse)
}

func (d DefaultInteractionHandler) RequestAskUser(req event.AskUserRequest) (event.AskUserResponse, error) {
	return event.RequestAskUser(req)
}

func (d DefaultInteractionHandler) RequestDiff(before, after string, contextLines int) string {
	return event.RequestDiff(before, after, contextLines)
}
