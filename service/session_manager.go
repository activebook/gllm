package service

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/util"
)

const (
	MainSessionName = "main"
	SessionSuffix   = ".jsonl"
)

// SessionMeta metadata for a session
type SessionMeta struct {
	Name     string
	Provider string
	ModTime  int64
	Empty    bool
}

func GetSessionsDir() string {
	dir := data.GetSessionsDirPath()
	os.MkdirAll(dir, 0750)
	return dir
}

// GetSessionPath returns the absolute directory path for a session
func GetSessionPath(name string) string {
	return filepath.Join(GetSessionsDir(), name)
}

// GetSessionMainFilePath returns the absolute file path for a session's main.jsonl
func GetSessionMainFilePath(name string) string {
	return filepath.Join(GetSessionPath(name), MainSessionName+SessionSuffix)
}

// SessionExists checks if a session exists (the folder exists)
func SessionExists(name string) bool {
	info, err := os.Stat(GetSessionPath(name))
	return err == nil && info.IsDir()
}

// RenameSession renames an existing session directory
func RenameSession(oldName, newName string) error {
	oldPath := GetSessionPath(oldName)
	newPath := GetSessionPath(newName)

	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		return fmt.Errorf("session '%s' not found", oldName)
	}
	if _, err := os.Stat(newPath); err == nil {
		return fmt.Errorf("session '%s' already exists", newName)
	}

	return os.Rename(oldPath, newPath)
}

// RemoveSession deletes an entire session directory
func RemoveSession(name string) error {
	path := GetSessionPath(name)
	return os.RemoveAll(path)
}

// ClearSession clears the main session contents (deletes main.jsonl)
func ClearSession(name string) error {
	mainFilePath := GetSessionMainFilePath(name)
	return os.Remove(mainFilePath)
}

// ReadSessionContent reads the contents of the main.jsonl file
func ReadSessionContent(name string) ([]byte, error) {
	mainFilePath := GetSessionMainFilePath(name)
	return os.ReadFile(mainFilePath)
}

// WriteSessionContent writes the data into the main.jsonl file
func WriteSessionContent(name string, data []byte) error {
	mainFilePath := GetSessionMainFilePath(name)
	
	// Preserve original file mode if it exists
	if fi, err := os.Stat(mainFilePath); err == nil {
		return os.WriteFile(mainFilePath, data, fi.Mode())
	}

	// If not exist, ensure dir exists and write with default perm
	os.MkdirAll(GetSessionPath(name), 0750)
	return os.WriteFile(mainFilePath, data, 0644)
}

