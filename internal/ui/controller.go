package ui

import "github.com/activebook/gllm/internal/event"

// StartUIEventListener runs event bus listeners. Call once at app startup.
// It bridges the decoupled service events to the concrete UI implementations.
func StartUIEventListener() {
	bus := event.GetBus()

	// 1. Fire-and-forget events (Status, Banner, Indicator, Session)
	go func() {
		for {
			select {
			case ev := <-bus.Status:
				// SendSyncEvent will update the background status immediately
				SendSyncEvent(StatusMsg{Text: ev.Text})
			case ev := <-bus.Banner:
				SendEvent(BannerMsg{Text: ev.Text})
			case ev := <-bus.Indicator:
				switch ev.Action {
				case event.IndicatorStart:
					GetIndicator().Start(ev.Text)
				case event.IndicatorStop:
					GetIndicator().Stop()
				}
				if ev.Done != nil {
					close(ev.Done)
				}
			case ev := <-bus.Session:
				SendEvent(SessionModeMsg{Mode: SessionMode(ev.Mode)})
			}
		}
	}()

	// 2. Request-response events (Blocking UI operations)
	// Each in its own goroutine to avoid head-of-line blocking if multiple events queue up
	// though in practice the service layer usually only blocks on one at a time.

	go func() {
		for req := range bus.Confirm {
			NeedUserConfirmToolUse("", req.Prompt, req.Description, req.ToolsUse)
			close(req.Done)
		}
	}()

	go func() {
		for req := range bus.AskUser {
			resp, err := RunAskUser(AskUserRequest{
				Question:     req.Question,
				QuestionType: QuestionType(req.QuestionType),
				Options:      req.Options,
				Placeholder:  req.Placeholder,
			})
			if err != nil {
				req.Response <- event.AskUserResponse{Cancelled: true}
			} else {
				req.Response <- event.AskUserResponse{
					Answer:    resp.Answer,
					Answers:   resp.Answers,
					Cancelled: resp.Cancelled,
				}
			}
		}
	}()

	go func() {
		for req := range bus.Diff {
			result := Diff(req.Before, req.After, req.File1, req.File2, req.ContextLines)
			req.Response <- result
		}
	}()
}
