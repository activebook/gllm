// File: cmd/editor.go
package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/activebook/gllm/data"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

// editorCmd represents the editor command
var editorCmd = &cobra.Command{
	Use:   "editor [NAME]",
	Short: "Manage preferred text editor for multi-line input",
	Long: `Manage the preferred text editor used for multi-line input editing.
This allows you to set, check, and list available text editors for use
with the /editor command in chat sessions.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			editorListCmd.Run(cmd, args)
			return
		}

		// Fallback for script compatibility
		arg := args[0]
		switch arg {
		case "list", "ls", "pr":
			editorListCmd.Run(cmd, args)
		case "switch", "sw", "select":
			// Handle error if switch fails
			if err := editorSwitchCmd.RunE(cmd, args[1:]); err != nil {
				fmt.Printf("Error switching editor: %v\n", err)
			}
		default:
			if err := setPreferredEditor(arg); err != nil {
				fmt.Printf("Error setting preferred editor: %v\n", err)
			}
		}
	},
}

// editorSwitchCmd represents the editor switch command
var editorSwitchCmd = &cobra.Command{
	Use:     "switch [NAME]",
	Aliases: []string{"sw", "select"},
	Short:   "Switch to a different text editor",
	RunE: func(cmd *cobra.Command, args []string) error {
		var name string
		store := data.NewConfigStore()

		if len(args) > 0 {
			name = args[0]
		} else {
			commonEditors := []string{"vim", "vi", "nvim", "neovim", "nano", "pico", "emacs", "emacsclient", "code", "code-insiders", "subl", "sublime_text", "atom", "gedit", "pluma", "kate", "kwrite", "notepad.exe", "notepad++", "textedit"}

			var installed []string
			for _, ed := range commonEditors {
				if _, err := exec.LookPath(ed); err == nil {
					installed = append(installed, ed)
				}
			}

			if len(installed) == 0 {
				return fmt.Errorf("no supported text editors found in PATH")
			}

			name = store.GetEditor()

			var options []huh.Option[string]
			for _, ed := range installed {
				label := ed
				if ed == name {
					label = highlightColor(ed + " (active)")
				}
				options = append(options, huh.NewOption(label, ed))
			}

			err := huh.NewSelect[string]().
				Title("Select Preferred Editor").
				Options(options...).
				Value(&name).
				Run()
			if err != nil {
				return nil
			}
		}

		return setPreferredEditor(name)
	},
}

// editorListCmd represents the editor list subcommand
var editorListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls", "pr"},
	Short:   "List all available text editors",
	Long:    `List all available text editors on the system with their installation status.`,
	Run: func(cmd *cobra.Command, args []string) {
		listAvailableEditors()
	},
}

func init() {
	rootCmd.AddCommand(editorCmd)
	editorCmd.AddCommand(editorListCmd)
	editorCmd.AddCommand(editorSwitchCmd)
}

// getPreferredEditor gets the user's preferred editor from config or detects it
func getPreferredEditor() string {
	store := data.NewConfigStore()

	// Check config first
	editor := store.GetEditor()
	if editor != "" {
		return editor
	}

	// Check environment variables
	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}

	// Check common editors in priority order
	commonEditors := []string{"vim", "nano", "vi", "emacs", "code", "gedit", "notepad.exe"}
	for _, editor := range commonEditors {
		if _, err := exec.LookPath(editor); err == nil {
			return editor
		}
	}

	// Ultimate fallback
	return ""
}

// setPreferredEditor sets the user's preferred editor in config
func setPreferredEditor(editor string) error {
	store := data.NewConfigStore()

	// Check if editor exists
	if _, err := exec.LookPath(editor); err != nil {
		fmt.Printf("Editor '%s' is not found in PATH.\n", editor)

		// Find the best available editor (ignoring current config)
		var bestEditor string

		// Check environment variables first
		if envEditor := os.Getenv("EDITOR"); envEditor != "" {
			if _, err := exec.LookPath(envEditor); err == nil {
				bestEditor = envEditor
			}
		}

		// If no valid env editor, check common editors
		if bestEditor == "" {
			// Check common editors in priority order
			commonEditors := []string{"vim", "nano", "vi", "emacs", "code", "gedit", "notepad.exe"}
			for _, ed := range commonEditors {
				if _, err := exec.LookPath(ed); err == nil {
					bestEditor = ed
					break
				}
			}
		}

		// Ultimate fallback
		if bestEditor == "" {
			bestEditor = "vim"
		}

		fmt.Printf("Using '%s' instead.\n", bestEditor)
		if err := store.SetEditor(bestEditor); err != nil {
			return fmt.Errorf("failed to save preferred editor: %w", err)
		}
	} else {
		// Editor exists, set it
		if err := store.SetEditor(editor); err != nil {
			return fmt.Errorf("failed to save preferred editor: %w", err)
		}
		fmt.Printf("Preferred editor set to: %s\n", editor)
	}

	return nil
}

// listAvailableEditors shows all available editors
func listAvailableEditors() {
	fmt.Println("Available editors:")

	commonEditors := []string{"vim", "vi", "nvim", "neovim", "nano", "pico", "emacs", "emacsclient", "code", "code-insiders", "subl", "sublime_text", "atom", "gedit", "pluma", "kate", "kwrite", "notepad.exe", "notepad++", "textedit"}

	store := data.NewConfigStore()
	current := store.GetEditor()

	for _, editor := range commonEditors {
		if _, err := exec.LookPath(editor); err == nil {
			indicator := "  "
			pname := fmt.Sprintf("%-14s", editor)
			if editor == current {
				indicator = highlightColor("* ")
				pname = highlightColor(pname)
			}
			fmt.Printf("%s%s %s\n", indicator, pname, greenColor("(installed)"))
		} else {
			fmt.Printf("  %-14s %s\n", editor, grayColor("(not found)"))
		}
	}

	if current != "" {
		fmt.Printf("\n(*) Indicates the current preferred editor.\n")
	} else {
		fmt.Println("\nNo preferred editor set. Use 'gllm editor switch' to select one.")
	}
}
