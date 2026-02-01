// File: cmd/agent.go
package cmd

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/internal/ui"
	"github.com/activebook/gllm/service"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

const (
	MaxRecursionsDescription = `[Max session turns]() controls the maximum number of user/model/tool turns to keep in a session.
- _Increase for complex multi-step tasks (100)_
- _Decrease for simple tasks (20-50) to save tokens_
- _For recursive agent ( [RLM]()), set to 200 to allow for more complex tasks_
- _-1 means unlimited, let agent decide when to stop, but will use more tokens_`
)

var ()

// agentCmd represents the agent subcommand for agents
var agentCmd = &cobra.Command{
	Use:     "agent",
	Aliases: []string{"ag"}, // Optional alias
	Short:   "Manage agent configurations",
	Long: `Manage agent configurations that allow you to quickly switch between
different AI assistant setups with different models, tools, and settings.`,
	// Add completion support
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return []string{"list", "add", "set", "remove", "switch", "info", "--help"}, cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	Run: func(cmd *cobra.Command, args []string) {
		// Show current agent configuration first
		store := data.NewConfigStore()
		activeAgent := store.GetActiveAgent()
		if activeAgent == nil {
			fmt.Println("No current agent configuration found.")
			fmt.Println()
			return
		}
		fmt.Println("Current agent configuration:")
		printAgentConfigDetails(activeAgent, "  ")
		fmt.Println()

		// Then show the list of available agents
		agentListCmd.Run(agentListCmd, args)
	},
}

// agentListCmd represents the list subcommand for agents
var agentListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all configured agents",
	Long:    `List all configured agent profiles with their names and basic information.`,
	Run: func(cmd *cobra.Command, args []string) {
		// List all agents
		store := data.NewConfigStore()
		agents := store.GetAllAgents()
		if agents == nil {
			fmt.Printf("No agents configured yet. Use 'gllm agent add' to create one.\n")
			return
		}

		if len(agents) == 0 {
			fmt.Printf("No agents configured yet. Use 'gllm agent add' to create one.\n")
			return
		}

		fmt.Println("Available agents:")
		fmt.Println()

		// Get agent names and sort them
		names := make([]string, 0, len(agents))
		for name := range agents {
			names = append(names, name)
		}
		sort.Strings(names)

		activeAgentName := store.GetActiveAgentName()
		// Display agents in a clean, simple list
		for _, name := range names {
			active := name == activeAgentName
			indicator := ui.FormatEnabledIndicator(active)
			displayName := name
			if active {
				displayName = data.SwitchOnColor + name + data.ResetSeq
			}
			fmt.Printf("  %s %s\n", indicator, displayName)
		}

		if activeAgentName != "" {
			fmt.Printf("\n%s = Current agent\n", ui.FormatEnabledIndicator(true))
		} else {
			fmt.Println("\nNo agent selected. Use 'gllm agent switch <name>' to select one.")
			fmt.Println("The first available agent will be used if needed.")
		}
	},
}

