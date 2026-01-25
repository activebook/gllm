// File: cmd/root.go (add this function)
package cmd

// ... other imports ...
import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/activebook/gllm/service"
)

// readStdin checks if there's piped input and reads it
// This is a more robust way to check if stdin is being piped.
// Add "console": "integratedTerminal" to launch.json for VSCode
// Because
// 1.The debugger's handling of stdin is different from normal terminal execution
// 2.The os.Stdin.Stat() check can be unreliable in debugging contexts
// 3.VS Code's debugging environment sometimes makes stdin appear as if it's being piped
func readStdin() string {
	// Check if stdin has data (is being piped)
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// Stdin is being piped
		reader := bufio.NewReader(os.Stdin)
		var buffer bytes.Buffer

		// Read all content from stdin
		_, err := io.Copy(&buffer, reader)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading from stdin: %v\n", err)
			return ""
		}

		return buffer.String()
	}

	return ""
}

func hasStdinData() bool {
	// Check if stdin has data (is being piped)
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) == 0
}

func checkIsLink(source string) bool {
	return strings.HasPrefix(source, "http") || strings.HasPrefix(source, "https")
}

type TextBuilder struct {
	builder  strings.Builder
	lastChar byte
}

// Appends text to builder with proper newline handling
func (tb *TextBuilder) appendText(text string) {
	if text == "" {
		return
	}
	if tb.builder.Len() > 0 && tb.lastChar != '\n' {
		tb.builder.WriteString("\n")
		tb.lastChar = '\n'
	}
	tb.builder.WriteString(text)
	if len(text) > 0 {
		tb.lastChar = text[len(text)-1]
	}
}

func (tb *TextBuilder) String() string {
	return tb.builder.String()
}

// readContentFromPath reads content from a specified source path.
// If the source is "-", it reads from standard input (os.Stdin).
// Otherwise, it reads from the file at the given path.
// Returns the content as a byte slice or an error if reading fails.
func readContentFromPath(source string) ([]byte, error) {
	if source == "-" {
		return io.ReadAll(os.Stdin)
	}
	if checkIsLink(source) {
		// Fetch content from the URL
		urls := []string{source}
		datas := service.FetchProcess(urls)
		return []byte(datas[0]), nil
	}
	return os.ReadFile(source)
}

func createTempFile(pattern string) (string, error) {
	// Try default temp directory first
	tempFile, err := os.CreateTemp("", pattern)
	if err == nil {
		return tempFile.Name(), tempFile.Close()
	}

	// Fallback 1: User's home directory
	if homeDir, homeErr := os.UserHomeDir(); homeErr == nil {
		tempFile, err := os.CreateTemp(homeDir, pattern)
		if err == nil {
			return tempFile.Name(), tempFile.Close()
		}
	}

	// Fallback 2: Current working directory
	cwd, cwdErr := os.Getwd()
	if cwdErr == nil {
		tempFile, err := os.CreateTemp(cwd, pattern)
		if err == nil {
			return tempFile.Name(), tempFile.Close()
		}
	}

	// If all fallbacks fail, return the original error
	return "", err
}

func StartsWith(s string, prefix string) bool {
	return strings.HasPrefix(s, prefix)
}

func RemoveFirst(s string, prefix string) string {
	return strings.TrimPrefix(s, prefix)
}

// convertUserInputToBool converts user-friendly strings to boolean values
// Handles: on/off, enable/disable, true/false, 1/0
func convertUserInputToBool(input string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "on", "enable", "enabled", "true", "1":
		return true, nil
	case "off", "disable", "disabled", "false", "0", "":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value: %s", input)
	}
}

// Define the hardcoded default system prompt
const defaultSystemPromptContent = "You are a helpful assistant."
const defaultTemplateContent = ""

func validateInt(s string) error {
	_, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("must be a valid number")
	}
	return nil
}

func toInt(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}

// copyDir recursively copies a directory
func copyDir(src, dst string) error {
	// Create destination directory
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			// Skip hidden directories
			if strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			// Skip hidden files
			if strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile copies a single file
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	// Preserve permissions
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, srcInfo.Mode())
}
