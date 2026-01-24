// File: cmd/capabilities.go
package cmd

import (
	"fmt"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/service"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(capsCmd)
	capsCmd.AddCommand(capsSwitchCmd)
}

var capsCmd = &cobra.Command{
	Use:     "features",
	Aliases: []string{"caps", "capabilities", "feats"},
	Short:   "Manage agent features and capabilities",
	Long: `View and manage the capabilities enabled for the current agent.
Use 'gllm features switch' to toggle capabilities on or off.`,
	Run: func(cmd *cobra.Command, args []string) {
		store := data.NewConfigStore()
		agent := store.GetActiveAgent()
		if agent == nil {
			fmt.Println("No active agent.")
			return
		}

		fmt.Println("Current Agent Features and Capabilities:")
		fmt.Println()

		printCapStatus("Token Usage", service.IsTokenUsageEnabled(agent.Capabilities))
		printCapStatus("Markdown Output", service.IsMarkdownEnabled(agent.Capabilities))
		printCapStatus("MCP Servers", service.IsMCPServersEnabled(agent.Capabilities))
		printCapStatus("Agent Skills", service.IsAgentSkillsEnabled(agent.Capabilities))
		printCapStatus("Sub Agents", service.IsSubAgentsEnabled(agent.Capabilities))

		fmt.Println()
		fmt.Println("Use 'gllm features switch' to change.")
	},
}

var capsSwitchCmd = &cobra.Command{
	Use:     "switch",
	Aliases: []string{"sw", "sel", "select"},
	Short:   "Toggle agent capabilities on/off",
	Long:    "Interactive switch to enable or disable agent capabilities.",
	Run: func(cmd *cobra.Command, args []string) {
		store := data.NewConfigStore()
		agent := store.GetActiveAgent()
		if agent == nil {
			fmt.Println("No active agent to configure.")
			return
		}

		// Build options with current state
		var options []huh.Option[string]
		var selected []string

		// MCP Servers
		if service.IsMCPServersEnabled(agent.Capabilities) {
			options = append(options, huh.NewOption("MCP Servers", service.CapabilityMCPServers).Selected(true))
			selected = append(selected, service.CapabilityMCPServers)
		} else {
			options = append(options, huh.NewOption("MCP Servers", service.CapabilityMCPServers))
		}

		// Agent Skills
		if service.IsAgentSkillsEnabled(agent.Capabilities) {
			options = append(options, huh.NewOption("Agent Skills", service.CapabilityAgentSkills).Selected(true))
			selected = append(selected, service.CapabilityAgentSkills)
		} else {
			options = append(options, huh.NewOption("Agent Skills", service.CapabilityAgentSkills))
		}

		// Token Usage
		if service.IsTokenUsageEnabled(agent.Capabilities) {
			options = append(options, huh.NewOption("Token Usage Stats", service.CapabilityTokenUsage).Selected(true))
			selected = append(selected, service.CapabilityTokenUsage)
		} else {
			options = append(options, huh.NewOption("Token Usage Stats", service.CapabilityTokenUsage))
		}

		// Markdown Output
		if service.IsMarkdownEnabled(agent.Capabilities) {
			options = append(options, huh.NewOption("Markdown Output", service.CapabilityMarkdown).Selected(true))
			selected = append(selected, service.CapabilityMarkdown)
		} else {
			options = append(options, huh.NewOption("Markdown Output", service.CapabilityMarkdown))
		}

		// Subagents Workflow
		if service.IsSubAgentsEnabled(agent.Capabilities) {
			options = append(options, huh.NewOption("Subagents Workflow", service.CapabilitySubAgents).Selected(true))
			selected = append(selected, service.CapabilitySubAgents)
		} else {
			options = append(options, huh.NewOption("Subagents Workflow", service.CapabilitySubAgents))
		}

		// Sort with selected at top
		SortMultiOptions(options, selected)

		err := huh.NewForm(
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title("Agent Capabilities").
					Description("Use space to toggle, enter to confirm.").
					Options(options...).
					Value(&selected),
				huh.NewNote().
					Title("---").
					Description(AgentMCPDescription+"\n\n"+AgentSkillsDescription+"\n\n"+AgentSubAgentsDescription),
			),
		).Run()

		if err != nil {
			fmt.Println("Operation cancelled.")
			return
		}

		// Create a set for fast lookup
		selectedSet := make(map[string]bool)
		for _, cap := range selected {
			selectedSet[cap] = true
		}

		// Build new capabilities slice
		var newCaps []string
		allCaps := []string{
			service.CapabilityMCPServers,
			service.CapabilityAgentSkills,
			service.CapabilityTokenUsage,
			service.CapabilityMarkdown,
			service.CapabilitySubAgents,
		}
		for _, cap := range allCaps {
			if selectedSet[cap] {
				newCaps = append(newCaps, cap)
			}
		}

		agent.Capabilities = newCaps

		if err := store.SetAgent(agent.Name, agent); err != nil {
			fmt.Printf("Error saving agent config: %v\n", err)
			return
		}

		fmt.Println()
		fmt.Printf("Capabilities updated. %d enabled.\n", len(newCaps))
	},
}

func printCapStatus(name string, enabled bool) {
	status := switchOffColor + "Disabled" + resetColor
	if enabled {
		status = switchOnColor + "Enabled" + resetColor
	}
	fmt.Printf("  %-20s %s\n", name+":", status)
}
