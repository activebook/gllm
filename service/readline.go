package service

import (
	"fmt"
	"strings"

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
	isLong := len(description) > 100

	// If description is too long, use a Note before the Confirm field
	if isLong {
		fields = append(fields, huh.NewNote().Description(description))
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
