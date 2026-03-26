package service

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/activebook/gllm/data"
)

// PluginVSCodeCompanion is the canonical plugin ID for the VSCode Companion integration.
const (
	PluginVSCodeCompanion      = "vscode-companion"
	PluginVSCodeCompanionTitle = "VS Code Companion"
	PluginVSCodeCompanionDesc  = "View and accept file changes suggested by gllm directly within VSCode — with native inline diffs."
)

// companionAction defines the type of event sent to the VSCode companion.
type companionAction string

const (
	ActionPreview companionAction = "preview"
	ActionSaved   companionAction = "saved"
	ActionDiscard companionAction = "discard"
)

// companionMsg represents the JSON payload expected by the VSCode companion extension.
type companionMsg struct {
	Action     companionAction `json:"action"`
	FilePath   string          `json:"filePath"`
	NewContent string          `json:"newContent,omitempty"`
}

// companionSocket resolves the appropriate network and address for the companion extension's socket.
func companionSocket() (string, string) {
	if runtime.GOOS == "windows" {
		return "pipe", "\\\\.\\pipe\\gllm-companion"
	}
	return "unix", filepath.Join(os.TempDir(), "gllm-companion.sock")
}

func sendCompanion(msg companionMsg) error {
	network, addr := companionSocket()

	// Use a short dial timeout since this is an optional companion feature.
	// If the extension isn't running, it should fail fast without blocking the CLI.
	conn, err := net.DialTimeout(network, addr, 500*time.Millisecond)
	if err != nil {
		return fmt.Errorf("failed to connect to companion extension (%s): %w", addr, err)
	}
	// The VSCode companion responds to the 'end' event to signify the payload is complete.
	defer conn.Close()

	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(msg); err != nil {
		return fmt.Errorf("failed to encode diff payload: %w", err)
	}

	return nil
}

// SendVSCodePreview sends the proposed file changes to VSCode for inline diffing before confirmation.
func SendVSCodePreview(filePath, newContent string) {
	if !data.GetSettingsStore().IsPluginEnabled(PluginVSCodeCompanion) || filePath == "" {
		return
	}
	go func() {
		_ = sendCompanion(companionMsg{
			Action:     ActionPreview,
			FilePath:   filePath,
			NewContent: newContent,
		})
	}()
}

// SendVSCodeSaved notifies VSCode that the file was successfully written to disk, permitting a clean reload.
func SendVSCodeSaved(filePath string) {
	if !data.GetSettingsStore().IsPluginEnabled(PluginVSCodeCompanion) || filePath == "" {
		return
	}
	go func() {
		_ = sendCompanion(companionMsg{
			Action:   ActionSaved,
			FilePath: filePath,
		})
	}()
}

// SendVSCodeDiscard notifies VSCode that the change was cancelled, reverting any dirty buffer.
func SendVSCodeDiscard(filePath string) {
	if !data.GetSettingsStore().IsPluginEnabled(PluginVSCodeCompanion) || filePath == "" {
		return
	}
	go func() {
		_ = sendCompanion(companionMsg{
			Action:   ActionDiscard,
			FilePath: filePath,
		})
	}()
}
