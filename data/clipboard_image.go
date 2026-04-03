package data

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// ClipboardImage encapsulates raw image data extracted from the system clipboard.
type ClipboardImage struct {
	Data []byte
	Mime string
	Ext  string
}

// ReadClipboardImage attempts to extract an image from the system clipboard.
// Returns a ClipboardImage if successful, or an error if no image is present or extraction fails.
func ReadClipboardImage() (*ClipboardImage, error) {
	switch runtime.GOOS {
	case "darwin":
		return readClipboardImageDarwin()
	case "linux":
		return readClipboardImageLinux()
	case "windows":
		return readClipboardImageWindows()
	default:
		return nil, fmt.Errorf("unsupported platform for clipboard image extraction: %s", runtime.GOOS)
	}
}

func readClipboardImageDarwin() (*ClipboardImage, error) {
	// Create a temporary file to hold the extracted image
	tmpFile, err := os.CreateTemp("", "gllm_paste_*.png")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close() // Close immediately so osascript can open it for writing
	defer os.Remove(tmpPath)

	// AppleScript snippet to query and extract «class PNGf» (PNG picture data)
	script := fmt.Sprintf(`
		try
			set theFile to (POSIX file "%s")
			open for access theFile with write permission
			write (the clipboard as «class PNGf») to theFile
			close access theFile
			return "OK"
		on error errMsg
			try
				close access theFile
			end try
			return errMsg
		end try
	`, tmpPath)

	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("osascript execution failed: %w, output: %s", err, string(out))
	}

	result := strings.TrimSpace(string(out))
	if result != "OK" {
		return nil, fmt.Errorf("clipboard does not contain a PNG image or extraction failed: %s", result)
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read extracted image: %w", err)
	}

	return &ClipboardImage{
		Data: data,
		Mime: "image/png",
		Ext:  ".png",
	}, nil
}

func readClipboardImageLinux() (*ClipboardImage, error) {
	// Priority 1: Wayland via wl-paste
	if _, err := exec.LookPath("wl-paste"); err == nil {
		out, err := exec.Command("wl-paste", "--list-types").Output()
		if err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "image/") {
					data, err := exec.Command("wl-paste", "--type", line).Output()
					if err != nil {
						return nil, fmt.Errorf("wl-paste extraction failed: %w", err)
					}
					return &ClipboardImage{
						Data: data,
						Mime: line,
						Ext:  mimeToExt(line),
					}, nil
				}
			}
		}
	}

	// Priority 2: X11 via xclip
	if _, err := exec.LookPath("xclip"); err == nil {
		out, err := exec.Command("xclip", "-selection", "clipboard", "-t", "TARGETS", "-o").Output()
		if err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "image/") {
					data, err := exec.Command("xclip", "-selection", "clipboard", "-t", line, "-o").Output()
					if err != nil {
						return nil, fmt.Errorf("xclip extraction failed: %w", err)
					}
					return &ClipboardImage{
						Data: data,
						Mime: line,
						Ext:  mimeToExt(line),
					}, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("no image found on clipboard or required tools (wl-paste, xclip) are missing")
}

func readClipboardImageWindows() (*ClipboardImage, error) {
	// Create a temp file to hold the extracted image
	tmpFile, err := os.CreateTemp("", "gllm_paste_*.png")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close() // Close immediately so PowerShell can write to it
	defer os.Remove(tmpPath)

	// PowerShell script to retrieve image and save it
	script := fmt.Sprintf(`
		$img = Get-Clipboard -Format Image
		if ($img -ne $null) {
			$img.Save('%s')
			Write-Output 'OK'
		} else {
			Write-Output 'NO_IMAGE'
		}
	`, tmpPath)

	out, err := exec.Command("powershell", "-NoProfile", "-Command", script).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("powershell execution failed: %w, output: %s", err, string(out))
	}

	result := strings.TrimSpace(string(out))
	if result != "OK" {
		return nil, fmt.Errorf("clipboard does not contain an image: %s", result)
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read extracted image: %w", err)
	}

	return &ClipboardImage{
		Data: data,
		Mime: "image/png",
		Ext:  ".png",
	}, nil
}

// Helper to reliably map mime type to extension
func mimeToExt(mimeType string) string {
	switch mimeType {
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ".png" // Fallback / explicit match for image/png
	}
}
