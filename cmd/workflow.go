package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/internal/ui"
	"github.com/activebook/gllm/service"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

const (
	workflowTempFile = ".gllm-workflow-*.tmp"
)

var workflowCmd = &cobra.Command{
	Use:     "workflow",
	Aliases: []string{"wf", "work", "wk"},
	Short:   "Manage workflow commands",
	Long:    `Manage user-defined workflow commands stored as markdown files.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Default action: list workflows
		workflowListCmd.Run(workflowListCmd, args)
	},
}

func init() {
	rootCmd.AddCommand(workflowCmd)
	workflowCmd.AddCommand(workflowListCmd)
	workflowCmd.AddCommand(workflowCreateCmd)
	workflowCmd.AddCommand(workflowRemoveCmd)
	workflowCmd.AddCommand(workflowRenameCmd)
	workflowCmd.AddCommand(workflowInfoCmd)
	workflowCmd.AddCommand(workflowSetCmd)
}

var workflowListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls", "pr", "print"},
	Short:   "List all available workflows",
	Run: func(cmd *cobra.Command, args []string) {
		wm := service.GetWorkflowManager()
		if err := wm.LoadMetadata(chatCommandMap); err != nil {
			service.Errorf("Failed to load workflows: %v", err)
			return
		}

		names := wm.GetWorkflowNames()
		if len(names) == 0 {
			fmt.Println("No workflows found.")
			return
		}

		fmt.Println("Available workflows:")
		for _, name := range names {
			_, desc, _ := wm.GetWorkflowByName(name)
			if desc != "" {
				fmt.Printf("  /%s - %s\n", name, desc)
			} else {
				fmt.Printf("  /%s\n", name)
			}
		}
	},
}

var workflowCreateCmd = &cobra.Command{
	Use:     "create [name]",
	Aliases: []string{"new", "add", "c", "n"},
	Short:   "Create a new workflow",
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		wm := service.GetWorkflowManager()
		if err := wm.LoadMetadata(chatCommandMap); err != nil {
			service.Errorf("Failed to load workflows: %v", err)
			return
		}

		var name string
		if len(args) > 0 {
			name = args[0]
		} else {
			err := huh.NewInput().
				Title("Workflow Name").
				Description("Enter the name for the new workflow (e.g. 'debug')").
				Value(&name).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("name cannot be empty")
					}
					if wm.IsReservedCommand(s) {
						return fmt.Errorf("name '%s' is a reserved command", s)
					}
					return nil
				}).
				Run()
			if err != nil {
				return
			}
		}

		// Prompt for description
		var description string
		err := huh.NewInput().
			Title("Workflow Description").
			Description("Brief description of what this workflow does").
			Value(&description).
			Run()
		if err != nil {
			return
		}

		// Open editor for content
		editor := getPreferredEditor()
		tempFile, err := createTempFile(workflowTempFile)
		if err != nil {
			service.Errorf("Failed to create temp file: %v", err)
			return
		}
		defer os.Remove(tempFile)

		cmdExec := exec.Command(editor, tempFile)
		cmdExec.Stdin = os.Stdin
		cmdExec.Stdout = os.Stdout
		cmdExec.Stderr = os.Stderr

		if err := cmdExec.Run(); err != nil {
			service.Errorf("Editor failed: %v", err)
			return
		}

		contentBytes, err := os.ReadFile(tempFile)
		if err != nil {
			service.Errorf("Failed to read content: %v", err)
			return
		}
		content := strings.TrimSpace(string(contentBytes))

		if content == "" {
			fmt.Println("Empty content, workflow creation aborted.")
			return
		}

		if err := wm.CreateWorkflow(name, description, content); err != nil {
			service.Errorf("Failed to create workflow: %v", err)
			return
		}

		fmt.Printf("Workflow '/%s' created successfully.\n", name)
	},
}

var workflowRemoveCmd = &cobra.Command{
	Use:     "remove [name]",
	Aliases: []string{"rm", "delete", "del"},
	Short:   "Remove a workflow",
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		wm := service.GetWorkflowManager()
		if err := wm.LoadMetadata(chatCommandMap); err != nil {
			service.Errorf("Failed to load workflows: %v", err)
			return
		}

		var name string
		if len(args) > 0 {
			name = args[0]
		} else {
			names := wm.GetWorkflowNames()
			if len(names) == 0 {
				fmt.Println("No workflows to remove.")
				return
			}
			options := make([]huh.Option[string], len(names))
			for i, n := range names {
				options[i] = huh.NewOption("/"+n, n)
			}
			ui.SortOptions(options, name)
			height := ui.GetTermFitHeight(len(options))
			err := huh.NewSelect[string]().
				Title("Remove Workflow").
				Description("Select a workflow to remove").
				Options(options...).
				Value(&name).
				Height(height).
				Run()
			if err != nil {
				return
			}
		}

		// Confirm
		var confirm bool
		err := huh.NewConfirm().
			Title(fmt.Sprintf("Are you sure you want to remove workflow '/%s'?", name)).
			Affirmative("Yes").
			Negative("No").
			Value(&confirm).
			Run()

		if err != nil || !confirm {
			fmt.Println("Aborted.")
			return
		}

		if err := wm.RemoveWorkflow(name); err != nil {
			service.Errorf("Failed to remove workflow: %v", err)
			return
		}

		fmt.Printf("Workflow '/%s' removed.\n", name)
	},
}

var workflowRenameCmd = &cobra.Command{
	Use:     "rename [old] [new]",
	Aliases: []string{"mv"},
	Short:   "Rename a workflow",
	Args:    cobra.MaximumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		wm := service.GetWorkflowManager()
		if err := wm.LoadMetadata(chatCommandMap); err != nil {
			service.Errorf("Failed to load workflows: %v", err)
			return
		}

		var oldName, newName string
		if len(args) >= 1 {
			oldName = args[0]
		}
		if len(args) >= 2 {
			newName = args[1]
		}

		if oldName == "" {
			names := wm.GetWorkflowNames()
			if len(names) == 0 {
				fmt.Println("No workflows to rename.")
				return
			}
			options := make([]huh.Option[string], len(names))
			for i, n := range names {
				options[i] = huh.NewOption("/"+n, n)
			}
			ui.SortOptions(options, oldName)
			height := ui.GetTermFitHeight(len(options))
			err := huh.NewSelect[string]().
				Title("Rename Workflow").
				Description("Select a workflow to rename").
				Options(options...).
				Value(&oldName).
				Height(height).
				Run()
			if err != nil {
				return
			}
		}

		if newName == "" {
			err := huh.NewInput().
				Title("New Name").
				Description(fmt.Sprintf("Enter new name for '/%s'", oldName)).
				Placeholder(oldName).
				Value(&newName).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("name cannot be empty")
					}
					if strings.TrimSpace(s) == oldName {
						return fmt.Errorf("name cannot be the same as old name")
					}
					if wm.IsReservedCommand(s) {
						return fmt.Errorf("name '%s' is a reserved command", s)
					}
					return nil
				}).
				Run()
			if err != nil {
				return
			}
		}

		if err := wm.RenameWorkflow(oldName, newName); err != nil {
			service.Errorf("Failed to rename workflow: %v", err)
			return
		}

		fmt.Printf("Renamed '/%s' to '/%s'.\n", oldName, newName)
	},
}

var workflowInfoCmd = &cobra.Command{
	Use:     "info [name]",
	Aliases: []string{"show", "view", "cat", "i"},
	Short:   "Display workflow information",
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		wm := service.GetWorkflowManager()
		if err := wm.LoadMetadata(chatCommandMap); err != nil {
			service.Errorf("Failed to load workflows: %v", err)
			return
		}

		var name string
		if len(args) > 0 {
			name = args[0]
		} else {
			names := wm.GetWorkflowNames()
			if len(names) == 0 {
				fmt.Println("No workflows found.")
				return
			}
			options := make([]huh.Option[string], len(names))
			for i, n := range names {
				options[i] = huh.NewOption("/"+n, n)
			}
			ui.SortOptions(options, name)
			height := ui.GetTermFitHeight(len(options))
			err := huh.NewSelect[string]().
				Title("Workflow Information").
				Description("Select a workflow to view details").
				Options(options...).
				Value(&name).
				Height(height).
				Run()
			if err != nil {
				return
			}
		}

		content, desc, err := wm.GetWorkflowByName(name)
		if err != nil {
			service.Errorf("%v", err)
			return
		}

		fmt.Printf("%sWorkflow:%s\n", data.HighlightColor, data.ResetSeq)
		fmt.Printf("%s---%s\n", data.BorderColor, data.ResetSeq)
		fmt.Printf("%sName: %s%s%s\n", data.LabelColor, data.ResetSeq, name, data.ResetSeq)
		fmt.Printf("%sDescription: %s%s%s\n", data.LabelColor, data.ResetSeq, desc, data.ResetSeq)
		fmt.Printf("%s---%s\n%s\n", data.BorderColor, data.ResetSeq, content)
	},
}

var workflowSetCmd = &cobra.Command{
	Use:     "set [name]",
	Aliases: []string{"edit", "update", "mod"},
	Short:   "Modify an existing workflow",
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		wm := service.GetWorkflowManager()
		if err := wm.LoadMetadata(chatCommandMap); err != nil {
			service.Errorf("Failed to load workflows: %v", err)
			return
		}

		var name string
		if len(args) > 0 {
			name = args[0]
		} else {
			names := wm.GetWorkflowNames()
			if len(names) == 0 {
				fmt.Println("No workflows to modify.")
				return
			}
			options := make([]huh.Option[string], len(names))
			for i, n := range names {
				options[i] = huh.NewOption("/"+n, n)
			}
			ui.SortOptions(options, name)
			height := ui.GetTermFitHeight(len(options))
			err := huh.NewSelect[string]().
				Title("Modify Workflow").
				Description("Select a workflow to modify").
				Options(options...).
				Value(&name).
				Height(height).
				Run()
			if err != nil {
				return
			}
		}

		content, desc, err := wm.GetWorkflowByName(name)
		if err != nil {
			service.Errorf("%v", err)
			return
		}

		// Prompt for description
		var newDescription string = desc
		err = huh.NewInput().
			Title("Workflow Description").
			Description("Brief description of what this workflow does (for /help)").
			Value(&newDescription).
			Run()

		if err != nil {
			return
		}

		// Open editor for content
		editor := getPreferredEditor()
		tempFile, err := createTempFile(workflowTempFile)
		if err != nil {
			service.Errorf("Failed to create temp file: %v", err)
			return
		}
		defer os.Remove(tempFile)

		if err := os.WriteFile(tempFile, []byte(content), 0644); err != nil {
			service.Errorf("Failed to write to temp file: %v", err)
			return
		}

		cmdExec := exec.Command(editor, tempFile)
		cmdExec.Stdin = os.Stdin
		cmdExec.Stdout = os.Stdout
		cmdExec.Stderr = os.Stderr

		if err := cmdExec.Run(); err != nil {
			service.Errorf("Editor failed: %v", err)
			return
		}

		contentBytes, err := os.ReadFile(tempFile)
		if err != nil {
			service.Errorf("Failed to read content: %v", err)
			return
		}
		newContent := strings.TrimSpace(string(contentBytes))

		if newContent == "" {
			fmt.Println("Empty content, workflow update aborted.")
			return
		}

		if err := wm.UpdateWorkflow(name, newDescription, newContent); err != nil {
			service.Errorf("Failed to update workflow: %v", err)
			return
		}

		fmt.Printf("Workflow '/%s' updated successfully.\n", name)
	},
}
