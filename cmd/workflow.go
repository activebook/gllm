package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/internal/ui"
	"github.com/activebook/gllm/io"
	"github.com/activebook/gllm/service"
	"github.com/activebook/gllm/util"
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
	workflowCmd.AddCommand(workflowAddCmd)
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
		if err := wm.LoadMetadata(replCommandMap); err != nil {
			util.Errorf(cmd, "Failed to load workflows: %v\n", err)
			return
		}

		names := wm.GetWorkflowNames()
		if len(names) == 0 {
			util.Println(cmd, "No workflows found.")
			return
		}

		util.Println(cmd, "Available workflows:")
		for _, name := range names {
			_, desc, _ := wm.GetWorkflowByName(name)
			if desc != "" {
				util.Printf(cmd, "/%s - %s\n", name, desc)
			} else {
				util.Printf(cmd, "/%s\n", name)
			}
		}
	},
}

var workflowAddCmd = &cobra.Command{
	Use:     "add [name]",
	Aliases: []string{"create", "new", "c", "n", "a"},
	Short:   "Create a new workflow",
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		wm := service.GetWorkflowManager()
		if err := wm.LoadMetadata(replCommandMap); err != nil {
			util.Errorf(cmd, "Failed to load workflows: %v\n", err)
			return
		}

		var name string
		if len(args) > 0 {
			name = args[0]
			if err := util.ValidateResourceName("workflow", name); err != nil {
				util.Errorf(cmd, "%v\n", err)
				return
			}
			if util.Contains(wm.GetWorkflowNames(), name, true) {
				util.Errorf(cmd, "workflow '%s' already exists\n", name)
				return
			}
		} else {
			err := huh.NewInput().
				Title("Workflow Name").
				Description("Enter the name for the new workflow (e.g. 'debug')").
				Value(&name).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("name cannot be empty")
					}
					if err := util.ValidateResourceName("workflow", s); err != nil {
						return err
					}
					if util.Contains(wm.GetWorkflowNames(), s, true) {
						return fmt.Errorf("workflow '%s' already exists", s)
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
			util.Errorf(cmd, "Failed to create temp file: %v\n", err)
			return
		}
		defer os.Remove(tempFile)

		cmdExec := exec.Command(editor, tempFile)
		cmdExec.Stdin = os.Stdin
		cmdExec.Stdout = os.Stdout
		cmdExec.Stderr = os.Stderr

		if err := cmdExec.Run(); err != nil {
			util.Errorf(cmd, "Editor failed: %v\n", err)
			return
		}

		contentBytes, err := os.ReadFile(tempFile)
		if err != nil {
			util.Errorf(cmd, "Failed to read content: %v\n", err)
			return
		}
		content := strings.TrimSpace(string(contentBytes))

		if content == "" {
			util.Println(cmd, "Empty content, workflow creation aborted.")
			return
		}

		if err := wm.CreateWorkflow(name, description, content); err != nil {
			util.Errorf(cmd, "Failed to create workflow: %v\n", err)
			return
		}

		util.Printf(cmd, "Workflow '/%s' created successfully.\n", name)
	},
}

var workflowRemoveCmd = &cobra.Command{
	Use:     "remove [name]",
	Aliases: []string{"rm", "delete", "del"},
	Short:   "Remove a workflow",
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		wm := service.GetWorkflowManager()
		if err := wm.LoadMetadata(replCommandMap); err != nil {
			util.Errorf(cmd, "Failed to load workflows: %v\n", err)
			return
		}

		var name string
		if len(args) > 0 {
			name = args[0]
		} else {
			names := wm.GetWorkflowNames()
			if len(names) == 0 {
				util.Println(cmd, "No workflows to remove.")
				return
			}
			options := make([]huh.Option[string], len(names))
			for i, n := range names {
				options[i] = huh.NewOption("/"+n, n)
			}
			ui.SortOptions(options, name)
			height := io.GetTermFitHeight(len(options))
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
			util.Println(cmd, "Aborted.")
			return
		}

		if err := wm.RemoveWorkflow(name); err != nil {
			util.Errorf(cmd, "Failed to remove workflow: %v\n", err)
			return
		}

		util.Printf(cmd, "Workflow '/%s' removed.\n", name)
	},
}

