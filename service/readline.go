package service

import (
	"fmt"
	"strings"

	"github.com/chzyer/readline"
)

func NeedUserConfirm(info string, prompt string) (bool, error) {
	// Output the info message if provided
	if len(strings.TrimSpace(info)) > 0 {
		fmt.Println(info)
	}

	// Set the prompt question
	rl, err := readline.New(prompt)
	if err != nil {
		return false, fmt.Errorf("error initializing readline: %v", err)
	}
	defer rl.Close()

	input, err := rl.Readline()
	if err != nil {
		return false, fmt.Errorf("error reading input: %v", err)
	}

	response := strings.ToLower(strings.TrimSpace(input))
	return response == "y" || response == "yes" || response == "ok" || response == "proceed", nil
}