var agentAddCmd = &cobra.Command{
	Use:   "add [NAME]",
	Short: "Add a new agent interactively",
	Long:  `Add a new agent with an interactive form configuration.`,
	Run: func(cmd *cobra.Command, args []string) {
		var name string
		if len(args) > 0 {
			name = args[0]
		}

		// Form variables
		var (
			model     string
			tools     []string
			think     string
			template  string
			sysPrompt string
		)

		// Initial defaults

		// Get available options from data layer
		store := data.NewConfigStore()

		// Models
		modelsMap := store.GetModels()
		var modelOptions []huh.Option[string]
		for m := range modelsMap {
			modelOptions = append(modelOptions, huh.NewOption(m, m))
		}
		// Sort models
		ui.SortOptions(modelOptions, "")

		// Templates
		templatesMap := store.GetTemplates()
		var templateOptions []huh.Option[string]
		templateOptions = append(templateOptions, huh.NewOption("None", ""))
		for t := range templatesMap {
			templateOptions = append(templateOptions, huh.NewOption(t, t))
		}
		ui.SortOptions(templateOptions, "")

		// System Prompts
		sysPromptsMap := store.GetSystemPrompts()
		var sysPromptOptions []huh.Option[string]
		sysPromptOptions = append(sysPromptOptions, huh.NewOption("None", ""))
		for s := range sysPromptsMap {
			sysPromptOptions = append(sysPromptOptions, huh.NewOption(s, s))
		}
		ui.SortOptions(sysPromptOptions, "")

		// Tools
		toolsList := service.GetAllEmbeddingTools()
		var toolsOptions []huh.Option[string]
		for _, s := range toolsList {
			toolsOptions = append(toolsOptions, huh.NewOption(s, s).Selected(true))
		}
		ui.SortOptions(toolsOptions, "")

		// Build form

		// MultiSelect return type is []string. Correct.
		// I will re-structure to use MultiSelect for the boolean flags.
		var capabilities []string

		// Agent Name
		err := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Agent Name").
					Value(&name).
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return fmt.Errorf("name is required")
						}
						if err := CheckAgentName(s); err != nil {
							return err
						}
						// Check if exists
						agent := store.GetAgent(s)
						if agent != nil {
							return fmt.Errorf("agent '%s' already exists", s)
						}
						return nil
					}),
			),
		).Run()
		if err != nil {
			fmt.Println("Aborted.")
			return
		}

		// Model
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Model").
					Options(modelOptions...).
					Value(&model),
			),
		).Run()
		if err != nil {
			return
		}

		// System Prompt
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("System Prompt").
					Description("The system prompt to use for agent responses").
					Options(sysPromptOptions...).
					Value(&sysPrompt),
			),
		).Run()
		if err != nil {
			return
		}

		// Template
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Template").
					Description("The template to use for agent responses").
					Options(templateOptions...).
					Value(&template),
			),
		).Run()
		if err != nil {
			return
		}

		// Tools
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title("Select Embedding Tools").
					Description("Choose which tools to enable for this agent. Press space to toggle, enter to confirm.").
					Options(toolsOptions...).
					Value(&tools),
				ui.GetStaticHuhNote("Tools Details", EmbeddingToolsDescription),
			),
		).Run()
		if err != nil {
			return
		}

		// Max Recursions
		recursionVal, err := runMaxRecursionsSelection(50) // Default 50 for new agent
		if err != nil {
			return
		}

		// Thinking Level
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Thinking Level").
					Description("Select the thinking level for this agent").
					Options(
						huh.NewOption("Off - Disable thinking", "off").Selected(true),
						huh.NewOption("Low - Minimal reasoning", "low").Selected(false),
						huh.NewOption("Medium - Moderate reasoning", "medium").Selected(false),
						huh.NewOption("High - Maximum reasoning", "high").Selected(false),
					).
					Value(&think),
			),
		).Run()
		if err != nil {
			return
		}

		// Capabilities
		// We can group these or keep them separate? Input is small. MultiSelect is potentially large-ish.
		// Let's keep them somewhat together or just split to be safe?
		// Split is safer.
		msfeatures := huh.NewMultiSelect[string]().
			Title("Agent Capabilities").
			Description("Use space to toggle, enter to confirm.").
			Options(huh.NewOption("Show Usage Stats", service.CapabilityTokenUsage).Selected(true),
				huh.NewOption("Show Markdown Output", service.CapabilityMarkdown).Selected(true),
				huh.NewOption("Enable MCP Servers", service.CapabilityMCPServers).Selected(false),
				huh.NewOption("Enable Agent Skills", service.CapabilityAgentSkills).Selected(false),
				huh.NewOption("Enable Agent Memory", service.CapabilityAgentMemory).Selected(false),
				huh.NewOption("Enable Sub Agents", service.CapabilitySubAgents).Selected(false),
				huh.NewOption("Enable Web Search", service.CapabilityWebSearch).Selected(false)).
			Value(&capabilities)
		featureNote := ui.GetDynamicHuhNote("Feature Details", msfeatures, getFeatureDescription)
		err = huh.NewForm(
			huh.NewGroup(
				msfeatures,
				featureNote,
			),
		).Run()
		if err != nil {
			fmt.Println("Aborted.")
			return
		}

		agentConfig := &data.AgentConfig{
			Name:          name,
			Model:         data.Model{Name: model},
			Tools:         tools,
			Capabilities:  capabilities,
			Think:         think,
			Template:      template,
			SystemPrompt:  sysPrompt,
			MaxRecursions: recursionVal,
		}

		store.SetAgent(name, agentConfig)
		if err != nil {
			fmt.Printf("Error adding agent: %v\n", err)
			return
		}

		fmt.Printf("Agent '%s' added successfully.\n", name)
	},
}

