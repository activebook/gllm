// File: cmd/config.go
package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// templateCmd represents the base command when called without any subcommands
var templateCmd = &cobra.Command{
	Use:     "template",
	Aliases: []string{"tmpl", "temp"}, // Optional alias
	Short:   "Manage gllm template prompt configuration",
	Long:    `Define, view, list, or delete reusable template prompts.`,
	// Run: func(cmd *cobra.Command, args []string) {
	//  fmt.Println("Use 'gllm config [subcommand] --help' for more information.")
	// },
	// Suggest showing help if 'gllm config' is run without subcommand
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Manage template prompts here (e.g., add, list, remove)")
		// Example: Access a viper setting for prompts (maybe a map or slice)
		// prompts := viper.Get("prompts")
		// fmt.Printf("Current prompts (from config): %v\n", prompts)
		cmd.Help() // Show help for now
	},
}

var templateListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all saved template prompt names",
	Run: func(cmd *cobra.Command, args []string) {
		templates := viper.GetStringMapString("templates")
		defaultSysPrompt := viper.GetString("default.template")

		if len(templates) == 0 {
			fmt.Println("No template prompts defined yet. Use 'gllm template add'.")
			return
		}

		fmt.Println("Available template prompts:")
		// Sort keys for consistent output
		names := make([]string, 0, len(templates))
		for name := range templates {
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
			fmt.Println("\n(*) Indicates the default template prompt.")
		} else {
			fmt.Println("\nNo default template prompt set.")
		}
	},
}

var templateAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new named template prompt",
	Long: `Adds a new template prompt with a specific name and content.
Example:
  gllm template add --name coder --content "You are an expert Go programmer"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		content, _ := cmd.Flags().GetString("content")

		templates := viper.GetStringMapString("templates")
		// Initialize map if it doesn't exist
		if templates == nil {
			templates = make(map[string]string)
		}

		if _, exists := templates[name]; exists {
			// Maybe add an --overwrite flag later? For now, error out.
			return fmt.Errorf("template prompt named '%s' already exists. Use 'remove' first or choose a different name", name)
		}

		templates[name] = content
		viper.Set("templates", templates)

		// Write the config file
		if err := writeConfig(); err != nil {
			return fmt.Errorf("failed to save template prompt: %w", err)
		}

		fmt.Printf("template prompt '%s' added successfully.\n", name)
		return nil
	},
}

var templateSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set a named template prompt",
	Long: `Sets a new template prompt with a specific name and content.
Example:
  gllm template set coder --content "You are an expert Go programmer"`,
	Args: cobra.ExactArgs(1), // Requires exactly one argument (the name)
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		templates := viper.GetStringMapString("templates")
		// Initialize map if it doesn't exist
		if templates == nil {
			return fmt.Errorf("there is no template prompt yet, use 'add' first")
		}

		if _, exists := templates[name]; exists {
			if content, err := cmd.Flags().GetString("content"); err == nil {
				templates[name] = content
			}
		} else {
			return fmt.Errorf("template prompt named '%s' not found", name)
		}

		viper.Set("templates", templates)

		// Write the config file
		if err := writeConfig(); err != nil {
			return fmt.Errorf("failed to save template prompt: %w", err)
		}

		fmt.Printf("template prompt '%s' set successfully.\n", name)
		fmt.Println("---")
		fmt.Printf("content: %s\n", templates[name])
		fmt.Println("---")
		return nil
	},
}

var templateInfoCmd = &cobra.Command{
	Use:     "info NAME",
	Aliases: []string{"in"},
	Short:   "Show the content of a specific template prompt",
	Args:    cobra.ExactArgs(1), // Requires exactly one argument (the name)
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		templates := viper.GetStringMapString("templates")

		content, exists := templates[name]
		if !exists {
			return fmt.Errorf("template prompt named '%s' not found", name)
		}

		fmt.Printf("template prompt '%s':\n---\n%s\n---\n", name, content)
		return nil
	},
}

var templateRemoveCmd = &cobra.Command{
	Use:     "remove NAME",
	Aliases: []string{"rm"},
	Short:   "Remove a named template prompt",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		templates := viper.GetStringMapString("templates")

		if _, exists := templates[name]; !exists {
			cmd.SilenceUsage = true // Don't show usage for this error
			if force, _ := cmd.Flags().GetBool("force"); force {
				fmt.Printf("template prompt '%s' does not exist, nothing to remove.\n", name)
				return nil
			}
			return fmt.Errorf("template prompt named '%s' not found", name)
		}

		// Delete the prompt
		delete(templates, name)
		viper.Set("templates", templates)

		// Check if the removed prompt was the default
		defaultSysPrompt := viper.GetString("default.template")
		if name == defaultSysPrompt {
			viper.Set("default.template", "") // Clear the default
			fmt.Printf("Note: Removed template prompt '%s' was the default. Default template prompt cleared.\n", name)
		}

		// Write the config file
		if err := writeConfig(); err != nil {
			return fmt.Errorf("failed to save configuration after removing template prompt: %w", err)
		}

		fmt.Printf("template prompt '%s' removed successfully.\n", name)
		return nil
	},
}

var templateClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all template prompts",
	Long: `Remove all saved template prompts from configuration.
This action cannot be undone.

