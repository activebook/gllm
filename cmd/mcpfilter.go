package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var mcpFilterCmd = &cobra.Command{
	Use:    "_mcp-filter [command] [args...]",
	Short:  "Internal command to filter MCP server output",
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {
		// "--" is a common convention in Unix-like systems to prevent arguments starting with - from being misinterpreted as flags or options.
		// Cobra handles "--" internally by stopping flag parsing when encountered and excluding it from the arguments passed to the command handler, ensuring that subsequent arguments are treated as positional regardless of leading - characters.
		// Debugging: Print received args
		// eg. gllm _mcp-filter -- uname -a
		// Received args: [uname -a]
		// fmt.Fprintf(os.Stderr, "[MCP-FILTER] Received args: %v\n", args)

		if len(args) == 0 {
			return
		}

		command := args[0]
		commandArgs := args[1:]

		subCmd := exec.Command(command, commandArgs...)
		subCmd.Stdin = os.Stdin
		subCmd.Stderr = os.Stderr

		// Pipe stdout to filter
		stdout, err := subCmd.StdoutPipe()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating stdout pipe: %v\n", err)
			os.Exit(1)
		}

		if err := subCmd.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Error starting command: %v\n", err)
			os.Exit(1)
		}

		// Filtering logic
		scanner := bufio.NewReader(stdout)
		foundJSON := false

		for {
			if foundJSON {
				// Efficient copy once header is found
				_, err := io.Copy(os.Stdout, scanner)
				if err != nil && err != io.EOF {
					// ignore generic errors during copy?
				}
				break
			}

			// Read line by line until we find '{'
			// We use ReadString to preserve delimiters
			line, err := scanner.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					// If we reached EOF without finding JSON, maybe output what we have?
					// Or just exit.
					// If line is not empty, it's the last line.
					if strings.TrimSpace(line) != "" {
						if strings.HasPrefix(strings.TrimSpace(line), "{") {
							fmt.Print(line)
						}
					}
					break
				}
				break
			}

			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "{") {
				foundJSON = true
				fmt.Print(line)
			} else {
				// Log filtered lines to stderr for debugging?
				// fmt.Fprintf(os.Stderr, "[MCP-FILTER-DROPPED]: %s", line)
			}
		}

		subCmd.Wait()
	},
}

func init() {
	rootCmd.AddCommand(mcpFilterCmd)
}