var agentSetCmd = &cobra.Command{
	Use:   "set [NAME]",
	Short: "Update an existing agent configuration",
	Long:  `Update an existing agent with detailed configuration settings using an interactive form.`,
	Run: func(cmd *cobra.Command, args []string) {
		store := data.NewConfigStore()
		var name string
		if len(args) > 0 {
			name = args[0]
		} else {
			// Default to current agent
			name = store.GetActiveAgentName()

			// Select agent
			agents := store.GetAllAgents()
			if len(agents) == 0 {
				fmt.Println("No agents found.")
				return
			}

			var options []huh.Option[string]

			for n := range agents {
				options = append(options, huh.NewOption(n, n))
			}
			// Sort names alphabetically and keep selected agent at top if exists
			ui.SortOptions(options, name)

			err := huh.NewSelect[string]().
				Title("Select Agent to Edit").
				Options(options...).
				Value(&name).
				Run()
			if err != nil {
				fmt.Println("Aborted.")
				return
			}
		}

		// Get existing agent configuration
		agent := store.GetAgent(name)
		if agent == nil {
			fmt.Printf("Agent '%s' not found.\n", name)
			return
		}

		// Form variables populated with existing config
		var (
			model        string
			tools        []string
			think        string
			template     string
			sysPrompt    string
			capabilities []string
		)

		// Access typed struct fields directly - no type assertions needed!
		model = agent.Model.Name
		tools = agent.Tools
		think = agent.Think
		template = agent.Template
		sysPrompt = agent.SystemPrompt

		// Populate capabilities from struct fields
		capabilities = agent.Capabilities

		// Reuse options logic - access data layer directly
		modelsMap := store.GetModels()
		var modelOptions []huh.Option[string]
		for name := range modelsMap {
			modelOptions = append(modelOptions, huh.NewOption(name, name))
		}
		ui.SortOptions(modelOptions, model)

		templatesMap := store.GetTemplates()
		var templateOptions []huh.Option[string]
		templateOptions = append(templateOptions, huh.NewOption("None", " "))
		for t := range templatesMap {
			templateOptions = append(templateOptions, huh.NewOption(t, t))
		}
		ui.SortOptions(templateOptions, template)

		sysPromptsMap := store.GetSystemPrompts()
		var sysPromptOptions []huh.Option[string]
		sysPromptOptions = append(sysPromptOptions, huh.NewOption("None", " "))
		for s := range sysPromptsMap {
			sysPromptOptions = append(sysPromptOptions, huh.NewOption(s, s))
		}
		ui.SortOptions(sysPromptOptions, sysPrompt)

		// Tools - build options with pre-selected state
		toolsList := service.GetAllEmbeddingTools()
		toolsSet := make(map[string]bool)
		for _, t := range tools {
			toolsSet[t] = true
		}
		var toolsOptions []huh.Option[string]
		for _, s := range toolsList {
			opt := huh.NewOption(s, s)
			if toolsSet[s] {
				opt = opt.Selected(true)
			}
			toolsOptions = append(toolsOptions, opt)
		}
		// Sort: selected items first, then alphabetically within each group
		ui.SortMultiOptions(toolsOptions, tools)

		// Think
		// Current level for pre-selection
		currentThinkLevel := service.ParseThinkingLevel(think)
		thinkOptions := []huh.Option[string]{
			huh.NewOption("Off - Disable thinking", "off").Selected(currentThinkLevel == service.ThinkingLevelOff),
			huh.NewOption("Low - Minimal reasoning", "low").Selected(currentThinkLevel == service.ThinkingLevelLow),
			huh.NewOption("Medium - Moderate reasoning", "medium").Selected(currentThinkLevel == service.ThinkingLevelMedium),
			huh.NewOption("High - Maximum reasoning", "high").Selected(currentThinkLevel == service.ThinkingLevelHigh),
		}
		ui.SortOptions(thinkOptions, think)

		// Build form
		// Model
		err := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Model").
					Options(modelOptions...).
					Value(&model),
			),
		).Run()
		if err != nil {
			fmt.Println("Aborted.")
			return
		}

		// System Prompt
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("System Prompt").
					Description("The system prompt to use for agent responses").
					Options(sysPromptOptions...).
					Value(&sysPrompt),
			),
		).Run()
		if err != nil {
			return
		}

		// Template
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Template").
					Description("The template to use for agent responses").
					Options(templateOptions...).
					Value(&template),
			),
		).Run()
		if err != nil {
			return
		}

		// Tools
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title("Select Embedding Tools").
					Description("Choose which tools to enable for this agent. Press space to toggle, enter to confirm.").
					Options(toolsOptions...).
					Value(&tools),
				ui.GetStaticHuhNote("Tools Details", EmbeddingToolsDescription),
			),
		).Run()
		if err != nil {
			return
		}

		// Max Recursions
		recursionVal, err := runMaxRecursionsSelection(agent.MaxRecursions)
		if err != nil {
			return
		}

		// Thinking Level
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Thinking Level").
					Description("Select the thinking level for this agent").
					Options(thinkOptions...).
					Value(&think),
			),
		).Run()
		if err != nil {
			return
		}

		// Capabilities - build options with pre-selected state
		capsSet := make(map[string]bool)
		for _, c := range capabilities {
			capsSet[c] = true
		}
		capsOpts := []huh.Option[string]{
			huh.NewOption("Show Usage Stats", service.CapabilityTokenUsage).Selected(capsSet[service.CapabilityTokenUsage]),
			huh.NewOption("Show Markdown Output", service.CapabilityMarkdown).Selected(capsSet[service.CapabilityMarkdown]),
			huh.NewOption("Enable MCP Servers", service.CapabilityMCPServers).Selected(capsSet[service.CapabilityMCPServers]),
			huh.NewOption("Enable Agent Skills", service.CapabilityAgentSkills).Selected(capsSet[service.CapabilityAgentSkills]),
			huh.NewOption("Enable Agent Memory", service.CapabilityAgentMemory).Selected(capsSet[service.CapabilityAgentMemory]),
			huh.NewOption("Enable Sub Agents", service.CapabilitySubAgents).Selected(capsSet[service.CapabilitySubAgents]),
			huh.NewOption("Enable Web Search", service.CapabilityWebSearch).Selected(capsSet[service.CapabilityWebSearch]),
		}
		ui.SortMultiOptions(capsOpts, capabilities)
		msfeatures := huh.NewMultiSelect[string]().
			Title("Agent Capabilities").
			Description("Use space to toggle, enter to confirm.").
			Options(capsOpts...).
			Value(&capabilities)
		featureNote := ui.GetDynamicHuhNote("Feature Details", msfeatures, getFeatureDescription)
		err = huh.NewForm(
			huh.NewGroup(
				msfeatures,
				featureNote,
			),
		).Run()

		if err != nil {
			fmt.Println("Aborted.")
			return
		}

		// Process capabilities - no conversion needed

		// Bugfix:
		// We set None options as " " in the form, so we need to trim them
		// Why set " " in the form: huh has a bug, without space, the sort doesn't work
		template = strings.TrimSpace(template)
		sysPrompt = strings.TrimSpace(sysPrompt)

		agentConfig := &data.AgentConfig{
			Name:          name,
			Model:         data.Model{Name: model},
			Tools:         tools,
			Capabilities:  capabilities,
			Think:         think,
			Template:      template,
			SystemPrompt:  sysPrompt,
			MaxRecursions: recursionVal,
		}

		err = store.SetAgent(name, agentConfig)
		if err != nil {
			fmt.Printf("Error updating agent: %v\n", err)
			return
		}

		fmt.Printf("Agent '%s' updated successfully.\n", name)
	},
}

