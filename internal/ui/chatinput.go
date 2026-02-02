package ui

import (
	"fmt"
	"strings"

	"github.com/activebook/gllm/data"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ChatInputResult holds the result of the chat input
type ChatInputResult struct {
	Value    string
	Canceled bool
}

// ChatInputModel is the Bubble Tea model for the chat input with autocomplete
type ChatInputModel struct {
	textarea         textarea.Model
	allCommands      []string
	filteredCommands []string
	suggestionIndex  int
	showSuggestions  bool
	width            int
	height           int
	canceled         bool
	submitted        bool
}

// NewChatInputModel creates a new chat input model
func NewChatInputModel(commands []string, initialValue string) ChatInputModel {
	ta := textarea.New()
	ta.KeyMap.InsertNewline = GetNewLineKeyBinding()
	ta.Placeholder = "Type your message... (Use / for commands, Enter to send)"
	ta.Focus()
	ta.Prompt = "┃ "
	ta.CharLimit = 0 // Unlimited
	ta.SetHeight(5)  // Start with a reasonable height
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

	// Check for command prefix
	val := m.textarea.Value()
	trimmedVal := strings.TrimSpace(val)

	// Only trigger suggestions if input starts with / AND we're still typing the command
	if strings.HasPrefix(trimmedVal, "/") {
		parts := strings.Fields(trimmedVal)

		// Only show suggestions if:
		// 1. We only have the command part (no space yet, or just one word)
		// 2. The command is not complete (not in allCommands list)

		if len(parts) == 0 {
			// Just "/" typed
			m.showSuggestions = true
			m.filteredCommands = m.allCommands
			m.suggestionIndex = 0
		} else if len(parts) == 1 {
			// Only one word, could be incomplete command
			typedCommand := parts[0]

			// Check if this is an exact match to a known command
			isExactMatch := false
			for _, cmd := range m.allCommands {
				if cmd == typedCommand {
					isExactMatch = true
					break
				}
			}

			// If exact match AND there's a space after, user is typing args
			if isExactMatch && strings.HasSuffix(val, " ") {
				m.showSuggestions = false
			} else {
				// Still typing the command, show suggestions
				m.showSuggestions = true

				var matches []string
				for _, c := range m.allCommands {
					if strings.HasPrefix(c, typedCommand) {
						matches = append(matches, c)
					}
				}

				m.filteredCommands = matches

				if len(matches) == 0 {
					m.showSuggestions = false
				}

				// Reset index if out of bounds
				if m.suggestionIndex >= len(m.filteredCommands) {
					m.suggestionIndex = 0
				}
			}
		} else {
			// Multiple words means user is typing subcommands/arguments
			// Don't show autocomplete for the main command anymore
			m.showSuggestions = false
		}
	} else {
		m.showSuggestions = false
	}

	return m, cmd
}

func (m *ChatInputModel) selectSuggestion() {
	if len(m.filteredCommands) > 0 && m.suggestionIndex < len(m.filteredCommands) {
		selected := m.filteredCommands[m.suggestionIndex]
		m.textarea.SetValue(selected + " ")
		m.textarea.SetCursor(len(selected) + 1)
		m.showSuggestions = false
	}
}

func (m ChatInputModel) View() string {
	// If input is hidden (user typed /exit), show nothing or a message
	// or if user submitted the input or use /commands
	if m.canceled || m.submitted {
		return ""
	}

	textAreaView := m.textarea.View()

	if !m.showSuggestions || len(m.filteredCommands) == 0 {
		return textAreaView
	}

	// Render suggestions
	// We'll render them *above* the text area

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(data.BorderHex)).
		// Background(lipgloss.Color(data.BackgroundHex)).
		Padding(0, 1)

	var listItems []string
	maxItems := 5 // Max suggestions to show

	start := 0
	end := len(m.filteredCommands)

	// Simple scrolling if many items (centering selection)
	if end > maxItems {
		start = m.suggestionIndex - (maxItems / 2)
		if start < 0 {
			start = 0
		}
		end = start + maxItems
		if end > len(m.filteredCommands) {
			end = len(m.filteredCommands)
			start = end - maxItems
		}
	}

	for i := start; i < end; i++ {
		cmd := m.filteredCommands[i]
		itemStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(data.DetailHex))
		prefix := "  "

		if i == m.suggestionIndex {
			itemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(data.KeyHex)).
				// Background(lipgloss.Color(data.BackgroundHex)).
				Bold(true)
			prefix = "> "
		}
		listItems = append(listItems, itemStyle.Render(fmt.Sprintf("%s%s", prefix, cmd)))
	}

	suggestionsView := style.Render(strings.Join(listItems, "\n"))

	// Join vertical: suggestions on top
	return lipgloss.JoinVertical(lipgloss.Left, suggestionsView, textAreaView)
}

// RunChatInput runs the chat input program
func RunChatInput(commands []string, initialValue string) (ChatInputResult, error) {
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
