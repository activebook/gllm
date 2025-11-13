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
	"strings"
	"time"

	"github.com/activebook/gllm/service"
	"github.com/chzyer/readline"
	"github.com/spf13/viper"
)

// writeConfig saves the current viper configuration to the determined config file path.
// It handles creation of the directory if needed.
func writeConfig() error {
	// Get the path where viper is currently configured to write
	// If --config flag was used, it respects that. Otherwise, uses the default path.
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		// If no config file was used (e.g., it didn't exist), use the default path
		configFile = getDefaultConfigFilePath()
		// We need to explicitly tell Viper to write to this file
		viper.SetConfigFile(configFile)
	}

	// Ensure the directory exists
	configDir := filepath.Dir(configFile)
	if err := os.MkdirAll(configDir, 0750); err != nil { // Use 0750 for permissions
		return fmt.Errorf("failed to create config directory '%s': %w", configDir, err)
	}

	// Write the config file
	// Use WriteConfigAs to ensure it writes even if the file doesn't exist yet
	//logger.Debugln("Saving configuration to:", configFile) // Debug message
	if err := viper.WriteConfigAs(configFile); err != nil {
		return fmt.Errorf("failed to write configuration file '%s': %w", configFile, err)
	}

	return nil
}

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

func validFilePath(filename string, force_overwritten bool) error {

	var err error
	// Check if file already exists
	if _, err := os.Stat(filename); err == nil && !force_overwritten {
		// File exists, ask for confirmation to overwrite
		rl, err := readline.New("")
		if err != nil {
			return fmt.Errorf("error initializing readline: %v", err)
		}
		defer rl.Close()

		// Use readline's prompt
		rl.SetPrompt(fmt.Sprintf("File %s already exists. Do you want to overwrite it? (y/N): ", filename))

		input, err := rl.Readline()
		if err != nil {
			return fmt.Errorf("error reading input: %v", err)
		}
		response := strings.ToLower(strings.TrimSpace(input))
		if response != "y" && response != "yes" {
			return fmt.Errorf("%s", "file not set. keeping current output file.")
		}
	} else if !os.IsNotExist(err) {
		// There was an error checking the file (other than not existing)
		return fmt.Errorf("error checking file %s: %v", filename, err)
	}

	// Try to create the file to check if we can write to it
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("error creating directory for %s: %v", filename, err)
	}

	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("error creating/opening file %s: %v", filename, err)
	}
	file.Close()
	return nil
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

func GenerateChatFileName() string {
	// Get the current time
	currentTime := time.Now()

	// Format the time as a string in the format "chat_YYYY-MM-DD_HH-MM-SS.json"
	filename := fmt.Sprintf("chat_%s", currentTime.Format("2006-01-02_15-04-05"))

	return filename
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
