package service

import (
	"os"
)

// ExecutorPath is the path to the executable to run for filtering.
// Defaults to os.Executable(). Can be overridden for testing.
var ExecutorPath string

/*
 * Execution: When main imports service, Go will find all
 * init() functions effectively in every file of the service package and run all of them automatically.
 * Order: They run before main() starts.
 * (The order between files is generally by filename, but you shouldn't rely on that specific order).
 */
func init() {
	exe, err := os.Executable()
	if err == nil {
		ExecutorPath = exe
	} else {
		ExecutorPath = "gllm" // Fallback
	}
}