var agentRemoveCmd = &cobra.Command{
	Use:     "remove [NAME]",
	Aliases: []string{"rm", "delete", "del"},
	Short:   "Remove an agent",
	Long:    `Remove an agent configuration. This action cannot be undone.`,
	Run: func(cmd *cobra.Command, args []string) {
		store := data.NewConfigStore()
		var name string
		if len(args) > 0 {
			name = args[0]
		} else {
			name = store.GetActiveAgentName()

			// Select agent to remove
			agents := store.GetAllAgents()
			if len(agents) == 0 {
				fmt.Println("No agents found.")
				return
			}

			var options []huh.Option[string]
			for n := range agents {
				options = append(options, huh.NewOption(n, n))
			}
			ui.SortOptions(options, name)

			err := huh.NewSelect[string]().
				Title("Select Agent to Remove").
				Options(options...).
				Value(&name).
				Run()
			if err != nil {
				fmt.Println("Aborted.")
				return
			}
		}

		// Confirm removal
		var confirm bool
		err := huh.NewConfirm().
			Title(fmt.Sprintf("Are you sure you want to remove agent '%s'?", name)).
			Affirmative("Yes").
			Negative("No").
			Value(&confirm).
			Run()

		if err != nil {
			fmt.Println("Aborted.")
			return
		}

		if !confirm {
			fmt.Println("Operation cancelled.")
			return
		}

		err = store.DeleteAgent(name)
		if err != nil {
			fmt.Printf("Error removing agent: %v\n", err)
			return
		}

		fmt.Printf("Agent '%s' removed successfully.\n", name)
	},
}

