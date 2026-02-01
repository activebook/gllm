// File: cmd/editor.go
package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/internal/ui"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

// editorCmd represents the editor command
var editorCmd = &cobra.Command{
	Use:   "editor [NAME]",
	Short: "Manage preferred text editor for multi-line input",
	Run: func(cmd *cobra.Command, args []string) {
		listAvailableEditors()
	},
}

// editorSwitchCmd represents the editor switch command
var editorSwitchCmd = &cobra.Command{
	Use:     "switch [NAME]",
	Aliases: []string{"sw", "select", "sel"},
	Short:   "Switch to a different text editor",
	RunE: func(cmd *cobra.Command, args []string) error {
		var name string
		store := data.GetSettingsStore()

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
				options = append(options, huh.NewOption(ed, ed))
			}
			ui.SortOptions(options, name)

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

func init() {
	configCmd.AddCommand(editorCmd)
	editorCmd.AddCommand(editorSwitchCmd)
}

// getPreferredEditor gets the user's preferred editor from config or detects it
func getPreferredEditor() string {
	store := data.GetSettingsStore()

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
	store := data.GetSettingsStore()

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
	fmt.Println()

	commonEditors := []string{"vim", "vi", "nvim", "neovim", "nano", "pico", "emacs", "emacsclient", "code", "code-insiders", "subl", "sublime_text", "atom", "gedit", "pluma", "kate", "kwrite", "notepad.exe", "notepad++", "textedit"}
	store := data.GetSettingsStore()
	current := store.GetEditor()

	for _, editor := range commonEditors {
		enableIndicator := ui.FormatEnabledIndicator(editor == current)
		if _, err := exec.LookPath(editor); err == nil {
			pname := editor
			fmt.Printf("  %s %-14s %s%s%s\n", enableIndicator, pname, data.SwitchOnColor, "(installed)", data.ResetSeq)
		} else {
			fmt.Printf("  %s %-14s %s%s%s\n", enableIndicator, editor, data.SwitchOffColor, "(not found)", data.ResetSeq)
		}
	}

	if current != "" {
		fmt.Printf("\n%s = Current preferred editor\n", ui.FormatEnabledIndicator(true))
	} else {
		fmt.Println("\nNo preferred editor set. Use 'gllm editor switch' to select one.")
	}
}
