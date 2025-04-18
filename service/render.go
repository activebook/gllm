package service

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/spf13/viper"
)

func removeCitations(text string) string {
	// Removes an optional space before citations like [1], [2, 3], etc. and citations like [1-3].
	re := regexp.MustCompile(`\s?\[\s*\d+(?:\s*,\s*\d+)*\s*\]`)
	return re.ReplaceAllString(text, "")
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
