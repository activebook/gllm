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
	"sync"
	"time"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/internal/event"
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
	ActionOpenDiff     companionAction = "openDiff"
	ActionDiffAccepted companionAction = "diffAccepted"
	ActionDiffRejected companionAction = "diffRejected"
	ActionGetContext   companionAction = "getContext"
	ActionSubscribe    companionAction = "subscribe"
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

// IsVSCodePluginEnabled checks if the VSCode plugin is enabled.
func IsVSCodePluginEnabled() bool {
	return data.GetSettingsStore().IsPluginEnabled(PluginVSCodeCompanion)
}

// SendVSCodeOpenDiff sends the proposed file changes to VSCode for inline diffing before confirmation.
func SendVSCodeOpenDiff(filePath, newContent string) {
	if !IsVSCodePluginEnabled() || filePath == "" {
		return
	}
	go func() {
		_ = sendCompanion(companionMsg{
			Action:     ActionOpenDiff,
			FilePath:   filePath,
			NewContent: newContent,
		})
	}()
}

// SendVSCodeDiffAccepted notifies VSCode that the file was successfully written to disk, permitting a clean reload.
func SendVSCodeDiffAccepted(filePath string) {
	if !IsVSCodePluginEnabled() || filePath == "" {
		return
	}
	go func() {
		_ = sendCompanion(companionMsg{
			Action:   ActionDiffAccepted,
			FilePath: filePath,
		})
	}()
}

// SendVSCodeDiffRejected notifies VSCode that the change was cancelled, reverting any dirty buffer.
func SendVSCodeDiffRejected(filePath string) {
	if !IsVSCodePluginEnabled() || filePath == "" {
		return
	}
	go func() {
		_ = sendCompanion(companionMsg{
			Action:   ActionDiffRejected,
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

// fetchVSCodeCurrentContext fetches real-time state from the VSCode companion plugin synchronously.
func fetchVSCodeCurrentContext() (*EditorContext, error) {
	network, addr := companionSocket()
	conn, err := net.DialTimeout(network, addr, 500*time.Millisecond)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to companion extension")
	}
	defer conn.Close()

	msg := companionMsg{Action: ActionGetContext}
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

// GetVSCodeContext formats the current VSCode state into a JSON block suitable for LLM injection.
func GetVSCodeContext() string {
	if !IsVSCodePluginEnabled() {
		return ""
	}

	// We unmarshal to validate structure and filter out omitted fields
	ctx, err := fetchVSCodeCurrentContext()
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

// --- VSCode Event Bus ---
// VSCode extension can send events to the CLI to control the UI.
// The events are sent through the companion socket, not through pipe line.
// Note: This is a one-way communication channel from VSCode to CLI.

var (
	startVSCodeEventBusOnce sync.Once
)

// StartVSCodeEventBus connects to the extension as a subscriber and continuously
// reads push events. Auto-reconnects on disconnect. Call once at startup.
func StartVSCodeEventBus() {
	if !IsVSCodePluginEnabled() {
		return
	}
	// Ensure we only start the goroutine ONCE
	startVSCodeEventBusOnce.Do(func() {
		go func() {
			for {
				ListenVSCodeEvents()
				time.Sleep(2 * time.Second) // backoff before retry
			}
		}()
	})
}

// ListenVSCodeEvents listens for push events from the VSCode companion extension.
func ListenVSCodeEvents() {
	network, addr := companionSocket()
	conn, err := net.DialTimeout(network, addr, 500*time.Millisecond)
	if err != nil {
		return
	}
	defer conn.Close()

	// Register as subscriber — extension keeps this socket alive
	err = json.NewEncoder(conn).Encode(companionMsg{Action: ActionSubscribe})
	if err != nil {
		return
	}

	decoder := json.NewDecoder(conn)
	for {
		var msg companionMsg
		if err := decoder.Decode(&msg); err != nil {
			return // EOF = disconnect
		}
		switch msg.Action {
		case ActionDiffAccepted:
			event.GetVSCodeConfirmBus().Confirm(msg.FilePath)
		case ActionDiffRejected:
			event.GetVSCodeConfirmBus().Reject(msg.FilePath)
		default:
			// Unknown action — ignore, keep listening
		}
	}
}
