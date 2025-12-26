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
	plainTemplate string
)

func GetAllTemplates() string {
	templates := viper.GetStringMapString("templates")
	var pairs []string
	for name, content := range templates {
		pairs = append(pairs, fmt.Sprintf("%s:\n\t%s\n", name, content))
	}
	sort.Strings(pairs)
	return strings.Join(pairs, "\n")
}

func SetEffectiveTemplate(tmpl string) error {
	// Reset template to empty
	if tmpl == "" {
		plainTemplate = ""
		return nil
	}
	// Check if the template is a plain string or a named one
	// If it contains spaces, tabs, or newlines, treat it as a plain string
	if strings.ContainsAny(tmpl, " \t\n") {
		plainTemplate = tmpl
		return nil
	}
	templates := viper.GetStringMapString("templates")
	if _, exists := templates[tmpl]; !exists {
		return fmt.Errorf("template prompt named '%s' not found", tmpl)
	}
	plainTemplate = templates[tmpl]
	return nil
}

// New helper function to get the effective template prompt based on config
func GetEffectiveTemplate() string {
	if plainTemplate != "" {
		return plainTemplate
	}
	// Get from active agent and resolve reference
	rawTmpl := GetAgentString("template")
	return service.ResolveTemplateReference(rawTmpl)
}

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
		templateListCmd.Run(cmd, args)
	},
}

var templateListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all saved template prompt names",
	Run: func(cmd *cobra.Command, args []string) {
		templates := viper.GetStringMapString("templates")

		if len(templates) == 0 {
			fmt.Println("No template prompts defined yet. Use 'gllm template add'.")
			return
		}

		verbose, _ := cmd.Flags().GetBool("verbose")

		if verbose {
			// Print name and details
			names := make([]string, 0, len(templates))
			for name := range templates {
				names = append(names, name)
			}
			sort.Strings(names)
			fmt.Println("Available template prompts (with details):")
			for _, name := range names {
				fmt.Printf(" %s\n %s\n\n", name, templates[name])
			}
		} else {
			fmt.Println("Available template prompts:")
			// Sort keys for consistent output
			names := make([]string, 0, len(templates))
			for name := range templates {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				fmt.Printf(" %s\n", name)
			}
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
							templates := viper.GetStringMapString("templates")
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

		templates := viper.GetStringMapString("templates")
		// Initialize map if it doesn't exist
		if templates == nil {
			templates = make(map[string]string)
		}

		if _, exists := templates[name]; exists {
			// Check if flag interaction
			if cmd.Flags().Changed("name") {
				return fmt.Errorf("template prompt named '%s' already exists", name)
			}
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
	Use:   "set [NAME]",
	Short: "Set a named template prompt",
	Long: `Sets a new template prompt with a specific name and content.
Example:
  gllm template set coder --content "You are an expert Go programmer"
  gllm template set (opens selection)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var name string
		templates := viper.GetStringMapString("templates")
		if templates == nil || len(templates) == 0 {
			return fmt.Errorf("there is no template prompt yet, use 'add' first")
		}

		if len(args) > 0 {
			name = args[0]
		} else {
			// Select prompt
			var options []huh.Option[string]
			var names []string
			for n := range templates {
				names = append(names, n)
			}
			sort.Strings(names)
			for _, n := range names {
				options = append(options, huh.NewOption(n, n))
			}

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

		templates[name] = content
		viper.Set("templates", templates)

		// Write the config file
		if err := writeConfig(); err != nil {
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
	Use:     "remove [NAME]",
	Aliases: []string{"rm"},
	Short:   "Remove a named template prompt",
	RunE: func(cmd *cobra.Command, args []string) error {
		templates := viper.GetStringMapString("templates")
		if len(templates) == 0 {
			fmt.Println("No templates to remove.")
			return nil
		}
		var name string
		if len(args) > 0 {
			name = args[0]
		} else {
			var names []string
			for n := range templates {
				names = append(names, n)
			}
			sort.Strings(names)
			var options []huh.Option[string]
			for _, n := range names {
				options = append(options, huh.NewOption(n, n))
			}

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

		// Delete the prompt
		delete(templates, name)
		viper.Set("templates", templates)

		// Write the config file
		if err := writeConfig(); err != nil {
			return fmt.Errorf("failed to save configuration after removing template prompt: %w", err)
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
		var name string
		if len(args) > 0 {
			name = args[0]
		} else {
			templates := viper.GetStringMapString("templates")
			if len(templates) == 0 {
				fmt.Println("No templates found.")
				return nil
			}

			currentName := GetAgentString("template")

			var options []huh.Option[string]
			var names []string
			for n := range templates {
				names = append(names, n)
			}
			sort.Strings(names)

			options = append(options, huh.NewOption("None", ""))
			for _, n := range names {
				options = append(options, huh.NewOption(n, n))
			}

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

		if err := SetAgentValue("template", name); err != nil {
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

		templates := viper.GetStringMapString("templates")
		for name := range templates {
			delete(templates, name)
		}

		// Delete the prompt
		viper.Set("templates", templates)

		// Write the config file
		if err := writeConfig(); err != nil {
			return fmt.Errorf("failed to save configuration after clearing template prompts: %w", err)
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
	// Mark flags as required - Cobra will handle error messages if they are missing
	// templateAddCmd.MarkFlagRequired("name")
	// templateAddCmd.MarkFlagRequired("content")
	templateSetCmd.Flags().StringP("content", "c", defaultTemplateContent, "Content/text of the template prompt")

	// Add flags for other prompt commands if needed in the future
	templateRemoveCmd.Flags().BoolP("force", "f", false, "Skip error when template prompt doesn't exist")
	templateClearCmd.Flags().BoolP("force", "f", false, "Force clear without confirmation")
}
