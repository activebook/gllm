// File: cmd/config.go
package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/activebook/gllm/service"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	plainSystemPrompt string
)

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
		systemListCmd.Run(cmd, args)
	},
}

var systemListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all saved system prompt names",
	Run: func(cmd *cobra.Command, args []string) {
		sys_prompts := viper.GetStringMapString("system_prompts")

		if len(sys_prompts) == 0 {
			fmt.Println("No system prompts defined yet. Use 'gllm system add'.")
			return
		}

		verbose, _ := cmd.Flags().GetBool("verbose")

		if verbose {
			names := make([]string, 0, len(sys_prompts))
			for name := range sys_prompts {
				names = append(names, name)
			}
			sort.Strings(names)
			fmt.Println("Available system prompts (with details):")
			for _, name := range names {
				fmt.Printf(" %s\n %s\n\n", name, sys_prompts[name])
			}
		} else {
			fmt.Println("Available system prompts:")
			// Sort keys for consistent output
			names := make([]string, 0, len(sys_prompts))
			for name := range sys_prompts {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				fmt.Printf(" %s\n", name)
			}
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

		// Interactive mode if flags are missing
		if name == "" || content == "" {
			form := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Name").
						Value(&name).
						Validate(func(str string) error {
							if str == "" {
								return fmt.Errorf("name is required")
							}
							sys_prompts := viper.GetStringMapString("system_prompts")
							if _, exists := sys_prompts[str]; exists {
								return fmt.Errorf("system prompt '%s' already exists", str)
							}
							return nil
						}),
					huh.NewText().
						Title("Content").
						Value(&content).
						Lines(5).
						Validate(func(str string) error {
							if str == "" {
								return fmt.Errorf("content is required")
							}
							return nil
						}),
				),
			).WithKeyMap(GetHuhKeyMap())
			err := form.Run()
			if err != nil {
				return nil // User cancelled
			}
		}

		sys_prompts := viper.GetStringMapString("system_prompts")
		// Initialize map if it doesn't exist
		if sys_prompts == nil {
			sys_prompts = make(map[string]string)
		}

		// Double check existence if passed via flags (interactive validation handles it above)
		if _, exists := sys_prompts[name]; exists {
			// If name was passed via flag and exists (not caught by interactive because form didn't run or didn't validate initial value? huh validates initial?)
			// Huh validates initial value ONLY if modified or we force it? Actually simpler to just check again or rely on form.
			// The form validation handles the interactive case. For flag case:
			if cmd.Flags().Changed("name") {
				return fmt.Errorf("system prompt named '%s' already exists", name)
			}
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
	Use:   "set [NAME]",
	Short: "Set a named system prompt",
	Long: `Sets a new system prompt with a specific name and content.
Example:
  gllm system set coder --content "You are an expert Go programmer"
  gllm system set (opens selection)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var name string
		sys_prompts := viper.GetStringMapString("system_prompts")
		if sys_prompts == nil || len(sys_prompts) == 0 {
			return fmt.Errorf("there is no system prompt yet, use 'add' first")
		}

		if len(args) > 0 {
			name = args[0]
		} else {
			// Select prompt
			var options []huh.Option[string]
			for n := range sys_prompts {
				options = append(options, huh.NewOption(n, n))
			}
			// Sort options by keys is handled by huh? No, we should sort.
			// Sort names first
			var names []string
			for n := range sys_prompts {
				names = append(names, n)
			}
			sort.Strings(names)
			options = make([]huh.Option[string], len(names))
			for i, n := range names {
				options[i] = huh.NewOption(n, n)
			}

			err := huh.NewSelect[string]().
				Title("Select System Prompt to Edit").
				Options(options...).
				Value(&name).
				Run()
			if err != nil {
				return nil
			}
		}

		if _, exists := sys_prompts[name]; !exists {
			return fmt.Errorf("system prompt named '%s' not found", name)
		}

		content, _ := cmd.Flags().GetString("content")
		// If content flag not changed, show form with existing content
		if !cmd.Flags().Changed("content") {
			content = sys_prompts[name]
			err := huh.NewForm(
				huh.NewGroup(
					huh.NewText().
						Title("Content").
						Value(&content).
						Lines(10),
				),
			).WithKeyMap(GetHuhKeyMap()).Run()
			if err != nil {
				return nil
			}
		}

		sys_prompts[name] = content
		viper.Set("system_prompts", sys_prompts)

		// Write the config file
		if err := writeConfig(); err != nil {
			return fmt.Errorf("failed to save system prompt: %w", err)
		}

		fmt.Printf("System prompt '%s' set successfully.\n", name)
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
	Use:     "remove [NAME]",
	Aliases: []string{"rm"},
	Short:   "Remove a named system prompt",
	RunE: func(cmd *cobra.Command, args []string) error {
		sys_prompts := viper.GetStringMapString("system_prompts")
		if len(sys_prompts) == 0 {
			fmt.Println("No system prompts to remove.")
			return nil
		}

		var name string
		if len(args) > 0 {
			name = args[0]
		} else {
			// Select prompt to remove
			var names []string
			for n := range sys_prompts {
				names = append(names, n)
			}
			sort.Strings(names)
			var options []huh.Option[string]
			for _, n := range names {
				options = append(options, huh.NewOption(n, n))
			}

			err := huh.NewSelect[string]().
				Title("Select System Prompt to Remove").
				Options(options...).
				Value(&name).
				Run()
			if err != nil {
				return nil
			}
		}

		if _, exists := sys_prompts[name]; !exists {
			cmd.SilenceUsage = true // Don't show usage for this error
			if force, _ := cmd.Flags().GetBool("force"); force {
				fmt.Printf("System prompt '%s' does not exist, nothing to remove.\n", name)
				return nil
			}
			return fmt.Errorf("system prompt named '%s' not found", name)
		}

		// Confirm
		if !cmd.Flags().Changed("force") {
			var confirm bool
			err := huh.NewConfirm().
				Title(fmt.Sprintf("Are you sure you want to remove '%s'?", name)).
				Value(&confirm).
				Run()
			if err != nil {
				return nil
			}
			if !confirm {
				fmt.Println("Operation cancelled.")
				return nil
			}
		}

		// Delete the prompt
		delete(sys_prompts, name)
		viper.Set("system_prompts", sys_prompts)

		// Write the config file
		if err := writeConfig(); err != nil {
			return fmt.Errorf("failed to save configuration after removing system prompt: %w", err)
		}

		fmt.Printf("System prompt '%s' removed successfully.\n", name)
		return nil
	},
}

var systemSwitchCmd = &cobra.Command{
	Use:     "switch [NAME]",
	Aliases: []string{"sw", "select"},
	Short:   "Switch to a different system prompt",
	Long:    `Switch the current agent's system prompt to the specified one.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var name string
		if len(args) > 0 {
			name = args[0]
		} else {
			// Get options
			sys_prompts := viper.GetStringMapString("system_prompts")
			if len(sys_prompts) == 0 {
				fmt.Println("No system prompts found.")
				return nil
			}

			// Get current
			currentName := GetAgentString("system_prompt")

			var options []huh.Option[string]
			var names []string
			for n := range sys_prompts {
				names = append(names, n)
			}
			sort.Strings(names)

			// Add "None" option
			options = append(options, huh.NewOption("None", ""))

			for _, n := range names {
				options = append(options, huh.NewOption(n, n))
			}

			// Pre-select if matches?
			// huh select value expects the pointer to contain the default.

			name = currentName // Pre-fill with current

			err := huh.NewSelect[string]().
				Title("Select System Prompt").
				Options(options...).
				Value(&name).
				Run()
			if err != nil {
				return nil
			}
		}

		// Update agent config
		if err := SetAgentValue("system_prompt", name); err != nil {
			return fmt.Errorf("failed to switch system prompt: %w", err)
		}

		if name == "" {
			fmt.Println("System prompt disabled.")
		} else {
			fmt.Printf("Switched system prompt to '%s'.\n", name)
		}
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
			var confirm bool
			err := huh.NewConfirm().
				Title("Are you sure you want to clear ALL system prompts? This cannot be undone.").
				Affirmative("Yes, delete all").
				Negative("Cancel").
				Value(&confirm).
				Run()
			if err != nil {
				return nil
			}
			if !confirm {
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

		// Write the config file
		if err := writeConfig(); err != nil {
			return fmt.Errorf("failed to save configuration after clearing system prompts: %w", err)
		}

		fmt.Println("All system prompts have been cleared.")
		return nil
	},
}

