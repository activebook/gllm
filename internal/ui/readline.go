package ui

import (
	"fmt"
	"strings"

	"github.com/activebook/gllm/data"
	"github.com/charmbracelet/huh"
)

// NeedUserConfirm prompts the user for confirmation using charmbracelet/huh.
func NeedUserConfirm(info string, prompt string, description string) (bool, error) {
	// Output the info message if provided
	if len(strings.TrimSpace(info)) > 0 {
		fmt.Println(info)
	}

	var confirm bool
	var fields []huh.Field

	description = strings.TrimSpace(description)
	isLong := len(description) > 400

	// If description is too long, use a Note before the Confirm field
	if isLong {
		fields = append(fields, GetStaticHuhNote("", description))
	}

	confirmField := huh.NewConfirm().
		Title(prompt).
		Value(&confirm).
		Affirmative("Yes").
		Negative("No")

	// If description is not too long and not empty, use the built-in Description
	if len(description) > 0 && !isLong {
		confirmField.Description(description)
	}

	fields = append(fields, confirmField)

	// Use a default styling or customize as needed
	form := huh.NewForm(
		huh.NewGroup(fields...),
	)

	err := form.Run()
	if err != nil {
		return false, fmt.Errorf("error in confirmation prompt: %v", err)
	}

	return confirm, nil
}

// For tools use confirmation
// If toolsUse.AutoApprove is true, return true
// If toolsUse.AutoApprove is false, prompt the user for confirmation
// If the user choose "All", set toolsUse.AutoApprove to true and toolsUse.Confirm to data.ToolConfirmYes
// If the user choose "Yes", set toolsUse.Confirm to data.ToolConfirmYes
// If the user choose "No", set toolsUse.Confirm to data.ToolConfirmCancel
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

	err := form.Run()
	if err != nil {
		// User aborted
		toolsUse.ConfirmCancel()
		return
	}

	switch choice {
	case "All":
		toolsUse.ConfirmAlways()
		data.SetToolCallAutoApproveInSession(true)
	case "Yes":
		toolsUse.ConfirmOnce()
	default:
		toolsUse.ConfirmCancel()
	}
}
