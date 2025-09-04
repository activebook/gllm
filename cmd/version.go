// File: cmd/version.go
package cmd

import (
	"fmt"
	// No need for "runtime" import here anymore

	"github.com/spf13/cobra"
)

// Hardcode the version string here
const version = "v1.11.7" // <<< Set your desired version

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of gllm",
	Long:  `Prints the current version of gllm.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Just print the hardcoded version from root.go
		// The default version template used by --version is slightly different,
		// so we mimic its core part here for consistency if desired, or just print the version.
		// fmt.Printf("gllm version: %s\n", version)
		// Or exactly match the default cobra --version output format:
		fmt.Printf("%s %s\n", rootCmd.CommandPath(), version)
	},
}
