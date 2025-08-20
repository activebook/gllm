package service

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/glamour"
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
	buffer           strings.Builder
	keepMarkdown     bool // keep streaming output
	keepMarkdownOnly bool // only markdown
}

// NewMarkdownRenderer creates a new instance of MarkdownRenderer
func NewMarkdownRenderer(keep bool) *MarkdownRenderer {
	mr := MarkdownRenderer{}
	mr.keepMarkdown = keep
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
	if mr.keepMarkdown || mr.keepMarkdownOnly {
		output := fmt.Sprintf(format, args...)
		mr.buffer.WriteString(output) // Write to the buffer
	}
}

// RenderMarkdown clears the streaming output and re-renders the entire Markdown
func (mr *MarkdownRenderer) RenderMarkdown() {
	if !mr.keepMarkdown && !mr.keepMarkdownOnly {
		// When markdown is off, we need to ensure the output ends with a newline
		// to prevent the shell from displaying % at the end
		fmt.Println()
		return
	}

	mr.StopStreaming()
	output := mr.buffer.String()
	if len(output) == 0 {
		return
	}

	// Remove citations
	// Only gemini has citations
	//output = removeCitations(output)

	// Print a separator or message
	if mr.keepMarkdown {
		prefix := "\n\n---\n\n# **MARKDOWN OUTPUT**\n\n---\n\n"
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

	// Ensure output ends with a newline to prevent shell from displaying %
	// the % character in shells like zsh when output doesn't end with newline
	if !strings.HasSuffix(out, "\n") {
		fmt.Println()
	}

	// Reset the buffer and line count
	mr.buffer.Reset()
}

type StdRenderer struct {
}

func NewStdRenderer() *StdRenderer {
	return &StdRenderer{}
}

func (r *StdRenderer) RenderString(format string, args ...interface{}) {
	// Print the output to the console
	fmt.Printf(format, args...)
}
