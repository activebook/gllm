// File: cmd/agents.go
package cmd

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/service"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

// agentCmd represents the agent subcommand for agents
var agentCmd = &cobra.Command{
	Use:     "agent",
	Aliases: []string{"ag"}, // Optional alias
	Short:   "Manage agent configurations",
	Long: `Manage agent configurations that allow you to quickly switch between
different AI assistant setups with different models, tools, and settings.`,
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

		// Get agent names and sort them
		names := make([]string, 0, len(agents))
		for name := range agents {
			names = append(names, name)
		}
		sort.Strings(names)

		activeAgentName := store.GetActiveAgentName()
		// Display agents in a clean, simple list
		for _, name := range names {
			// change color for selected agent
			prefix := "  "
			if name == activeAgentName {
				prefix = highlightColor("* ")
				name = highlightColor(name)
			}
			fmt.Printf("%s%s\n", prefix, name)
		}

		if activeAgentName != "" {
			fmt.Println("\n(*) Indicates the current agent.")
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
			model         string
			tools         []string
			mcp           bool
			search        string
			template      string
			sysPrompt     string
			usage         bool
			markdown      bool
			think         bool
			maxRecursions string
		)

		// Initial defaults
		markdown = true // Default to true typically
		maxRecursions = "10"

		// Get available options from data layer
		store := data.NewConfigStore()

		// Models
		modelsMap := store.GetModels()
		var modelOptions []huh.Option[string]
		for m := range modelsMap {
			modelOptions = append(modelOptions, huh.NewOption(m, m))
		}
		// Sort models
		sort.Slice(modelOptions, func(i, j int) bool {
			return modelOptions[i].Key < modelOptions[j].Key
		})

		// Templates
		templatesMap := store.GetTemplates()
		var templateOptions []huh.Option[string]
		templateOptions = append(templateOptions, huh.NewOption("None", ""))
		for t := range templatesMap {
			templateOptions = append(templateOptions, huh.NewOption(t, t))
		}
		sort.Slice(templateOptions, func(i, j int) bool {
			return templateOptions[i].Key < templateOptions[j].Key
		})

		// System Prompts
		sysPromptsMap := store.GetSystemPrompts()
		var sysPromptOptions []huh.Option[string]
		sysPromptOptions = append(sysPromptOptions, huh.NewOption("None", ""))
		for s := range sysPromptsMap {
			sysPromptOptions = append(sysPromptOptions, huh.NewOption(s, s))
		}
		sort.Slice(sysPromptOptions, func(i, j int) bool {
			return sysPromptOptions[i].Key < sysPromptOptions[j].Key
		})

		// Search Engines
		engines := store.GetSearchEngines()
		var searchOptions []huh.Option[string]
		searchOptions = append(searchOptions, huh.NewOption("None", ""))
		for s := range engines {
			searchOptions = append(searchOptions, huh.NewOption(s, s))
		}
		sort.Slice(searchOptions, func(i, j int) bool {
			return searchOptions[i].Key < searchOptions[j].Key
		})

		// Tools
		toolsList := service.GetAllEmbeddingTools()
		var toolsOptions []huh.Option[string]
		for _, s := range toolsList {
			toolsOptions = append(toolsOptions, huh.NewOption(s, s))
		}
		sort.Slice(toolsOptions, func(i, j int) bool {
			return toolsOptions[i].Key < toolsOptions[j].Key
		})

		// Build form

		// MultiSelect return type is []string. Correct.
		// I will re-structure to use MultiSelect for the boolean flags.
		var capabilities []string

		// 1. Agent Name
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

		// 2. Model
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

		// 3. System Prompt
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

		// 4. Template
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

		// 5. Search Engine
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Search Engine").
					Description("The search engine to use for web search capabilities").
					Options(searchOptions...).
					Value(&search),
			),
		).Run()
		if err != nil {
			return
		}

		// 6. Tools
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title("Tools").
					Description("The tools to use for agent responses").
					Options(toolsOptions...).
					Value(&tools),
			),
		).Run()
		if err != nil {
			return
		}

		// 7. Max Recursions
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Max Recursions").
					Description("The maximum number of Model calling recursions allowed").
					Value(&maxRecursions).
					Validate(ValidateMaxRecursions),
			),
		).Run()
		if err != nil {
			return
		}

		// 8. Capabilities
		// We can group these or keep them separate? Input is small. MultiSelect is potentially large-ish.
		// Let's keep them somewhat together or just split to be safe?
		// Split is safer.
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title("Capabilities").
					Options(
						huh.NewOption("Enable MCP", "mcp"),
						huh.NewOption("Show Usage", "usage"),
						huh.NewOption("Render Markdown", "markdown"),
						huh.NewOption("Think Mode", "think"),
					).
					Value(&capabilities),
			),
		).Run()
		if err != nil {
			fmt.Println("Aborted.")
			return
		}

		// Process capabilities
		for _, cap := range capabilities {
			switch cap {
			case "mcp":
				mcp = true
			case "usage":
				usage = true
			case "markdown":
				markdown = true
			case "think":
				think = true
			}
		}

		// Construct typed config
		var recursionVal int
		if _, err := fmt.Sscanf(maxRecursions, "%d", &recursionVal); err != nil || recursionVal <= 0 {
			recursionVal = 10
		}

		agentConfig := &data.AgentConfig{
			Name:          name,
			Model:         data.Model{Name: model},
			Tools:         tools,
			MCP:           mcp,
			Usage:         usage,
			Markdown:      markdown,
			Think:         think,
			Search:        data.SearchEngine{Name: search},
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

			sortedNames := make([]string, 0, len(agents))
			for n := range agents {
				sortedNames = append(sortedNames, n)
			}
			sort.Strings(sortedNames)

			for _, n := range sortedNames {
				options = append(options, huh.NewOption(n, n))
			}

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
			model         string
			search        string
			tools         []string
			template      string
			sysPrompt     string
			maxRecursions string
			capabilities  []string
		)

		// Access typed struct fields directly - no type assertions needed!
		model = agent.Model.Name
		search = agent.Search.Name
		tools = agent.Tools
		template = agent.Template
		sysPrompt = agent.SystemPrompt
		if agent.MaxRecursions > 0 {
			maxRecursions = fmt.Sprintf("%d", agent.MaxRecursions)
		} else {
			maxRecursions = "10"
		}

		// Populate capabilities from struct fields
		if agent.MCP {
			capabilities = append(capabilities, "mcp")
		}
		if agent.Usage {
			capabilities = append(capabilities, "usage")
		}
		if agent.Markdown {
			capabilities = append(capabilities, "markdown")
		}
		if agent.Think {
			capabilities = append(capabilities, "think")
		}

		// Reuse options logic - access data layer directly
		modelsMap := store.GetModels()
		var modelOptions []huh.Option[string]
		for name := range modelsMap {
			modelOptions = append(modelOptions, huh.NewOption(name, name))
		}
		sort.Slice(modelOptions, func(i, j int) bool { return modelOptions[i].Key < modelOptions[j].Key })

		templatesMap := store.GetTemplates()
		var templateOptions []huh.Option[string]
		templateOptions = append(templateOptions, huh.NewOption("None", ""))
		for t := range templatesMap {
			templateOptions = append(templateOptions, huh.NewOption(t, t))
		}
		sort.Slice(templateOptions, func(i, j int) bool { return templateOptions[i].Key < templateOptions[j].Key })

		sysPromptsMap := store.GetSystemPrompts()
		var sysPromptOptions []huh.Option[string]
		sysPromptOptions = append(sysPromptOptions, huh.NewOption("None", ""))
		for s := range sysPromptsMap {
			sysPromptOptions = append(sysPromptOptions, huh.NewOption(s, s))
		}
		sort.Slice(sysPromptOptions, func(i, j int) bool { return sysPromptOptions[i].Key < sysPromptOptions[j].Key })

		engines := store.GetSearchEngines()
		var searchOptions []huh.Option[string]
		searchOptions = append(searchOptions, huh.NewOption("None", ""))
		for s := range engines {
			searchOptions = append(searchOptions, huh.NewOption(s, s))
		}
		sort.Slice(searchOptions, func(i, j int) bool { return searchOptions[i].Key < searchOptions[j].Key })

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
		// This fixes the huh MultiSelect UI issue where scroll starts at last selected item
		sort.Slice(toolsOptions, func(i, j int) bool {
			iSelected := toolsSet[toolsOptions[i].Value]
			jSelected := toolsSet[toolsOptions[j].Value]
			if iSelected != jSelected {
				return iSelected // selected items come first
			}
			return toolsOptions[i].Key < toolsOptions[j].Key
		})

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

		// Search
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Search Engine").
					Description("The search engine to use for web search capabilities").
					Options(searchOptions...).
					Value(&search),
			),
		).Run()
		if err != nil {
			return
		}

		// Tools
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title("Tools").
					Description("The tools to use for agent responses").
					Options(toolsOptions...).
					Value(&tools),
			),
		).Run()
		if err != nil {
			return
		}

		// Max Recursions
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Max Recursions").
					Description("The maximum number of Model calling recursions allowed").
					Value(&maxRecursions).
					Validate(ValidateMaxRecursions),
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
			huh.NewOption("Enable MCP", "mcp").Selected(capsSet["mcp"]),
			huh.NewOption("Show Usage", "usage").Selected(capsSet["usage"]),
			huh.NewOption("Render Markdown", "markdown").Selected(capsSet["markdown"]),
			huh.NewOption("Think Mode", "think").Selected(capsSet["think"]),
		}
		sort.Slice(capsOpts, func(i, j int) bool {
			iSelected := capsSet[capsOpts[i].Value]
			jSelected := capsSet[capsOpts[j].Value]
			if iSelected != jSelected {
				return iSelected // selected items come first
			}
			return capsOpts[i].Key < capsOpts[j].Key
		})
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title("Capabilities").
					Options(capsOpts...).
					Value(&capabilities),
			),
		).Run()

		if err != nil {
			fmt.Println("Aborted.")
			return
		}

		// Reconstruct typed config
		var recursionVal int
		fmt.Sscanf(maxRecursions, "%d", &recursionVal)
		if recursionVal <= 0 {
			recursionVal = 10
		}

		// Process capabilities to booleans
		mcpEnabled := false
		usageEnabled := false
		markdownEnabled := false
		thinkEnabled := false
		for _, cap := range capabilities {
			switch cap {
			case "mcp":
				mcpEnabled = true
			case "usage":
				usageEnabled = true
			case "markdown":
				markdownEnabled = true
			case "think":
				thinkEnabled = true
			}
		}

		agentConfig := &data.AgentConfig{
			Name:          name,
			Model:         data.Model{Name: model},
			Tools:         tools,
			MCP:           mcpEnabled,
			Usage:         usageEnabled,
			Markdown:      markdownEnabled,
			Think:         thinkEnabled,
			Search:        data.SearchEngine{Name: search},
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
			sort.Slice(options, func(i, j int) bool { return options[i].Key < options[j].Key })

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

			sortedNames := make([]string, 0, len(agents))
			for n := range agents {
				sortedNames = append(sortedNames, n)
			}
			sort.Strings(sortedNames)

			for _, n := range sortedNames {
				options = append(options, huh.NewOption(n, n))
			}

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
	Run: func(cmd *cobra.Command, args []string) {
		store := data.NewConfigStore()
		var name string
		if len(args) > 0 {
			name = args[0]
		} else {
			// Default to active agent
			name = store.GetActiveAgentName()
			if name == "" {
				fmt.Println("No active agent.")
				return
			}
		}

		agentConfig := store.GetAgent(name)
		if agentConfig == nil {
			fmt.Printf("Agent '%s' not found.\n", name)
			return
		}

		fmt.Printf("Agent '%s' configuration:\n", name)
		fmt.Println()

		// Display configuration using the same formatting as add/set commands
		printAgentConfigDetails(agentConfig, "  ")
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

	// Note: We removed flags for interactive commands, but we could keep them for scripting if needed.
	// agentAddCmd flags

	// agentSetCmd flags

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
		if len(resolvedSysPrompt) > 50 {
			fmt.Printf("%sSystem Prompt: %s...\n", spaceholder, resolvedSysPrompt[:47])
		} else {
			fmt.Printf("%sSystem Prompt: %s\n", spaceholder, resolvedSysPrompt)
		}
	} else {
		fmt.Printf("%sSystem Prompt: \n", spaceholder)
	}

	if agent.Template != "" {
		resolvedTemplate := store.GetTemplate(agent.Template)
		if len(resolvedTemplate) > 50 {
			fmt.Printf("%sTemplate: %s...\n", spaceholder, resolvedTemplate[:47])
		} else {
			fmt.Printf("%sTemplate: %s\n", spaceholder, resolvedTemplate)
		}
	} else {
		fmt.Printf("%sTemplate: \n", spaceholder)
	}

	fmt.Printf("%sSearch: %s\n", spaceholder, agent.Search.Name)

	toolsSlice := ""
	for _, tool := range agent.Tools {
		toolsSlice += fmt.Sprintf("\n%s  - %s", spaceholder, tool)
	}
	fmt.Printf("%sTools:%s\n", spaceholder, toolsSlice)

	fmt.Printf("%sMCP: %v\n", spaceholder, agent.MCP)
	fmt.Printf("%sUsage: %v\n", spaceholder, agent.Usage)
	fmt.Printf("%sMarkdown: %v\n", spaceholder, agent.Markdown)
	fmt.Printf("%sThink: %v\n", spaceholder, agent.Think)
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
