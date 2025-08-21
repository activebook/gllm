package service

import (
	"time"

	"github.com/briandowns/spinner"
)

type Indicator struct {
	s *spinner.Spinner
}

func NewIndicator(text string) *Indicator {
	i := &Indicator{}
	i.s = spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	i.s.Prefix = text
	i.s.Color("cyan", "bold")
	i.s.Start()
	return i
}

func (i *Indicator) Stop() {
	if i.s.Active() {
		i.s.Stop()
	}
}

func (i *Indicator) Start(text string) {
	if i.s.Active() {
		i.s.Stop()
		i.s.Prefix = text
		i.s.Start()
	} else {
		i.s.Prefix = text
		i.s.Start()
	}
}
