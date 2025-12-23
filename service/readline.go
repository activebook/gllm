package service

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
)

// NeedUserConfirm prompts the user for confirmation using charmbracelet/huh.
func NeedUserConfirm(info string, prompt string) (bool, error) {
	// Output the info message if provided
	if len(strings.TrimSpace(info)) > 0 {
		fmt.Println(info)
	}

	var confirm bool
	// Set the prompt question
	// Use a default styling or customize as needed
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(prompt).
				Value(&confirm).
				Affirmative("Yes").
				Negative("No"),
		),
	)

	err := form.Run()
	if err != nil {
		return false, fmt.Errorf("error in confirmation prompt: %v", err)
	}

	return confirm, nil
}