var agentSwitchCmd = &cobra.Command{
	Use:     "switch [NAME]",
	Aliases: []string{"select", "sw", "sel"},
	Short:   "Switch to a different agent",
	Long: `Switch to a different agent configuration. This will change your current AI model,
tools, search settings, and other preferences to match the selected agent.`,
	Run: func(cmd *cobra.Command, args []string) {
		store := data.NewConfigStore()

		var name string
		if len(args) > 0 {
			name = args[0]
		} else {
			name = store.GetActiveAgentName()

			// Interactive select
			agents := store.GetAllAgents()
			if len(agents) == 0 {
				fmt.Println("No agents found.")
				return
			}

			var options []huh.Option[string]

			for n := range agents {
				options = append(options, huh.NewOption(n, n))
			}
			// Sort names alphabetically and keep selected agent at top if exists
			ui.SortOptions(options, name)

			err := huh.NewSelect[string]().
				Title("Select Agent").
				Options(options...).
				Value(&name).
				Run()

			if err != nil {
				fmt.Println("Aborted.")
				return
			}
		}

		err := store.SetActiveAgent(name)
		if err != nil {
			fmt.Printf("Error switching to agent: %v\n", err)
			return
		}

		fmt.Printf("Switched to agent '%s'.\n", name)
	},
}