Example:
gllm template clear
gllm template clear --force`,
	RunE: func(cmd *cobra.Command, args []string) error {

		force, _ := cmd.Flags().GetBool("force")

		if !force {
			fmt.Print("Are you sure you want to clear all template prompts? This cannot be undone. [y/N]: ")
			var response string
			fmt.Scanln(&response)

			response = strings.ToLower(strings.TrimSpace(response))
			if response != "y" && response != "yes" {
				fmt.Println("Operation cancelled.")
				return nil
			}
		}

		templates := viper.GetStringMapString("templates")
		for name := range templates {
			delete(templates, name)
		}

		// Delete the prompt
		viper.Set("templates", templates)

		// Check if the removed prompt was the default
		viper.Set("default.template", "") // Clear the default

		// Write the config file
		if err := writeConfig(); err != nil {
			return fmt.Errorf("failed to save configuration after clearing template prompts: %w", err)
		}

		fmt.Println("All template prompts have been cleared.")
		return nil
	},
}

var templateDefaultCmd = &cobra.Command{
	Use:     "default <name>",
	Aliases: []string{"def"},
	Short:   "Set the default template prompt to use",
	Long: `Set the default template prompt to use.
If no name is provided, the default template prompt will be cleared.
Example:
  gllm template default coder`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			viper.Set("default.template", "")
			// Write the config file
			if err := writeConfig(); err != nil {
				return fmt.Errorf("failed to clear default template prompt: %w", err)
			}
			fmt.Println("Default template prompt cleared.")
			return nil
		}
		name := args[0]
		templates := viper.GetStringMapString("templates")

		// Check if the prompt exists before setting it as default
		if _, exists := templates[name]; !exists {
			return fmt.Errorf("template prompt named '%s' not found. Cannot set as default", name)
		}

		viper.Set("default.template", name)

		// Write the config file
		if err := writeConfig(); err != nil {
			return fmt.Errorf("failed to save default template prompt setting: %w", err)
		}

		fmt.Printf("Default template prompt set to '%s' successfully.\n", name)
		return nil
	},
}

func GetAllTemplates() string {
	defaultName := viper.GetString("default.template")
	templates := viper.GetStringMapString("templates")
	var pairs []string
	for name, content := range templates {
		if name == defaultName {
			pairs = append(pairs, fmt.Sprintf("*%s*:\n\t%s", name, content))
			continue
		} else {
			pairs = append(pairs, fmt.Sprintf("%s:\n\t%s", name, content))
		}
	}
	return strings.Join(pairs, "\n")
}

func SetEffectiveTemplate(name string) error {
	templates := viper.GetStringMapString("templates")
	if _, exists := templates[name]; !exists {
		return fmt.Errorf("template prompt named '%s' not found", name)
	}
	viper.Set("default.template", name)
	if err := writeConfig(); err != nil {
		return fmt.Errorf("failed to save default template prompt setting: %w", err)
	}
	return nil
}

// New helper function to get the effective template prompt based on config
func GetEffectiveTemplate() string {
	defaultName := viper.GetString("default.template")
	if defaultName == "" {
		// No default set, return empty string
		return ""
	}
	templates := viper.GetStringMapString("templates")

	// 1. Check if defaultName is set and exists in prompts
	if defaultName != "" {
		if content, ok := templates[defaultName]; ok {
			return content
		}
		// If default_prompt references a non-existent prompt, fall through
		logger.Warnf("Warning: Default template prompt '%s' not found in configuration. Falling back...\n", defaultName)
	}

	// 2. If no valid default, check if any prompts exist
	if len(templates) > 0 {
		// Try to get the "first" one. Map iteration order isn't guaranteed,
		// but this fulfills the requirement loosely. Sorting keys gives consistency.
		names := make([]string, 0, len(templates))
		for name := range templates {
			names = append(names, name)
		}
		sort.Strings(names)
		if len(names) > 0 {
			firstPromptName := names[0]
			logger.Debugf("Using first available template prompt '%s' as fallback.\n", firstPromptName)
			return templates[firstPromptName]
		}
	}

	// 3. If no default and no prompts, use the hardcoded default
	logger.Debugln("Using built-in default template prompt.")
	return defaultTemplateContent // Use the constant defined in root.go
}

func init() {
	// Add configCmd to the root command
	rootCmd.AddCommand(templateCmd)

	// Add subcommands to configPromptCmd
	templateCmd.AddCommand(templateListCmd)
	templateCmd.AddCommand(templateAddCmd)
	templateCmd.AddCommand(templateSetCmd)
	templateCmd.AddCommand(templateInfoCmd)
	templateCmd.AddCommand(templateRemoveCmd)
	templateCmd.AddCommand(templateClearCmd)
	templateCmd.AddCommand(templateDefaultCmd)

	// Add flags for promptAddCmd
	templateAddCmd.Flags().StringP("name", "n", "", "Name for the new template prompt (required)")
	templateAddCmd.Flags().StringP("content", "c", "", "Content/text of the new template prompt (required)")
	// Mark flags as required - Cobra will handle error messages if they are missing
	templateAddCmd.MarkFlagRequired("name")
	templateAddCmd.MarkFlagRequired("content")
	templateSetCmd.Flags().StringP("content", "c", defaultTemplateContent, "Content/text of the template prompt")

	// Add flags for other prompt commands if needed in the future
	templateRemoveCmd.Flags().BoolP("force", "f", false, "Skip error when template prompt doesn't exist")
	templateClearCmd.Flags().BoolP("force", "f", false, "Force clear without confirmation")
}
