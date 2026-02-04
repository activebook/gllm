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
// If the user choose "All", set toolsUse.AutoApprove to true and return true
// If the user choose "Yes", return true
// If the user choose "No", return false
func NeedUserConfirmToolUse(info string, prompt string, description string, toolsUse *data.ToolsUse) (bool, error) {
	// Output the info message if provided
	if len(strings.TrimSpace(info)) > 0 {
		fmt.Println(info)
	}

	if toolsUse.AutoApprove {
		return true, nil
	}

	var fields []huh.Field

	description = strings.TrimSpace(description)
	isLong := len(description) > 400

	// If description is too long, use a Note before the Confirm field
	if isLong {
		fields = append(fields, GetStaticHuhNote("", description))
	}

	var choice string
	confirmField := huh.NewSelect[string]().
		Title(prompt).
		Options(
			huh.NewOption("Yes", "Yes, once"),
			huh.NewOption("No", "No, cancel it"),
			huh.NewOption("All", "Allow for this session"),
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
		return false, fmt.Errorf("error in confirmation prompt: %v", err)
	}

	var confirm bool
	switch choice {
	case "All":
		toolsUse.AutoApprove = true
		confirm = true
	case "Yes":
		confirm = true
	default:
		confirm = false
	}

	return confirm, nil
}
