// Package data provides the foundational data layer for all file I/O operations.
// It encapsulates config, memory, MCP, and conversation data access behind strongly-typed structs.
//
// Architecture: cmd → service → data
// The data layer is the only layer that should directly access files or viper.
package data

import (
	"os"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
)

// GetConfigDir returns the application configuration directory.
// Uses os.UserConfigDir() for cross-platform support.
// Example: ~/.config/gllm on Linux, ~/Library/Application Support/gllm on macOS
func GetConfigDir() string {
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		// Fallback to home directory if UserConfigDir fails
		userConfigDir, _ = homedir.Dir()
		userConfigDir = filepath.Join(userConfigDir, ".config")
	}
	return filepath.Join(userConfigDir, "gllm")
}

// GetConfigFilePath returns the path to the configuration file.
func GetConfigFilePath() string {
	return filepath.Join(GetConfigDir(), "gllm.yaml")
}

// GetMcpFilePath returns the path to the mcp file.
func GetMcpFilePath() string {
	return filepath.Join(GetConfigDir(), "mcp.yaml")
}

// GetMemoryFilePath returns the path to the memory file.
func GetMemoryFilePath() string {
	return filepath.Join(GetConfigDir(), "memory.json")
}

// GetConvoDirPath returns the path to the conversation directory.
func GetConvoDirPath() string {
	return filepath.Join(GetConfigDir(), "convo")
}

// EnsureConfigDir creates the config directory if it doesn't exist.
func EnsureConfigDir() error {
	return os.MkdirAll(GetConfigDir(), 0750)
}
