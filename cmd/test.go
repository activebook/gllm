package cmd

import (
	"fmt"

	"github.com/activebook/gllm/test"
	"github.com/spf13/cobra"
)

// testCmd represents the test command
var testCmd = &cobra.Command{
	Use:   "test [test_name]",
	Short: "Run various test functions",
	Long: `Run different test functions for gllm functionality.
Available tests: mcp, channels, atref, all`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		testName := "all"
		if len(args) > 0 {
			testName = args[0]
		}

		runTests(testName)
	},
}

func runTests(testName string) {
	fmt.Printf("Running test: %s\n", testName)

	switch testName {
	case "mcp":
		test.TestMCP()
	case "channels":
		test.TestChannelsD()
	case "atref":
		test.TestAtRefProcessor()
	case "all":
		fmt.Println("Running all tests...")
	default:
		fmt.Printf("Unknown test: %s\n", testName)
		fmt.Println("Available tests: mcp, channels, atref, all")
	}
}

func init() {
	//rootCmd.AddCommand(testCmd)
}
