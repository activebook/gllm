// File: cmd/agents.go
package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/activebook/gllm/service"
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
			fmt.Printf("No agents configured yet. Use 'gllm agent add <name>' to create one.\n")
			return
		}

		if len(agents) == 0 {
			fmt.Printf("No agents configured yet. Use 'gllm agent add <name>' to create one.\n")
			return
		}

		fmt.Println("Available agents:")
		fmt.Println("=================")

		// Get agent names and sort them
		names := make([]string, 0, len(agents))
		for name := range agents {
			names = append(names, name)
		}
		sort.Strings(names)

		// Display agents in a clean, simple list
		for _, name := range names {
			fmt.Printf("  %s\n", name)
		}

		if len(names) > 0 {
			fmt.Println("\nUse 'gllm agent switch <name>' to change agents.")
			fmt.Println("Use 'gllm agent info <name>' to see agent details.")
		}
	},
}

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage agent configurations",
	Long: `Manage agent configurations that allow you to quickly switch between
different AI assistant setups with different models, tools, and settings.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Delegate to list command for consistency with model command
		agentListCmd.Run(cmd, args)
	},
}

var agentAddCmd = &cobra.Command{
	Use:   "add NAME",
	Short: "Add a new agent with detailed configuration",
	Long: `Add a new agent with detailed configuration settings.
Example:
gllm agent add research --model gemini-pro --search google --tools on --template research`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		// Validate name
		if strings.TrimSpace(name) == "" {
			fmt.Println("Error: agent name cannot be empty")
			return
		}

		// Check for reserved names
		if name == "current" || name == "active" {
			fmt.Printf("Error: '%s' is a reserved name\n", name)
			return
		}

		// Get flag values
		model, _ := cmd.Flags().GetString("model")
		tools, _ := cmd.Flags().GetString("tools")
		mcp, _ := cmd.Flags().GetString("mcp")
		search, _ := cmd.Flags().GetString("search")
		template, _ := cmd.Flags().GetString("template")
		sysPrompt, _ := cmd.Flags().GetString("system")
		usage, _ := cmd.Flags().GetString("usage")
		markdown, _ := cmd.Flags().GetString("markdown")
		think, _ := cmd.Flags().GetString("think")
		maxRecursions, _ := cmd.Flags().GetInt("max-recursions")

		// Validate required fields
		if model == "" {
			fmt.Println("Error: model is required. Use --model flag.")
			return
		}

		// Validate model exists in configuration
		encodedModelName := encodeModelName(model)
		modelsMap := viper.GetStringMap("models")
		if _, exists := modelsMap[encodedModelName]; !exists {
			fmt.Printf("Error: model '%s' is not configured. Please add the model first using 'gllm model add'.\n", model)
			return
		}

		// Create agent configuration with explicit defaults
		agentConfig := make(service.AgentConfig)

		// Set model (required)
		agentConfig["model"] = encodeModelName(model)

		// Set boolean fields with explicit defaults (false if not provided)
		if tools != "" {
			if boolVal, err := convertUserInputToBool(tools); err == nil {
				agentConfig["tools"] = boolVal
			} else {
				agentConfig["tools"] = false
			}
		} else {
			agentConfig["tools"] = false // explicit default
		}

		if mcp != "" {
			if boolVal, err := convertUserInputToBool(mcp); err == nil {
				agentConfig["mcp"] = boolVal
			} else {
				agentConfig["mcp"] = false
			}
		} else {
			agentConfig["mcp"] = false // explicit default
		}

		if usage != "" {
			if boolVal, err := convertUserInputToBool(usage); err == nil {
				agentConfig["usage"] = boolVal
			} else {
				agentConfig["usage"] = false
			}
		} else {
			agentConfig["usage"] = false // explicit default
		}

		if markdown != "" {
			if boolVal, err := convertUserInputToBool(markdown); err == nil {
				agentConfig["markdown"] = boolVal
			} else {
				agentConfig["markdown"] = false
			}
		} else {
			agentConfig["markdown"] = false // explicit default
		}

		if think != "" {
			if boolVal, err := convertUserInputToBool(think); err == nil {
				agentConfig["think"] = boolVal
			} else {
				agentConfig["think"] = false
			}
		} else {
			agentConfig["think"] = false // explicit default
		}

		// Set search (empty string if not provided)
		if search != "" {
			agentConfig["search"] = search
		} else {
			agentConfig["search"] = "" // explicit empty default
		}

		// Store template as-is (lazy resolution will happen during switch)
		if template != "" {
			agentConfig["template"] = template
		} else {
			agentConfig["template"] = "" // explicit empty default
		}

		// Store system prompt as-is (lazy resolution will happen during switch)
		if sysPrompt != "" {
			agentConfig["system_prompt"] = sysPrompt
		} else {
			agentConfig["system_prompt"] = "" // explicit empty default
		}

		// Set max_recursions with explicit default
		if maxRecursions > 0 {
			agentConfig["max_recursions"] = maxRecursions
		} else {
			agentConfig["max_recursions"] = 10 // explicit default
		}

		err := service.AddAgentWithConfig(name, agentConfig)
		if err != nil {
			fmt.Printf("Error adding agent: %v\n", err)
			return
		}

		fmt.Printf("Agent '%s' added successfully.\n", name)
		fmt.Println("Use 'gllm agents switch", name, "' to activate it.")

		// Display agent details
		fmt.Println("\nAgent Details:")
		printAgentConfigDetails(agentConfig)
	},
}

var agentSetCmd = &cobra.Command{
	Use:   "set NAME",
	Short: "Update an existing agent configuration",
	Long: `Update an existing agent with detailed configuration settings.
