// File: cmd/template.go
package cmd

import (
	"fmt"
	"sort"

	"github.com/activebook/gllm/data"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

// templateCmd represents the base command when called without any subcommands
var templateCmd = &cobra.Command{
	Use:     "template",
	Aliases: []string{"tmpl", "temp"}, // Optional alias
	Short:   "Manage gllm template prompt configuration",
	Long:    `Define, view, list, or delete reusable template prompts.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Print current template prompt
		store := data.NewConfigStore()
		activeAgent := store.GetActiveAgent()
		if activeAgent == nil {
			fmt.Println("No active agent defined yet. Use 'gllm agent add'.")
			return
		}
		name := activeAgent.Template
		content := store.GetTemplate(name)
		fmt.Printf("Name: %s\nContent: %s\n", name, content)
	},
}

var templateListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all saved template prompt names",
	Run: func(cmd *cobra.Command, args []string) {
		store := data.NewConfigStore()
		activeAgent := store.GetActiveAgent()
		if activeAgent == nil {
			fmt.Println("No active agent defined yet. Use 'gllm agent add'.")
			return
		}
		templates := store.GetTemplates()
		activeTemplate := activeAgent.Template

		if len(templates) == 0 {
			fmt.Println("No template prompts defined yet. Use 'gllm template add'.")
			return
		}

		verbose, _ := cmd.Flags().GetBool("verbose")

		// Sort keys for consistent output
		names := make([]string, 0, len(templates))
		for name := range templates {
			names = append(names, name)
		}
		sort.Strings(names)

		if verbose {
			fmt.Println("Available template prompts (with details):")
			for _, name := range names {
				prefix := "  "
				pname := name
				if name == activeTemplate {
					prefix = highlightColor("* ")
					pname = highlightColor(name)
				}
				fmt.Printf("%s%s\n %s\n\n", prefix, pname, templates[name])
			}
		} else {
			fmt.Println("Available template prompts:")
			for _, name := range names {
				prefix := "  "
				pname := name
				if name == activeTemplate {
					prefix = highlightColor("* ")
					pname = highlightColor(name)
				}
				fmt.Printf("%s%s\n", prefix, pname)
			}
		}

		if activeTemplate != "" {
			fmt.Println("\n(*) Indicates the current template prompt.")
		} else {
			fmt.Println("\nNo template prompt selected. Use 'gllm template switch <name>' to select one.")
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
		store := data.NewConfigStore()

		// Interactive inputs
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
							templates := store.GetTemplates()
							if _, exists := templates[str]; exists {
								return fmt.Errorf("template '%s' already exists", str)
							}
							return nil
						}),
					huh.NewText().
						Title("Content").
						Value(&content).
						Lines(10).
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
				return nil
			}
		}

		templates := store.GetTemplates()
		if _, exists := templates[name]; exists {
			// Check if flag interaction
			if cmd.Flags().Changed("name") {
				return fmt.Errorf("template prompt named '%s' already exists", name)
			}
		}

		if err := store.SetTemplate(name, content); err != nil {
			return fmt.Errorf("failed to save template prompt: %w", err)
		}

		fmt.Printf("template prompt '%s' added successfully.\n", name)
		return nil
	},
}

var templateSetCmd = &cobra.Command{
	Use:   "set [NAME]",
	Short: "Set a named template prompt",
	Long: `Sets a new template prompt with a specific name and content.
Example:
  gllm template set coder --content "You are an expert Go programmer"
  gllm template set (opens selection)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var name string
		store := data.NewConfigStore()
		templates := store.GetTemplates()
		if len(templates) == 0 {
			return fmt.Errorf("there is no template prompt yet, use 'add' first")
		}

		if len(args) > 0 {
			name = args[0]
		} else {
			activeAgent := store.GetActiveAgent()
			if activeAgent != nil {
				name = activeAgent.Template
			}
			// Select prompt
			var options []huh.Option[string]
			for n := range templates {
				options = append(options, huh.NewOption(n, n))
			}
			SortOptions(options, name)

			err := huh.NewSelect[string]().
				Title("Select Template to Edit").
				Options(options...).
				Value(&name).
				Run()
			if err != nil {
				return nil
			}
		}

		if _, exists := templates[name]; !exists {
			return fmt.Errorf("template prompt named '%s' not found", name)
		}

		content, _ := cmd.Flags().GetString("content")
		if !cmd.Flags().Changed("content") {
			content = templates[name]
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

		if err := store.SetTemplate(name, content); err != nil {
			return fmt.Errorf("failed to save template prompt: %w", err)
		}

		fmt.Printf("template prompt '%s' set successfully.\n", name)
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
		store := data.NewConfigStore()
		templates := store.GetTemplates()

		content, exists := templates[name]
		if !exists {
			return fmt.Errorf("template prompt named '%s' not found", name)
		}

		fmt.Printf("template prompt '%s':\n---\n%s\n---\n", name, content)
		return nil
	},
}

var templateRemoveCmd = &cobra.Command{
	Use:     "remove [NAME]",
	Aliases: []string{"rm"},
	Short:   "Remove a named template prompt",
	RunE: func(cmd *cobra.Command, args []string) error {
		store := data.NewConfigStore()
		templates := store.GetTemplates()
		if len(templates) == 0 {
			fmt.Println("No templates to remove.")
			return nil
		}
		var name string
		if len(args) > 0 {
			name = args[0]
		} else {
			activeAgent := store.GetActiveAgent()
			if activeAgent != nil {
				name = activeAgent.Template
			}
			var options []huh.Option[string]
			for n := range templates {
				options = append(options, huh.NewOption(n, n))
			}
			SortOptions(options, name)

			err := huh.NewSelect[string]().
				Title("Select Template to Remove").
				Options(options...).
				Value(&name).
				Run()
			if err != nil {
				return nil
			}
		}

		if _, exists := templates[name]; !exists {
			cmd.SilenceUsage = true // Don't show usage for this error
			if force, _ := cmd.Flags().GetBool("force"); force {
				fmt.Printf("template prompt '%s' does not exist, nothing to remove.\n", name)
				return nil
			}
			return fmt.Errorf("template prompt named '%s' not found", name)
		}

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
		if err := store.DeleteTemplate(name); err != nil {
			return fmt.Errorf("failed to remove template prompt: %w", err)
		}

		fmt.Printf("template prompt '%s' removed successfully.\n", name)
		return nil
	},
}

var templateSwitchCmd = &cobra.Command{
	Use:     "switch [NAME]",
	Aliases: []string{"sw", "select"},
	Short:   "Switch to a different template",
	Long:    `Switch the current agent's template to the specified one.`,
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

			templates := store.GetTemplates()
			if len(templates) == 0 {
				fmt.Println("No templates found.")
				return nil
			}

			currentName := activeAgent.Template

			var options []huh.Option[string]
			var names []string
			for n := range templates {
				names = append(names, n)
			}

			// Add "None" option
			// bugfix: must set a non-empty value, otherwise the sort will fail
			options = append(options, huh.NewOption("None", " "))

			for _, n := range names {
				options = append(options, huh.NewOption(n, n))
			}
			SortOptions(options, currentName)

			name = currentName

			err := huh.NewSelect[string]().
				Title("Select Template").
				Options(options...).
				Value(&name).
				Run()
			if err != nil {
				return nil
			}
		}

		activeAgent.Template = name
		if err := store.SetAgent(activeAgent.Name, activeAgent); err != nil {
			return fmt.Errorf("failed to switch template: %w", err)
		}

		if name == "" {
			fmt.Println("Template disabled.")
		} else {
			fmt.Printf("Switched template to '%s'.\n", name)
		}
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
			var confirm bool
			err := huh.NewConfirm().
				Title("Are you sure you want to clear ALL template prompts? This cannot be undone.").
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
		templates := store.GetTemplates()
		for name := range templates {
			if err := store.DeleteTemplate(name); err != nil {
				return fmt.Errorf("failed to delete template '%s': %w", name, err)
			}
		}

		fmt.Println("All template prompts have been cleared.")
		return nil
	},
}

func init() {
	// Add configCmd to the root command
	rootCmd.AddCommand(templateCmd)

	// Add subcommands to configPromptCmd
	templateCmd.AddCommand(templateListCmd)
	templateCmd.AddCommand(templateAddCmd)
	templateCmd.AddCommand(templateSetCmd)
	templateCmd.AddCommand(templateSwitchCmd)
	templateCmd.AddCommand(templateInfoCmd)
	templateCmd.AddCommand(templateRemoveCmd)
	templateCmd.AddCommand(templateClearCmd)

	// Add flags for templateListCmd
	templateListCmd.Flags().BoolP("verbose", "v", false, "Show template names and their content")
	// Add flags for promptAddCmd
	templateAddCmd.Flags().StringP("name", "n", "", "Name for the new template prompt (required)")
	templateAddCmd.Flags().StringP("content", "c", "", "Content/text of the new template prompt (required)")
	templateSetCmd.Flags().StringP("content", "c", defaultTemplateContent, "Content/text of the template prompt")

	// Add flags for other prompt commands if needed in the future
	templateRemoveCmd.Flags().BoolP("force", "f", false, "Skip error when template prompt doesn't exist")
	templateClearCmd.Flags().BoolP("force", "f", false, "Force clear without confirmation")
}
