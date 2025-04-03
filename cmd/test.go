// File: cmd/version.go
package cmd

import (
	"fmt"

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

		colors := map[string]string{
			"Black":   "\033[30m",
			"Red":     "\033[31m",
			"Green":   "\033[32m",
			"Yellow":  "\033[33m",
			"Blue":    "\033[34m",
			"Magenta": "\033[35m",
			"Cyan":    "\033[36m",
			"White":   "\033[37m",

			"Orange (256)":      "\033[38;5;214m",
			"Dark Orange (256)": "\033[38;5;208m",
			"True Orange (RGB)": "\033[38;2;255;165;0m",

			"Bright Black":   "\033[90m",
			"Bright Red":     "\033[91m",
			"Bright Green":   "\033[92m",
			"Bright Yellow":  "\033[93m",
			"Bright Blue":    "\033[94m",
			"Bright Magenta": "\033[95m",
			"Bright Cyan":    "\033[96m",
			"Bright White":   "\033[97m",

			"BG Black":   "\033[40m",
			"BG Red":     "\033[41m",
			"BG Green":   "\033[42m",
			"BG Yellow":  "\033[43m",
			"BG Blue":    "\033[44m",
			"BG Magenta": "\033[45m",
			"BG Cyan":    "\033[46m",
			"BG White":   "\033[47m",

			"Bright BG Black":   "\033[100m",
			"Bright BG Red":     "\033[101m",
			"Bright BG Green":   "\033[102m",
			"Bright BG Yellow":  "\033[103m",
			"Bright BG Blue":    "\033[104m",
			"Bright BG Magenta": "\033[105m",
			"Bright BG Cyan":    "\033[106m",
			"Bright BG White":   "\033[107m",

			"Reset": "\033[0m",
		}

		fmt.Println("Printing all colors:")
		for name, code := range colors {
			fmt.Println(code, name, colors["Reset"])
		}

		//test.TestSearch2()
		//test.TestQwQ()
		//test.TestVV()
		return
	},
}
