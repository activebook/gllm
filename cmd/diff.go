package cmd

import (
	"fmt"
	"os"

	"github.com/activebook/gllm/internal/ui"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(diffCmd)
	diffCmd.Flags().IntP("context", "c", 3, "Number of context lines to show")
}

var diffCmd = &cobra.Command{
	Use:   "diff [file1] [file2]",
	Short: "Show a pretty diff between two files",
	Long: `Show a pretty diff between two text files with colored output.

The command will display lines that are different between the two files:
  - Red lines with '-' prefix indicate lines removed from the first file
  - Green lines with '+' prefix indicate lines added in the second file
  - White lines are unchanged lines
  - Line numbers are shown for context lines`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		file1 := args[0]
		file2 := args[1]

		contextLines, _ := cmd.Flags().GetInt("context")

		content1, err := os.ReadFile(file1)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", file1, err)
			os.Exit(1)
		}

		content2, err := os.ReadFile(file2)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", file2, err)
			os.Exit(1)
		}

		diff := ui.Diff(string(content1), string(content2), file1, file2, contextLines)
		fmt.Println(diff)
	},
}
