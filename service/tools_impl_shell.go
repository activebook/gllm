package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/activebook/gllm/data"
)

const (
	DefaultShellTimeout = 30 * time.Second

	// ToolRespShellOutput is the template for the response to the user after executing a command.
	ToolRespShellOutput = `shell executed: %s
Status:
%s
%s`
)

func shellToolCallImpl(argsMap *map[string]interface{}, op *OpenProcessor) (string, error) {
	if err := CheckToolPermission(ToolShell, argsMap); err != nil {
		return "", err
	}

	cmdStr, ok := (*argsMap)["command"].(string)
	if !ok {
		return "", fmt.Errorf("command not found in arguments")
	}

	// Get timeout from arguments, default to DefaultShellTimeout
	timeout := DefaultShellTimeout
	if timeoutValue, exists := (*argsMap)["timeout"]; exists {
		switch v := timeoutValue.(type) {
		case float64:
			if v > 0 {
				timeout = time.Duration(v) * time.Second
			}
		case int:
			if v > 0 {
				timeout = time.Duration(v) * time.Second
			}
		}
	}

	if !op.toolsUse.AutoApprove {
		// Directly prompt user for confirmation
		descStr, ok := (*argsMap)["purpose"].(string)
		if !ok {
			//return "", fmt.Errorf("purpose not found in arguments")
			descStr = ""
		}
		// Use the command string as the info for confirmation
		if op.interaction != nil {
			op.interaction.RequestConfirm(descStr, op.toolsUse)
		}
		if op.toolsUse.Confirm == data.ToolConfirmCancel {
			return fmt.Sprintf("Operation cancelled by user: shell command '%s'", cmdStr), UserCancelError{Reason: UserCancelReasonDeny}
		}
	}

	var errStr string

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Do the real command with timeout
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", cmdStr)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", cmdStr)
	}

	out, err := cmd.CombinedOutput()

	// Handle command exec failed
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			errStr = fmt.Sprintf("Command timed out after %v", timeout)
		} else {
			var exitCode int
			if exitError, ok := err.(*exec.ExitError); ok {
				exitCode = exitError.ExitCode()
			}
			errStr = fmt.Sprintf("Command failed with exit code %d: %v", exitCode, err)
		}
	}

	// Output the result
	outStr := string(out)
	if outStr != "" {
		outStr = outStr + "\n"
	}

	// Format error info if present
	errorInfo := ""
	if errStr != "" {
		errorInfo = fmt.Sprintf("Error: %s", errStr)
	}
	// Format output info
	outputInfo := ""
	if outStr != "" {
		outputInfo = fmt.Sprintf("Output:\n%s", outStr)
	} else {
		outputInfo = "Output: <no output>"
	}
	// Create a response that prompts the LLM to provide insightful analysis of the command output
	finalResponse := fmt.Sprintf(ToolRespShellOutput, cmdStr, errorInfo, outputInfo)

	// Respect QuietMode – only output to Console if NOT in quiet mode and Verbose is enabled
	if !op.quiet && data.GetSettingsStore().GetVerboseEnabled() {
		fmt.Fprintf(os.Stderr, "%s$ %s%s\n", data.ToolCallColor, cmdStr, data.ResetSeq)
		if outStr != "" {
			fmt.Fprintf(os.Stderr, "%s%s%s", data.ShellOutputColor, outStr, data.ResetSeq)
		}
		if errStr != "" {
			fmt.Fprintf(os.Stderr, "%s%s%s\n", data.StatusErrorColor, errStr, data.ResetSeq)
		}
	}

	return finalResponse, nil
}
