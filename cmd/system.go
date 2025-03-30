// File: cmd/config.go
package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	plainSystemPrompt string
)

func SetPlainSystemPrompt(prompt string) {
	plainSystemPrompt = prompt
}

// systemCmd represents the base command when called without any subcommands
var systemCmd = &cobra.Command{
	Use:     "system",
	Aliases: []string{"sys", "system_prompt"}, // Optional alias
	Short:   "Manage gllm system prompt configuration",
	Long:    `Define, view, list, or delete reusable system prompts.`,
	// Run: func(cmd *cobra.Command, args []string) {
	//  fmt.Println("Use 'gllm config [subcommand] --help' for more information.")
	// },
	// Suggest showing help if 'gllm config' is run without subcommand
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Manage system prompts here (e.g., add, list, remove)")
		// Example: Access a viper setting for prompts (maybe a map or slice)
		// prompts := viper.Get("prompts")
		// fmt.Printf("Current prompts (from config): %v\n", prompts)
		cmd.Help() // Show help for now
	},
}

var systemListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all saved system prompt names",
	Run: func(cmd *cobra.Command, args []string) {
		sys_prompts := viper.GetStringMapString("system_prompts")
		defaultSysPrompt := viper.GetString("default.system_prompt")

		if len(sys_prompts) == 0 {
			fmt.Println("No system prompts defined yet. Use 'gllm system add'.")
			return
		}

		fmt.Println("Available system prompts:")
		// Sort keys for consistent output
		names := make([]string, 0, len(sys_prompts))
		for name := range sys_prompts {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			indicator := " "
			if name == defaultSysPrompt {
				indicator = "*" // Mark the default prompt
			}
			fmt.Printf(" %s %s\n", indicator, name)
		}
		if defaultSysPrompt != "" {
			fmt.Println("\n(*) Indicates the default system prompt.")
		} else {
			fmt.Println("\nNo default system prompt set.")
		}
	},
}

var systemAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new named system prompt",
	Long: `Adds a new system prompt with a specific name and content.
Example:
  gllm system add --name coder --content "You are an expert Go programmer"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		content, _ := cmd.Flags().GetString("content")

		sys_prompts := viper.GetStringMapString("system_prompts")
		// Initialize map if it doesn't exist
		if sys_prompts == nil {
			sys_prompts = make(map[string]string)
		}

		if _, exists := sys_prompts[name]; exists {
			// Maybe add an --overwrite flag later? For now, error out.
			return fmt.Errorf("system prompt named '%s' already exists. Use 'remove' first or choose a different name", name)
		}

		sys_prompts[name] = content
		viper.Set("system_prompts", sys_prompts)

		// Write the config file
		if err := writeConfig(); err != nil {
			return fmt.Errorf("failed to save system prompt: %w", err)
		}

		fmt.Printf("System prompt '%s' added successfully.\n", name)
		return nil
	},
}

var systemSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set a named system prompt",
	Long: `Sets a new system prompt with a specific name and content.
