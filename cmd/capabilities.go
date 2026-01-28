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

const (
	AgentMCPTitle                 = "MCP (Model Context Protocol)"
	AgentSkillsTitle              = "Agent Skills"
	AgentMemoryTitle              = "Agent Memory"
	AgentSubAgentsTitle           = "Sub Agents"
	AgentWebSearchTitle           = "Web Search"
	AgentTokenUsageTitle          = "Token usage"
	AgentMarkdownTitle            = "Markdown output"
	AgentMCPTitleHighlight        = "[MCP (Model Context Protocol)]()"
	AgentSkillsTitleHighlight     = "[Agent Skills]()"
	AgentMemoryTitleHighlight     = "[Agent Memory]()"
	AgentSubAgentsTitleHighlight  = "[Sub Agents]()"
	AgentWebSearchTitleHighlight  = "[Web Search]()"
	AgentTokenUsageTitleHighlight = "[Token usage]()"
	AgentMarkdownTitleHighlight   = "[Markdown output]()"

	AgentMCPBody        = "enables communication with locally running MCP servers that provide additional tools and resources to extend capabilities.\nYou need to set up MCP servers specifically to use this feature."
	AgentSkillsBody     = "are a lightweight, open format for extending AI agent capabilities with specialized knowledge and workflows.\nAfter integrating skills, **agent** will use skills automatically."
	AgentMemoryBody     = "allows agents to remember important facts about you across sessions.\nFacts are used to personalize responses."
	AgentSubAgentsBody  = "allow an agent to respawn other agents to perform tasks or workflows.\nUse when you need to orchestrate multiple agents working in parallel."
	AgentWebSearchBody  = "enables the agent to search the web for real-time information.\nYou must configure a search engine (Google, Bing, Tavily) to use this feature."
	AgentTokenUsageBody = "allows agents to track their token usage.\nThis helps you to control the cost of using the agent."
	AgentMarkdownBody   = "allows agents to generate final response in Markdown format.\nThis helps you to format the response in a more readable way."

	AgentMCPDescription        = AgentMCPTitle + " " + AgentMCPBody
	AgentSkillsDescription     = AgentSkillsTitle + " " + AgentSkillsBody
	AgentMemoryDescription     = AgentMemoryTitle + " " + AgentMemoryBody
	AgentSubAgentsDescription  = AgentSubAgentsTitle + " " + AgentSubAgentsBody
	AgentWebSearchDescription  = AgentWebSearchTitle + " " + AgentWebSearchBody
	AgentTokenUsageDescription = AgentTokenUsageTitle + " " + AgentTokenUsageBody
	AgentMarkdownDescription   = AgentMarkdownTitle + " " + AgentMarkdownBody

	// Agent Features Description Highlight
	AgentMCPDescriptionHighlight        = AgentMCPTitleHighlight + AgentMCPBody
	AgentSkillsDescriptionHighlight     = AgentSkillsTitleHighlight + AgentSkillsBody
	AgentMemoryDescriptionHighlight     = AgentMemoryTitleHighlight + AgentMemoryBody
	AgentSubAgentsDescriptionHighlight  = AgentSubAgentsTitleHighlight + AgentSubAgentsBody
	AgentWebSearchDescriptionHighlight  = AgentWebSearchTitleHighlight + AgentWebSearchBody
	AgentTokenUsageDescriptionHighlight = AgentTokenUsageTitleHighlight + AgentTokenUsageBody
	AgentMarkdownDescriptionHighlight   = AgentMarkdownTitleHighlight + AgentMarkdownBody
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
			options = append(options, huh.NewOption("Subagents Workflow", service.CapabilitySubAgents).Selected(true))
			selected = append(selected, service.CapabilitySubAgents)
		} else {
			options = append(options, huh.NewOption("Subagents Workflow", service.CapabilitySubAgents))
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

		// Sort with selected at top
		ui.SortMultiOptions(options, selected)

		// Create multi select
		msfeatures := huh.NewMultiSelect[string]().
			Title("Agent Capabilities").
			Description("Use space to toggle, enter to confirm.").
			Options(options...).
			Value(&selected)
		featureNote := ui.GetDynamicHuhNote("Feature Details", msfeatures, getFeatureDescription)
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
			service.CapabilityAgentMemory,
			service.CapabilityWebSearch,
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

func getFeatureDescription(cap string) string {
	switch cap {
	case service.CapabilityMCPServers:
		return AgentMCPDescriptionHighlight
	case service.CapabilityAgentSkills:
		return AgentSkillsDescriptionHighlight
	case service.CapabilityTokenUsage:
		return AgentTokenUsageDescriptionHighlight
	case service.CapabilityMarkdown:
		return AgentMarkdownDescriptionHighlight
	case service.CapabilitySubAgents:
		return AgentSubAgentsDescriptionHighlight
	case service.CapabilityAgentMemory:
		return AgentMemoryDescriptionHighlight
	case service.CapabilityWebSearch:
		return AgentWebSearchDescriptionHighlight
	default:
		return ""
	}
}

func printCapSummary(caps []string) {
	fmt.Println("Current Agent Features and Capabilities:")
	fmt.Println()

	printCapStatus("Token Usage", service.IsTokenUsageEnabled(caps))
	printCapStatus("Markdown Output", service.IsMarkdownEnabled(caps))
	printCapStatus("Web Search", service.IsWebSearchEnabled(caps))
	printCapStatus("MCP Servers", service.IsMCPServersEnabled(caps))
	printCapStatus("Agent Skills", service.IsAgentSkillsEnabled(caps))
	printCapStatus("Agent Memory", service.IsAgentMemoryEnabled(caps))
	printCapStatus("Sub Agents", service.IsSubAgentsEnabled(caps))

	fmt.Printf("%s = Enabled capability\n", ui.FormatEnabledIndicator(true))
}

func printCapStatus(name string, enabled bool) {
	indicator := ui.FormatEnabledIndicator(enabled)
	fmt.Printf("  %s %s\n", indicator, name)

	var desc string
	switch name {
	case "MCP Servers":
		desc = AgentMCPDescription
	case "Agent Skills":
		desc = AgentSkillsDescription
	case "Sub Agents":
		desc = AgentSubAgentsDescription
	case "Agent Memory":
		desc = AgentMemoryDescription
	case "Web Search":
		desc = AgentWebSearchDescription
	case "Token Usage":
		desc = AgentTokenUsageDescription
	case "Markdown Output":
		desc = AgentMarkdownDescription
	}

	if desc != "" {
		lines := strings.Split(desc, "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				fmt.Printf("  %s%s%s\n", data.DetailColor, line, data.ResetSeq)
			}
		}
	}
	fmt.Println()
}
