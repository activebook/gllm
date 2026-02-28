package service

import (
	"errors"
	"fmt"
)

// SwitchAgentError is a sentinel error used to signal that the agent should be switched.
// This error is returned by the switch_agent tool and handled by the agent execution loop.

type SwitchAgentError struct {
	TargetAgent string
	Instruction string
}

func (e SwitchAgentError) Error() string {
	if e.Instruction != "" {
		return fmt.Sprintf("switching to agent: %s with instruction: %s", e.TargetAgent, e.Instruction)
	}
	return fmt.Sprintf("switching to agent: %s", e.TargetAgent)
}

func IsSwitchAgentError(err error) bool {
	var target SwitchAgentError
	return errors.As(err, &target)
}

const (
	UserCancelCommon        = "[Operation Cancelled]"
	UserCancelReasonUnknown = "Unknown"
	UserCancelReasonTimeout = "Timeout"
	UserCancelReasonDeny    = "User denied execution."
	UserCancelReasonCancel  = "User canceled execution."
)

// UserCancelError is a sentinel error used to signal that the user has cancelled an operation.
// This error is returned by the tool calls and handled by the agent execution loop.

type UserCancelError struct {
	Reason string
}

// Error implements [error].
func (e UserCancelError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("%s Reason: %s", UserCancelCommon, e.Reason)
	}
	return UserCancelCommon
}

func IsUserCancelError(err error) bool {
	var target UserCancelError
	return errors.As(err, &target)
}
