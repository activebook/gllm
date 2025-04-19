package service

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/spf13/viper"
)

// removeCitations removes citation references from text, including:
// - Parenthesized citations like ([1], [2,3])
// - Standalone citations like [1] or [2,3]
// - Empty parentheses
// - Extra spaces (collapses multiple spaces to single space)
// Returns the cleaned and trimmed text.
func removeCitations(text string) string {
	// Step 1: Remove citations inside parentheses with commas
	// This handles cases like ([1, 2, 3]) and ([7], [9], [10], [11], [12])
	reParenCitations := regexp.MustCompile(`\s*\(\s*(?:\[\s*\d+(?:\s*,\s*\d+)*\s*\](?:\s*,\s*)?)+\s*\)`)
	text = reParenCitations.ReplaceAllString(text, "")

	// Step 2: Remove standalone citations like [1], [2, 3], etc.
	reCitations := regexp.MustCompile(`\s*\[\s*\d+(?:\s*,\s*\d+)*\s*\]`)
	text = reCitations.ReplaceAllString(text, "")

	// Step 3: Remove leftover empty parentheses (with or without spaces/commas inside)
	reParens := regexp.MustCompile(`\(\s*[,]*\s*\)`)
	text = reParens.ReplaceAllString(text, "")

	// Do NOT remove commas globally!

	// Clean up extra spaces
	reSpaces := regexp.MustCompile(`[ ]{2,}`)
	text = reSpaces.ReplaceAllString(text, " ")

	text = strings.TrimSpace(text)
	return text
}

type MarkdownRenderer struct {
	buffer               strings.Builder
	keepStreamingContent bool // keep streaming output
	keepMarkdownOnly     bool // only markdown
}

// NewMarkdownRenderer creates a new instance of MarkdownRenderer
func NewMarkdownRenderer() *MarkdownRenderer {
	mr := MarkdownRenderer{}
	mark := viper.GetString("default.markdown")
	if mark == "on" {
		mr.keepStreamingContent = true
		mr.keepMarkdownOnly = false
	} else if mark == "only" {
		mr.keepStreamingContent = false
		mr.keepMarkdownOnly = true
	} else {
		mr.keepStreamingContent = false
		mr.keepMarkdownOnly = false
	}
	(&mr).StartStreaming()
	return &mr
}

// Start streaming mode (call this before any RenderString)
func (mr *MarkdownRenderer) StartStreaming() {
	if mr.keepMarkdownOnly {
		fmt.Print("\033[?1049h") // Switch to alternate screen buffer
	}
}

func (mr *MarkdownRenderer) StopStreaming() {
	if mr.keepMarkdownOnly {
		// When finished
		fmt.Print("\033[?1049l")
	}
}

// RenderString streams output incrementally and tracks the number of lines
func (mr *MarkdownRenderer) RenderString(format string, args ...interface{}) {
	if mr.keepStreamingContent || mr.keepMarkdownOnly {
		output := fmt.Sprintf(format, args...)
		mr.buffer.WriteString(output) // Write to the buffer
	}
	// Print the output to the console
	fmt.Printf(format, args...)
}

// RenderMarkdown clears the streaming output and re-renders the entire Markdown
func (mr *MarkdownRenderer) RenderMarkdown() {
	if !mr.keepStreamingContent && !mr.keepMarkdownOnly {
		return
	}

	mr.StopStreaming()
	output := mr.buffer.String()
	if len(output) == 0 {
		return
	}
	// Remove citations
	output = removeCitations(output)

	// Print a separator or message
	if mr.keepStreamingContent {
		prefix := fmt.Sprintln("") + fmt.Sprintln("")
		prefix = prefix + fmt.Sprintln("# **MARKDOWN OUTPUT**")
		prefix = prefix + fmt.Sprintln("=================")
		prefix = prefix + fmt.Sprintln()
		output = prefix + output
	}

	// Render the Markdown using glamour
	r, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(), // Auto-detect dark/light mode
		// glamour.WithWordWrap(80), // Optional: wrap output at 80 characters
	)

	out, err := r.Render(output)
	if err != nil {
		Warnf("Cannot render Markdown correctly: %v", err)
		return
	}

	// Print the rendered Markdown
	fmt.Print(out)

	// Reset the buffer and line count
	mr.buffer.Reset()
}
