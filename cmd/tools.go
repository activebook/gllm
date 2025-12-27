package cmd

import (
	"fmt"
	"sort"

	"github.com/activebook/gllm/service"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var toolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "Configure embedding tools for current agent",
	Long: `Tools give gllm the ability to interact with the file system, execute commands, and perform other operations.
Use 'gllm tools sw' to select which tools to enable for the current agent.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(cmd.Long)
		fmt.Println()
		ListAllTools()
	},
}

var toolsSwCmd = &cobra.Command{
	Use:   "sw",
	Short: "Switch tools on/off",
	Long:  "Choose which embedding tools to enable for the current agent.",
	Run: func(cmd *cobra.Command, args []string) {
		// Get all available tools
		allTools := service.GetAllEmbeddingTools()

		// Get currently enabled tools
		enabledTools := GetAgentStringSlice("tools")
		if enabledTools == nil {
			// If no tools configured, default to all tools
			enabledTools = nil
		}

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
		// This fixes the huh MultiSelect UI issue where scroll starts at last selected item
		sort.Slice(options, func(i, j int) bool {
			iSelected := enabledSet[options[i].Value]
			jSelected := enabledSet[options[j].Value]
			if iSelected != jSelected {
				return iSelected // selected items come first
			}
			return options[i].Key < options[j].Key
		})

		var selectedTools []string
		err := huh.NewForm(
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title("Select Embedding Tools").
					Description("Choose which tools to enable for this agent. Press space to toggle, enter to confirm.").
					Options(options...).
					Value(&selectedTools),
			),
		).Run()

		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		// Save selected tools
		err = SetAgentValue("tools", selectedTools)
		if err != nil {
			fmt.Printf("Error saving tools config: %v\n", err)
			return
		}

		fmt.Printf("\n%d tool(s) enabled for current agent.\n\n", len(selectedTools))
		ListAllTools()
	},
}

func init() {
	toolsCmd.AddCommand(toolsSwCmd)
	rootCmd.AddCommand(toolsCmd)
}

// GetEnabledTools returns the list of enabled tools for the current agent
// If nil/empty, returns all tools (default behavior)
func GetEnabledTools() []string {
	enabledTools := GetAgentStringSlice("tools")
	if enabledTools == nil {
		return nil
	}
	return enabledTools
}

// AreToolsEnabled returns true if tools are enabled (at least one tool is configured)
func AreToolsEnabled() bool {
	enabledTools := GetAgentStringSlice("tools")
	// Consider tools enabled if the slice exists and is not empty
	return len(enabledTools) > 0
}

func SwitchUseTools(s string) {
	switch s {
	case "sw":
		toolsSwCmd.Run(toolsSwCmd, nil)
	default:
		toolsCmd.Run(toolsCmd, nil)
	}
}

func ListEmbeddingTools() {
	enabledTools := GetAgentStringSlice("tools")
	allTools := service.GetAllEmbeddingTools()

	// Sort for consistent display
	sortedTools := make([]string, len(allTools))
	copy(sortedTools, allTools)
	sort.Strings(sortedTools)

	// Create a set of enabled tools for lookup
	enabledSet := make(map[string]bool)
	if enabledTools == nil {
		// If nil, all tools are enabled by default
		for _, t := range allTools {
			enabledSet[t] = true
		}
	} else {
		for _, t := range enabledTools {
			enabledSet[t] = true
		}
	}

	enabledCount := 0
	for range enabledSet {
		enabledCount++
	}

	fmt.Println("Embedding tools:")
	for _, tool := range sortedTools {
		if enabledSet[tool] {
			fmt.Printf("[✔] %s\n", tool)
		} else {
			fmt.Printf("[ ] %s\n", tool)
		}
	}
}

func ListAllTools() {
	ListEmbeddingTools()
	fmt.Println()
	ListSearchTools()
}