Example:
  gllm system set coder --content "You are an expert Go programmer"`,
	Args: cobra.ExactArgs(1), // Requires exactly one argument (the name)
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		sys_prompts := viper.GetStringMapString("system_prompts")
		// Initialize map if it doesn't exist
		if sys_prompts == nil {
			return fmt.Errorf("there is no system prompt yet, use 'add' first")
		}

		if _, exists := sys_prompts[name]; exists {
			if content, err := cmd.Flags().GetString("content"); err == nil {
				sys_prompts[name] = content
			}
		} else {
			return fmt.Errorf("system prompt named '%s' not found", name)
		}

		viper.Set("system_prompts", sys_prompts)

		// Write the config file
		if err := writeConfig(); err != nil {
			return fmt.Errorf("failed to save system prompt: %w", err)
		}

		fmt.Printf("System prompt '%s' set successfully.\n", name)
		fmt.Println("---")
		fmt.Printf("content: %s\n", sys_prompts[name])
		fmt.Println("---")
		return nil
	},
}

var systemInfoCmd = &cobra.Command{
	Use:     "info NAME",
	Aliases: []string{"in"},
	Short:   "Show the content of a specific system prompt",
	Args:    cobra.ExactArgs(1), // Requires exactly one argument (the name)
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		sys_prompts := viper.GetStringMapString("system_prompts")

		content, exists := sys_prompts[name]
		if !exists {
			return fmt.Errorf("system prompt named '%s' not found", name)
		}

		fmt.Printf("System prompt '%s':\n---\n%s\n---\n", name, content)
		return nil
	},
}

var systemRemoveCmd = &cobra.Command{
	Use:     "remove NAME",
	Aliases: []string{"rm"},
	Short:   "Remove a named system prompt",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		sys_prompts := viper.GetStringMapString("system_prompts")

		if _, exists := sys_prompts[name]; !exists {
			cmd.SilenceUsage = true // Don't show usage for this error
			if force, _ := cmd.Flags().GetBool("force"); force {
				fmt.Printf("System prompt '%s' does not exist, nothing to remove.\n", name)
				return nil
			}
			return fmt.Errorf("system prompt named '%s' not found", name)
		}

		// Delete the prompt
		delete(sys_prompts, name)
		viper.Set("system_prompts", sys_prompts)

		// Check if the removed prompt was the default
		defaultSysPrompt := viper.GetString("default.system_prompt")
		if name == defaultSysPrompt {
			viper.Set("default.system_prompt", "") // Clear the default
			fmt.Printf("Note: Removed system prompt '%s' was the default. Default system prompt cleared.\n", name)
		}

		// Write the config file
		if err := writeConfig(); err != nil {
			return fmt.Errorf("failed to save configuration after removing system prompt: %w", err)
		}

		fmt.Printf("System prompt '%s' removed successfully.\n", name)
		return nil
	},
}

var systemClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all system prompts",
	Long: `Remove all saved system prompts from configuration.
This action cannot be undone.

Example:
gllm system clear
gllm system clear --force`,
	RunE: func(cmd *cobra.Command, args []string) error {

		force, _ := cmd.Flags().GetBool("force")

		if !force {
			fmt.Print("Are you sure you want to clear all system prompts? This cannot be undone. [y/N]: ")
			var response string
			fmt.Scanln(&response)

			response = strings.ToLower(strings.TrimSpace(response))
			if response != "y" && response != "yes" {
				fmt.Println("Operation cancelled.")
				return nil
			}
		}

		sys_prompts := viper.GetStringMapString("system_prompts")
		for name := range sys_prompts {
			delete(sys_prompts, name)
		}

		// Delete the prompt
		viper.Set("system_prompts", sys_prompts)

		// Check if the removed prompt was the default
		viper.Set("default.system_prompt", "") // Clear the default

		// Write the config file
		if err := writeConfig(); err != nil {
			return fmt.Errorf("failed to save configuration after clearing system prompts: %w", err)
		}

		fmt.Println("All system prompts have been cleared.")
		return nil
	},
}

var systemDefaultCmd = &cobra.Command{
	Use:     "default <name>",
	Aliases: []string{"def"},
	Short:   "Set the default system prompt to use",
	Long: `Set the default system prompt to use.
