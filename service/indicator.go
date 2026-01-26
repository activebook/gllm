package service

import (
	"fmt"
	"time"

	"github.com/briandowns/spinner"
)

const (
	IndicatorProcessing = "Processing..."
	IndicatorLoadingMCP = "Loading MCP servers..."
)

type Indicator struct {
	s *spinner.Spinner
}

func NewIndicator() *Indicator {
	i := &Indicator{}
	i.s = spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	i.s.Suffix = fmt.Sprintf(" %s", IndicatorProcessing)
	i.s.Color("fgHiMagenta", "bold")
	i.s.Start()
	return i
}

func (i *Indicator) Stop() {
	if i.s.Active() {
		i.s.Stop()
	}
}

func (i *Indicator) Start(text string) {
	if text == "" {
		text = IndicatorProcessing
	}
	if i.s.Active() {
		i.s.Stop()
		i.s.Suffix = fmt.Sprintf(" %s", text)
		i.s.Start()
	} else {
		i.s.Suffix = fmt.Sprintf(" %s", text)
		i.s.Start()
	}
}
