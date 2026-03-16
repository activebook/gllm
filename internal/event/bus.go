package event

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/activebook/gllm/data"
)

type Bus struct {
	Status    chan StatusEvent
	Banner    chan BannerEvent
	Indicator chan IndicatorEvent
	Session   chan SessionModeEvent

	Confirm chan ConfirmRequest
	AskUser chan AskUserRequest
	Diff    chan DiffRequest

	// Thread-safe flag to track if indicator is currently active
	indicatorActive atomic.Bool
}

var (
	globalBus *Bus
	busOnce   sync.Once
)

func GetBus() *Bus {
	busOnce.Do(func() {
		globalBus = &Bus{
			Status:    make(chan StatusEvent, 32),
			Banner:    make(chan BannerEvent, 32),
			Indicator: make(chan IndicatorEvent, 32),
			Session:   make(chan SessionModeEvent, 16),
			Confirm:   make(chan ConfirmRequest), // unbuffered: blocks service until handled
			AskUser:   make(chan AskUserRequest), // unbuffered: blocks service until handled
			Diff:      make(chan DiffRequest),    // unbuffered: blocks service until handled
		}
	})
	return globalBus
}

// Convenience methods for fire-and-forget events

func SendStatus(text string) {
	GetBus().Status <- StatusEvent{Text: text}
}

func SendBanner(text string) {
	GetBus().Banner <- BannerEvent{Text: text}
}

func StartIndicator(text string) {
	GetBus().indicatorActive.Store(true)
	GetBus().Indicator <- IndicatorEvent{Action: IndicatorStart, Text: text}
}

func StopIndicator() {
	GetBus().indicatorActive.Store(false)
	done := make(chan struct{})
	// bugfix: if we don't wait for done, the indicator won't stop before the next event is sent
	GetBus().Indicator <- IndicatorEvent{Action: IndicatorStop, Done: done}
	<-done
}

func IsIndicatorActive() bool {
	return GetBus().indicatorActive.Load()
}

// --- Request-response helpers ---

// RequestConfirm sends a ConfirmRequest to the UI and blocks until the user responds.
// The ToolsUse.Confirm field will be set by the subscriber before unblocking.
func RequestConfirm(description string, toolsUse *data.ToolsUse) {
	done := make(chan struct{})
	GetBus().Confirm <- ConfirmRequest{Description: description, ToolsUse: toolsUse, Done: done}
	<-done
}

// RequestDiff sends a DiffRequest to the UI and returns the rendered diff string.
func RequestDiff(before, after string, contextLines int) string {
	respCh := make(chan string, 1)
	GetBus().Diff <- DiffRequest{Before: before, After: after, ContextLines: contextLines, Response: respCh}
	return <-respCh
}

// RequestAskUser sends an AskUserRequest to the UI and returns the response.
func RequestAskUser(req AskUserRequest) (AskUserResponse, error) {
	respCh := make(chan AskUserResponse, 1)
	req.Response = respCh
	GetBus().AskUser <- req
	resp := <-respCh
	if resp.Cancelled {
		return resp, fmt.Errorf("user cancelled")
	}
	return resp, nil
}
