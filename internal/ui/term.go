package ui

import (
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/muesli/termenv"
	"golang.org/x/term"
)

// GetTerminalWidth returns the width of the terminal using a robust fallback chain:
// 1. Direct TTY query via golang.org/x/term (most reliable)
// 2. Tmux pane width (if inside tmux)
// 3. COLUMNS environment variable
// 4. tput cols command
// 5. Default fallback of 80
func GetTerminalWidth() int {
	// Priority 1: Direct TTY query via syscall (most reliable, works with tmux panes)
	for _, fd := range []int{int(os.Stdout.Fd()), int(os.Stdin.Fd()), int(os.Stderr.Fd())} {
		if width, _, err := term.GetSize(fd); err == nil && width > 0 {
			return width
		}
	}

	// Priority 2: If inside tmux, query tmux for pane width
	if os.Getenv("TMUX") != "" {
		if width := getTmuxPaneWidth(); width > 0 {
			return width
		}
	}

	// Priority 3: Check COLUMNS environment variable (set by many shells)
	if cols := os.Getenv("COLUMNS"); cols != "" {
		if width, err := strconv.Atoi(cols); err == nil && width > 0 {
			return width
		}
	}

	// Priority 4: Fall back to tput cols command
	if width := getTputCols(); width > 0 {
		return width
	}

	// Default fallback
	return 80
}

// getTmuxPaneWidth queries tmux for the current pane width
func getTmuxPaneWidth() int {
	cmd := exec.Command("tmux", "display-message", "-p", "#{pane_width}")
	output, err := cmd.Output()
	if err != nil {
		return 0
	}
	width, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		return 0
	}
	return width
}

// getTputCols uses tput to query terminal width
func getTputCols() int {
	cmd := exec.Command("tput", "cols")
	output, err := cmd.Output()
	if err != nil {
		return 0
	}
	width, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		return 0
	}
	return width
}

// TerminalSupportsTrueColor detects if the terminal supports true color (24-bit)
// Returns true if COLORTERM is set to "truecolor" or "24bit"
func TerminalSupportsTrueColor() bool {
	/*
		colorTerm := os.Getenv("COLORTERM")
		return colorTerm == "truecolor" || colorTerm == "24bit"
	*/
	return termenv.ColorProfile() == termenv.TrueColor
}
