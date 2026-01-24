// File: cmd/subagents.go
package cmd

import (
	"fmt"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/service"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

// subagentsCmd represents the subagents command
var subagentsCmd = &cobra.Command{
	Use:   "subagents",
	Short: "Manage subagents capability",
	Long: `Enable or disable subagents workflow for the current agent.
Use 'gllm subagents switch' to toggle this feature on or off.`,
	Run: func(cmd *cobra.Command, args []string) {
		// If no args, show status
		showSubagentsStatus()
	},
}

var subagentsSwitchCmd = &cobra.Command{
	Use:     "switch",
	Aliases: []string{"sw", "sel", "select"},
	Short:   "Switch subagents workflow on/off",
	Long:    "Interactive switch to enable or disable subagents workflow.",
	Run: func(cmd *cobra.Command, args []string) {
		store := data.NewConfigStore()
		agent := store.GetActiveAgent()
		if agent == nil {
			fmt.Println("No active agent to configure.")
			return
		}

		current := service.IsSubAgentsEnabled(agent.Capabilities)
		var enable bool

		// Helper for options
		onOpt := huh.NewOption("On  - Enable subagents workflow", true).Selected(current)
		offOpt := huh.NewOption("Off - Disable subagents workflow", false).Selected(!current)

		err := huh.NewSelect[bool]().
			Title("Subagents Workflow").
			Description("Allow agent to manage and call sub-agents?").
			Options(onOpt, offOpt).
			Value(&enable).
			Run()

		if err != nil {
			fmt.Println("Operation cancelled.")
			return
		}

		if enable {
			agent.Capabilities = service.EnableSubAgents(agent.Capabilities)
			fmt.Println("Subagents workflow " + switchOnColor + "Enabled" + resetColor)
		} else {
			agent.Capabilities = service.DisableSubAgents(agent.Capabilities)
			fmt.Println("Subagents workflow " + switchOffColor + "Disabled" + resetColor)
		}

		if err := store.SetAgent(agent.Name, agent); err != nil {
			fmt.Printf("Error saving agent configuration: %v\n", err)
			return
		}
	},
}

func init() {
	rootCmd.AddCommand(subagentsCmd)
	subagentsCmd.AddCommand(subagentsSwitchCmd)
}

func showSubagentsStatus() {
	store := data.NewConfigStore()
	agent := store.GetActiveAgent()
	if agent == nil {
		fmt.Println("No active agent.")
		return
	}

	enabled := service.IsSubAgentsEnabled(agent.Capabilities)
	status := switchOffColor + "Disabled" + resetColor
	if enabled {
		status = switchOnColor + "Enabled" + resetColor
	}
	fmt.Printf("Subagents Workflow: %s\n", status)
	fmt.Println("\nUse 'gllm subagents switch' to change.")
}
