package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/activebook/gllm/data"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	suggestionTypeNone = iota
	suggestionTypeCommand
	suggestionTypeFile
)

const (
	defaultHeight  = 5 // Default height of the chat input
	maxSuggestions = 8 // Max suggestions to show
)

// ChatInputResult holds the result of the chat input
type ChatInputResult struct {
	Value    string
	Canceled bool
}

// Suggestion represents a suggestion item
type Suggestion struct {
	Command     string
	Description string
}

// ChatInputModel is the Bubble Tea model for the chat input with autocomplete
type ChatInputModel struct {
	textarea         textarea.Model
	allCommands      []Suggestion // all /commands
	filteredCommands []Suggestion // filtered /commands and file paths
	suggestionIndex  int          // index of the current suggestion
	showSuggestions  bool         // whether suggestions are shown
	width            int          // terminal width
	height           int          // terminal height
	canceled         bool         // whether the input was canceled
	submitted        bool         // whether the input was submitted
	suggestionType   int          // type of suggestion
	suggestionStart  int          // start index of the suggestion(cursor position)
}

// NewChatInputModel creates a new chat input model
func NewChatInputModel(commands []Suggestion, initialValue string) ChatInputModel {
	ta := textarea.New()
	ta.KeyMap.InsertNewline = GetNewLineKeyBinding()
	ta.Placeholder = "Type your message... (Use / for commands, @ for files, Enter to send)"
	ta.Focus()
	ta.Prompt = "┃ "
	ta.CharLimit = 0            // Unlimited
	ta.SetHeight(defaultHeight) // Start with a reasonable height
	ta.ShowLineNumbers = false
	ta.SetValue(initialValue)

	// Remove all default backgrounds and use theme colors
	baseStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(data.CurrentTheme.Foreground))

	ta.FocusedStyle.Base = baseStyle
	ta.FocusedStyle.Text = lipgloss.NewStyle()
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().
		Foreground(lipgloss.Color(data.DetailHex))
	ta.FocusedStyle.Prompt = lipgloss.NewStyle().
		Foreground(lipgloss.Color(data.LabelHex)).
		Bold(true)

	// Move cursor to end if initial value provided
	if initialValue != "" {
		ta.SetCursor(len(initialValue))
	}

	width := GetTerminalWidth()
	return ChatInputModel{
		textarea:    ta,
		allCommands: commands,
		width:       width,
	}
}

func (m ChatInputModel) Init() tea.Cmd {
	return textarea.Blink
}

// User input, move cursor, type text, all trigger Update
func (m ChatInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.textarea.SetWidth(msg.Width)
		// We don't set height here as we want it to auto-grow/shrink or be fixed small

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			m.canceled = true
			return m, tea.Quit

		case tea.KeyCtrlD:
			m.textarea.SetValue("")
			return m, nil

		case tea.KeyEnter:
			// If suggestions are shown, select the command
			if m.showSuggestions {
				m.selectSuggestion()
				return m, nil
			}

			// Otherwise submit the message
			m.submitted = true
			return m, tea.Quit

		case tea.KeyUp, tea.KeyDown:
			if m.showSuggestions {
				if msg.Type == tea.KeyUp {
					m.suggestionIndex--
					if m.suggestionIndex < 0 {
						m.suggestionIndex = len(m.filteredCommands) - 1
					}
				} else {
					m.suggestionIndex++
					if m.suggestionIndex >= len(m.filteredCommands) {
						m.suggestionIndex = 0
					}
				}
				return m, nil
			}

		case tea.KeyTab:
			if m.showSuggestions {
				m.selectSuggestion()
				return m, nil
			}

		case tea.KeyEsc:
			if m.showSuggestions {
				m.showSuggestions = false
				return m, nil
			}
		}
	}

	// Handle character input and suggestions logic
	m.textarea, cmd = m.textarea.Update(msg)

	// Detect suggestions
	val := m.textarea.Value()
	cursor := getCursorIndex(m.textarea)

	// Find word start
	start := 0
	// Bounds check
	if cursor > len(val) {
		cursor = len(val)
	}

	// Find the start of the word from backward(start from cursor position)
	for i := cursor - 1; i >= 0; i-- {
		if val[i] == ' ' || val[i] == '\n' {
			start = i + 1
			break
		}
	}

	wordSoFar := val[start:cursor]
	m.showSuggestions = false

	if strings.HasPrefix(wordSoFar, "@") {
		// File mode
		m.suggestionType = suggestionTypeFile
		m.suggestionStart = start

		// Use the substring after @ as the pattern
		// substring* as glob pattern
		pattern := wordSoFar[1:]
		matches := getFileSuggestions(pattern)
		m.filteredCommands = []Suggestion{}
		for _, match := range matches {
			m.filteredCommands = append(m.filteredCommands, Suggestion{Command: match})
		}

		// Show suggestions if there are any
		if len(m.filteredCommands) > 0 {
			m.showSuggestions = true
			// Check whether last suggestion index is valid
			if m.suggestionIndex >= len(m.filteredCommands) {
				m.suggestionIndex = 0
			}
		}
	} else if start == 0 && strings.HasPrefix(wordSoFar, "/") {
		// Command mode (only at start of line)
		m.suggestionType = suggestionTypeCommand
		m.suggestionStart = 0

		// Filter commands
		var matches []Suggestion
		for _, c := range m.allCommands {
			if strings.HasPrefix(c.Command, wordSoFar) {
				matches = append(matches, c)
			}
		}

		if len(matches) == 1 && matches[0].Command == wordSoFar {
			// Only one match, no need to show suggestions
			m.showSuggestions = false
		} else if len(matches) > 0 {
			// Multiple matches, show suggestions
			m.showSuggestions = true
			m.filteredCommands = matches
			// Check whether last suggestion index is valid
			if m.suggestionIndex >= len(matches) {
				m.suggestionIndex = 0
			}
		}
	}

	return m, cmd
}