// EnsureSessionCompatibility checks if the existing session is compatible with the current agent's provider.
// If not, it attempts to convert the session history.
func EnsureSessionCompatibility(agent *data.AgentConfig, sessionName string) error {
	// 1. Get session Data
	sessionData, err := ReadSessionContent(sessionName)
	if err != nil {
		// If session doesn't exist, that's fine, nothing to check/convert
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	// 2. Check Compatibility
	isCompatible, provider, modelProvider := CheckSessionFormat(agent, sessionData)
	if !isCompatible {
		util.Debugf("session '%s' [%s] is not compatible with the current model provider [%s].\n", sessionName, provider, modelProvider)

		// 3. Convert Data
		convertData, err := ConvertMessages(sessionData, provider, modelProvider)
		if err != nil {
			return fmt.Errorf("error converting session: %v", err)
		}

		// 4. Write Back
		if err := WriteSessionContent(sessionName, convertData); err != nil {
			return err
		}
		util.Debugf("session '%s' converted to compatible format [%s].\n", sessionName, modelProvider)
	}

	return nil
}

// CheckSessionFormat verifies if the session data is compatible with the agent's provider.
func CheckSessionFormat(agent *data.AgentConfig, sessionData []byte) (isCompatible bool, provider string, modelProvider string) {
	modelProvider = agent.Model.Provider

	// Detect provider based on message format
	provider = DetectMessageProviderByContent(sessionData)

	// Check compatibility
	isCompatible = provider == modelProvider
	if !isCompatible {
		isCompatible = provider == ModelProviderUnknown ||
			(provider == ModelProviderOpenAI && modelProvider == ModelProviderOpenAICompatible) ||
			(provider == ModelProviderOpenAICompatible && modelProvider == ModelProviderOpenAI) ||
			(provider == ModelProviderOpenAICompatible && modelProvider == ModelProviderAnthropic)
	}

	return isCompatible, provider, modelProvider
}

// ExportSession exports a session's main.jsonl to a destination path
func ExportSession(name, destPath string) error {
	data, err := ReadSessionContent(name)
	if err != nil {
		return err
	}

	if destPath == "" {
		destPath = name + SessionSuffix
	} else if info, err := os.Stat(destPath); err == nil && info.IsDir() {
		destPath = filepath.Join(destPath, name+SessionSuffix)
	}

	return os.WriteFile(destPath, data, 0644)
}

// ClearEmptySessionsAsync clears all empty sessions in background
// An empty session is a folder whose main.jsonl file is empty or missing.
func ClearEmptySessionsAsync() {
	go func() {
		entries, err := os.ReadDir(GetSessionsDir())
		if err != nil {
			return
		}
		for _, entry := range entries {
			// Skip flat files in the root sessions directory
			if !entry.IsDir() {
				continue
			}

			sessionDir := filepath.Join(GetSessionsDir(), entry.Name())
			mainFile := filepath.Join(sessionDir, MainSessionName+SessionSuffix)

			info, err := os.Stat(mainFile)
			// Remove the entire folder if main.jsonl doesn't exist or is empty
			if err != nil || info.Size() == 0 {
				os.RemoveAll(sessionDir)
			}
		}
	}()
}

// FindSessionByIndex finds a session by index
// If the index is out of range, it returns an error
// If the index is valid, it returns the session name
func FindSessionByIndex(idx string) (string, error) {
	if strings.TrimSpace(idx) == "" {
		return "", nil
	}
	// check if it's an index
	index, err := strconv.Atoi(idx)
	if err == nil {
		// It's an index, resolve to session name using your sorted list logic
		sessions, err := ListSortedSessions(false, false)
		if err != nil {
			return "", err
		}
		if index < 1 || index > len(sessions) {
			// handle out of range
			return "", fmt.Errorf("session index out of range: %d", index)
		} else {
			title := sessions[index-1].Name
			return title, nil
		}
	} else {
		// idx is not a index
		return idx, nil
	}
}

// ListSortedSessions returns a slice of sessionMeta sorted by modTime descending
// ListSortedSessions(false, false)  // Fast - no file reads
// ListSortedSessions(true, false)   // Fast - only metadata
// ListSortedSessions(false, true)   // Slow - reads all files for provider
func ListSortedSessions(onlyNonEmpty bool, detectProvider bool) ([]SessionMeta, error) {
	sessionDir := GetSessionsDir()
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		return nil, fmt.Errorf("fail to read session directory: %v", err)
	}

	var sessions []SessionMeta
	for _, entry := range entries {
		// Only look at directories
		if !entry.IsDir() {
			continue
		}

		title := entry.Name() // folder name is the sanitized session title
		mainFile := filepath.Join(sessionDir, title, MainSessionName+SessionSuffix)

		info, err := os.Stat(mainFile)
		if err != nil {
			// Skip if no main.jsonl exists (not a valid session folder)
			continue
		}

		if onlyNonEmpty && info.Size() == 0 {
			continue
		}

		var provider string
		if detectProvider {
			provider = DetectMessageProvider(mainFile)
		}

		sessions = append(sessions, SessionMeta{
			Name:     title,
			Provider: provider,
			ModTime:  info.ModTime().Unix(),
			Empty:    info.Size() == 0,
		})
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].ModTime > sessions[j].ModTime
	})
	return sessions, nil
}

// FindSessionsByPattern finds all sessions matching a given pattern (including index, exact name, or wildcard)
func FindSessionsByPattern(pattern string) ([]string, error) {
	var matches []string

	// Try to parse as index
	index, err := strconv.Atoi(pattern)
	if err == nil {
		sessions, err := ListSortedSessions(false, true)
		if err != nil {
			return nil, err
		}
		if index >= 1 && index <= len(sessions) {
			// Use the resolved file name as the pattern
			pattern = sessions[index-1].Name
		}
	}

	// Now pattern is either a name or a wildcard
	sessionDir := GetSessionsDir()
	sessionPathPattern := filepath.Join(sessionDir, pattern)

	// Find matching directories using the pattern
	paths, err := filepath.Glob(sessionPathPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pattern: %w", err)
	}

	// Convert paths to session names (base dir name)
	for _, p := range paths {
		info, err := os.Stat(p)
		// Only include if it's a directory
		if err == nil && info.IsDir() {
			matches = append(matches, filepath.Base(p))
		}
	}

	return matches, nil
}
