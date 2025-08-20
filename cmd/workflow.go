package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fatih/color"
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

	// Add flags to the add command
	workflowAddCmd.Flags().StringP("name", "n", "", "Agent name (required)")
	workflowAddCmd.Flags().StringP("model", "m", "", "Model to use (required)")
	workflowAddCmd.Flags().StringP("tools", "t", "enabled", "Tools setting (enabled/disabled)")
	workflowAddCmd.Flags().StringP("search", "s", "none", "Search setting")
	workflowAddCmd.Flags().StringP("template", "T", "", "Template to use")
	workflowAddCmd.Flags().StringP("system-prompt", "S", "", "System prompt to use")
	workflowAddCmd.Flags().StringP("usage", "u", "off", "Usage setting (on/off)")
	workflowAddCmd.Flags().StringP("markdown", "M", "off", "Markdown setting (on/off)")
	workflowAddCmd.Flags().StringP("role", "r", "", "Role of the agent (master/worker)")
	workflowAddCmd.Flags().StringP("input", "i", "", "Input directory")
	workflowAddCmd.Flags().StringP("output", "o", "", "Output directory")

	workflowAddCmd.MarkFlagRequired("name")
	workflowAddCmd.MarkFlagRequired("model")

	// Add flags to the set command
	workflowSetCmd.Flags().StringP("model", "m", "", "Model to use")
	workflowSetCmd.Flags().StringP("tools", "t", "", "Tools setting (enabled/disabled)")
	workflowSetCmd.Flags().StringP("search", "s", "", "Search setting")
	workflowSetCmd.Flags().StringP("template", "T", "", "Template to use")
	workflowSetCmd.Flags().StringP("system-prompt", "S", "", "System prompt to use")
	workflowSetCmd.Flags().StringP("usage", "u", "", "Usage setting (on/off)")
	workflowSetCmd.Flags().StringP("markdown", "M", "", "Markdown setting (on/off)")
	workflowSetCmd.Flags().StringP("role", "r", "", "Role of the agent (master/worker)")
	workflowSetCmd.Flags().StringP("input", "i", "", "Input directory")
	workflowSetCmd.Flags().StringP("output", "o", "", "Output directory")
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
	Aliases: []string{"ls"},
	Short:   "List all workflow agents",
	Run: func(cmd *cobra.Command, args []string) {
		keyColor := color.New(color.FgMagenta, color.Bold).SprintFunc()

		agents := viper.Get("workflow.agents")
		if agents == nil {
			fmt.Println("No workflow agents defined yet. Use 'gllm workflow add'.")
			return
		}

		agentsSlice, ok := agents.([]interface{})
		if !ok || len(agentsSlice) == 0 {
			fmt.Println("No workflow agents defined yet. Use 'gllm workflow add'.")
			return
		}

		fmt.Println("Workflow agents:")
		for i, agent := range agentsSlice {
			if agentMap, ok := agent.(map[string]interface{}); ok {
				name, _ := agentMap["name"].(string)

				fmt.Printf(" %d. %s\n", i+1, keyColor(name))

				// Print all properties
				properties := []string{"role", "model", "tools", "search", "template", "system_prompt", "usage", "markdown", "input", "output"}
				for _, prop := range properties {
					if val, exists := agentMap[prop]; exists && val != "" {
						displayProp := strings.ReplaceAll(prop, "_", "-")
						fmt.Printf("     %s: %v\n", displayProp, val)
					}
				}
			}
		}
	},
}

var workflowAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new agent to workflow",
	Long: `Adds a new agent to the workflow configuration.
Example:
gllm workflow add --name planner --model groq-oss --tools enabled --template planner --system-prompt planner --role master`,
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		model, _ := cmd.Flags().GetString("model")
		tools, _ := cmd.Flags().GetString("tools")
		search, _ := cmd.Flags().GetString("search")
		template, _ := cmd.Flags().GetString("template")
		sysPrompt, _ := cmd.Flags().GetString("system-prompt")
		usage, _ := cmd.Flags().GetString("usage")
		markdown, _ := cmd.Flags().GetString("markdown")
		role, _ := cmd.Flags().GetString("role")
		input, _ := cmd.Flags().GetString("input")
		output, _ := cmd.Flags().GetString("output")

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
			"name":  name,
			"model": model,
		}

		// Add optional fields if provided
		if tools != "" {
			newAgent["tools"] = tools
		}
		if search != "" {
			newAgent["search"] = search
		}
		if template != "" {
			newAgent["template"] = template
		}
		if sysPrompt != "" {
			newAgent["system_prompt"] = sysPrompt
		}
		if usage != "" {
			newAgent["usage"] = usage
		}
		if markdown != "" {
			newAgent["markdown"] = markdown
		}
		if role != "" {
			newAgent["role"] = role
		}
		if input != "" {
			newAgent["input"] = input
		}
		if output != "" {
			newAgent["output"] = output
		}

		// Add the new agent
		agentsSlice = append(agentsSlice, newAgent)
		viper.Set("workflow.agents", agentsSlice)

		// Write the config file
		if err := writeConfig(); err != nil {
			return fmt.Errorf("failed to save workflow agent: %w", err)
		}

		fmt.Printf("Agent '%s' added successfully to workflow.\n", name)
		return nil
	},
}

