package service

import (
	"time"

	"github.com/briandowns/spinner"
)

func NewSpinner(text string) *spinner.Spinner {
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Prefix = text
	s.Color("cyan", "bold")
	s.Start()
	return s
}

func StopSpinner(s *spinner.Spinner) {
	s.Stop()
}
