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

// GetTerminalHeight returns the height of the terminal using a robust fallback chain:
// 1. Direct TTY query via golang.org/x/term (most reliable)
// 2. Tmux pane height (if inside tmux)
// 3. LINES environment variable
// 4. tput lines command
// 5. Default fallback of 24
func GetTerminalHeight() int {
	// Priority 1: Direct TTY query via syscall (most reliable, works with tmux panes)
	for _, fd := range []int{int(os.Stdout.Fd()), int(os.Stdin.Fd()), int(os.Stderr.Fd())} {
		if _, height, err := term.GetSize(fd); err == nil && height > 0 {
			return height
		}
	}

	// Priority 2: If inside tmux, query tmux for pane height
	if os.Getenv("TMUX") != "" {
		if height := getTmuxPaneHeight(); height > 0 {
			return height
		}
	}

	// Priority 3: Check LINES environment variable (set by many shells)
	if lines := os.Getenv("LINES"); lines != "" {
		if height, err := strconv.Atoi(lines); err == nil && height > 0 {
			return height
		}
	}

	// Priority 4: Fall back to tput lines command
	if height := getTputLines(); height > 0 {
		return height
	}

	// Default fallback
	return 24
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

// getTmuxPaneHeight queries tmux for the current pane height
func getTmuxPaneHeight() int {
	cmd := exec.Command("tmux", "display-message", "-p", "#{pane_height}")
	output, err := cmd.Output()
	if err != nil {
		return 0
	}
	height, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		return 0
	}
	return height
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

// getTputLines uses tput to query terminal height
func getTputLines() int {
	cmd := exec.Command("tput", "lines")
	output, err := cmd.Output()
	if err != nil {
		return 0
	}
	height, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		return 0
	}
	return height
}

func GetTermFitHeight(lines int) int {
	height := GetTerminalHeight()

	mh := min(lines+2, height, height*3/5)
	// fmt.Printf("Terminal height: %d lines:%d minheight:%d\n", height, lines, mh)
	return mh
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
