package service

type StreamStatus int

const (
	StatusUnknown StreamStatus = iota
	StatusStarted
	StatusFinished
	StatusError
	StatusData
	StatusReasoning
	StatusReasoningOver
)

type StreamNotify struct {
	Status StreamStatus
	Data   string // For text content or error messages
}
