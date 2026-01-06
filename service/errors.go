package service

import (
	"fmt"
)

// SwitchAgentError is a sentinel error used to signal that the agent should be switched.
// This error is returned by the switch_agent tool and handled by the agent execution loop.

type SwitchAgentError struct {
	TargetAgent string
}

func (e *SwitchAgentError) Error() string {
	return fmt.Sprintf("switching to agent: %s", e.TargetAgent)
}

func IsSwitchAgentError(err error) bool {
	switch err.(type) {
	case *SwitchAgentError:
		return true
	default:
		return false
	}
}
