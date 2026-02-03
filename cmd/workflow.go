package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/activebook/gllm/internal/ui"
	"github.com/activebook/gllm/service"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
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
	workflowCmd.AddCommand(workflowShowCmd)
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
	Use:     "create <name>",
	Aliases: []string{"new", "add", "c", "n"},
	Short:   "Create a new workflow",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		wm := service.GetWorkflowManager()
		if err := wm.LoadMetadata(chatCommandMap); err != nil {
			service.Errorf("Failed to load workflows: %v", err)
			return
		}

		if wm.IsReservedCommand(name) {
			service.Errorf("Cannot create workflow '%s': conflicts with reserved command", name)
			return
		}

		// Prompt for description
		var description string
		input := huh.NewInput().
			Title("Workflow Description").
			Description("Brief description of what this workflow does (for /help)").
			Value(&description)

		if err := input.WithKeyMap(ui.GetHuhKeyMap()).Run(); err != nil {
			return
		}

		// Open editor for content
		editor := getPreferredEditor()
		tempFile, err := createTempFile(".gllm-workflow-*.md")
		if err != nil {
			service.Errorf("Failed to create temp file: %v", err)
			return
		}
		defer os.Remove(tempFile)

		// Pre-fill with a template or instruction?
		// Plan said "Users add via /workflow command... no template needed"
		// But maybe a comment to guide them?
		// "Enter your workflow instructions here."

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
	Use:     "remove <name>",
	Aliases: []string{"rm", "delete", "del"},
	Short:   "Remove a workflow",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		wm := service.GetWorkflowManager()
		if err := wm.LoadMetadata(chatCommandMap); err != nil {
			service.Errorf("Failed to load workflows: %v", err)
			return
		}

		// Confirm
		var confirm bool
		err := huh.NewConfirm().
			Title(fmt.Sprintf("Are you sure you want to remove workflow '/%s'?", name)).
			Affirmative("Yes").
			Negative("No").
			Value(&confirm).
			WithKeyMap(ui.GetHuhKeyMap()).
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
	Use:     "rename <old> <new>",
	Aliases: []string{"mv"},
	Short:   "Rename a workflow",
	Args:    cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		oldName := args[0]
		newName := args[1]
		wm := service.GetWorkflowManager()
		if err := wm.LoadMetadata(chatCommandMap); err != nil {
			service.Errorf("Failed to load workflows: %v", err)
			return
		}

		if err := wm.RenameWorkflow(oldName, newName); err != nil {
			service.Errorf("Failed to rename workflow: %v", err)
			return
		}

		fmt.Printf("Renamed '/%s' to '/%s'.\n", oldName, newName)
	},
}

var workflowShowCmd = &cobra.Command{
	Use:     "show <name>",
	Aliases: []string{"cat", "view"},
	Short:   "Display workflow content",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		wm := service.GetWorkflowManager()
		if err := wm.LoadMetadata(chatCommandMap); err != nil {
			service.Errorf("Failed to load workflows: %v", err)
			return
		}

		content, desc, err := wm.GetWorkflowByName(name)
		if err != nil {
			service.Errorf("%v", err)
			return
		}

		fmt.Printf("Workflow: /%s\n", name)
		if desc != "" {
			fmt.Printf("Description: %s\n", desc)
		}
		fmt.Println("---")
		fmt.Println(content)
	},
}
