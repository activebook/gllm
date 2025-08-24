package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/activebook/gllm/service"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(workflowCmd)
	workflowCmd.AddCommand(workflowListCmd)
	workflowCmd.AddCommand(workflowAddCmd)
	workflowCmd.AddCommand(workflowRemoveCmd)
	workflowCmd.AddCommand(workflowSetCmd)
	workflowCmd.AddCommand(workflowMoveCmd)
	workflowCmd.AddCommand(workflowInfoCmd)
	workflowCmd.AddCommand(workflowStartCmd)

	// Add flags to the add command
	workflowAddCmd.Flags().StringP("name", "n", "", "Agent name (required)")
	workflowAddCmd.Flags().StringP("model", "m", "", "Model to use (required)")
	workflowAddCmd.Flags().StringP("tools", "t", "", "Tools setting (enabled/disabled)")
	workflowAddCmd.Flags().StringP("search", "s", "", "Search setting")
	workflowAddCmd.Flags().StringP("template", "p", "", "Template to use")
	workflowAddCmd.Flags().StringP("system", "S", "", "System prompt to use")
	workflowAddCmd.Flags().StringP("usage", "u", "", "Usage setting (on/off)")
	workflowAddCmd.Flags().StringP("markdown", "M", "", "Markdown setting (on/off)")
	workflowAddCmd.Flags().StringP("role", "r", "master", "Role of the agent (master/worker)")
	workflowAddCmd.Flags().StringP("input", "i", "", "Input directory")
	workflowAddCmd.Flags().StringP("output", "o", "", "Output directory")
	workflowAddCmd.Flags().StringP("think", "T", "", "Think mode (on/off)")

	workflowAddCmd.MarkFlagRequired("name")
	workflowAddCmd.MarkFlagRequired("model")

	// Add flags to the set command
	workflowSetCmd.Flags().StringP("name", "n", "", "Agent name")
	workflowSetCmd.Flags().StringP("model", "m", "", "Model to use")
	workflowSetCmd.Flags().StringP("tools", "t", "", "Tools setting (enabled/disabled)")
	workflowSetCmd.Flags().StringP("search", "s", "", "Search setting")
	workflowSetCmd.Flags().StringP("template", "p", "", "Template to use")
	workflowSetCmd.Flags().StringP("system", "S", "", "System prompt to use")
	workflowSetCmd.Flags().StringP("usage", "u", "", "Usage setting (on/off)")
	workflowSetCmd.Flags().StringP("markdown", "M", "", "Markdown setting (on/off)")
	workflowSetCmd.Flags().StringP("role", "r", "", "Role of the agent (master/worker)")
	workflowSetCmd.Flags().StringP("input", "i", "", "Input directory")
	workflowSetCmd.Flags().StringP("output", "o", "", "Output directory")
	workflowSetCmd.Flags().StringP("think", "T", "", "Think mode (on/off)")
}

// configCmd represents the base command when called without any subcommands
var workflowCmd = &cobra.Command{
	Use:     "workflow",
	Aliases: []string{"wf"}, // Optional alias
	Short:   "Manage gllm workflow configuration",
	Long:    `The 'gllm workflow' command allows you to manage your configured agent workflows.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// If no subcommand is given, show help
		if len(args) == 0 {
			return cmd.Help()
		}
		return cmd.Help()
	},
}

var workflowListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all agents in the workflow",
	Aliases: []string{"ls", "pr"},
	Long:    `List all agents in the current workflow configuration with their properties.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		agents := viper.Get("workflow.agents")
		if agents == nil {
			fmt.Println("No agents defined in workflow.")
			return nil
		}

		agentsSlice, ok := agents.([]interface{})
		if !ok || len(agentsSlice) == 0 {
			fmt.Println("No agents defined in workflow.")
			return nil
		}

		fmt.Printf("Workflow Agents (%d):\n", len(agentsSlice))
		fmt.Println("==================")
		for i, agent := range agentsSlice {
			if agentMap, ok := agent.(map[string]interface{}); ok {
				fmt.Printf("%d. \n", i+1)
				printAgentDetails(agentMap)
				fmt.Println()
			}
		}

		return nil
	},
}

