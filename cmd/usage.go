package cmd

import (
	"fmt"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/service"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(usageCmd)
	usageCmd.AddCommand(usageSwitchCmd)
}

var usageCmd = &cobra.Command{
	Use:     "usage",
	Aliases: []string{"ua", "usage"}, // Optional alias
	Short:   "Manage token usage statistics output",
	Long: `Manage whether to include token usage metainfo (prompt tokens, completion tokens, duration) in the output.
Use 'gllm usage switch' to toggle this feature on or off.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Show status if no subcommand
		store := data.NewConfigStore()
		agent := store.GetActiveAgent()
		if agent == nil {
			fmt.Println("No active agent.")
			return
		}

		fmt.Print("Usage output is currently: ")
		if service.IsTokenUsageEnabled(agent.Capabilities) {
			fmt.Println(switchOnColor + "Enabled" + resetColor)
		} else {
			fmt.Println(switchOffColor + "Disabled" + resetColor)
		}
		fmt.Println("\nUse 'gllm usage switch' to change.")
	},
}

var usageSwitchCmd = &cobra.Command{
	Use:     "switch",
	Aliases: []string{"sw", "sel", "select"},
	Short:   "Switch usage output on/off",
	Long:    "Interactive switch to enable or disable token usage statistics output.",
	Run: func(cmd *cobra.Command, args []string) {
		store := data.NewConfigStore()
		agent := store.GetActiveAgent()
		if agent == nil {
			fmt.Println("No active agent to configure.")
			return
		}

		current := service.IsTokenUsageEnabled(agent.Capabilities)
		var enable bool

		// Helper for options
		onOpt := huh.NewOption("On  - Enable usage stats", true).Selected(current)
		offOpt := huh.NewOption("Off - Disable usage stats", false).Selected(!current)

		err := huh.NewSelect[bool]().
			Title("Token Usage Statistics").
			Description("Include token usage metainfo in the output?").
			Options(onOpt, offOpt).
			Value(&enable).
			Run()

		if err != nil {
			fmt.Println("Operation cancelled.")
			return
		}

		if enable {
			agent.Capabilities = service.EnableTokenUsage(agent.Capabilities)
			fmt.Println("Usage output switched " + switchOnColor + "On" + resetColor)
		} else {
			agent.Capabilities = service.DisableTokenUsage(agent.Capabilities)
			fmt.Println("Usage output switched " + switchOffColor + "Off" + resetColor)
		}

		if err := store.SetAgent(agent.Name, agent); err != nil {
			service.Errorf("failed to save agent config: %v", err)
			return
		}
	},
}
