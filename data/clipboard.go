package data

import (
	"sync"

	"github.com/atotto/clipboard"
)

var (
	clipboardText string
	clipboardMu   sync.RWMutex
)

// SaveClipboardText securely saves the latest formatted markdown response
func SaveClipboardText(text string) {
	clipboardMu.Lock()
	defer clipboardMu.Unlock()
	clipboardText = text
}

// GetClipboardText retrieves the latest formatted markdown response
func GetClipboardText() string {
	clipboardMu.RLock()
	defer clipboardMu.RUnlock()
	return clipboardText
}

func ClearClipboardText() {
	clipboardMu.Lock()
	defer clipboardMu.Unlock()
	clipboardText = ""
}

// Actually copy to clipboard using atotto/clipboard
func WriteClipboardText(text string) error {
	err := clipboard.WriteAll(text)
	if err != nil {
		return err
	}
	return nil
}