// validateWorkflow checks if the workflow configuration follows all the required rules
func validateWorkflow(agentsSlice []interface{}) error {
	if len(agentsSlice) == 0 {
		return fmt.Errorf("workflow must have at least one agent")
	}

	// Check uniqueness of names
	names := make(map[string]bool)
	for _, agent := range agentsSlice {
		if agentMap, ok := agent.(map[string]interface{}); ok {
			if name, exists := agentMap["name"]; exists {
				nameStr := name.(string)
				if names[nameStr] {
					return fmt.Errorf("duplicate agent name found: %s", nameStr)
				}
				names[nameStr] = true
			}
		}
	}

	// Get the first agent
	firstAgent, ok := agentsSlice[0].(map[string]interface{})
	if !ok {
		return fmt.Errorf("first agent has invalid format")
	}

	// Check: the first agent must be master
	role := string(service.WorkflowAgentTypeMaster) // default role
	if roleVal, exists := firstAgent["role"]; exists {
		if roleStr, ok := roleVal.(string); ok && roleStr != "" {
			role = roleStr
		}
	}

	if role != string(service.WorkflowAgentTypeMaster) {
		return fmt.Errorf("the first agent must be %s", service.WorkflowAgentTypeMaster)
	}

	// Check: the first agent must have output (input is optional)
	if outputVal, exists := firstAgent["output"]; !exists || outputVal == "" {
		return fmt.Errorf("the first agent (%s) must have output directory", service.WorkflowAgentTypeMaster)
	}

	// Validate all agents
	for i, agentItem := range agentsSlice {
		agent, ok := agentItem.(map[string]interface{})
		if !ok {
			return fmt.Errorf("agent at index %d has invalid format", i)
		}

		// Check: every agent must have a role
		agentRole := string(service.WorkflowAgentTypeMaster) // default role
		if roleVal, exists := agent["role"]; exists {
			if roleStr, ok := roleVal.(string); ok && roleStr != "" {
				agentRole = roleStr
			}
		}

		if agentRole != string(service.WorkflowAgentTypeMaster) && agentRole != string(service.WorkflowAgentTypeWorker) {
			return fmt.Errorf("agent '%s' has invalid role '%s'. Role must be either '%s' or '%s'", agent["name"], agentRole, service.WorkflowAgentTypeMaster, service.WorkflowAgentTypeWorker)
		}

		// Check: every agent must have a model
		if model, exists := agent["model"]; !exists || model == "" {
			return fmt.Errorf("agent '%s' must have a model", agent["name"])
		} else {
			// Validate model exists in configuration
			modelName := model.(string)
			modelsMap := viper.GetStringMap("models")
			if _, exists := modelsMap[modelName]; !exists {
				return fmt.Errorf("agent '%s' references model '%s' which is not configured. Please add the model first using 'gllm model add'", agent["name"], modelName)
			}
		}

		// Check search setting if specified (field exists in config)
		if search, exists := agent["search"]; exists {
			// If search field exists, it must be non-empty and valid
			if searchStr, ok := search.(string); ok && searchStr != "" {
				if searchStr != "google" && searchStr != "tavily" && searchStr != "bing" {
					return fmt.Errorf("agent '%s' has invalid search setting '%s'. Valid options are: google, tavily, bing", agent["name"], searchStr)
				}

				// Validate search engine is configured
				key := viper.GetString(fmt.Sprintf("search_engines.%s.key", searchStr))
				if key == "" {
					return fmt.Errorf("agent '%s' references search engine '%s' which is not configured. Please configure it first using 'gllm search %s --key YOUR_KEY'", agent["name"], searchStr, searchStr)
				}
			}
			// If search field exists but is empty string, that's valid (means no search engine)
		}
		// If search field doesn't exist in config, that's also valid (means no search engine)

		// Check: everyone except the first must have input and output
		if i > 0 {
			// Input field must exist and be non-empty
			if inputVal, hasInput := agent["input"]; !hasInput || inputVal == "" {
				return fmt.Errorf("agent '%s' must have input directory", agent["name"])
			}

			// Output field must exist and be non-empty
			if outputVal, hasOutput := agent["output"]; !hasOutput || outputVal == "" {
				return fmt.Errorf("agent '%s' must have output directory", agent["name"])
			}
		}
	}

	return nil
}

