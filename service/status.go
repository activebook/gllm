package service

import "fmt"

type StreamStatus int

const (
	StatusUnknown StreamStatus = iota
	StatusProcessing
	StatusStarted
	StatusFinished
	StatusWarn
	StatusError
	StatusReasoning
	StatusReasoningOver
	StatusFunctionCalling
	StatusFunctionCallingOver
	StatusDiffConfirm
	StatusDiffConfirmOver
	StatusSwitchAgent
	StatusUserCancel
)

type StreamNotify struct {
	Status StreamStatus
	Data   string      // For text content or error messages
	Extra  interface{} // For additional metadata (e.g., switch instruction)
}

// StateStack is a stack data structure for managing states.
type StatusStack struct {
	statuses []StreamStatus
}

// Push adds a state to the top of the stack.
func (s *StatusStack) Push(status StreamStatus) {
	s.statuses = append(s.statuses, status)
}

// Pop removes and returns the state from the top of the stack.
// If the stack is empty, it returns StateNormal.
func (s *StatusStack) Pop() StreamStatus {
	if len(s.statuses) == 0 {
		return StatusUnknown
	}
	status := s.statuses[len(s.statuses)-1]
	s.statuses = s.statuses[:len(s.statuses)-1]
	return status
}

// Peek returns the state from the top of the stack without removing it.
// If the stack is empty, it returns StateNormal.
func (s *StatusStack) Peek() StreamStatus {
	if len(s.statuses) == 0 {
		return StatusUnknown
	}
	return s.statuses[len(s.statuses)-1]
}

func (s *StatusStack) IsEmpty() bool {
	return len(s.statuses) == 0
}

func (s *StatusStack) Clear() {
	s.statuses = []StreamStatus{}
}

func (s *StatusStack) Size() int {
	return len(s.statuses)
}

func (s *StatusStack) IsTop(status StreamStatus) bool {
	return s.Peek() == status
}

func (s *StatusStack) ChangeTo(
	proc chan<- StreamNotify, // Sub Channel to send notifications
	notify StreamNotify,
	proceed <-chan bool) { // Sub Channel to receive proceed signal

	// If a proc channel is provided, send the notification
	if proc != nil {
		proc <- notify
	}

	// Update the status stack based on the new status
	switch notify.Status {
	case StatusStarted:
		for s.IsTop(StatusProcessing) {
			s.Pop() // Remove the processing status
		}
	case StatusReasoning:
		// If we are entering reasoning, we push it onto the stack
		if !s.IsTop(StatusReasoning) {
			s.Push(StatusReasoning)
		}
	case StatusFunctionCalling:
		// If we are entering function calling, we push it onto the stack
		if !s.IsTop(StatusFunctionCalling) {
			s.Push(StatusFunctionCalling)
		}
	case StatusFinished:
		// If we are finished, we pop all statuses
		for !s.IsEmpty() {
			s.Clear()
		}
	case StatusReasoningOver:
		for s.IsTop(StatusReasoning) || s.IsTop(StatusReasoningOver) {
			s.Pop() // Remove the reasoning status
		}
	case StatusFunctionCallingOver:
		for s.IsTop(StatusFunctionCalling) || s.IsTop(StatusFunctionCallingOver) {
			s.Pop() // Remove the function calling status
		}
	case StatusDiffConfirm:
		// If we are entering diff confirm, we push it onto the stack
		if !s.IsTop(StatusDiffConfirm) {
			s.Push(StatusDiffConfirm)
		}
	case StatusDiffConfirmOver:
		for s.IsTop(StatusDiffConfirm) || s.IsTop(StatusDiffConfirmOver) {
			s.Pop() // Remove the diff confirm status
		}
	case StatusWarn:
		// Do nothing
	default:
		// For other statuses, we just push the new status
		// This allows us to keep track of the current status stack
		s.Push(notify.Status)
	}

	// If a proceed channel is provided, wait for the signal to proceed
	if proceed != nil {
		<-proceed
	}
}

func (s *StatusStack) Debug() {
	fmt.Printf("Current status stack: %v\n", s.statuses)
}
