// File: cmd/version.go
package cmd

import (
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(testCmd)
}

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Test some inner mechanism of gllm",
	Long:  `Just for testing purposes.`,
	Run: func(cmd *cobra.Command, args []string) {
		// This is where you would implement the logic for your test command
		// For now, we'll just print a message
		cmd.Println("Test command executed. For future interactive REPL mode, use 'gllm chat'")
	},
}
