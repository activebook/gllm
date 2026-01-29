// File: cmd/version.go
package cmd

import (
	"fmt"
	// No need for "runtime" import here anymore

	"github.com/spf13/cobra"
)

// These variables are populated via -ldflags at build time
var (
	version = "dev" // Default to 'dev' if not set
)

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of gllm",
	Long:  `Print the version number of gllm.`,
	// Add completion support
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version)
	},
}
