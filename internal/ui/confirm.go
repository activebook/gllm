package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/internal/event"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

// clearableForm is a wrapper for huh.Form that clears the view on exit.
// It ensures the terminal remains clean by rendering an empty string as the
// final frame when the form is completed, aborted, or externally cancelled.
type clearableForm struct {
	form     *huh.Form
	quitting bool
}

// forceQuitMsg is a custom message used to trigger a graceful shutdown
// of the Bubble Tea program, allowing it to render one last empty frame.
type forceQuitMsg struct{}

// Init delegates initialization to the underlying huh.Form.
func (m *clearableForm) Init() tea.Cmd {
	return m.form.Init()
}

// Update handles incoming messages. It intercepts completion and abortion
// states to set the quitting flag and return tea.Quit, ensuring the final
// render is empty. It also handles forceQuitMsg for external cancellation.
func (m *clearableForm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case forceQuitMsg:
		m.quitting = true
		return m, tea.Quit
	}

	res, cmd := m.form.Update(msg)
	m.form = res.(*huh.Form)

	if m.form.State == huh.StateCompleted || m.form.State == huh.StateAborted {
		m.quitting = true
		return m, tea.Quit
	}
	return m, cmd
}

// View renders the form or an empty string if the program is quitting.
func (m *clearableForm) View() string {
	if m.quitting {
		return ""
	}
	return m.form.View()
}

// RunFormClearable runs a huh.Form with context and clears it from the terminal on exit.
// It uses a goroutine to listen for context cancellation and sends a forceQuitMsg
// to ensure the TUI clears the screen before the process continues.
func RunFormClearable(form *huh.Form, ctx context.Context) error {
	p := tea.NewProgram(&clearableForm{form: form})

	// Listen for context cancellation to trigger a graceful shutdown
	// which allows Bubble Tea to render the final empty frame and clear the screen.
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			p.Send(forceQuitMsg{})
		case <-done:
		}
	}()

	_, err := p.Run()
	close(done)

	if err != nil {
		return err
	}
	if form.State == huh.StateAborted {
		return huh.ErrUserAborted
	}
	return nil
}

// NeedUserConfirmToolUse prompts the user for tool execution confirmation.
// If toolsUse.AutoApprove is true, it returns ToolConfirmYes immediately.
// Otherwise, it displays a selection menu for the user to choose "once", "session", or "cancel".
func NeedUserConfirmToolUse(info string, prompt string, description string, toolsUse *data.ToolsUse) {
	// Output the info message if provided
	if len(strings.TrimSpace(info)) > 0 {
		fmt.Println(info)
	}

	if toolsUse.AutoApprove {
		toolsUse.Confirm = data.ToolConfirmYes
		return
	}

	var fields []huh.Field

	description = strings.TrimSpace(description)
	isLong := len(description) > 400

	// If description is too long, use a Note before the Confirm field
	if isLong {
		fields = append(fields, GetStaticHuhNoteFull("", description))
	}

	var choice string
	confirmField := huh.NewSelect[string]().
		Title(prompt).
		Options(
			huh.NewOption("Yes, allow once", "Yes"),
			huh.NewOption("Yes, allow for this session", "All"),
			// huh.NewOption("Yes, allow always", "Always"),
			huh.NewOption("No, suggest changes", "No"),
		).
		Value(&choice)

	// If description is not too long and not empty, use the built-in Description
	if len(description) > 0 && !isLong {
		confirmField.Description(description)
	}

	fields = append(fields, confirmField)

	// Use a default styling or customize as needed
	form := huh.NewForm(
		huh.NewGroup(fields...),
	)

	// Set up context for cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register the cancel function with the VSCode confirm bus
	bus := event.GetVSCodeConfirmBus()
	bus.RegisterConfirmCancel(cancel, toolsUse.FilePath)
	defer bus.ClearConfirmCancel()

	// Run the form
	err := RunFormClearable(form, ctx)

	// Always check if the context was cancelled (e.g. by VSCode companion)
	// huh might return nil error even if the context was cancelled.
	if ctx.Err() != nil && errors.Is(ctx.Err(), context.Canceled) {
		accepted := event.GetVSCodeConfirmBus().GetAccepted()
		if accepted {
			toolsUse.ConfirmOnce()
		} else {
			toolsUse.ConfirmCancel()
		}
		return
	}

	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			// User aborted
			toolsUse.ConfirmCancel()
		} else {
			// Unexpected error
			toolsUse.ConfirmCancel()
		}
		return
	}

	switch choice {
	case "All":
		toolsUse.ConfirmAlways()
		// If user choose "All", it means user want to use tools without confirmation in this session
		// So we need to set yolo mode in session
		planModeInSession, yoloModeInSession := data.GetSessionMode()
		// If user is in plan mode, don't set yolo mode
		if !planModeInSession && !yoloModeInSession {
			data.SetYoloModeInSession(true)
		}
	case "Yes":
		toolsUse.ConfirmOnce()
	default:
		toolsUse.ConfirmCancel()
	}
}
