package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// editorCmd represents the editor command
var editorCmd = &cobra.Command{
	Use:   "editor [editor_name]",
	Short: "Manage preferred text editor for multi-line input",
	Long: `Manage the preferred text editor used for multi-line input editing.
This allows you to set, check, and list available text editors for use
with the /editor command in chat sessions.

Examples:
  gllm editor           # Show current editor
  gllm editor vim       # Set vim as preferred editor
  gllm editor list      # List all available editors`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			// Show current editor
			current := viper.GetString("chat.editor")
			if current != "" {
				fmt.Printf("Current preferred editor: %s\n", current)
			} else {
				fmt.Println("No preferred editor set.")
				fmt.Println("Use 'gllm editor list' to see available editors.")
			}
			return
		}

		if args[0] == "list" || args[0] == "pr" {
			listAvailableEditors()
		} else {
			// Set editor
			setPreferredEditor(args[0])
		}
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
}

// getPreferredEditor gets the user's preferred editor from config or detects it
func getPreferredEditor() string {
	// Check config first
	editor := viper.GetString("chat.editor")
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
		viper.Set("chat.editor", bestEditor)
	} else {
		// Editor exists, set it
		viper.Set("chat.editor", editor)
		fmt.Printf("Preferred editor set to: %s\n", editor)
	}

	if err := writeConfig(); err != nil {
		return fmt.Errorf("failed to save preferred editor: %w", err)
	}

	return nil
}

// listAvailableEditors shows all available editors
func listAvailableEditors() {
	fmt.Println("Available editors:")
	fmt.Println("==================")

	commonEditors := []string{"vim", "vi", "nvim", "neovim", "nano", "pico", "emacs", "emacsclient", "code", "code-insiders", "subl", "sublime_text", "atom", "gedit", "pluma", "kate", "kwrite", "notepad.exe", "notepad++", "textedit"}

	green := color.New(color.FgGreen).SprintFunc()
	gray := color.New(color.FgHiBlack).SprintFunc()

	for _, editor := range commonEditors {
		if _, err := exec.LookPath(editor); err == nil {
			fmt.Printf("[%s] %s (installed)\n", green("✔"), editor)
		} else {
			fmt.Printf("[%s] %s (not found)\n", gray("x"), editor)
		}
	}

	current := viper.GetString("chat.editor")
	if current != "" {
		fmt.Printf("\nCurrent preferred: %s\n", current)
	} else {
		fmt.Println("\nNo preferred editor set.")
	}

	fmt.Println("\nUsage:")
	fmt.Println("  gllm editor <name>      - Set preferred editor")
	fmt.Println("  gllm editor list        - Show this list")
	fmt.Println("  gllm editor             - Show current editor")
}