// convertUserInputToBool converts user-friendly strings to boolean values
// Handles: on/off, enable/disable, true/false, 1/0
func convertUserInputToBool(input string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "on", "enable", "enabled", "true", "1":
		return true, nil
	case "off", "disable", "disabled", "false", "0", "":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value: %s", input)
	}
}

var workflowAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new agent to workflow",
	Long: `Adds a new agent to the workflow configuration.
Example:
gllm workflow add --name planner --model groq-oss --tools enabled --template planner --system planner --role master`,
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		model, _ := cmd.Flags().GetString("model")
		tools, _ := cmd.Flags().GetString("tools")
		search, _ := cmd.Flags().GetString("search")
		template, _ := cmd.Flags().GetString("template")
		sysPrompt, _ := cmd.Flags().GetString("system")
		usage, _ := cmd.Flags().GetString("usage")
		markdown, _ := cmd.Flags().GetString("markdown")
		role, _ := cmd.Flags().GetString("role")
		input, _ := cmd.Flags().GetString("input")
		output, _ := cmd.Flags().GetString("output")
		think, _ := cmd.Flags().GetString("think")

		// Validate required fields
		if output == "" {
			return fmt.Errorf("output directory cannot be empty")
		}

		// Get existing agents slice
		agents := viper.Get("workflow.agents")
		var agentsSlice []interface{}

		if agents != nil {
			if existingSlice, ok := agents.([]interface{}); ok {
				agentsSlice = existingSlice
			}
		}

		// Check if agent with same name already exists
		for _, agent := range agentsSlice {
			if agentMap, ok := agent.(map[string]interface{}); ok {
				if agentName, exists := agentMap["name"]; exists && agentName == name {
					return fmt.Errorf("agent named '%s' already exists. Use a different name or remove the existing agent first", name)
				}
			}
		}

		// Create new agent config
		newAgent := map[string]interface{}{
			"name":   name,
			"model":  model,
			"output": output, // output is required and validated above
		}

		// Add optional fields if provided
		if tools != "" {
			if boolVal, err := convertUserInputToBool(tools); err == nil {
				newAgent["tools"] = boolVal
			} else {
				newAgent["tools"] = tools
			}
		}
		if search != "" {
			newAgent["search"] = search
		}
		// If search is empty string, we don't add it to the config (meaning no search)

		if template != "" {
			newAgent["template"] = template
		}
		if sysPrompt != "" {
			newAgent["system"] = sysPrompt
		}
		if usage != "" {
			if boolVal, err := convertUserInputToBool(usage); err == nil {
				newAgent["usage"] = boolVal
			} else {
				newAgent["usage"] = usage
			}
		}
		if markdown != "" {
			if boolVal, err := convertUserInputToBool(markdown); err == nil {
				newAgent["markdown"] = boolVal
			} else {
				newAgent["markdown"] = markdown
			}
		}
		if role != "" {
			if role == "" {
				return fmt.Errorf("role cannot be empty")
			}
			newAgent["role"] = role
		} else {
			newAgent["role"] = string(service.WorkflowAgentTypeMaster) // default role
		}
		if input != "" {
			newAgent["input"] = input
		}
		// Always add input field even if empty to satisfy validation requirements
		if _, exists := newAgent["input"]; !exists {
			newAgent["input"] = ""
		}

		if output != "" {
			newAgent["output"] = output
		}
		if think != "" {
			if boolVal, err := convertUserInputToBool(think); err == nil {
				newAgent["think"] = boolVal
			} else {
				newAgent["think"] = think
			}
		}

		// Add the new agent
		agentsSlice = append(agentsSlice, newAgent)

		// Validate workflow configuration
		if err := validateWorkflow(agentsSlice); err != nil {
			return fmt.Errorf("invalid workflow configuration: %w", err)
		}

		viper.Set("workflow.agents", agentsSlice)

		// Write the config file
		if err := writeConfig(); err != nil {
			return fmt.Errorf("failed to save workflow agent: %w", err)
		}

		fmt.Printf("Agent '%s' added successfully to workflow.\n", name)

		// Print agent details
		fmt.Println("\nAgent Details:")
		printAgentDetails(newAgent)

		return nil
	},
}

