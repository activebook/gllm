package ui

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/huh"
)

// QuestionType enumerates the supported interaction modes.
type QuestionType string

const (
	QuestionTypeSelect      QuestionType = "select"
	QuestionTypeMultiSelect QuestionType = "multiselect"
	QuestionTypeText        QuestionType = "text"
	QuestionTypeConfirm     QuestionType = "confirm"
)

// AskUserRequest is the structured input from the LLM tool call.
type AskUserRequest struct {
	Question     string
	QuestionType QuestionType
	Options      []string
	Placeholder  string
}

// AskUserResponse carries the user's answer back to the model.
type AskUserResponse struct {
	Answer    string   `json:"answer,omitempty"`    // text / confirm / single-select
	Answers   []string `json:"answers,omitempty"`   // multi-select
	Cancelled bool     `json:"cancelled,omitempty"` // user pressed Ctrl-C / Esc
}

// RunAskUser stops the spinner, renders the appropriate huh form, and returns the answer.
func RunAskUser(req AskUserRequest) (AskUserResponse, error) {
	// Stop any running indicator — huh and bubbletea alt-screens conflict.
	GetIndicator().Stop()

	switch req.QuestionType {
	case QuestionTypeSelect:
		return runSelect(req)
	case QuestionTypeMultiSelect:
		return runMultiSelect(req)
	case QuestionTypeText:
		return runText(req)
	case QuestionTypeConfirm:
		return runConfirm(req)
	default:
		return AskUserResponse{}, fmt.Errorf("unknown question_type: %q", req.QuestionType)
	}
}

func runSelect(req AskUserRequest) (AskUserResponse, error) {
	if len(req.Options) == 0 {
		return AskUserResponse{}, errors.New("ask_user: 'select' requires at least one option")
	}
	opts := make([]huh.Option[string], len(req.Options))
	for i, o := range req.Options {
		opts[i] = huh.NewOption(o, o)
	}
	height := GetTermFitHeight(len(opts))
	var answer string
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(req.Question).
				Options(opts...).
				Height(height).
				Value(&answer),
		),
	).Run()
	if errors.Is(err, huh.ErrUserAborted) {
		return AskUserResponse{Cancelled: true}, nil
	}
	return AskUserResponse{Answer: answer}, err
}

func runMultiSelect(req AskUserRequest) (AskUserResponse, error) {
	if len(req.Options) == 0 {
		return AskUserResponse{}, errors.New("ask_user: 'multiselect' requires at least one option")
	}
	opts := make([]huh.Option[string], len(req.Options))
	for i, o := range req.Options {
		opts[i] = huh.NewOption(o, o)
	}
	// height := GetTermFitHeight(len(opts))
	var answers []string
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title(req.Question).
				Description("Space to toggle · Enter to confirm").
				Options(opts...).
				// Height(height).
				Value(&answers),
		),
	).Run()
	if errors.Is(err, huh.ErrUserAborted) {
		return AskUserResponse{Cancelled: true}, nil
	}
	return AskUserResponse{Answers: answers}, err
}

func runText(req AskUserRequest) (AskUserResponse, error) {
	placeholder := req.Placeholder
	if placeholder == "" {
		placeholder = "Type your answer..."
	}
	var answer string
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewText().
				Title(req.Question).
				Placeholder(placeholder).
				Value(&answer),
		),
	).WithKeyMap(GetHuhKeyMap()).Run()
	if errors.Is(err, huh.ErrUserAborted) {
		return AskUserResponse{Cancelled: true}, nil
	}
	return AskUserResponse{Answer: answer}, err
}

func runConfirm(req AskUserRequest) (AskUserResponse, error) {
	var confirmed bool
	var answer string
	// if options is not empty, use select
	var err error
	if len(req.Options) > 1 {
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(req.Question).
					Affirmative(req.Options[0]).
					Negative(req.Options[1]).
					Value(&confirmed),
			),
		).Run()
		if errors.Is(err, huh.ErrUserAborted) {
			return AskUserResponse{Cancelled: true}, nil
		}
		answer = req.Options[1]
		if confirmed {
			answer = req.Options[0]
		}
	} else {
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(req.Question).
					Affirmative("Yes").
					Negative("No").
					Value(&confirmed),
			),
		).Run()
		if errors.Is(err, huh.ErrUserAborted) {
			return AskUserResponse{Cancelled: true}, nil
		}
		answer = "No"
		if confirmed {
			answer = "Yes"
		}
	}

	return AskUserResponse{Answer: answer}, err
}
