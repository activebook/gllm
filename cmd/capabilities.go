// File: cmd/capabilities.go
package cmd

import (
	"fmt"
	"strings"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/internal/ui"
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

		printCapSummary(agent.Capabilities)

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
			options = append(options, huh.NewOption("Sub Agents", service.CapabilitySubAgents).Selected(true))
			selected = append(selected, service.CapabilitySubAgents)
		} else {
			options = append(options, huh.NewOption("Sub Agents", service.CapabilitySubAgents))
		}

		// Agent Delegation
		if service.IsAgentDelegationEnabled(agent.Capabilities) {
			options = append(options, huh.NewOption("Agent Delegation", service.CapabilityAgentDelegation).Selected(true))
			selected = append(selected, service.CapabilityAgentDelegation)
		} else {
			options = append(options, huh.NewOption("Agent Delegation", service.CapabilityAgentDelegation))
		}

		// Agent Memory
		if service.IsAgentMemoryEnabled(agent.Capabilities) {
			options = append(options, huh.NewOption("Agent Memory", service.CapabilityAgentMemory).Selected(true))
			selected = append(selected, service.CapabilityAgentMemory)
		} else {
			options = append(options, huh.NewOption("Agent Memory", service.CapabilityAgentMemory))
		}

		// Web Search
		if service.IsWebSearchEnabled(agent.Capabilities) {
			options = append(options, huh.NewOption("Web Search", service.CapabilityWebSearch).Selected(true))
			selected = append(selected, service.CapabilityWebSearch)
		} else {
			options = append(options, huh.NewOption("Web Search", service.CapabilityWebSearch))
		}

		// Auto Compression
		if service.IsAutoCompressionEnabled(agent.Capabilities) {
			options = append(options, huh.NewOption("Auto Compression", service.CapabilityAutoCompression).Selected(true))
			selected = append(selected, service.CapabilityAutoCompression)
		} else {
			options = append(options, huh.NewOption("Auto Compression", service.CapabilityAutoCompression))
		}

		// Plan Mode
		if service.IsPlanModeEnabled(agent.Capabilities) {
			options = append(options, huh.NewOption("Plan Mode", service.CapabilityPlanMode).Selected(true))
			selected = append(selected, service.CapabilityPlanMode)
		} else {
			options = append(options, huh.NewOption("Plan Mode", service.CapabilityPlanMode))
		}

		// Sort with selected at top
		ui.SortMultiOptions(options, selected)

		// Create multi select
		msfeatures := huh.NewMultiSelect[string]().
			Title("Agent Capabilities").
			Description("Use space to toggle, enter to confirm.").
			Options(options...).
			Value(&selected)
		featureNote := ui.GetDynamicHuhNote("Feature Details", msfeatures, service.GetCapabilityDescHighlight)
		err := huh.NewForm(
			huh.NewGroup(msfeatures, featureNote),
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
			service.CapabilityAgentDelegation,
			service.CapabilityAgentMemory,
			service.CapabilityWebSearch,
			service.CapabilityAutoCompression,
			service.CapabilityPlanMode,
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

		fmt.Printf("Capabilities updated. %d enabled.\n", len(newCaps))
		fmt.Println()
		printCapSummary(agent.Capabilities)
	},
}

func printCapSummary(caps []string) {
	fmt.Println("Current Agent Features and Capabilities:")
	fmt.Println()

	printCapStatus(service.CapabilityTokenUsageTitle, service.IsTokenUsageEnabled(caps))
	printCapStatus(service.CapabilityMarkdownTitle, service.IsMarkdownEnabled(caps))
	printCapStatus(service.CapabilityWebSearchTitle, service.IsWebSearchEnabled(caps))
	printCapStatus(service.CapabilityMCPTitle, service.IsMCPServersEnabled(caps))
	printCapStatus(service.CapabilitySkillsTitle, service.IsAgentSkillsEnabled(caps))
	printCapStatus(service.CapabilityMemoryTitle, service.IsAgentMemoryEnabled(caps))
	printCapStatus(service.CapabilitySubAgentsTitle, service.IsSubAgentsEnabled(caps))
	printCapStatus(service.CapabilityDelegationTitle, service.IsAgentDelegationEnabled(caps))
	printCapStatus(service.CapabilityAutoCompressTitle, service.IsAutoCompressionEnabled(caps))
	printCapStatus(service.CapabilityPlanModeTitle, service.IsPlanModeEnabled(caps))

	fmt.Printf("%s = Enabled capability\n", ui.FormatEnabledIndicator(true))
}

func printCapStatus(name string, enabled bool) {
	indicator := ui.FormatEnabledIndicator(enabled)
	fmt.Printf("%s %s\n", indicator, name)

	desc := service.GetCapabilityDescription(name)
	if desc != "" {
		lines := strings.Split(desc, "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				fmt.Printf("%s%s%s\n", data.DetailColor, line, data.ResetSeq)
			}
		}
	}
	fmt.Println()
}
