package ui

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/briandowns/spinner"

	"github.com/activebook/gllm/data"
)

// FormatEnabledIndicator returns a consistent enabled/disabled bracket indicator
// When enabled: [✔] with green color
// When disabled: [ ] with no color
func FormatEnabledIndicator(enabled bool) string {
	if enabled {
		return fmt.Sprintf("[%s✔%s]", data.SwitchOnColor, data.ResetSeq)
	}
	return "[ ]"
}

// Indicators
// For long-running operations, we can use a spinner indicator

const (
	IndicatorLoadingMCP = "Loading MCP servers..."
)

// WhimsicalProcessingWords is a collection of fun, playful processing indicators
// inspired by Claude Code's delightful personality
var WhimsicalProcessingWords = []string{
	// Thoughtful/Philosophical
	"Channelling...",
	"Pondering...",
	"Cogitating...",
	"Contemplating...",
	"Ruminating...",
	"Philosophising...",
	"Musing...",
	"Deliberating...",

	// Magical/Whimsical
	"Conjuring...",
	"Wizarding...",
	"Enchanting...",
	"Spellcrafting...",
	"Transmuting...",
	"Alchemizing...",

	// Technical
	"Computing...",
	"Calculating...",
	"Synthesizing...",
	"Compiling...",
	"Architecting...",
	"Optimizing...",

	// Playful/Fun
	"Booping...",
	"Smooshing...",
	"Discombobulating...",
	"Flibbertigibbeting...",
	"Bamboozling...",
	"Zigzagging...",
	"Percolating...",
	"Noodling...",

	// Action-oriented
	"Crafting...",
	"Assembling...",
	"Constructing...",
	"Fabricating...",
	"Orchestrating...",
	"Choreographing...",

	// Creative
	"Imagining...",
	"Dreaming...",
	"Inventing...",
	"Innovating...",
	"Improvising...",

	// Quirky
	"Tinkering...",
	"Fiddling...",
	"Futzing...",
	"Wrangling...",
	"Massaging...",
	"Finessing...",
}

// GetRandomProcessingWord returns a random whimsical processing word
func GetRandomProcessingWord() string {
	return WhimsicalProcessingWords[rand.Intn(len(WhimsicalProcessingWords))]
}

type Indicator struct {
	s *spinner.Spinner
}

func NewIndicator() *Indicator {
	i := &Indicator{}
	i.s = spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	i.s.Suffix = fmt.Sprintf(" %s", GetRandomProcessingWord())
	i.s.Color("fgHiMagenta", "bold")
	i.s.Start()
	return i
}

// NewIndicatorWithText creates a new indicator with custom text
func NewIndicatorWithText(text string) *Indicator {
	i := &Indicator{}
	i.s = spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	i.s.Suffix = fmt.Sprintf(" %s", text)
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
		text = GetRandomProcessingWord()
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
