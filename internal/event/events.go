package event

import (
	"github.com/activebook/gllm/data"
)

// --- Fire-and-forget events (one-way) ---

type StatusEvent struct {
	Text string
}

type BannerEvent struct {
	Text string
}

type IndicatorEvent struct {
	Action IndicatorAction
	Text   string
	Done   chan struct{}
}

type IndicatorAction int

const (
	IndicatorStart IndicatorAction = iota
	IndicatorStop
)

type SessionModeEvent struct {
	Mode int // 0=Normal, 1=Plan, 2=Yolo
}

// --- Request-response events (bidirectional) ---

type ConfirmRequest struct {
	Prompt      string
	Description string
	ToolsUse    *data.ToolsUse
	Done        chan struct{} // closed when UI finishes; service blocks on this
}

type AskUserRequest struct {
	Question     string
	QuestionType string
	Options      []string
	Placeholder  string
	Response     chan AskUserResponse // UI sends response back on this channel
}

type AskUserResponse struct {
	Answer    string   `json:"answer,omitempty"`
	Answers   []string `json:"answers,omitempty"`
	Cancelled bool     `json:"cancelled,omitempty"`
}

type DiffRequest struct {
	Before       string
	After        string
	File1        string
	File2        string
	ContextLines int
	Response     chan string // UI sends rendered diff string back
}