func GetAllSystemPrompts() string {
	sys_prompts := viper.GetStringMapString("system_prompts")
	var pairs []string
	for name, content := range sys_prompts {
		pairs = append(pairs, fmt.Sprintf("%s:\n\t%s\n", name, content))
	}
	sort.Strings(pairs)
	return strings.Join(pairs, "\n")
}

func SetEffectiveSystemPrompt(sys string) error {
	// Reset system prompt to empty
	if sys == "" {
		plainSystemPrompt = ""
		return nil
	}
	// Check if the system prompt is a plain string or a named one
	if strings.ContainsAny(sys, " \t\n") {
		plainSystemPrompt = sys
		return nil
	}
	sys_prompts := viper.GetStringMapString("system_prompts")
	if _, ok := sys_prompts[sys]; !ok {
		return fmt.Errorf("system prompt named '%s' not found", sys)
	}
	plainSystemPrompt = sys_prompts[sys]
	return nil
}

// New helper function to get the effective system prompt based on config
// Memory content is automatically appended to the base system prompt
func GetEffectiveSystemPrompt() string {
	var sysPrompt string
	if plainSystemPrompt != "" {
		sysPrompt = plainSystemPrompt
	} else {
		// Get from active agent and resolve reference
		rawSys := GetAgentString("system_prompt")
		sysPrompt = service.ResolveSystemPromptReference(rawSys)
	}

	// Get memory content and append if not empty
	memoryContent := service.GetMemoryContent()
	if memoryContent != "" {
		if sysPrompt != "" {
			sysPrompt = sysPrompt + "\n\n" + memoryContent
		} else {
			sysPrompt = memoryContent
		}
	}

	return sysPrompt
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
	systemCmd.AddCommand(systemSwitchCmd)
	systemCmd.AddCommand(systemInfoCmd)
	systemCmd.AddCommand(systemRemoveCmd)
	systemCmd.AddCommand(systemClearCmd)

	// Add flags for systemListCmd
	systemListCmd.Flags().BoolP("verbose", "v", false, "Show system prompt names and their content")
	// Add flags for promptAddCmd
	systemAddCmd.Flags().StringP("name", "n", "", "Name for the new system prompt (required)")
	systemAddCmd.Flags().StringP("content", "c", "", "Content/text of the new system prompt (required)")
	// Mark flags as required - Cobra will handle error messages if they are missing
	// systemAddCmd.MarkFlagRequired("name")
	// systemAddCmd.MarkFlagRequired("content")
	systemSetCmd.Flags().StringP("content", "c", defaultSystemPromptContent, "Content/text of the system prompt")

	// Add flags for other prompt commands if needed in the future
	systemRemoveCmd.Flags().BoolP("force", "f", false, "Skip error when system prompt doesn't exist")
	systemClearCmd.Flags().BoolP("force", "f", false, "Force clear without confirmation")
}