// printAgentDetails prints the agent details in a formatted way
func printAgentDetails(agent map[string]interface{}) {
	if name, exists := agent["name"]; exists {
		fmt.Printf("  Name: \033[95m%s\033[0m\n", name)
	}

	if role, exists := agent["role"]; exists {
		roleStr := role.(string)
		// Color coding for roles
		if roleStr == string(service.WorkflowAgentTypeMaster) {
			// Green for master role
			fmt.Printf("  Role: \033[32m%s\033[0m\n", role)
		} else if roleStr == string(service.WorkflowAgentTypeWorker) {
			// Blue for worker role
			fmt.Printf("  Role: \033[34m%s\033[0m\n", role)
		} else {
			// Default coloring for other roles
			fmt.Printf("  Role: %s\n", role)
		}
	} else {
		// Default role is master, shown in green
		fmt.Printf("  Role: \033[32m%s\033[0m (default)\n", service.WorkflowAgentTypeMaster)
	}

	if model, exists := agent["model"]; exists {
		fmt.Printf("  Model: %s\n", model)
	}

	if input, exists := agent["input"]; exists {
		if inputStr, ok := input.(string); ok {
			fmt.Printf("  Input: %s\n", inputStr)
		} else {
			fmt.Printf("  Input: %s\n", input)
		}
	} else {
		fmt.Printf("  Input: \n")
	}

	if output, exists := agent["output"]; exists {
		fmt.Printf("  Output: %s\n", output)
	}

	if template, exists := agent["template"]; exists {
		fmt.Printf("  Template: %s\n", template)
	} else {
		fmt.Printf("  Template: \n")
	}

	if system, exists := agent["system"]; exists {
		fmt.Printf("  System Prompt: %s\n", system)
	} else {
		fmt.Printf("  System Prompt: \n")
	}

	if search, exists := agent["search"]; exists {
		fmt.Printf("  Search: %s\n", search)
	} else {
		fmt.Printf("  Search: \n")
	}

	if tools, exists := agent["tools"]; exists {
		fmt.Printf("  Tools: %v\n", tools)
	} else {
		fmt.Printf("  Tools: false\n")
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
}

var workflowRemoveCmd = &cobra.Command{
	Use:     "remove INDEX|NAME",
	Short:   "Remove an agent from workflow",
	Aliases: []string{"rm"},
	Long: `Remove a workflow agent by index (1-based) or name.
Example:
gllm workflow remove 1
gllm workflow remove planner`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		identifier := args[0]

		// Get existing agents
		agents := viper.Get("workflow.agents")
		if agents == nil {
			return fmt.Errorf("no workflow agents defined")
		}

		agentsSlice, ok := agents.([]interface{})
		if !ok || len(agentsSlice) == 0 {
			return fmt.Errorf("no workflow agents defined")
		}

		// Find agent by index or name
		var agentIndex int
		found := false

		// Try to parse as index first
		index, err := strconv.Atoi(identifier)
		if err == nil {
			// Index mode (1-based)
			if index < 1 || index > len(agentsSlice) {
				return fmt.Errorf("index out of range. Valid range is 1-%d", len(agentsSlice))
			}

			agentIndex = index - 1
			found = true
		} else {
			// Name mode
			name := identifier
			for i, agent := range agentsSlice {
				if agentMap, ok := agent.(map[string]interface{}); ok {
					if agentName, exists := agentMap["name"]; exists && agentName == name {
						agentIndex = i
						found = true
						break
					}
				}
			}
		}

		if !found {
			return fmt.Errorf("agent '%s' not found", identifier)
		}

		// Get the agent name for the success message
		agentName := "unknown"
		if agentMap, ok := agentsSlice[agentIndex].(map[string]interface{}); ok {
			if name, exists := agentMap["name"]; exists {
				agentName = name.(string)
			}
		}

		// Remove the agent
		agentsSlice = append(agentsSlice[:agentIndex], agentsSlice[agentIndex+1:]...)
		viper.Set("workflow.agents", agentsSlice)

		// Write the config file
		if err := writeConfig(); err != nil {
			return fmt.Errorf("failed to save workflow configuration: %w", err)
		}

		fmt.Printf("Agent '%s' removed successfully from workflow.\n", agentName)
		return nil
	},
}

