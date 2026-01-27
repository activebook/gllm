package data

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// get current task dir, it should append a date under tasks
func GetCurrentTaskDirPath() string {
	return filepath.Join(GetTasksDirPath(), time.Now().Format("20060102"))
}

// EnsureTasksDir creates the tasks directory if it doesn't exist.
func EnsureTasksDir() error {
	tasksDir := GetCurrentTaskDirPath()
	return os.MkdirAll(tasksDir, 0755)
}

// GenerateTaskFilePath generates a persistent file path for sub-agent tasks.
func GenerateTaskFilePath(prefix string, ext string) string {
	if err := EnsureTasksDir(); err != nil {
		return generateTempTasksPath(prefix, ext)
	}

	// Create a unique filename using timestamp and random suffix
	timestamp := time.Now().Format("20060102150405.000")
	filename := fmt.Sprintf("%s_%s%s", prefix, timestamp, ext)

	tasksDir := GetCurrentTaskDirPath()
	return filepath.Join(tasksDir, filename)
}

// generateTempTasksPath generates a temporary file path with the given prefix and extension.
// The file is stored in the system's temp directory.
func generateTempTasksPath(prefix string, ext string) string {
	currentTime := time.Now()
	filename := fmt.Sprintf("%s_%s%s", prefix, currentTime.Format("20060102150405.000"), ext)
	return filepath.Join(os.TempDir(), filename)
}