var agentInfoCmd = &cobra.Command{
	Use:     "info [NAME]",
	Aliases: []string{"show", "details"},
	Short:   "Show detailed information about an agent",
	Long:    `Display detailed configuration information for a specific agent.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		store := data.NewConfigStore()
		agents := store.GetAllAgents()
		if len(agents) == 0 {
			return fmt.Errorf("no agents found")
		}

		var name string
		if len(args) > 0 {
			name = args[0]
		} else {
			name = store.GetActiveAgentName()

			// Select agent to check
			var options []huh.Option[string]
			for n := range agents {
				options = append(options, huh.NewOption(n, n))
			}
			ui.SortOptions(options, name)

			err := huh.NewSelect[string]().
				Title("Select Agent to Check").
				Options(options...).
				Value(&name).
				Run()
			if err != nil {
				return err
			}
		}

		agentConfig := store.GetAgent(name)
		if agentConfig == nil {
			return fmt.Errorf("agent '%s' not found", name)
		}

		fmt.Printf("Agent '%s' configuration:\n", name)
		// Display configuration using the same formatting as add/set commands
		printAgentConfigDetails(agentConfig, "  ")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(agentCmd)

	// Add subcommands
	agentCmd.AddCommand(agentListCmd)
	agentCmd.AddCommand(agentAddCmd)
	agentCmd.AddCommand(agentSetCmd)
	agentCmd.AddCommand(agentRemoveCmd)
	agentCmd.AddCommand(agentSwitchCmd)
	agentCmd.AddCommand(agentInfoCmd)
}

// NOTE: getToolsFromConfig is no longer needed - data.AgentConfig.Tools is already []string
// Legacy function removed in data layer refactoring

// printAgentConfigDetails prints the agent details in a formatted way
func printAgentConfigDetails(agent *data.AgentConfig, spaceholder string) {
	if agent.Name != "" {
		fmt.Printf("%sName: %s\n", spaceholder, agent.Name)
	}

	if agent.Model.Name != "" {
		fmt.Printf("%sModel: %s\n", spaceholder, agent.Model.Name)
	}

	store := data.NewConfigStore()
	if agent.SystemPrompt != "" {
		resolvedSysPrompt := store.GetSystemPrompt(agent.SystemPrompt)
		// if len(resolvedSysPrompt) > 50 {
		// 	fmt.Printf("%sSystem Prompt: %s...\n", spaceholder, resolvedSysPrompt[:47])
		// } else {
		// 	fmt.Printf("%sSystem Prompt: %s\n", spaceholder, resolvedSysPrompt)
		// }
		fmt.Printf("%sSystem Prompt: %s\n", spaceholder, resolvedSysPrompt)
	} else {
		fmt.Printf("%sSystem Prompt: \n", spaceholder)
	}

	if agent.Template != "" {
		resolvedTemplate := store.GetTemplate(agent.Template)
		// if len(resolvedTemplate) > 50 {
		// 	fmt.Printf("%sTemplate: %s...\n", spaceholder, resolvedTemplate[:47])
		// } else {
		// 	fmt.Printf("%sTemplate: %s\n", spaceholder, resolvedTemplate)
		// }
		fmt.Printf("%sTemplate: %s\n", spaceholder, resolvedTemplate)
	} else {
		fmt.Printf("%sTemplate: \n", spaceholder)
	}

	toolsSlice := ""
	for _, tool := range agent.Tools {
		toolsSlice += fmt.Sprintf("\n%s  - %s", spaceholder, tool)
	}
	fmt.Printf("%sTools:%s\n", spaceholder, toolsSlice)
	fmt.Printf("%sThink: %v\n", spaceholder, agent.Think)

	// capabilities
	capsSlice := ""
	for _, cap := range agent.Capabilities {
		capsSlice += fmt.Sprintf("\n%s  - %s", spaceholder, cap)
	}
	fmt.Printf("%sCapabilities:%s\n", spaceholder, capsSlice)

	fmt.Printf("%sMax Recursions: %d\n", spaceholder, agent.MaxRecursions)
}

func CheckAgentName(name string) error {
	if strings.Contains(name, ".") {
		return fmt.Errorf("agent name '%s' contains a dot, which is not allowed", name)
	}
	return nil
}

func ValidateMaxRecursions(s string) error {
	if s == "" {
		return nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return err
	}
	if v <= 0 {
		return fmt.Errorf("max recursions must be greater than 0")
	}
	return nil
}

// runMaxRecursionsSelection handles the interactive selection for Max Session Turns
func runMaxRecursionsSelection(currentVal int) (int, error) {
	var selection string

	// Define constants for display - sorted numerically
	const (
		OptNoLimit  = "No limit (-1)"
		OptQuick    = "Quick task (20 turns)"
		OptStandard = "Standard (50 turns)"
		OptComplex  = "Complex task (100 turns)"
		OptExtended = "Extended (200 turns)"
	)

	// Map generic display name to value
	valMap := map[string]int{
		OptNoLimit:  -1,
		OptQuick:    20,
		OptStandard: 50,
		OptComplex:  100,
		OptExtended: 200,
	}

	// Determine initial selection based on currentVal
	initialSelection := OptStandard // Default
	for k, v := range valMap {
		if v == currentVal {
			initialSelection = k
			break
		}
	}

	// Helper to check if option is selected
	isSelected := func(opt string) bool {
		return initialSelection == opt
	}

	options := []huh.Option[string]{
		huh.NewOption(OptNoLimit, OptNoLimit).Selected(isSelected(OptNoLimit)),
		huh.NewOption(OptQuick, OptQuick).Selected(isSelected(OptQuick)),
		huh.NewOption(OptStandard, OptStandard).Selected(isSelected(OptStandard)),
		huh.NewOption(OptComplex, OptComplex).Selected(isSelected(OptComplex)),
		huh.NewOption(OptExtended, OptExtended).Selected(isSelected(OptExtended)),
	}
	ui.SortOptions(options, initialSelection)

	// Use a single form with conditional visibility for smoother UX
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Max Session Turns").
				Description("Maximum number of user/model/tool turns to keep in a session. -1 means unlimited.").
				Options(options...).
				Value(&selection),

			// Re-add the description note as requested
			ui.GetStaticHuhNote("Details", MaxRecursionsDescription),
		),
	).Run()

	if err != nil {
		return 0, err
	}

	return valMap[selection], nil
}
