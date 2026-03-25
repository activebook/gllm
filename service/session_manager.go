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
	MainSessionName      = "main"
	SessionFileExtension = ".jsonl"
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

// GetSessionFilePath returns the absolute file path for a session's specific jsonl file.
// If name is "sessionA::taskB", it returns "sessions/sessionA/taskB.jsonl"
// If name is "sessionA", it returns "sessions/sessionA/main.jsonl"
func GetSessionFilePath(name string) string {
	parts := strings.Split(name, "::")
	if len(parts) == 2 {
		return filepath.Join(GetSessionPath(parts[0]), util.GetSanitizeTitle(parts[1])+SessionFileExtension)
	}
	return filepath.Join(GetSessionPath(name), MainSessionName+SessionFileExtension)
}

// GetSessionMainFilePath returns the absolute file path for a session's main.jsonl
func GetSessionMainFilePath(name string) string {
	return filepath.Join(GetSessionPath(name), MainSessionName+SessionFileExtension)
}

// SessionExists checks if a top-level session folder exists, or if a subagent file exists
func SessionExists(name string, checkSubAgent bool) bool {
	parts := strings.Split(name, "::")
	if len(parts) >= 2 {
		if !checkSubAgent {
			return false
		}
		path := GetSessionFilePath(name)
		info, err := os.Stat(path)
		return err == nil && !info.IsDir() && info.Size() > 0
	}
	path := GetSessionPath(name)
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// RenameSession renames an existing session directory or subagent file
func RenameSession(oldName, newName string) error {
	partsOld := strings.Split(oldName, "::")

	if len(partsOld) == 2 {
		partsNew := strings.Split(newName, "::")
		if len(partsNew) == 1 {
			newName = partsOld[0] + "::" + newName
		} else if partsNew[0] != partsOld[0] {
			return fmt.Errorf("cannot move subagent session to a different parent session")
		} else if len(partsNew) == 2 {
			newName = partsOld[0] + "::" + partsNew[1]
		}

		oldPath := GetSessionFilePath(oldName)
		newPath := GetSessionFilePath(newName)

		if _, err := os.Stat(oldPath); os.IsNotExist(err) {
			return fmt.Errorf("subagent session '%s' not found", oldName)
		}
		if _, err := os.Stat(newPath); err == nil {
			return fmt.Errorf("subagent session '%s' already exists", newName)
		}

		return os.Rename(oldPath, newPath)
	}

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

// RemoveSession deletes an entire session directory or a specific subagent file
func RemoveSession(name string) error {
	parts := strings.Split(name, "::")
	if len(parts) == 2 {
		return os.Remove(GetSessionFilePath(name))
	}
	path := GetSessionPath(name)
	return os.RemoveAll(path)
}

// ClearSession deletes the specific session or subagent file.
// Passing only the top-level name "my_session" will only delete "main.jsonl".
func ClearSession(name string) error {
	filePath := GetSessionFilePath(name)
	return os.Remove(filePath)
}

// ReadSessionContent reads the contents of a session or subagent jsonl file
func ReadSessionContent(name string) ([]byte, error) {
	filePath := GetSessionFilePath(name)
	return os.ReadFile(filePath)
}

// WriteSessionContent writes the data into a session or subagent jsonl file
func WriteSessionContent(name string, data []byte) error {
	filePath := GetSessionFilePath(name)

	// Preserve original file mode if it exists
	if fi, err := os.Stat(filePath); err == nil {
		return os.WriteFile(filePath, data, fi.Mode())
	}

	// If not exist, ensure dir exists and write with default perm
	sessionFolder := GetSessionPath(strings.Split(name, "::")[0])
	os.MkdirAll(sessionFolder, 0750)
	return os.WriteFile(filePath, data, 0644)
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
		destPath = name + SessionFileExtension
	} else if info, err := os.Stat(destPath); err == nil && info.IsDir() {
		destPath = filepath.Join(destPath, name+SessionFileExtension)
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
			mainFile := filepath.Join(sessionDir, MainSessionName+SessionFileExtension)

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
		sessions, err := ListSortedSessions(false, true)
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
// detectProvider: whether to detect the provider of the session
// includeSubAgents: whether to include subagent sessions
func ListSortedSessions(detectProvider bool, includeSubAgents bool) ([]SessionMeta, error) {
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

		sessionName := entry.Name() // folder name is the sanitized session title
		sessionPath := filepath.Join(sessionDir, sessionName)

		// Read all .jsonl files in this session directory
		files, err := os.ReadDir(sessionPath)
		if err != nil {
			continue
		}

		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(file.Name(), SessionFileExtension) {
				continue
			}

			filePath := filepath.Join(sessionPath, file.Name())

			info, err := file.Info()
			if err != nil {
				continue
			}

			var provider string
			if detectProvider {
				provider = DetectMessageProvider(filePath)
			}

			// Generate the display name based on whether it's main or subagent
			taskKey := strings.TrimSuffix(file.Name(), SessionFileExtension)
			displayName := sessionName
			if taskKey != MainSessionName {
				if !includeSubAgents {
					continue
				}
				displayName = sessionName + "::" + taskKey
			}

			sessions = append(sessions, SessionMeta{
				Name:     displayName,
				Provider: provider,
				ModTime:  info.ModTime().Unix(),
				Empty:    info.Size() == 0,
			})
		}
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
			// Specific index matched, return just that one
			return []string{sessions[index-1].Name}, nil
		}
	}

	// Now pattern is either a name or a wildcard. We match against all available sessions.
	sessions, err := ListSortedSessions(false, true)
	if err != nil {
		return nil, err
	}

	for _, session := range sessions {
		matched, err := filepath.Match(pattern, session.Name)
		if err != nil {
			// Malformed pattern
			return nil, fmt.Errorf("failed to parse pattern: %w", err)
		}
		if matched {
			matches = append(matches, session.Name)
		}
	}

	return matches, nil
}