var workflowSetCmd = &cobra.Command{
	Use:   "set INDEX|NAME",
	Short: "Set properties of an existing agent",
	Long: `Sets properties of an existing workflow agent by index or name.
Example:
gllm workflow set 1 --model gemini-pro --template worker
gllm workflow set planner --model groq-oss --role master`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		identifier := args[0]

		// Get existing agents
		agents := viper.Get("workflow.agents")
		if agents == nil {
			return fmt.Errorf("no workflow agents defined")
		}

		agentsSlice, ok := agents.([]interface{})
		if !ok || len(agentsSlice) == 0 {
			return fmt.Errorf("no workflow agents defined")
		}

		// Find agent by index or name
		var agentIndex int
		var agentMap map[string]interface{}
		found := false

		// Try to parse as index first
		index, err := strconv.Atoi(identifier)
		if err == nil {
			// Index mode (1-based)
			if index < 1 || index > len(agentsSlice) {
				return fmt.Errorf("index out of range. Valid range is 1-%d", len(agentsSlice))
			}

			if agent, ok := agentsSlice[index-1].(map[string]interface{}); ok {
				agentIndex = index - 1
				agentMap = agent
				found = true
			} else {
				return fmt.Errorf("agent at index %d has invalid format", index)
			}
		} else {
			// Name mode
			name := identifier
			for i, agent := range agentsSlice {
				if am, ok := agent.(map[string]interface{}); ok {
					if agentName, exists := am["name"]; exists && agentName == name {
						agentIndex = i
						agentMap = am
						found = true
						break
					}
				}
			}
		}

		if !found {
			return fmt.Errorf("agent '%s' not found", identifier)
		}

		// Update fields if flags are provided
		updated := false

		// Handle name change first to ensure uniqueness validation works correctly
		if name, err := cmd.Flags().GetString("name"); err == nil && name != "" {
			// Check if another agent already has this name
			for i, agent := range agentsSlice {
				if i == agentIndex { // Skip the current agent
					continue
				}
				if agentMap, ok := agent.(map[string]interface{}); ok {
					if agentName, exists := agentMap["name"]; exists && agentName == name {
						return fmt.Errorf("agent named '%s' already exists. Please choose a different name", name)
					}
				}
			}
			agentMap["name"] = name
			updated = true
		}

		if model, err := cmd.Flags().GetString("model"); err == nil && model != "" {
			agentMap["model"] = model
			updated = true
		}

		if tools, err := cmd.Flags().GetString("tools"); err == nil && tools != "" {
			if boolVal, err := convertUserInputToBool(tools); err == nil {
				agentMap["tools"] = boolVal
			} else {
				agentMap["tools"] = tools
			}
			updated = true
		}

		if search, err := cmd.Flags().GetString("search"); err == nil {
			if search != "" {
				agentMap["search"] = search
			} else {
				// If search is explicitly set to empty string, remove it from config (meaning no search)
				delete(agentMap, "search")
			}
			updated = true
		}

		if template, err := cmd.Flags().GetString("template"); err == nil && template != "" {
			agentMap["template"] = template
			updated = true
		}

		if sysPrompt, err := cmd.Flags().GetString("system"); err == nil && sysPrompt != "" {
			agentMap["system"] = sysPrompt
			updated = true
		}

		if usage, err := cmd.Flags().GetString("usage"); err == nil && usage != "" {
			if boolVal, err := convertUserInputToBool(usage); err == nil {
				agentMap["usage"] = boolVal
			} else {
				agentMap["usage"] = usage
			}
			updated = true
		}

		if markdown, err := cmd.Flags().GetString("markdown"); err == nil && markdown != "" {
			if boolVal, err := convertUserInputToBool(markdown); err == nil {
				agentMap["markdown"] = boolVal
			} else {
				agentMap["markdown"] = markdown
			}
			updated = true
		}

		// Only update role if the flag was explicitly provided
		if cmd.Flags().Changed("role") {
			role, _ := cmd.Flags().GetString("role")
			if role != "" {
				agentMap["role"] = role
			} else {
				// Don't allow setting role to empty - that would be invalid
				return fmt.Errorf("role cannot be set to empty string")
			}
			updated = true
		}

		// Only update input if the flag was explicitly provided
		if cmd.Flags().Changed("input") {
			input, _ := cmd.Flags().GetString("input")
			// Always update input field even if empty (but only when explicitly provided)
			agentMap["input"] = input
			updated = true
		}

		// Only update output if the flag was explicitly provided
		if cmd.Flags().Changed("output") {
			output, _ := cmd.Flags().GetString("output")
			if output != "" {
				agentMap["output"] = output
			} else {
				// Don't allow setting output to empty - that would be invalid
				return fmt.Errorf("output directory cannot be set to empty string")
			}
			updated = true
		}

		if think, err := cmd.Flags().GetString("think"); err == nil && think != "" {
			if boolVal, err := convertUserInputToBool(think); err == nil {
				agentMap["think"] = boolVal
			} else {
				agentMap["think"] = think
			}
			updated = true
		}

		if !updated {
			return fmt.Errorf("no properties to update. Please specify at least one property")
		}

		// Update the agent in the slice
		agentsSlice[agentIndex] = agentMap

		// Validate workflow configuration
		if err := validateWorkflow(agentsSlice); err != nil {
			return fmt.Errorf("invalid workflow configuration: %w", err)
		}

		viper.Set("workflow.agents", agentsSlice)

		// Write the config file
		if err := writeConfig(); err != nil {
			return fmt.Errorf("failed to save agent: %w", err)
		}

		fmt.Printf("Agent '%s' updated successfully.\n", agentMap["name"])

		// Print agent details
		fmt.Println("\nAgent Details:")
		printAgentDetails(agentMap)

		return nil
	},
}

