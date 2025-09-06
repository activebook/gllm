package service

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/pmezard/go-difflib/difflib"
)

func Diff(content1, content2, file1, file2 string, contextLines int) string {
	// Colors
	var red, green, cyan, yellow, dim func(string) string

	red = func(s string) string { return color.New(color.FgRed).Sprint(s) }
	green = func(s string) string { return color.New(color.FgGreen).Sprint(s) }
	cyan = func(s string) string { return color.New(color.FgCyan, color.Bold).Sprint(s) }
	yellow = func(s string) string { return color.New(color.FgYellow, color.Bold).Sprint(s) }
	dim = func(s string) string { return color.New(color.Faint).Sprint(s) }

	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(content1),
		B:        difflib.SplitLines(content2),
		FromFile: file1,
		ToFile:   file2,
		Context:  contextLines,
	}

	// Get the diff as a string
	diffText, _ := difflib.GetUnifiedDiffString(diff)

	// Parse and display the diff with line numbers
	lines := strings.Split(diffText, "\n")

	// Variables to track line numbers
	lineNum1 := 0 // Line number for file 1
	lineNum2 := 0 // Line number for file 2

	var output strings.Builder

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "---"), strings.HasPrefix(line, "+++"):
			// File headers
			output.WriteString(cyan(line) + "\n")
		case strings.HasPrefix(line, "@@"):
			// Hunk header - parse line numbers
			output.WriteString(yellow(line) + "\n")

			// Parse the hunk header to get starting line numbers
			// Format: @@ -line,count +line,count @@
			parts := strings.Split(strings.Trim(line, "@ "), " ")
			if len(parts) >= 2 {
				// Parse file1 line number (format: -line,count)
				file1Part := strings.Split(strings.TrimPrefix(parts[0], "-"), ",")
				if len(file1Part) > 0 {
					lineNum1, _ = strconv.Atoi(file1Part[0])
					lineNum1-- // Adjust for 0-based indexing
				}

				// Parse file2 line number (format: +line,count)
				file2Part := strings.Split(strings.TrimPrefix(parts[1], "+"), ",")
				if len(file2Part) > 0 {
					lineNum2, _ = strconv.Atoi(file2Part[0])
					lineNum2-- // Adjust for 0-based indexing
				}
			}
		case strings.HasPrefix(line, "-"):
			// Removed line from file 1
			lineNum1++
			lineNumStr := fmt.Sprintf("%-6d", lineNum1)
			output.WriteString(red(lineNumStr) + red(line) + "\n")
		case strings.HasPrefix(line, "+"):
			// Added line to file 2
			lineNum2++
			lineNumStr := fmt.Sprintf("%-6d", lineNum2)
			output.WriteString(green(lineNumStr) + green(line) + "\n")
		case strings.HasPrefix(line, " "):
			// Unchanged line
			lineNum1++
			lineNum2++
			lineNumStr1 := fmt.Sprintf("%-6d", lineNum1)
			lineNumStr2 := fmt.Sprintf("%-6d", lineNum2)
			// Show both line numbers for context lines
			output.WriteString(dim(lineNumStr1) + dim(lineNumStr2) + line + "\n")
		case line == "":
			// Empty line at the end
			continue
		}
	}
	return (output.String())
}