var workflowRemoveCmd = &cobra.Command{
	Use:     "remove INDEX|NAME",
	Aliases: []string{"rm"},
	Short:   "Remove an agent from workflow",
	Long: `Removes an agent from workflow by index or name.
	
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

		// Try to parse as index first
		index, err := strconv.Atoi(identifier)
		if err == nil {
			// Index mode (1-based)
			if index < 1 || index > len(agentsSlice) {
				return fmt.Errorf("index out of range. Valid range is 1-%d", len(agentsSlice))
			}

			// Remove by index (convert to 0-based)
			agentName := "unknown"
			if agentMap, ok := agentsSlice[index-1].(map[string]interface{}); ok {
				agentName, _ = agentMap["name"].(string)
			}

			agentsSlice = append(agentsSlice[:index-1], agentsSlice[index:]...)
			viper.Set("workflow.agents", agentsSlice)

			if err := writeConfig(); err != nil {
				return fmt.Errorf("failed to save configuration after removing agent: %w", err)
			}

			fmt.Printf("Agent '%s' (index %d) removed successfully.\n", agentName, index)
			return nil
		}

		// Name mode
		name := identifier
		found := false
		newAgentsSlice := make([]interface{}, 0, len(agentsSlice))

		for _, agent := range agentsSlice {
			if agentMap, ok := agent.(map[string]interface{}); ok {
				if agentName, exists := agentMap["name"]; !exists || agentName != name {
					newAgentsSlice = append(newAgentsSlice, agent)
				} else {
					found = true
				}
			} else {
				newAgentsSlice = append(newAgentsSlice, agent)
			}
		}

		if !found {
			return fmt.Errorf("agent named '%s' not found", name)
		}

		viper.Set("workflow.agents", newAgentsSlice)

		if err := writeConfig(); err != nil {
			return fmt.Errorf("failed to save configuration after removing agent: %w", err)
		}

		fmt.Printf("Agent '%s' removed successfully.\n", name)
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

		if model, err := cmd.Flags().GetString("model"); err == nil && model != "" {
			agentMap["model"] = model
			updated = true
		}

		if tools, err := cmd.Flags().GetString("tools"); err == nil && tools != "" {
			agentMap["tools"] = tools
			updated = true
		}

		if search, err := cmd.Flags().GetString("search"); err == nil && search != "" {
			agentMap["search"] = search
			updated = true
		}

		if template, err := cmd.Flags().GetString("template"); err == nil && template != "" {
			agentMap["template"] = template
			updated = true
		}

		if sysPrompt, err := cmd.Flags().GetString("system-prompt"); err == nil && sysPrompt != "" {
			agentMap["system_prompt"] = sysPrompt
			updated = true
		}

		if usage, err := cmd.Flags().GetString("usage"); err == nil && usage != "" {
			agentMap["usage"] = usage
			updated = true
		}

		if markdown, err := cmd.Flags().GetString("markdown"); err == nil && markdown != "" {
			agentMap["markdown"] = markdown
			updated = true
		}

		if role, err := cmd.Flags().GetString("role"); err == nil && role != "" {
			agentMap["role"] = role
			updated = true
		}

		if input, err := cmd.Flags().GetString("input"); err == nil && input != "" {
			agentMap["input"] = input
			updated = true
		}

		if output, err := cmd.Flags().GetString("output"); err == nil && output != "" {
			agentMap["output"] = output
			updated = true
		}

		if !updated {
			return fmt.Errorf("no properties to update. Please specify at least one property")
		}

		// Update the agent in the slice
		agentsSlice[agentIndex] = agentMap
		viper.Set("workflow.agents", agentsSlice)

		// Write the config file
		if err := writeConfig(); err != nil {
			return fmt.Errorf("failed to save agent: %w", err)
		}

		fmt.Printf("Agent '%s' updated successfully.\n", agentMap["name"])
		return nil
	},
}

var workflowMoveCmd = &cobra.Command{
	Use:     "move FROM TO",
	Aliases: []string{"mv"},
	Short:   "Move an agent to a different position",
	Long: `Moves an agent from one position to another in the workflow.
	
Example:
gllm workflow move 1 3
gllm workflow move planner worker`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		fromIdentifier := args[0]
		toIdentifier := args[1]

		// Get existing agents
		agents := viper.Get("workflow.agents")
		if agents == nil {
			return fmt.Errorf("no workflow agents defined")
		}

		agentsSlice, ok := agents.([]interface{})
		if !ok || len(agentsSlice) == 0 {
			return fmt.Errorf("no workflow agents defined")
		}

		// Find fromIndex
		fromIndex := -1

		// Try to parse fromIdentifier as index first
		if index, err := strconv.Atoi(fromIdentifier); err == nil {
			// Index mode (1-based)
			if index >= 1 && index <= len(agentsSlice) {
				fromIndex = index - 1
			}
		} else {
			// Name mode
			name := fromIdentifier
			for i, agent := range agentsSlice {
				if agentMap, ok := agent.(map[string]interface{}); ok {
					if agentName, exists := agentMap["name"]; exists && agentName == name {
						fromIndex = i
						break
					}
				}
			}
		}

		if fromIndex == -1 {
			return fmt.Errorf("source agent '%s' not found", fromIdentifier)
		}

		// Find toIndex
		toIndex := -1

		// Try to parse toIdentifier as index first
		if index, err := strconv.Atoi(toIdentifier); err == nil {
			// Index mode (1-based)
			if index >= 1 && index <= len(agentsSlice) {
				toIndex = index - 1
			}
		} else {
			// Name mode
			name := toIdentifier
			for i, agent := range agentsSlice {
				if agentMap, ok := agent.(map[string]interface{}); ok {
					if agentName, exists := agentMap["name"]; exists && agentName == name {
						toIndex = i
						break
					}
				}
			}
		}

		if toIndex == -1 {
			return fmt.Errorf("target agent '%s' not found", toIdentifier)
		}

		// Move agent
		if fromIndex == toIndex {
			return fmt.Errorf("source and target positions are the same")
		}

		// Get the agent to move
		agentToMove := agentsSlice[fromIndex]

		// Remove from original position
		agentsSlice = append(agentsSlice[:fromIndex], agentsSlice[fromIndex+1:]...)

		// Insert at new position
		if toIndex > fromIndex {
			toIndex-- // Adjust for removed element
		}

		// Extend slice if needed
		agentsSlice = append(agentsSlice, nil)

		// Shift elements to make space
		copy(agentsSlice[toIndex+1:], agentsSlice[toIndex:])

		// Insert moved agent
		agentsSlice[toIndex] = agentToMove

		viper.Set("workflow.agents", agentsSlice)

		// Write the config file
		if err := writeConfig(); err != nil {
			return fmt.Errorf("failed to save workflow configuration: %w", err)
		}

		fromName := "unknown"
		if agentMap, ok := agentToMove.(map[string]interface{}); ok {
			fromName, _ = agentMap["name"].(string)
		}

		fmt.Printf("Agent '%s' moved successfully from position %d to %d.\n", fromName, fromIndex+1, toIndex+1)
		return nil
	},
}