var workflowMoveCmd = &cobra.Command{
	Use:     "move FROM TO",
	Short:   "Move an agent from one position to another",
	Aliases: []string{"mv"},
	Long: `Move a workflow agent from one position to another by index (1-based).
Example:
gllm workflow move 1 3`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		fromStr := args[0]
		toStr := args[1]

		// Parse indices
		from, err := strconv.Atoi(fromStr)
		if err != nil {
			return fmt.Errorf("invalid from index '%s': %w", fromStr, err)
		}

		to, err := strconv.Atoi(toStr)
		if err != nil {
			return fmt.Errorf("invalid to index '%s': %w", toStr, err)
		}

		// Get existing agents
		agents := viper.Get("workflow.agents")
		if agents == nil {
			return fmt.Errorf("no workflow agents defined")
		}

		agentsSlice, ok := agents.([]interface{})
		if !ok || len(agentsSlice) == 0 {
			return fmt.Errorf("no workflow agents defined")
		}

		// Convert to 0-based indices
		fromIndex := from - 1
		toIndex := to - 1

		// Validate indices
		if fromIndex < 0 || fromIndex >= len(agentsSlice) {
			return fmt.Errorf("from index %d out of range. Valid range is 1-%d", from, len(agentsSlice))
		}

		if toIndex < 0 || toIndex >= len(agentsSlice) {
			return fmt.Errorf("to index %d out of range. Valid range is 1-%d", to, len(agentsSlice))
		}

		// Get the agent being moved for the success message
		agentMap, ok := agentsSlice[fromIndex].(map[string]interface{})
		if !ok {
			return fmt.Errorf("agent at index %d has invalid format", from)
		}
		agentName := "unknown"
		if name, exists := agentMap["name"]; exists {
			agentName = name.(string)
		}

		// Move the agent
		agent := agentsSlice[fromIndex]
		// Remove from original position
		agentsSlice = append(agentsSlice[:fromIndex], agentsSlice[fromIndex+1:]...)
		// Insert at new position
		agentsSlice = append(agentsSlice[:toIndex], append([]interface{}{agent}, agentsSlice[toIndex:]...)...)

		// Validate workflow configuration
		if err := validateWorkflow(agentsSlice); err != nil {
			return fmt.Errorf("invalid workflow configuration after move: %w", err)
		}

		viper.Set("workflow.agents", agentsSlice)

		// Write the config file
		if err := writeConfig(); err != nil {
			return fmt.Errorf("failed to save workflow configuration: %w", err)
		}

		fmt.Printf("Agent '%s' moved successfully from position %d to %d.\n", agentName, from, to)
		return nil
	},
}

