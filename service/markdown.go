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

type Markdown struct {
	buffer strings.Builder
}

// NewMarkdown creates a new instance of Markdown
func NewMarkdown() *Markdown {
	mr := Markdown{}
	//(&mr).StartStreaming()
	return &mr
}

// Start streaming mode (call this before any RenderString)
// func (mr *MarkdownRenderer) StartStreaming() {
// 	if mr.keepMarkdownOnly {
// 		fmt.Print("\033[?1049h") // Switch to alternate screen buffer
// 	}
// }

// func (mr *MarkdownRenderer) StopStreaming() {
// 	if mr.keepMarkdownOnly {
// 		// When finished
// 		fmt.Print("\033[?1049l")
// 	}
// }

// RenderString streams output incrementally and tracks the number of lines
func (mr *Markdown) Writef(format string, args ...interface{}) {
	output := fmt.Sprintf(format, args...)
	mr.buffer.WriteString(output) // Write to the buffer
}

func (mr *Markdown) Write(args ...interface{}) {
	output := fmt.Sprint(args...)
	mr.buffer.WriteString(output) // Write to the buffer
}

// RenderMarkdown clears the streaming output and re-renders the entire Markdown
func (mr *Markdown) Render(r Render) {

	// When markdown is off, we need to ensure the output ends with a newline
	// to prevent the shell from displaying % at the end
	//r.Writeln()

	//mr.StopStreaming()

	output := mr.buffer.String()
	if len(output) == 0 {
		return
	}

	// Remove citations
	// Only gemini has citations
	//output = removeCitations(output)

	// Print a separator or message

	prefix := "\n\n| • **MARKDOWN OUTPUT** • |\n|---|\n|" // Using backticks for highlighting in Markdown
	output = prefix + output

	// Render the Markdown using glamour
	tr, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(), // Auto-detect dark/light mode
		// glamour.WithWordWrap(80), // Optional: wrap output at 80 characters
	)

	out, err := tr.Render(output)
	if err != nil {
		Warnf("Cannot render Markdown correctly: %v", err)
		return
	}

	// Print the rendered Markdown
	r.Write(out)

	// Ensure output ends with a newline to prevent shell from displaying %
	// the % character in shells like zsh when output doesn't end with newline
	if !strings.HasSuffix(out, "\n") {
		r.Writeln("")
	}

	// Reset the buffer and line count
	mr.buffer.Reset()
}
