package cmd

import (
	"fmt"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/internal/ui"
	"github.com/activebook/gllm/service"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var thinkCmd = &cobra.Command{
	Use:   "think [off|low|medium|high]",
	Short: "View or set thinking level",
	Long: `View or set the thinking/reasoning level for the active agent.
	
Thinking levels:
  off    - Disable thinking mode
  low    - Minimal reasoning effort
  medium - Moderate reasoning effort  
  high   - Maximum reasoning effort

The actual behavior depends on the model provider:
  OpenAI:    Maps to reasoning_effort parameter
  Anthropic: Maps to thinking budget tokens
  Gemini:    Maps to ThinkingLevel or ThinkingBudget`,
	Run: func(cmd *cobra.Command, args []string) {
		store := data.NewConfigStore()
		agent := store.GetActiveAgent()
		if agent == nil {
			fmt.Println("No active agent found")
			return
		}

		// If argument provided, set that level directly
		if len(args) > 0 {
			level := service.ParseThinkingLevel(args[0])
			agent.Think = string(level)
			if err := store.SetAgent(agent.Name, agent); err != nil {
				service.Errorf("failed to save thinking level: %v", err)
				return
			}
			fmt.Printf("Thinking level: %s\n", level.Display())
			return
		}

		// No argument - display current level
		level := service.ParseThinkingLevel(agent.Think)
		fmt.Printf("Thinking level: %s\n", level.Display())
	},
}

var thinkSwitchCmd = &cobra.Command{
	Use:     "switch",
	Aliases: []string{"sw", "select", "sel"},
	Short:   "Interactively select thinking level",
	Long:    `Opens an interactive selector to choose the thinking level.`,
	Run: func(cmd *cobra.Command, args []string) {
		store := data.NewConfigStore()
		agent := store.GetActiveAgent()
		if agent == nil {
			fmt.Println("No active agent found")
			return
		}

		// Current level for pre-selection
		currentLevel := service.ParseThinkingLevel(agent.Think)

		// Interactive selection
		selected := currentLevel.String()
		options := []huh.Option[string]{
			huh.NewOption("Off - Disable thinking", "off"),
			huh.NewOption("Low - Minimal reasoning", "low"),
			huh.NewOption("Medium - Moderate reasoning", "medium"),
			huh.NewOption("High - Maximum reasoning", "high"),
		}
		// Sort options by Selected at first
		ui.SortOptions(options, selected)

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Select thinking level").
					Options(options...).
					Value(&selected),
			),
		)
		if err := form.Run(); err != nil {
			return
		}

		level := service.ParseThinkingLevel(selected)
		agent.Think = string(level)
		if err := store.SetAgent(agent.Name, agent); err != nil {
			service.Errorf("failed to save thinking level: %v", err)
			return
		}
		fmt.Printf("Thinking level: %s\n", level.Display())
	},
}

func init() {
	// Add switch subcommand
	thinkCmd.AddCommand(thinkSwitchCmd)

	// Add the main think command to the root command
	rootCmd.AddCommand(thinkCmd)
}
