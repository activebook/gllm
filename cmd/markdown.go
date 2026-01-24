package cmd

import (
	"fmt"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/service"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(markdownCmd)
	markdownCmd.AddCommand(markdownSwitchCmd)
}

var markdownCmd = &cobra.Command{
	Use:     "markdown",
	Aliases: []string{"mk"}, // Optional alias
	Short:   "Manage markdown formatting in output",
	Long: `Manage whether to include Markdown formatting in the output.
Use 'gllm markdown switch' to toggle this feature on or off.`,
	Run: func(cmd *cobra.Command, args []string) {
		store := data.NewConfigStore()
		agent := store.GetActiveAgent()
		if agent == nil {
			fmt.Println("No active agent.")
			return
		}

		fmt.Print("Markdown output is currently: ")
		if service.IsMarkdownEnabled(agent.Capabilities) {
			fmt.Println(switchOnColor + "Enabled" + resetColor)
		} else {
			fmt.Println(switchOffColor + "Disabled" + resetColor)
		}
		fmt.Println("\nUse 'gllm markdown switch' to change.")
	},
}

var markdownSwitchCmd = &cobra.Command{
	Use:     "switch",
	Aliases: []string{"sw", "sel", "select"},
	Short:   "Switch markdown output on/off",
	Long:    "Interactive switch to enable or disable markdown formatting in output.",
	Run: func(cmd *cobra.Command, args []string) {
		store := data.NewConfigStore()
		agent := store.GetActiveAgent()
		if agent == nil {
			fmt.Println("No active agent to configure.")
			return
		}

		current := service.IsMarkdownEnabled(agent.Capabilities)
		var enable bool

		// Helper for options
		onOpt := huh.NewOption("On  - Enable markdown formatting", true).Selected(current)
		offOpt := huh.NewOption("Off - Disable markdown formatting", false).Selected(!current)

		err := huh.NewSelect[bool]().
			Title("Markdown Output").
			Description("Format output using Markdown?").
			Options(onOpt, offOpt).
			Value(&enable).
			Run()

		if err != nil {
			fmt.Println("Operation cancelled.")
			return
		}

		if enable {
			agent.Capabilities = service.EnableMarkdown(agent.Capabilities)
			fmt.Println("Markdown output switched " + switchOnColor + "On" + resetColor)
		} else {
			agent.Capabilities = service.DisableMarkdown(agent.Capabilities)
			fmt.Println("Markdown output switched " + switchOffColor + "Off" + resetColor)
		}

		if err := store.SetAgent(agent.Name, agent); err != nil {
			service.Errorf("failed to save agent config: %v", err)
			return
		}
	},
}