If no name is provided, the default system prompt will be cleared.
Example:
  gllm system default coder`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			// Clear the default
			viper.Set("default.system_prompt", "")
			// Write the config file
			if err := writeConfig(); err != nil {
				return fmt.Errorf("failed to save default system prompt setting: %w", err)
			}
			fmt.Println("Default system prompt cleared.")
			return nil
		}
		name := args[0]
		sys_prompts := viper.GetStringMapString("system_prompts")

		// Check if the prompt exists before setting it as default
		if _, exists := sys_prompts[name]; !exists {
			return fmt.Errorf("system prompt named '%s' not found. Cannot set as default", name)
		}

		viper.Set("default.system_prompt", name)

		// Write the config file
		if err := writeConfig(); err != nil {
			return fmt.Errorf("failed to save default system prompt setting: %w", err)
		}

		fmt.Printf("Default system prompt set to '%s' successfully.\n", name)
		return nil
	},
}

func GetAllSystemPrompts() string {
	defaultName := viper.GetString("default.system_prompt")
	sys_prompts := viper.GetStringMapString("system_prompts")
	var pairs []string
	for name, content := range sys_prompts {
		if name == defaultName {
			pairs = append(pairs, fmt.Sprintf("*%s*:\n\t%s", name, content))
			continue
		} else {
			pairs = append(pairs, fmt.Sprintf("%s:\n\t%s", name, content))
		}
	}
	return strings.Join(pairs, "\n")
}

func SetEffectiveSystemPrompt(name string) error {
	sys_prompts := viper.GetStringMapString("system_prompts")
	if _, ok := sys_prompts[name]; !ok {
		return fmt.Errorf("system prompt named '%s' not found", name)
	}
	viper.Set("default.system_prompt", name)
	if err := writeConfig(); err != nil {
		return fmt.Errorf("failed to save default system prompt setting: %w", err)
	}
	return nil
}

// New helper function to get the effective system prompt based on config
func GetEffectiveSystemPrompt() string {
	if plainSystemPrompt != "" {
		return plainSystemPrompt
	}

	defaultName := viper.GetString("default.system_prompt")
	if defaultName == "" {
		// If no default, return empty string
		return ""
	}
	sys_prompts := viper.GetStringMapString("system_prompts")

	// 1. Check if defaultName is set and exists in prompts
	if defaultName != "" {
		if content, ok := sys_prompts[defaultName]; ok {
			return content
		}
		// If default_prompt references a non-existent prompt, fall through
		logger.Warnf("Warning: Default system prompt '%s' not found in configuration. Falling back...\n", defaultName)
	}

	// 2. If no valid default, check if any prompts exist
	if len(sys_prompts) > 0 {
		// Try to get the "first" one. Map iteration order isn't guaranteed,
		// but this fulfills the requirement loosely. Sorting keys gives consistency.
		names := make([]string, 0, len(sys_prompts))
		for name := range sys_prompts {
			names = append(names, name)
		}
		sort.Strings(names)
		if len(names) > 0 {
			firstPromptName := names[0]
			logger.Debugf("Using first available system prompt '%s' as fallback.\n", firstPromptName)
			return sys_prompts[firstPromptName]
		}
	}

	// 3. If no default and no prompts, use the hardcoded default
	logger.Debugln("Using built-in default system prompt.")
	return defaultSystemPromptContent // Use the constant defined in root.go
}

func init() {
	// Add configCmd to the root command
	rootCmd.AddCommand(systemCmd)

	// Add flags specific to config subcommands here if needed
	// e.g., configModelCmd.Flags().StringP("set-default", "s", "", "Set the default model name")

	// Add subcommands to configPromptCmd
	systemCmd.AddCommand(systemListCmd)
	systemCmd.AddCommand(systemAddCmd)
	systemCmd.AddCommand(systemSetCmd)
	systemCmd.AddCommand(systemInfoCmd)
	systemCmd.AddCommand(systemRemoveCmd)
	systemCmd.AddCommand(systemClearCmd)
	systemCmd.AddCommand(systemDefaultCmd)

	// Add flags for promptAddCmd
	systemAddCmd.Flags().StringP("name", "n", "", "Name for the new system prompt (required)")
	systemAddCmd.Flags().StringP("content", "c", "", "Content/text of the new system prompt (required)")
	// Mark flags as required - Cobra will handle error messages if they are missing
	systemAddCmd.MarkFlagRequired("name")
	systemAddCmd.MarkFlagRequired("content")
	systemSetCmd.Flags().StringP("content", "c", defaultSystemPromptContent, "Content/text of the system prompt")

	// Add flags for other prompt commands if needed in the future
	systemRemoveCmd.Flags().BoolP("force", "f", false, "Skip error when system prompt doesn't exist")
	systemClearCmd.Flags().BoolP("force", "f", false, "Force clear without confirmation")
}