You can also rename the agent using the --name flag.

Examples:
gllm agent set research --model gpt4 --tools off
gllm agent set research --name newresearch --model gpt4`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		newName, _ := cmd.Flags().GetString("name")

		// Get existing agent configuration
		existingConfig, err := service.GetAgent(name)
		if err != nil {
			fmt.Printf("Error getting agent: %v\n", err)
			return
		}

		// Handle renaming if new name is provided
		renamed := false
		if newName != "" && newName != name {
			err := service.RenameAgent(name, newName)
			if err != nil {
				fmt.Printf("Error renaming agent: %v\n", err)
				return
			}
			fmt.Printf("Agent '%s' renamed to '%s'.\n", name, newName)
			// Update name variable to use new name for subsequent operations
			name = newName
			renamed = true
		}

		// Get flag values
		model, _ := cmd.Flags().GetString("model")
		tools, _ := cmd.Flags().GetString("tools")
		mcp, _ := cmd.Flags().GetString("mcp")
		search, _ := cmd.Flags().GetString("search")
		template, _ := cmd.Flags().GetString("template")
		sysPrompt, _ := cmd.Flags().GetString("system")
		usage, _ := cmd.Flags().GetString("usage")
		markdown, _ := cmd.Flags().GetString("markdown")
		think, _ := cmd.Flags().GetString("think")
		maxRecursions, _ := cmd.Flags().GetInt("max-recursions")

		// Start with existing configuration
		agentConfig := make(service.AgentConfig)
		for k, v := range existingConfig {
			agentConfig[k] = v
		}

		// Update fields if flags are provided
		updated := false

		if model != "" {
			// Validate model exists in configuration
			encodedModelName := encodeModelName(model)
			modelsMap := viper.GetStringMap("models")
			if _, exists := modelsMap[encodedModelName]; !exists {
				fmt.Printf("Error: model '%s' is not configured. Please add the model first using 'gllm model add'.\n", model)
				return
			}
			agentConfig["model"] = encodeModelName(model)
			updated = true
		}

		if tools != "" {
			if boolVal, err := convertUserInputToBool(tools); err == nil {
				agentConfig["tools"] = boolVal
			} else {
				agentConfig["tools"] = false
			}
			updated = true
		}
		if mcp != "" {
			if boolVal, err := convertUserInputToBool(mcp); err == nil {
				agentConfig["mcp"] = boolVal
			} else {
				agentConfig["mcp"] = false
			}
			updated = true
		}
		if search != "" {
			agentConfig["search"] = search
			updated = true
		}
		// If search is empty string, we don't add it to the config (meaning no search engine)

		// Store template as-is (lazy resolution will happen during switch)
		if template != "" {
			agentConfig["template"] = template
			updated = true
		}

		// Store system prompt as-is (lazy resolution will happen during switch)
		if sysPrompt != "" {
			agentConfig["system_prompt"] = sysPrompt
			updated = true
		}

		if usage != "" {
			if boolVal, err := convertUserInputToBool(usage); err == nil {
				agentConfig["usage"] = boolVal
			} else {
				agentConfig["usage"] = false
			}
			updated = true
		}
		if markdown != "" {
			if boolVal, err := convertUserInputToBool(markdown); err == nil {
				agentConfig["markdown"] = boolVal
			} else {
				agentConfig["markdown"] = false
			}
			updated = true
		}
		if think != "" {
			if boolVal, err := convertUserInputToBool(think); err == nil {
				agentConfig["think"] = boolVal
			} else {
				agentConfig["think"] = false
			}
			updated = true
		}
		if maxRecursions > 0 {
			agentConfig["max_recursions"] = maxRecursions
			updated = true
		}

		// If only renaming was done, we don't need to update the config
		if renamed && !updated {
			return
		}

		if !updated {
			fmt.Println("No properties to update. Please specify at least one property.")
			return
		}

		err = service.SetAgent(name, agentConfig)
		if err != nil {
			fmt.Printf("Error updating agent: %v\n", err)
			return
		}

		fmt.Printf("Agent '%s' updated successfully.\n", name)

		// Display agent details
		fmt.Println("\nAgent Details:")
		printAgentConfigDetails(agentConfig)
	},
}

var agentRemoveCmd = &cobra.Command{
	Use:     "remove NAME",
	Aliases: []string{"rm", "delete", "del"},
	Short:   "Remove an agent",
	Long:    `Remove an agent configuration. This action cannot be undone.`,
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		// Confirm removal
		fmt.Printf("Are you sure you want to remove agent '%s'? [y/N]: ", name)
		var response string
		fmt.Scanln(&response)
		response = strings.ToLower(strings.TrimSpace(response))
		if response != "y" && response != "yes" {
			fmt.Println("Operation cancelled.")
			return
		}

		err := service.RemoveAgent(name)
		if err != nil {
			fmt.Printf("Error removing agent: %v\n", err)
			return
		}

		fmt.Printf("Agent '%s' removed successfully.\n", name)
	},
}

var agentSwitchCmd = &cobra.Command{
	Use:     "switch NAME",
	Aliases: []string{"select", "sw", "sel"},
	Short:   "Switch to a different agent",
	Long: `Switch to a different agent configuration. This will change your current AI model,
