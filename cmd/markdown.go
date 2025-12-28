package cmd

import (
	"fmt"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/service"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(markdownCmd)
	markdownCmd.AddCommand(markdownOnCmd)
	markdownCmd.AddCommand(markdownOffCmd)
}

var markdownCmd = &cobra.Command{
	Use:     "markdown",
	Aliases: []string{"md"}, // Optional alias
	Short:   "Whether to include Markdown formatting in the output",
	Long: `When Markdown is switched on, the output will include a Markdown-formatted version.
When Markdown is switched off, the output will not include any Markdown formatting.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(cmd.Long)
		fmt.Println()
		fmt.Print("Markdown output is currently switched: ")
		store := data.NewConfigStore()
		agent := store.GetActiveAgent()
		if agent == nil {
			fmt.Println(switchOffColor + "off" + resetColor)
			return
		}
		mark := agent.Markdown
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
		store := data.NewConfigStore()
		agent := store.GetActiveAgent()
		if agent == nil {
			fmt.Println(switchOffColor + "off" + resetColor)
			return
		}
		agent.Markdown = true
		if err := store.SetAgent(agent.Name, agent); err != nil {
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
		store := data.NewConfigStore()
		agent := store.GetActiveAgent()
		if agent == nil {
			fmt.Println(switchOffColor + "off" + resetColor)
			return
		}
		agent.Markdown = false
		if err := store.SetAgent(agent.Name, agent); err != nil {
			service.Errorf("failed to save markdown format output: %w", err)
			return
		}

		fmt.Println("Makedown output switched " + switchOffColor + "off" + resetColor)
	},
}
