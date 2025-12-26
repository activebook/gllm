package cmd

import (
	"fmt"

	"github.com/activebook/gllm/service"
	"github.com/spf13/cobra"
)

const (
	// Terminal colors
	switchOffColor  = "\033[90m" // Bright Black
	switchOnColor   = "\033[92m" // Bright Green
	switchOnlyColor = "\033[41m" // BG Red
	resetColor      = "\033[0m"

	//cmdOutputColor = "\033[93m" // Light yellow
	//cmdErrorColor  = "\033[95m" // bright magenta

	cmdOutputColor = "\033[38;5;187m" // Light yellow
	cmdErrorColor  = "\033[38;5;175m" // bright magenta
)

func init() {
	rootCmd.AddCommand(markdownCmd)
	markdownCmd.AddCommand(markdownOnCmd)
	markdownCmd.AddCommand(markdownOffCmd)
}

var markdownCmd = &cobra.Command{
	Use:   "markdown",
	Short: "Whether to include Markdown formatting in the output",
	Long: `When Markdown is switched on, the output will include a Markdown-formatted version.
When Markdown is switched off, the output will not include any Markdown formatting.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(cmd.Long)
		fmt.Println()
		fmt.Print("Markdown output is currently switched: ")
		mark := GetAgentBool("markdown")
		if mark {
			fmt.Println(switchOnColor + "on" + resetColor)
		} else {
			fmt.Println(switchOffColor + "off" + resetColor)
		}
	},
}

var markdownOnCmd = &cobra.Command{
	Use:   "on",
	Short: "Switch markdown output on",
	Run: func(cmd *cobra.Command, args []string) {
		if err := SetAgentValue("markdown", true); err != nil {
			service.Errorf("failed to save markdown format output: %w", err)
			return
		}

		fmt.Println("Makedown output switched " + switchOnColor + "on" + resetColor)
	},
}

var markdownOffCmd = &cobra.Command{
	Use:   "off",
	Short: "Switch markdown output off",
	Run: func(cmd *cobra.Command, args []string) {
		if err := SetAgentValue("markdown", false); err != nil {
			service.Errorf("failed to save markdown format output: %w", err)
			return
		}

		fmt.Println("Makedown output switched " + switchOffColor + "off" + resetColor)
	},
}

func SwitchMarkdown(s string) {
	switch s {
	case "on":
		markdownOnCmd.Run(markdownCmd, []string{})
	case "off":
		markdownOffCmd.Run(markdownCmd, []string{})
	default:
		markdownCmd.Run(markdownCmd, []string{})
	}
}

func IncludeMarkdown() bool {
	mark := GetAgentBool("markdown")
	return mark
}
