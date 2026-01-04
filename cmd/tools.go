package cmd

import (
	"fmt"
	"sort"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/service"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
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
		SortMultiOptions(options, enabledTools)

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
		agent.Tools = selectedTools
		err = store.SetAgent(agent.Name, agent)
		if err != nil {
			fmt.Printf("Error saving tools config: %v\n", err)
			return
		}

		fmt.Printf("\n%d tool(s) enabled for current agent.\n\n", len(selectedTools))
		ListAllTools()
	},
}

func ListEmbeddingTools() {
	store := data.NewConfigStore()
	agent := store.GetActiveAgent()
	if agent == nil {
		fmt.Println("No active agent found")
		return
	}

	allTools := service.GetAllEmbeddingTools()

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