var workflowInfoCmd = &cobra.Command{
	Use:     "info NAME",
	Short:   "Display details of a specific agent",
	Aliases: []string{"in"},
	Long:    `Display detailed configuration information for a specific workflow agent by name.`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentName := args[0]

		// Get existing agents
		agents := viper.Get("workflow.agents")
		if agents == nil {
			return fmt.Errorf("no workflow agents defined")
		}

		agentsSlice, ok := agents.([]interface{})
		if !ok || len(agentsSlice) == 0 {
			return fmt.Errorf("no workflow agents defined")
		}

		// Find agent by name
		var agentMap map[string]interface{}
		found := false

		for _, agent := range agentsSlice {
			if am, ok := agent.(map[string]interface{}); ok {
				if name, exists := am["name"]; exists && name == agentName {
					agentMap = am
					found = true
					break
				}
			}
		}

		if !found {
			return fmt.Errorf("agent '%s' not found", agentName)
		}

		// Print agent details
		fmt.Printf("Agent Details for '%s':\n", agentName)
		fmt.Println("========================")
		printAgentDetails(agentMap)

		return nil
	},
}

var workflowStartCmd = &cobra.Command{
	Use:   "start [prompt]",
	Short: "Start executing the workflow with an optional prompt",
	Long: `Start executing the configured workflow with an optional prompt.
The prompt will be passed to the first agent in the workflow.
Example:
gllm workflow start "What are the latest advancements in AI?"
gllm workflow start`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get existing agents
		agents := viper.Get("workflow.agents")
		if agents == nil {
			return fmt.Errorf("no workflow agents defined")
		}

		agentsSlice, ok := agents.([]interface{})
		if !ok || len(agentsSlice) == 0 {
			return fmt.Errorf("no workflow agents defined")
		}

		// Validate workflow configuration
		if err := validateWorkflow(agentsSlice); err != nil {
			return fmt.Errorf("invalid workflow configuration: %w", err)
		}

		// Convert agents configuration to service.WorkflowConfig
		workflowConfig := &service.WorkflowConfig{}

		for _, agentItem := range agentsSlice {
			agent, ok := agentItem.(map[string]interface{})
			if !ok {
				return fmt.Errorf("agent has invalid format")
			}

			workflowAgent := service.WorkflowAgent{
				Name:         getStringValue(agent, "name"),
				Role:         service.WorkflowAgentType(getStringValue(agent, "role")),
				Template:     getStringValue(agent, "template"),
				SystemPrompt: getStringValue(agent, "system"),
				Tools:        getBoolValue(agent, "tools"),
				Think:        getBoolValue(agent, "think"),
				Usage:        getBoolValue(agent, "usage"),
				Markdown:     getBoolValue(agent, "markdown"),
				InputDir:     getStringValue(agent, "input"),
				OutputDir:    getStringValue(agent, "output"),
			}

			// Handle model configuration
			modelName := getStringValue(agent, "model")
			if modelName != "" {
				modelConfig := GetModelInfo(modelName)
				if len(modelConfig) > 0 {
					workflowAgent.Model = &modelConfig
				} else {
					return fmt.Errorf("model '%s' referenced by agent '%s' is not configured", modelName, workflowAgent.Name)
				}
			}

			// Handle search configuration
			searchEngine := getStringValue(agent, "search")
			if searchEngine != "" {
				searchConfig := GetSearchEngineInfo(searchEngine)
				if len(searchConfig) > 0 {
					workflowAgent.Search = &searchConfig
				} else {
					return fmt.Errorf("error getting search engine info for '%s'", searchEngine)
				}
			}

			workflowConfig.Agents = append(workflowConfig.Agents, workflowAgent)
		}

		// Get prompt if provided
		prompt := ""
		if len(args) > 0 {
			prompt = args[0]
		}

		// Run the workflow
		if err := service.RunWorkflow(workflowConfig, prompt); err != nil {
			return fmt.Errorf("failed to run workflow: %w", err)
		}

		return nil
	},
}

// Helper function to get string value from map with default
func getStringValue(m map[string]interface{}, key string) string {
	if val, exists := m[key]; exists {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// Helper function to get bool value from map with default
func getBoolValue(m map[string]interface{}, key string) bool {
	if val, exists := m[key]; exists {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}
