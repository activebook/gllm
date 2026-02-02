package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/huh"
)

// Embed the JSON style as a string constant
const compactStyleJSON = `{
  "document": {
    "margin": 0
  },
  "paragraph": {
    "margin": 0
  },
  "heading": {
    "margin": 0
  },
  "h1": {
    "margin": 0
  },
  "h2": {
    "margin": 0
  },
  "h3": {
    "margin": 0
  },
  "list": {
    "margin": 0
  },
  "code_block": {
    "margin": 0
  }
}`

// GetHuhKeyMap returns a custom keymap for huh forms
// Specifically disables the Editor key binding for Text fields as it interferes with input
func GetHuhKeyMap() *huh.KeyMap {
	// 1. Start with the default keymap
	keyMap := huh.NewDefaultKeyMap()

	// 2. Remap the Text field keys
	// We swap 'enter' to be the submission key and 'alt+enter' for new lines
	keyMap.Text.Submit = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "submit"))
	// The Prev/Next keys are meant to navigate between multiple fields (like going from an Input field to a Text field to a Select field). Since there's only one field, pressing ctrl+[ or ctrl+] has nowhere to go!
	// keyMap.Text.Prev = key.NewBinding(key.WithKeys("ctrl+["), key.WithHelp("ctrl+[", "prev"))
	// keyMap.Text.Next = key.NewBinding(key.WithKeys("ctrl+]"), key.WithHelp("ctrl+]", "next"))
	keyMap.Text.NewLine.SetHelp("ctrl+j", "new line")

	// 3. Disable the Editor (Ctrl+E) keybinding
	keyMap.Text.Editor = key.NewBinding(key.WithDisabled())

	return keyMap
}

// GetNewLineKeyBinding returns a key binding for inserting a newline
func GetNewLineKeyBinding() key.Binding {
	return key.NewBinding(key.WithKeys("ctrl+j"), key.WithHelp("ctrl+j", "insert newline"))
}

// Custom type to track cursor updates
type onCursorUpdate struct {
	Field interface{ Hovered() (string, bool) }
}

// Hash implements the hashable interface for huh
func (u onCursorUpdate) Hash() (uint64, error) {
	val, ok := u.Field.Hovered()
	if !ok {
		return 0, nil
	}
	// Simple hash - convert string to uint64
	h := uint64(0)
	for _, c := range val {
		h = h*31 + uint64(c)
	}
	return h, nil
}

func GetStaticHuhNote(title string, description string) *huh.Note {
	// Parse the JSON into StyleConfig
	var renderer *glamour.TermRenderer
	var err error
	// Create renderer with the parsed style
	renderer, err = glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		// For some reason, this doesn't work well with DescriptionFunc
		// it sometimes trims the description output
		glamour.WithStylesFromJSONBytes([]byte(compactStyleJSON)),
		glamour.WithPreservedNewLines(),
	)
	if err != nil {
		renderer, err = glamour.NewTermRenderer(glamour.WithAutoStyle())
	}

	var note *huh.Note
	if strings.TrimSpace(title) == "" {
		note = huh.NewNote()
	} else {
		note = huh.NewNote().Title(title)
	}
	renderedDesc, err := renderer.Render(description)
	if err != nil {
		renderedDesc = description
	}
	renderedDesc = strings.TrimRight(renderedDesc, "\n")
	note.Description(renderedDesc)
	lines := strings.Split(renderedDesc, "\n")
	height := len(lines) + 2
	if height < 5 {
		height = 5
	}
	if height > 20 {
		height = 20
	}
	note.Height(height)
	return note
}

/*
 *
 */
func GetDynamicHuhNote(title string, ms *huh.MultiSelect[string], descFunc func(string) string) *huh.Note {
	// Parse the JSON into StyleConfig
	var renderer *glamour.TermRenderer
	var err error
	// Create renderer with the parsed style
	renderer, err = glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		// For some reason, this doesn't work well with DescriptionFunc
		// it sometimes trims the description output
		glamour.WithStylesFromJSONBytes([]byte(compactStyleJSON)),
		glamour.WithPreservedNewLines(),
	)
	if err != nil {
		renderer, err = glamour.NewTermRenderer(glamour.WithAutoStyle())
	}

	var note *huh.Note
	if strings.TrimSpace(title) == "" {
		note = huh.NewNote()
	} else {
		note = huh.NewNote().Title(title)
	}

	// Description function
	note.DescriptionFunc(func() string {
		// Show info about the currently hovered item
		hovered, _ := ms.Hovered()
		desc := descFunc(hovered)
		renderedDesc, err := renderer.Render(desc)
		if err != nil {
			renderedDesc = desc
		}

		// Count lines in the rendered description
		lineCount := strings.Count(renderedDesc, "\n") + 1

		// Set height dynamically based on content (with a min and max)
		minHeight := 5
		maxHeight := 15
		height := lineCount + 2
		if height < minHeight {
			height = minHeight
		}
		if height > maxHeight {
			height = maxHeight
		}

		note.Height(height)

		return renderedDesc
	}, onCursorUpdate{ms})

	return note
}
