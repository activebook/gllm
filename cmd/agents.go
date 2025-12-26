// File: cmd/agents.go
package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/activebook/gllm/service"
	"github.com/charmbracelet/huh"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// agentListCmd represents the list subcommand for agents
var agentListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all configured agents",
	Long:    `List all configured agent profiles with their names and basic information.`,
	Run: func(cmd *cobra.Command, args []string) {
		// List all agents
		agents, err := service.GetAllAgents()
		if err != nil {
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

		activeAgent := service.GetCurrentAgentName()

		highlightColor := color.New(color.FgGreen, color.Bold).SprintFunc()
		// Display agents in a clean, simple list
		for _, name := range names {
			// change color for selected agent
			prefix := "  "
			if name == activeAgent {
				prefix = highlightColor("* ")
				name = highlightColor(name)
			}
			fmt.Printf("%s%s\n", prefix, name)
		}

		if activeAgent != "" {
			fmt.Println("\n(*) Indicates the current agent.")
		} else {
			fmt.Println("\nNo agent selected. Use 'gllm agent switch <name>' to select one.")
			fmt.Println("The first available agent will be used if needed.")
		}
	},
}

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage agent configurations",
	Long: `Manage agent configurations that allow you to quickly switch between
different AI assistant setups with different models, tools, and settings.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Show current agent configuration first
		currentConfig := service.GetCurrentAgentConfig()
		if len(currentConfig) > 0 {
			fmt.Println("Current agent configuration:")
			printAgentConfigDetails(currentConfig, "  ")
			fmt.Println()
		} else {
			fmt.Println("No current agent configuration found.")
			fmt.Println()
		}

		// Then show the list of available agents
		agentListCmd.Run(cmd, args)
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
			tools         bool
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

		// Get available options
		// Models
		modelsMap := viper.GetStringMap("models")
		var modelOptions []huh.Option[string]
		for m := range modelsMap {
			modelOptions = append(modelOptions, huh.NewOption(decodeModelName(m), decodeModelName(m)))
		}
		// Sort models
		sort.Slice(modelOptions, func(i, j int) bool {
			return modelOptions[i].Key < modelOptions[j].Key
		})

		// Templates
		templatesMap := viper.GetStringMapString("templates")
		var templateOptions []huh.Option[string]
		templateOptions = append(templateOptions, huh.NewOption("None", ""))
		for t := range templatesMap {
			templateOptions = append(templateOptions, huh.NewOption(t, t))
		}
		sort.Slice(templateOptions, func(i, j int) bool {
			return templateOptions[i].Key < templateOptions[j].Key
		})

		// System Prompts
		sysPromptsMap := viper.GetStringMapString("system_prompts")
		var sysPromptOptions []huh.Option[string]
		sysPromptOptions = append(sysPromptOptions, huh.NewOption("None", ""))
		for s := range sysPromptsMap {
			sysPromptOptions = append(sysPromptOptions, huh.NewOption(s, s))
		}
		sort.Slice(sysPromptOptions, func(i, j int) bool {
			return sysPromptOptions[i].Key < sysPromptOptions[j].Key
		})

		// Search Engines
		searchMap := viper.GetStringMap("search_engines")
		var searchOptions []huh.Option[string]
		searchOptions = append(searchOptions, huh.NewOption("None", ""))
		for s := range searchMap {
			searchOptions = append(searchOptions, huh.NewOption(s, s))
		}
		sort.Slice(searchOptions, func(i, j int) bool {
			return searchOptions[i].Key < searchOptions[j].Key
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
						// Check if exists
						if _, err := service.GetAgent(s); err == nil {
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

		// 6. Max Recursions & 7. Capabilities
		// We can group these or keep them separate? Input is small. MultiSelect is potentially large-ish.
		// Let's keep them somewhat together or just split to be safe?
		// Split is safer.
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Max Recursions").
					Description("The maximum number of Model calling recursions allowed").
					Value(&maxRecursions),
			),
		).Run()
		if err != nil {
			return
		}

		err = huh.NewForm(
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title("Capabilities").
					Options(
						huh.NewOption("Enable Tools", "tools"),
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
			case "tools":
				tools = true
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

		// Construct config
		agentConfig := make(service.AgentConfig)
		agentConfig["model"] = encodeModelName(model)
		agentConfig["tools"] = tools
		agentConfig["mcp"] = mcp
		agentConfig["usage"] = usage
		agentConfig["markdown"] = markdown
		agentConfig["think"] = think
		agentConfig["search"] = search
		agentConfig["template"] = template
		agentConfig["system_prompt"] = sysPrompt

		// Parse maxRecursions

		// Parse maxRecursions
		var val int
		if _, err := fmt.Sscanf(maxRecursions, "%d", &val); err != nil || val <= 0 {
			agentConfig["max_recursions"] = 10
		} else {
			agentConfig["max_recursions"] = val
		}

		err = service.AddAgentWithConfig(name, agentConfig)
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
		var name string
		if len(args) > 0 {
			name = args[0]
		} else {
			// Default to current agent
			current := service.GetCurrentAgentName()
			if current != "" {
				name = current
			}
			// Select agent
			agents, err := service.GetAllAgents()
			if err != nil || len(agents) == 0 {
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

			err = huh.NewSelect[string]().
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
		existingConfig, err := service.GetAgent(name)
		if err != nil {
			fmt.Printf("Error getting agent: %v\n", err)
			return
		}

		// Form variables populated with existing config
		var (
			model         string
			search        string
			template      string
			sysPrompt     string
			maxRecursions string
			capabilities  []string
		)

		if v, ok := existingConfig["model"].(string); ok {
			model = decodeModelName(v)
		}
		if v, ok := existingConfig["search"].(string); ok {
			search = v
		}
		if v, ok := existingConfig["template"].(string); ok {
			template = v
		}
		if v, ok := existingConfig["system_prompt"].(string); ok {
			sysPrompt = v
		}
		if v, ok := existingConfig["max_recursions"]; ok {
			maxRecursions = fmt.Sprintf("%v", v)
		} else {
			maxRecursions = "10"
		}

		// Populate capabilities
		if v, ok := existingConfig["tools"].(bool); ok && v {
			capabilities = append(capabilities, "tools")
		}
		if v, ok := existingConfig["mcp"].(bool); ok && v {
			capabilities = append(capabilities, "mcp")
		}
		if v, ok := existingConfig["usage"].(bool); ok && v {
			capabilities = append(capabilities, "usage")
		}
		if v, ok := existingConfig["markdown"].(bool); ok && v {
			capabilities = append(capabilities, "markdown")
		}
		if v, ok := existingConfig["think"].(bool); ok && v {
			capabilities = append(capabilities, "think")
		}

		// Reuse options logic (simplified copy-paste for safety)
		modelsMap := viper.GetStringMap("models")
		var modelOptions []huh.Option[string]
		for m := range modelsMap {
			modelOptions = append(modelOptions, huh.NewOption(decodeModelName(m), decodeModelName(m)))
		}
		sort.Slice(modelOptions, func(i, j int) bool { return modelOptions[i].Key < modelOptions[j].Key })

		templatesMap := viper.GetStringMapString("templates")
		var templateOptions []huh.Option[string]
		templateOptions = append(templateOptions, huh.NewOption("None", ""))
		for t := range templatesMap {
			templateOptions = append(templateOptions, huh.NewOption(t, t))
		}
		sort.Slice(templateOptions, func(i, j int) bool { return templateOptions[i].Key < templateOptions[j].Key })

		sysPromptsMap := viper.GetStringMapString("system_prompts")
		var sysPromptOptions []huh.Option[string]
		sysPromptOptions = append(sysPromptOptions, huh.NewOption("None", ""))
		for s := range sysPromptsMap {
			sysPromptOptions = append(sysPromptOptions, huh.NewOption(s, s))
		}
		sort.Slice(sysPromptOptions, func(i, j int) bool { return sysPromptOptions[i].Key < sysPromptOptions[j].Key })

		searchMap := viper.GetStringMap("search_engines")
		var searchOptions []huh.Option[string]
		searchOptions = append(searchOptions, huh.NewOption("None", ""))
		for s := range searchMap {
			searchOptions = append(searchOptions, huh.NewOption(s, s))
		}
		sort.Slice(searchOptions, func(i, j int) bool { return searchOptions[i].Key < searchOptions[j].Key })

		// Build form
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

		// Max Recursions
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Max Recursions").
					Description("The maximum number of Model calling recursions allowed").
					Value(&maxRecursions),
			),
		).Run()
		if err != nil {
			return
		}

		// Capabilities
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title("Capabilities").
					Options(
						huh.NewOption("Enable Tools", "tools"),
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

		// Reconstruct config
		agentConfig := make(service.AgentConfig)
		agentConfig["model"] = encodeModelName(model)
		agentConfig["search"] = search
		agentConfig["template"] = template
		agentConfig["system_prompt"] = sysPrompt

		var val int
		fmt.Sscanf(maxRecursions, "%d", &val)
		agentConfig["max_recursions"] = val

		// Reset bools
		agentConfig["tools"] = false
		agentConfig["mcp"] = false
		agentConfig["usage"] = false
		agentConfig["markdown"] = false
		agentConfig["think"] = false

		for _, cap := range capabilities {
			agentConfig[cap] = true
		}

		// Keep other existing keys if any (though we reconstructed practically everything)

		err = service.SetAgent(name, agentConfig)
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
		var name string
		if len(args) > 0 {
			name = args[0]
		} else {
			// Select agent to remove
			agents, err := service.GetAllAgents()
			if err != nil || len(agents) == 0 {
				fmt.Println("No agents found.")
				return
			}

			var options []huh.Option[string]
			for n := range agents {
				options = append(options, huh.NewOption(n, n))
			}
			sort.Slice(options, func(i, j int) bool { return options[i].Key < options[j].Key })

			err = huh.NewSelect[string]().
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

		err = service.RemoveAgent(name)
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
		var name string
		if len(args) > 0 {
			name = args[0]
		} else {
			// Default to current agent
			current := service.GetCurrentAgentName()
			if current != "" {
				name = current
			}
			// Interactive select
			agents, err := service.GetAllAgents()
			if err != nil || len(agents) == 0 {
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

			err = huh.NewSelect[string]().
				Title("Select Agent").
				Options(options...).
				Value(&name).
				Run()

			if err != nil {
				fmt.Println("Aborted.")
				return
			}
		}

		err := service.SwitchToAgent(name)
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
		var name string
		if len(args) > 0 {
			name = args[0]
		} else {
			// Default to active agent
			name = service.GetCurrentAgentName()
			if name == "unknown" {
				fmt.Println("No active agent.")
				return
			}
		}

		agentConfig, err := service.GetAgent(name)
		if err != nil {
			fmt.Printf("Error getting agent info: %v\n", err)
			return
		}

		fmt.Printf("Agent '%s' configuration:\n", name)
		fmt.Println("==========================")

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
	// However, the requirement emphasizes interactivity.
	// If the user wants to use scripting, they might need to use `gllm config` or we'd need to add flags back and check if they are set.
	// For now, I'm focusing on the interactive requirement as primary.
}

// printAgentConfigDetails prints the agent details in a formatted way
func printAgentConfigDetails(agent map[string]interface{}, spaceholder string) {
	if name, exists := agent["name"]; exists {
		fmt.Printf("%sName: %s\n", spaceholder, name)
	}

	if model, exists := agent["model"]; exists {
		fmt.Printf("%sModel: %s\n", spaceholder, decodeModelName(model.(string)))
	}

	if system, exists := agent["system_prompt"]; exists {
		if sysPromptStr, ok := system.(string); ok && sysPromptStr != "" {
			// Resolve system prompt reference for display (don't modify stored config)
			resolvedSysPrompt := service.ResolveSystemPromptReference(sysPromptStr)
			// Truncate if too long?
			if len(resolvedSysPrompt) > 50 {
				fmt.Printf("%sSystem Prompt: %s...\n", spaceholder, resolvedSysPrompt[:47])
			} else {
				fmt.Printf("%sSystem Prompt: %s\n", spaceholder, resolvedSysPrompt)
			}
		} else {
			fmt.Printf("%sSystem Prompt: \n", spaceholder)
		}
	} else {
		fmt.Printf("%sSystem Prompt: \n", spaceholder)
	}

	if template, exists := agent["template"]; exists {
		if templateStr, ok := template.(string); ok && templateStr != "" {
			// Resolve template reference for display (don't modify stored config)
			resolvedTemplate := service.ResolveTemplateReference(templateStr)
			if len(resolvedTemplate) > 50 {
				fmt.Printf("%sTemplate: %s...\n", spaceholder, resolvedTemplate[:47])
			} else {
				fmt.Printf("%sTemplate: %s\n", spaceholder, resolvedTemplate)
			}
		} else {
			fmt.Printf("%sTemplate: \n", spaceholder)
		}
	} else {
		fmt.Printf("%sTemplate: \n", spaceholder)
	}

	if search, exists := agent["search"]; exists {
		fmt.Printf("%sSearch: %s\n", spaceholder, search)
	} else {
		fmt.Printf("%sSearch: \n", spaceholder)
	}

	if mcp, exists := agent["mcp"]; exists {
		fmt.Printf("%sMCP: %v\n", spaceholder, mcp)
	} else {
		fmt.Printf("%sMCP: false\n", spaceholder)
	}

	if usage, exists := agent["usage"]; exists {
		fmt.Printf("%sUsage: %v\n", spaceholder, usage)
	} else {
		fmt.Printf("%sUsage: false\n", spaceholder)
	}

	if markdown, exists := agent["markdown"]; exists {
		fmt.Printf("%sMarkdown: %v\n", spaceholder, markdown)
	} else {
		fmt.Printf("%sMarkdown: false\n", spaceholder)
	}

	if think, exists := agent["think"]; exists {
		fmt.Printf("%sThink: %v\n", spaceholder, think)
	} else {
		fmt.Printf("%sThink: false\n", spaceholder)
	}

	if maxRecursions, exists := agent["max_recursions"]; exists {
		fmt.Printf("%sMax Recursions: %v\n", spaceholder, maxRecursions)
	} else {
		fmt.Printf("%sMax Recursions: 0\n", spaceholder)
	}
}

func GetMaxRecursions() int {
	maxRecursions := GetAgentInt("max_recursions")
	if maxRecursions <= 0 {
		maxRecursions = 10 // Default value
	}
	return maxRecursions
}
