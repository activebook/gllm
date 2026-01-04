// File: cmd/system.go
package cmd

import (
	"fmt"
	"slices"
	"sort"

	"github.com/activebook/gllm/data"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
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
	Run: func(cmd *cobra.Command, args []string) {
		// Print current system prompt
		store := data.NewConfigStore()
		activeAgent := store.GetActiveAgent()
		if activeAgent == nil {
			fmt.Println("No active agent defined yet. Use 'gllm agent add'.")
			return
		}

		name := activeAgent.SystemPrompt
		content := store.GetSystemPrompt(name)
		fmt.Printf("Name: %s\nContent: %s\n", name, content)
	},
}

var systemListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all saved system prompt names",
	Run: func(cmd *cobra.Command, args []string) {
		store := data.NewConfigStore()
		activeAgent := store.GetActiveAgent()
		if activeAgent == nil {
			fmt.Println("No active agent defined yet. Use 'gllm agent add'.")
			return
		}
		sysPrompts := store.GetSystemPrompts()
		activeSystem := activeAgent.SystemPrompt

		if len(sysPrompts) == 0 {
			fmt.Println("No system prompts defined yet. Use 'gllm system add'.")
			return
		}

		verbose, _ := cmd.Flags().GetBool("verbose")

		// Sort keys for consistent output
		names := make([]string, 0, len(sysPrompts))
		for name := range sysPrompts {
			names = append(names, name)
		}
		sort.Strings(names)

		if verbose {
			fmt.Println("Available system prompts (with details):")
			for _, name := range names {
				prefix := "  "
				pname := name
				if name == activeSystem {
					prefix = highlightColor("* ")
					pname = highlightColor(name)
				}
				fmt.Printf("%s%s\n %s\n\n", prefix, pname, sysPrompts[name])
			}
		} else {
			fmt.Println("Available system prompts:")
			for _, name := range names {
				prefix := "  "
				pname := name
				if name == activeSystem {
					prefix = highlightColor("* ")
					pname = highlightColor(name)
				}
				fmt.Printf("%s%s\n", prefix, pname)
			}
		}

		if activeSystem != "" {
			fmt.Println("\n(*) Indicates the current system prompt.")
		} else {
			fmt.Println("\nNo system prompt selected. Use 'gllm system switch <name>' to select one.")
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
		store := data.NewConfigStore()

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
							sysPrompts := store.GetSystemPrompts()
							if _, exists := sysPrompts[str]; exists {
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

		sysPrompts := store.GetSystemPrompts()
		if _, exists := sysPrompts[name]; exists {
			if cmd.Flags().Changed("name") {
				return fmt.Errorf("system prompt named '%s' already exists", name)
			}
		}

		if err := store.SetSystemPrompt(name, content); err != nil {
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
		store := data.NewConfigStore()
		sysPrompts := store.GetSystemPrompts()
		if len(sysPrompts) == 0 {
			return fmt.Errorf("there is no system prompt yet, use 'add' first")
		}

		if len(args) > 0 {
			name = args[0]
		} else {
			activeAgent := store.GetActiveAgent()
			if activeAgent != nil {
				name = activeAgent.SystemPrompt
			}
			// Select prompt
			var options []huh.Option[string]
			for n := range sysPrompts {
				options = append(options, huh.NewOption(n, n))
			}
			SortOptions(options, name)

			err := huh.NewSelect[string]().
				Title("Select System Prompt to Edit").
				Options(options...).
				Value(&name).
				Run()
			if err != nil {
				return nil
			}
		}

		if _, exists := sysPrompts[name]; !exists {
			return fmt.Errorf("system prompt named '%s' not found", name)
		}

		content, _ := cmd.Flags().GetString("content")
		// If content flag not changed, show form with existing content
		if !cmd.Flags().Changed("content") {
			content = sysPrompts[name]
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

		if err := store.SetSystemPrompt(name, content); err != nil {
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
	// Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store := data.NewConfigStore()
		sysPrompts := store.GetSystemPrompts()
		if len(sysPrompts) == 0 {
			return fmt.Errorf("there is no system prompt yet.")
		}
		var name string
		if len(args) > 0 {
			name = args[0]
		} else {
			activeAgent := store.GetActiveAgent()
			if activeAgent != nil {
				name = activeAgent.SystemPrompt
			}
			// Select prompt to remove
			var options []huh.Option[string]
			for n := range sysPrompts {
				options = append(options, huh.NewOption(n, n))
			}
			SortOptions(options, name)

			err := huh.NewSelect[string]().
				Title("Select System Prompt to Check").
				Options(options...).
				Value(&name).
				Run()
			if err != nil {
				return nil
			}
		}

		content, exists := sysPrompts[name]
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
		store := data.NewConfigStore()
		sysPrompts := store.GetSystemPrompts()
		if len(sysPrompts) == 0 {
			fmt.Println("No system prompts to remove.")
			return nil
		}

		var name string
		if len(args) > 0 {
			name = args[0]
		} else {
			activeAgent := store.GetActiveAgent()
			if activeAgent != nil {
				name = activeAgent.SystemPrompt
			}
			// Select prompt to remove
			var options []huh.Option[string]
			for n := range sysPrompts {
				options = append(options, huh.NewOption(n, n))
			}
			SortOptions(options, name)

			err := huh.NewSelect[string]().
				Title("Select System Prompt to Remove").
				Options(options...).
				Value(&name).
				Run()
			if err != nil {
				return nil
			}
		}

		if _, exists := sysPrompts[name]; !exists {
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

		// Delete the prompt using data layer
		if err := store.DeleteSystemPrompt(name); err != nil {
			return fmt.Errorf("failed to remove system prompt: %w", err)
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
		store := data.NewConfigStore()
		activeAgent := store.GetActiveAgent()
		if activeAgent == nil {
			fmt.Println("No active agent defined yet. Use 'gllm agent add'.")
			return nil
		}

		var name string
		if len(args) > 0 {
			name = args[0]
		} else {
			// Get options

			sysPrompts := store.GetSystemPrompts()
			if len(sysPrompts) == 0 {
				fmt.Println("No system prompts found.")
				return nil
			}

			// Get current
			currentName := activeAgent.SystemPrompt

			var options []huh.Option[string]
			var names []string
			for n := range sysPrompts {
				names = append(names, n)
			}
			for _, n := range names {
				options = append(options, huh.NewOption(n, n))
			}
			// Add "None" option, if there isn't one
			if !slices.Contains(names, "None") {
				// bugfix: must set a non-empty value, otherwise the sort will fail
				options = append(options, huh.NewOption("None", " "))
			}

			SortOptions(options, currentName)

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
		activeAgent.SystemPrompt = name
		if err := store.SetAgent(activeAgent.Name, activeAgent); err != nil {
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

		store := data.NewConfigStore()
		sysPrompts := store.GetSystemPrompts()
		for name := range sysPrompts {
			if err := store.DeleteSystemPrompt(name); err != nil {
				return fmt.Errorf("failed to delete system prompt '%s': %w", name, err)
			}
		}

		fmt.Println("All system prompts have been cleared.")
		return nil
	},
}

func init() {
	// Add configCmd to the root command
	rootCmd.AddCommand(systemCmd)

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
	systemSetCmd.Flags().StringP("content", "c", defaultSystemPromptContent, "Content/text of the system prompt")

	// Add flags for other prompt commands if needed in the future
	systemRemoveCmd.Flags().BoolP("force", "f", false, "Skip error when system prompt doesn't exist")
	systemClearCmd.Flags().BoolP("force", "f", false, "Force clear without confirmation")
}