tools, search settings, and other preferences to match the selected agent.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		err := service.SwitchToAgent(name)
		if err != nil {
			fmt.Printf("Error switching to agent: %v\n", err)
			return
		}

		fmt.Printf("Switched to agent '%s'.\n", name)
		fmt.Println("Your current configuration now matches this agent.")
	},
}

var agentInfoCmd = &cobra.Command{
	Use:     "info NAME",
	Aliases: []string{"show", "details"},
	Short:   "Show detailed information about an agent",
	Long:    `Display detailed configuration information for a specific agent.`,
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		agentConfig, err := service.GetAgent(name)
		if err != nil {
			fmt.Printf("Error getting agent info: %v\n", err)
			return
		}

		fmt.Printf("Agent '%s' configuration:\n", name)
		fmt.Println("==========================")

		// Display configuration in a nice format
		keys := make([]string, 0, len(agentConfig))
		for key := range agentConfig {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		for _, key := range keys {
			value := agentConfig[key]
			fmt.Printf("  %s: %v\n", key, value)
		}
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

	// Add flags to the add command
	agentAddCmd.Flags().StringP("model", "m", "", "Model to use (required)")
	agentAddCmd.Flags().StringP("tools", "t", "", "Tools setting (enabled/disabled)")
	agentAddCmd.Flags().StringP("mcp", "", "", "MCP setting (enabled/disabled)")
	agentAddCmd.Flags().StringP("search", "s", "", "Search setting")
	agentAddCmd.Flags().StringP("template", "p", "", "Template to use")
	agentAddCmd.Flags().StringP("system", "S", "", "System prompt to use")
	agentAddCmd.Flags().StringP("usage", "u", "", "Usage setting (on/off)")
	agentAddCmd.Flags().StringP("markdown", "M", "", "Markdown setting (on/off)")
	agentAddCmd.Flags().StringP("think", "T", "", "Think mode (on/off)")
	agentAddCmd.Flags().Int("max-recursions", 0, "Maximum recursions")

	agentAddCmd.MarkFlagRequired("model")

	// Add flags to the set command
	agentSetCmd.Flags().StringP("name", "n", "", "New name for the agent (optional)")
	agentSetCmd.Flags().StringP("model", "m", "", "Model to use")
	agentSetCmd.Flags().StringP("tools", "t", "", "Tools setting (enabled/disabled)")
	agentSetCmd.Flags().StringP("mcp", "", "", "MCP setting (enabled/disabled)")
	agentSetCmd.Flags().StringP("search", "s", "", "Search setting")
	agentSetCmd.Flags().StringP("template", "p", "", "Template to use")
	agentSetCmd.Flags().StringP("system", "S", "", "System prompt to use")
	agentSetCmd.Flags().StringP("usage", "u", "", "Usage setting (on/off)")
	agentSetCmd.Flags().StringP("markdown", "M", "", "Markdown setting (on/off)")
	agentSetCmd.Flags().StringP("think", "T", "", "Think mode (on/off)")
	agentSetCmd.Flags().Int("max-recursions", 0, "Maximum recursions")
}

// printAgentConfigDetails prints the agent details in a formatted way
func printAgentConfigDetails(agent map[string]interface{}) {
	if name, exists := agent["name"]; exists {
		fmt.Printf("  Name: %s\n", name)
	}

	if model, exists := agent["model"]; exists {
		fmt.Printf("  Model: %s\n", decodeModelName(model.(string)))
	}

	if search, exists := agent["search"]; exists {
		fmt.Printf("  Search: %s\n", search)
	} else {
		fmt.Printf("  Search: \n")
	}

	if system, exists := agent["system_prompt"]; exists {
		fmt.Printf("  System Prompt: %s\n", system)
	} else {
		fmt.Printf("  System Prompt: \n")
	}

	if template, exists := agent["template"]; exists {
		fmt.Printf("  Template: %s\n", template)
	} else {
		fmt.Printf("  Template: \n")
	}

	if tools, exists := agent["tools"]; exists {
		fmt.Printf("  Tools: %v\n", tools)
	} else {
		fmt.Printf("  Tools: false\n")
	}

	if mcp, exists := agent["mcp"]; exists {
		fmt.Printf("  MCP: %v\n", mcp)
	} else {
		fmt.Printf("  MCP: false\n")
	}

	if usage, exists := agent["usage"]; exists {
		fmt.Printf("  Usage: %v\n", usage)
	} else {
		fmt.Printf("  Usage: false\n")
	}

	if markdown, exists := agent["markdown"]; exists {
		fmt.Printf("  Markdown: %v\n", markdown)
	} else {
		fmt.Printf("  Markdown: false\n")
	}

	if think, exists := agent["think"]; exists {
		fmt.Printf("  Think: %v\n", think)
	} else {
		fmt.Printf("  Think: false\n")
	}

	if maxRecursions, exists := agent["max_recursions"]; exists {
		fmt.Printf("  Max Recursions: %v\n", maxRecursions)
	} else {
		fmt.Printf("  Max Recursions: 10\n")
	}
}
