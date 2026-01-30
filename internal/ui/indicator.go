package ui

import (
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/briandowns/spinner"

	"github.com/activebook/gllm/data"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

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
	mu           sync.Mutex
	s            *spinner.Spinner
	rotating     bool
	lastRotation time.Time
	lastWord     string
}

var (
	globalIndicator *Indicator
	indicatorOnce   sync.Once
)

// GetIndicator returns the singleton indicator instance
func GetIndicator() *Indicator {
	indicatorOnce.Do(func() {
		globalIndicator = &Indicator{
			rotating: true,
		}
		globalIndicator.setupSpinner()
	})
	return globalIndicator
}

func (i *Indicator) setupSpinner() {
	i.s = spinner.New(spinner.CharSets[14],
		100*time.Millisecond,
		spinner.WithWriter(os.Stderr))
	i.s.Color("fgHiMagenta", "bold")

	// Setup the pre-update hook for word rotation
	i.s.PreUpdate = func(s *spinner.Spinner) {
		i.mu.Lock()
		defer i.mu.Unlock()
		if i.rotating {
			// Change word every 2 seconds for a whimsical feel
			if time.Since(i.lastRotation) > 2000*time.Millisecond {
				newWord := GetRandomProcessingWord()
				// Try to avoid repeating the same word
				for newWord == i.lastWord && len(WhimsicalProcessingWords) > 1 {
					newWord = GetRandomProcessingWord()
				}
				s.Suffix = fmt.Sprintf(" %s", newWord)
				i.lastWord = newWord
				i.lastRotation = time.Now()
			}
		}
	}
}
func (i *Indicator) IsActive() bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.s != nil && i.s.Active()
}

func (i *Indicator) Stop() {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.s != nil && i.s.Active() {
		i.s.Stop()
	}
}

func (i *Indicator) Start(text string) {
	i.mu.Lock()
	defer i.mu.Unlock()

	if text == "" {
		i.rotating = true
		text = GetRandomProcessingWord()
		i.lastWord = text
		i.lastRotation = time.Now()
	} else {
		i.rotating = false
	}

	// Always restart to ensure fresh state
	if i.s.Active() {
		i.s.Stop()
	}

	i.s.Lock()
	i.s.Suffix = fmt.Sprintf(" %s", text)
	i.s.Unlock()
	i.s.Start()
}
