package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/util"
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
	ActionContext companionAction = "context"
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

// EditorContext describes the state of the VSCode environment
type EditorContext struct {
	ActiveEditor     *ActiveEditor    `json:"activeEditor"`
	OpenFiles        []EditorOpenFile `json:"otherOpenFiles"`
	WorkspaceFolders []string         `json:"workspaceFolders"`
}

// ActiveEditor represents the active editor in VSCode
// We don't need Content and VisibleRanges for now
// For Content, model can use filePath to read the file
// For VisibleRanges, model can use cursorPosition to infer the visible range
type ActiveEditor struct {
	FilePath   string `json:"filePath"`
	LanguageId string `json:"languageId"`
	IsDirty    bool   `json:"isDirty"`
	// Content        string            `json:"content"`
	Selections     []EditorSelection `json:"selections"`
	CursorPosition EditorPosition    `json:"cursorPosition"`
	// VisibleRanges  []EditorRange     `json:"visibleRanges"`
}

type EditorSelection struct {
	Start EditorPosition `json:"start"`
	End   EditorPosition `json:"end"`
	Text  string         `json:"text"`
}

type EditorPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// type EditorRange struct {
// 	Start EditorPosition `json:"start"`
// 	End   EditorPosition `json:"end"`
// }

type EditorOpenFile struct {
	FilePath string `json:"filePath"`
	IsDirty  bool   `json:"isDirty"`
}

// fetchVSCodeContext fetches real-time state from the VSCode companion plugin synchronously.
func fetchVSCodeContext() (*EditorContext, error) {
	network, addr := companionSocket()
	conn, err := net.DialTimeout(network, addr, 500*time.Millisecond)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to companion extension")
	}
	defer conn.Close()

	msg := companionMsg{Action: ActionContext}
	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(msg); err != nil {
		return nil, err
	}

	// Try to half-close if supported
	if unixConn, ok := conn.(*net.UnixConn); ok {
		_ = unixConn.CloseWrite()
	}

	// Read response (extension will close socket when done or we read until EOF)
	raw, err := io.ReadAll(conn)
	if err != nil {
		return nil, err
	}
	// Avoid trailing newlines
	rawStr := strings.TrimSpace(string(raw))

	var ctx EditorContext
	if err := json.Unmarshal([]byte(rawStr), &ctx); err != nil {
		return nil, err
	}

	return &ctx, nil
}

// GetVSCodeContextString formats the current VSCode state into a JSON block suitable for LLM injection.
func GetVSCodeContextString() string {
	if !data.GetSettingsStore().IsPluginEnabled(PluginVSCodeCompanion) {
		return ""
	}

	// We unmarshal to validate structure and filter out omitted fields
	ctx, err := fetchVSCodeContext()
	if err != nil || ctx == nil {
		return "" // Silently fallback if VSCode is not running or error
	}

	jsonBytes, err := json.MarshalIndent(ctx, "", "  ")
	if err != nil {
		return ""
	}

	context := fmt.Sprintf("Here is the user's editor context as a JSON object.\n```json\n%s\n```\n", string(jsonBytes))
	util.Debugf("VSCode Context: %s\n", context)
	return context
}
