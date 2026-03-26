package service

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// PluginVSCodeCompanion is the canonical plugin ID for the VSCode Companion integration.
const (
	PluginVSCodeCompanion      = "vscode-companion"
	PluginVSCodeCompanionTitle = "VS Code Companion"
	PluginVSCodeCompanionDesc  = "View and accept file changes suggested by gllm directly within VSCode — with native inline diffs."
)

// diffMsg represents the JSON payload expected by the VSCode companion extension.
type diffMsg struct {
	FilePath   string `json:"filePath"`
	NewContent string `json:"newContent"`
}

// companionSocket resolves the appropriate network and address for the companion extension's socket.
func companionSocket() (string, string) {
	if runtime.GOOS == "windows" {
		return "pipe", "\\\\.\\pipe\\gllm-companion"
	}
	return "unix", filepath.Join(os.TempDir(), "gllm-companion.sock")
}

// SendVSCodeDiff attempts to send the file path and new content to the VSCode companion extension.
// It executes a fire-and-forget socket connection with a short timeout.
func SendVSCodeDiff(filePath, newContent string) error {
	network, addr := companionSocket()

	// Use a short dial timeout since this is an optional companion feature.
	// If the extension isn't running, it should fail fast without blocking the CLI.
	conn, err := net.DialTimeout(network, addr, 500*time.Millisecond)
	if err != nil {
		return fmt.Errorf("failed to connect to companion extension (%s): %w", addr, err)
	}
	// The VSCode companion responds to the 'end' event to signify the payload is complete.
	defer conn.Close()

	payload := diffMsg{
		FilePath:   filePath,
		NewContent: newContent,
	}

	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(payload); err != nil {
		return fmt.Errorf("failed to encode diff payload: %w", err)
	}

	return nil
}