// selectSuggestion selects the current suggestion
// It replaces the current word with the selected suggestion
// and updates the cursor position
func (m *ChatInputModel) selectSuggestion() {
	if !m.showSuggestions || len(m.filteredCommands) == 0 {
		return
	}

	selected := m.filteredCommands[m.suggestionIndex].Command
	val := m.textarea.Value()
	cursor := getCursorIndex(m.textarea)

	switch m.suggestionType {
	case suggestionTypeCommand:
		// Replace everything (since it's start of line) with command + space
		m.textarea.SetValue(selected + " ")
		m.textarea.SetCursor(len(selected) + 1)
	case suggestionTypeFile:
		// Replace @word with @selected

		// Safety check
		if m.suggestionStart > len(val) {
			m.suggestionStart = len(val)
		}
		if cursor > len(val) {
			cursor = len(val)
		}

		prefix := val[:m.suggestionStart]
		suffix := val[cursor:]

		// Note: selected file path doesn't include @
		newValue := prefix + "@" + selected

		// If it is a directory, don't add space, because it will be followed by a slash
		// And suggestion still needs to be triggered by slash again
		// If it is a file, add space, because the suggestion is already done
		if !strings.HasSuffix(selected, string(os.PathSeparator)) {
			newValue += " "
		}

		m.textarea.SetValue(newValue + suffix)
		m.textarea.SetCursor(len(newValue))
	}

	m.showSuggestions = false
}

// Helper to get absolute cursor index
func getCursorIndex(ta textarea.Model) int {
	val := ta.Value()

	// We assume Line() and LineInfo() exist on textarea.Model.
	line := ta.Line()
	col := ta.LineInfo().CharOffset

	lines := strings.Split(val, "\n")

	pos := 0
	for i := 0; i < line && i < len(lines); i++ {
		pos += len(lines[i]) + 1 // +1 for newline
	}

	if line < len(lines) {
		currentLineLen := len(lines[line])
		if col > currentLineLen {
			col = currentLineLen
		}
		pos += col
	}

	return pos
}

// getFileSuggestions returns a list of file suggestions based on the prefix
// It uses filepath.Glob to find matching files and directories
// It returns a list of file paths, with directories appended with a path separator
func getFileSuggestions(prefix string) []string {
	search := prefix + "*"
	// Handle empty prefix or current dir
	if prefix == "" {
		search = "*"
	}

	matches, err := filepath.Glob(search)
	if err != nil {
		return []string{}
	}

	var suggestions []string
	for _, m := range matches {
		info, err := os.Stat(m)
		if err == nil {
			if info.IsDir() {
				suggestions = append(suggestions, m+string(os.PathSeparator))
			} else {
				suggestions = append(suggestions, m)
			}
		}
	}

	// Simple limit
	if len(suggestions) > 20 {
		suggestions = suggestions[:20]
	}
	return suggestions
}

// View renders the chat input
// Whether to show suggestions or not is determined by the model
// View renders the program's UI, which is just a string. The view is
// rendered after every Update.
func (m ChatInputModel) View() string {
	// If input is hidden (user typed /exit), show nothing or a message
	// or if user submitted the input or use /commands
	if m.canceled || m.submitted {
		return ""
	}

	teaView := m.textarea.View()

	if !m.showSuggestions || len(m.filteredCommands) == 0 {
		return teaView
	}

	// Render suggestions
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(data.BorderHex)).
		Width(GetTerminalWidth()-2).
		Padding(0, 1)

	var listItems []string

	start := 0
	end := len(m.filteredCommands)

	// Simple scrolling
	if end > maxSuggestions {
		start = m.suggestionIndex - (maxSuggestions / 2)
		if start < 0 {
			start = 0
		}
		end = start + maxSuggestions
		if end > len(m.filteredCommands) {
			end = len(m.filteredCommands)
			start = end - maxSuggestions
		}
	}

	// Calculate max width for alignment
	maxLen := 0
	for i := start; i < end; i++ {
		if len(m.filteredCommands[i].Command) > maxLen {
			maxLen = len(m.filteredCommands[i].Command)
		}
	}

	// Render suggestions one by one, highlight the selected one
	for i := start; i < end; i++ {
		s := m.filteredCommands[i]

		// Base styles
		textStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(data.DetailHex))
		descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(data.DetailHex)).Faint(true)
		prefix := "  "

		// Selected styles
		if i == m.suggestionIndex {
			textStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(data.KeyHex)).
				Bold(true)
			descStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(data.LabelHex)) // Brighter description when selected
			prefix = "> "
		}

		// Pad the command text
		paddedText := fmt.Sprintf("%-*s", maxLen, s.Command)

		line := fmt.Sprintf("%s%s   %s", prefix, textStyle.Render(paddedText), descStyle.Render(s.Description))
		listItems = append(listItems, line)
	}

	suggestionsView := style.Render(strings.Join(listItems, "\n"))

	// Join vertical: suggestions on bottom
	return lipgloss.JoinVertical(lipgloss.Left, teaView, suggestionsView)
}

// RunChatInput runs the chat input program
func RunChatInput(commands []Suggestion, initialValue string) (ChatInputResult, error) {
	model := NewChatInputModel(commands, initialValue)
	p := tea.NewProgram(model)

	finalModel, err := p.Run()
	if err != nil {
		return ChatInputResult{}, err
	}

	m := finalModel.(ChatInputModel)
	if m.canceled {
		return ChatInputResult{Canceled: true}, nil
	}

	return ChatInputResult{Value: strings.TrimSpace(m.textarea.Value()), Canceled: false}, nil
}
