package util

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// HasGit checks if the git executable is available in the system PATH.
func HasGit() bool {
	// Debug
	// return false
	_, err := exec.LookPath("git")
	return err == nil
}

// IsGitHubURL checks if the given URL is a standard GitHub repository URL.
func IsGitHubURL(urlStr string) bool {
	return strings.HasPrefix(urlStr, "https://github.com/") || strings.HasPrefix(urlStr, "http://github.com/")
}

// GetGitHubZipURL converts a GitHub repository clone URL to its zip archive download URL.
func GetGitHubZipURL(urlStr string) string {
	// Remove .git suffix if present
	urlStr = strings.TrimSuffix(urlStr, ".git")

	// Ensure no trailing slash
	urlStr = strings.TrimSuffix(urlStr, "/")

	// Format: https://github.com/user/repo/archive/refs/heads/main.zip
	// Note: We default to 'main' branch here as it's the modern standard. If 'master' is needed, it might fail to find the zip via this simple fallback.
	return fmt.Sprintf("%s/archive/refs/heads/main.zip", urlStr)
}

// DownloadAndExtractZip downloads a zip file from the given URL and extracts it to the target directory.
// It creates the target directory if it doesn't exist.
// This function expects the zip file to contain a single root directory (like GitHub zips do)
// and extracts the *contents* of that root directory directly into destDir.
func DownloadAndExtractZip(urlStr, destDir string) error {
	// 1. Download the zip file to a temporary location
	tmpZipFile, err := os.CreateTemp("", "gllm-skill-zip-*.zip")
	if err != nil {
		return fmt.Errorf("failed to create temp file for download: %w", err)
	}
	defer os.Remove(tmpZipFile.Name())
	defer tmpZipFile.Close()

	fmt.Printf("Downloading archive from %s...\n", urlStr)
	resp, err := http.Get(urlStr)
	if err != nil {
		return fmt.Errorf("failed to download from %s: %w", urlStr, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Fallback to checking 'master' branch if 'main' returns 404
		if resp.StatusCode == http.StatusNotFound && strings.HasSuffix(urlStr, "main.zip") {
			masterUrl := strings.Replace(urlStr, "main.zip", "master.zip", 1)
			return DownloadAndExtractZip(masterUrl, destDir)
		}
		return fmt.Errorf("bad status: %s when downloading from %s", resp.Status, urlStr)
	}

	// Create a progress writer to show download progress
	pw := &ProgressWriter{Total: resp.ContentLength}
	if _, err = io.Copy(io.MultiWriter(tmpZipFile, pw), resp.Body); err != nil {
		return fmt.Errorf("failed to save downloaded zip: %w", err)
	}
	fmt.Println() // New line after progress

	// Ensure all data is written before unzipping
	tmpZipFile.Close()

	// 2. Extract the zip file
	fmt.Printf("Extracting to %s...\n", destDir)
	reader, err := zip.OpenReader(tmpZipFile.Name())
	if err != nil {
		return fmt.Errorf("failed to open downloaded zip: %w", err)
	}
	defer reader.Close()

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// GitHub zips put everything inside a single root folder (e.g., repo-name-main/)
	// We want to skip this root folder when extracting to the destination.
	var rootPrefix string
	if len(reader.File) > 0 {
		rootPrefix = reader.File[0].Name
		if !strings.HasSuffix(rootPrefix, "/") {
			// If the first file isn't a directory, we don't have a common root to strip, which is unexpected for GitHub zips but handleable
			rootPrefix = ""
		}
	}

	totalFiles := len(reader.File)
	for i, file := range reader.File {
		// Skip root directory entry itself
		if file.Name == rootPrefix {
			continue
		}

		// Show extraction progress for every 10% or if it's the last file
		if i%10 == 0 || i == totalFiles-1 {
			fmt.Printf("\rExtracting files: %d%% (%d/%d)", (i+1)*100/totalFiles, i+1, totalFiles)
		}

		// Calculate relative path inside the zip, stripping the root folder name
		relPath := file.Name
		if rootPrefix != "" && strings.HasPrefix(relPath, rootPrefix) {
			relPath = strings.TrimPrefix(relPath, rootPrefix)
		}

		path := filepath.Join(destDir, relPath)

		// Prevent zip slip vulnerability
		if !strings.HasPrefix(path, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", path)
		}

		if file.FileInfo().IsDir() {
			os.MkdirAll(path, file.Mode())
			continue
		}

		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}

		outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return err
		}

		rc, err := file.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()

		if err != nil {
			return err
		}
	}
	fmt.Println("\nExtraction complete.")

	return nil
}

// ProgressWriter tracks the number of bytes written to it.
type ProgressWriter struct {
	Total      int64
	Downloaded int64
}

func (pw *ProgressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.Downloaded += int64(n)
	pw.showProgress()
	return n, nil
}

func (pw *ProgressWriter) showProgress() {
	// \033[K instructs the terminal to "clear from the cursor to the end of the line" right after the carriage return moves the cursor to the beginning of the line.
	if pw.Total > 0 {
		fmt.Printf("\r\033[KDownloading: %d%% (%s / %s)", pw.Downloaded*100/pw.Total, formatBytes(pw.Downloaded), formatBytes(pw.Total))
	} else {
		fmt.Printf("\r\033[KDownloading: %s", formatBytes(pw.Downloaded))
	}
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
