package cmd

import (
	"fmt"
	"sort"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/internal/ui"
	"github.com/activebook/gllm/service"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

const (
	EmbeddingToolsDescription = `[Tools]() enable file system operations, command execution, and agent switching.

Run shell command or script ( [shell]()):
   - _Use when need to run a local command such as python, node, bash, etc._
   - _Or run any other command-line tool or script_
   - _Best for: "Run this python script and give me result"_

Automatic agent switch ( [switch\\_agent]()):
   - _Use when you want to delegate control completely to another agent_
   - _Best for: "Already done the planning, switch to code mode"_`
)

func init() {
	toolsCmd.AddCommand(toolsSwCmd)
	rootCmd.AddCommand(toolsCmd)
}

var toolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "Configure embedding tools for current agent",
	Long: `Tools give gllm the ability to interact with the file system, execute commands, and perform other operations.
Use 'gllm tools sw' to select which tools to enable for the current agent.`,
	// Add completion support
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return []string{"list", "switch", "enable", "disable", "config", "--help"}, cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(cmd.Long)
		fmt.Println()
		ListAllTools()
	},
}

var toolsSwCmd = &cobra.Command{
	Use:     "switch",
	Aliases: []string{"sw", "select", "sel"},
	Short:   "Switch tools on/off",
	Long:    "Choose which embedding tools to enable for the current agent.",
	Run: func(cmd *cobra.Command, args []string) {
		store := data.NewConfigStore()
		agent := store.GetActiveAgent()
		if agent == nil {
			fmt.Println("No active agent found")
			return
		}

		// Get all available tools
		allTools := service.GetAllEmbeddingTools()

		// Get currently enabled tools
		enabledTools := agent.Tools

		// Create a set for quick lookup
		enabledSet := make(map[string]bool)
		for _, t := range enabledTools {
			enabledSet[t] = true
		}

		// Build options with current state
		var options []huh.Option[string]
		for _, tool := range allTools {
			opt := huh.NewOption(tool, tool)
			if enabledSet[tool] {
				opt = opt.Selected(true)
			}
			options = append(options, opt)
		}
		// Sort: selected items first, then alphabetically within each group
		ui.SortMultiOptions(options, enabledTools)

		var selectedTools []string
		err := huh.NewForm(
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title("Select Embedding Tools").
					Description("Choose which tools to enable for this agent. Press space to toggle, enter to confirm.").
					Options(options...).
					Value(&selectedTools),
				ui.GetStaticHuhNote("Tools Details", EmbeddingToolsDescription),
			),
		).Run()

		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		// Save selected tools
		agent.Tools = selectedTools
		err = store.SetAgent(agent.Name, agent)
		if err != nil {
			fmt.Printf("Error saving tools config: %v\n", err)
			return
		}

		ListAllTools()
	},
}

func ListAllTools() {
	store := data.NewConfigStore()
	agent := store.GetActiveAgent()
	if agent == nil {
		fmt.Println("No active agent found")
		return
	}

	allTools := service.GetAllOpenTools()

	// Sort for consistent display
	sortedTools := make([]string, len(allTools))
	copy(sortedTools, allTools)
	sort.Strings(sortedTools)

	// Create a set of enabled tools for lookup
	enabledSet := make(map[string]bool)
	if agent.Tools == nil {
		for _, t := range allTools {
			enabledSet[t] = false
		}
	} else {
		for _, t := range agent.Tools {
			enabledSet[t] = true
		}
	}

	// Add skill tools if skills are enabled
	if service.IsAgentSkillsEnabled(agent.Capabilities) {
		skillTools := service.GetAllSkillTools()
		for _, t := range skillTools {
			enabledSet[t] = true
		}
	}

	// Add web search tools if web search is enabled
	if service.IsWebSearchEnabled(agent.Capabilities) {
		webSearchTools := service.GetAllSearchTools()
		for _, t := range webSearchTools {
			enabledSet[t] = true
		}
	}

	// Add sub agents tools if sub agents are enabled
	if service.IsSubAgentsEnabled(agent.Capabilities) {
		subAgentsTools := service.GetAllSubagentTools()
		for _, t := range subAgentsTools {
			enabledSet[t] = true
		}
	}

	// Add agent memory tools if agent memory is enabled
	if service.IsAgentMemoryEnabled(agent.Capabilities) {
		agentMemoryTools := service.GetAllMemoryTools()
		for _, t := range agentMemoryTools {
			enabledSet[t] = true
		}
	}

	enabledCount := 0
	for range enabledSet {
		enabledCount++
	}

	// Append char ' behind the tool name of those non-embedding tools
	// to tell user those tools are not switchable
	// they are built-in capabilities tools
	for _, tool := range sortedTools {
		displayName := tool
		if !service.AvailableEmbeddingTool(tool) {
			displayName += "'"
		}
		indicator := ui.FormatEnabledIndicator(enabledSet[tool])
		fmt.Printf("%s %s\n", indicator, displayName)
	}
	fmt.Printf("\n%d tool(s) enabled for current agent.\n", enabledCount)
	fmt.Println("' behind the tool name means it is a built-in capabilities tool which can be switched on/off in agent capabilities settings.")
}