var workflowRenameCmd = &cobra.Command{
	Use:     "rename [old] [new]",
	Aliases: []string{"mv", "rn"},
	Short:   "Rename a workflow",
	Args:    cobra.MaximumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		wm := service.GetWorkflowManager()
		if err := wm.LoadMetadata(replCommandMap); err != nil {
			util.Errorf(cmd, "Failed to load workflows: %v\n", err)
			return
		}

		var oldName, newName string
		if len(args) >= 1 {
			oldName = args[0]
		}
		if len(args) >= 2 {
			newName = args[1]
			if err := util.ValidateResourceName("workflow", newName); err != nil {
				util.Errorf(cmd, "%v\n", err)
				return
			}
			if util.Contains(wm.GetWorkflowNames(), newName, true) {
				util.Errorf(cmd, "workflow '%s' already exists\n", newName)
				return
			}
		}

		if oldName == "" {
			names := wm.GetWorkflowNames()
			if len(names) == 0 {
				util.Println(cmd, "No workflows to rename.")
				return
			}
			options := make([]huh.Option[string], len(names))
			for i, n := range names {
				options[i] = huh.NewOption("/"+n, n)
			}
			ui.SortOptions(options, oldName)
			height := io.GetTermFitHeight(len(options))
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
			newName = oldName
			err := huh.NewInput().
				Title("New Name").
				Description(fmt.Sprintf("Enter new name for '/%s'", oldName)).
				Value(&newName).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("name cannot be empty")
					}
					if err := util.ValidateResourceName("workflow", s); err != nil {
						return err
					}
					if !strings.EqualFold(s, oldName) && util.Contains(wm.GetWorkflowNames(), s, true) {
						return fmt.Errorf("workflow '%s' already exists", s)
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
			util.Errorf(cmd, "Failed to rename workflow: %v\n", err)
			return
		}

		util.Printf(cmd, "Renamed '/%s' to '/%s'.\n", oldName, newName)
	},
}

var workflowInfoCmd = &cobra.Command{
	Use:     "info [name]",
	Aliases: []string{"show", "view", "cat", "i"},
	Short:   "Display workflow information",
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		wm := service.GetWorkflowManager()
		if err := wm.LoadMetadata(replCommandMap); err != nil {
			util.Errorf(cmd, "Failed to load workflows: %v\n", err)
			return
		}

		var name string
		if len(args) > 0 {
			name = args[0]
		} else {
			names := wm.GetWorkflowNames()
			if len(names) == 0 {
				util.Println(cmd, "No workflows found.")
				return
			}
			options := make([]huh.Option[string], len(names))
			for i, n := range names {
				options[i] = huh.NewOption("/"+n, n)
			}
			ui.SortOptions(options, name)
			height := io.GetTermFitHeight(len(options))
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
			util.Errorf(cmd, "%v\n", err)
			return
		}

		util.Printf(cmd, "%sWorkflow:%s\n", data.HighlightColor, data.ResetSeq)
		util.Printf(cmd, "%s---%s\n", data.BorderColor, data.ResetSeq)
		util.Printf(cmd, "%sName: %s%s%s\n", data.LabelColor, data.ResetSeq, name, data.ResetSeq)
		util.Printf(cmd, "%sDescription: %s%s%s\n", data.LabelColor, data.ResetSeq, desc, data.ResetSeq)
		util.Printf(cmd, "%s---%s\n%s\n", data.BorderColor, data.ResetSeq, content)
	},
}

var workflowSetCmd = &cobra.Command{
	Use:     "set [name]",
	Aliases: []string{"edit", "update", "mod"},
	Short:   "Modify an existing workflow",
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		wm := service.GetWorkflowManager()
		if err := wm.LoadMetadata(replCommandMap); err != nil {
			util.Errorf(cmd, "Failed to load workflows: %v\n", err)
			return
		}

		var name string
		if len(args) > 0 {
			name = args[0]
		} else {
			names := wm.GetWorkflowNames()
			if len(names) == 0 {
				util.Println(cmd, "No workflows to modify.")
				return
			}
			options := make([]huh.Option[string], len(names))
			for i, n := range names {
				options[i] = huh.NewOption("/"+n, n)
			}
			ui.SortOptions(options, name)
			height := io.GetTermFitHeight(len(options))
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
			util.Errorf(cmd, "%v\n", err)
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
			util.Errorf(cmd, "Failed to create temp file: %v\n", err)
			return
		}
		defer os.Remove(tempFile)

		if err := os.WriteFile(tempFile, []byte(content), 0644); err != nil {
			util.Errorf(cmd, "Failed to write to temp file: %v\n", err)
			return
		}

		cmdExec := exec.Command(editor, tempFile)
		cmdExec.Stdin = os.Stdin
		cmdExec.Stdout = os.Stdout
		cmdExec.Stderr = os.Stderr

		if err := cmdExec.Run(); err != nil {
			util.Errorf(cmd, "Editor failed: %v\n", err)
			return
		}

		contentBytes, err := os.ReadFile(tempFile)
		if err != nil {
			util.Errorf(cmd, "Failed to read content: %v\n", err)
			return
		}
		newContent := strings.TrimSpace(string(contentBytes))

		if newContent == "" {
			util.Println(cmd, "Empty content, workflow update aborted.")
			return
		}

		if err := wm.UpdateWorkflow(name, newDescription, newContent); err != nil {
			util.Errorf(cmd, "Failed to update workflow: %v\n", err)
			return
		}

		util.Printf(cmd, "Workflow '/%s' updated successfully.\n", name)
	},
}
