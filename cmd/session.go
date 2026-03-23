package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/activebook/gllm/internal/ui"
	"github.com/activebook/gllm/io"
	"github.com/activebook/gllm/service"
	"github.com/activebook/gllm/util"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var ()

// sessionCmd represents the session command
var sessionCmd = &cobra.Command{
	Use:     "session",
	Aliases: []string{"s"},
	Short:   "Manage sessions",
	Long:    `Commands to list, remove, and show details of sessions.`,
	Args:    cobra.NoArgs,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return []string{"list", "remove", "info", "clear", "rename", "share"}, cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return sessionListCmd.RunE(cmd, args)
	},
}

// sessionListCmd represents the session list command
var sessionListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all sessions",
	Long:    `List all available sessions in sorted order.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		sessions, err := service.ListSortedSessions(false, true)
		if err != nil {
			fmt.Println(err)
			return nil
		}
		if len(sessions) == 0 {
			fmt.Println("No sessions found.")
			return nil
		}

		fmt.Println("Available sessions:")
		width := len(fmt.Sprintf("%d", len(sessions)))
		for index, session := range sessions {
			// Visually indent sub-sessions
			prefix := ""
			if strings.Contains(session.Name, ":") {
				prefix = "└─ "
			}

			// Display with title if available
			if session.Provider != "" {
				fmt.Printf("[%*d] %s%s [%s]\n", width, index+1, prefix, session.Name, session.Provider)
			} else {
				fmt.Printf("[%*d] %s%s\n", width, index+1, prefix, session.Name)
			}
		}
		return nil
	},
}

// sessionRemoveCmd represents the session remove command
var sessionRemoveCmd = &cobra.Command{
	Use:     "remove [session|pattern|index]",
	Aliases: []string{"rm"},
	Short:   "Remove a session by name or index, or multiple sessions using a pattern",
	Long: `Remove a specific session by name or index, or multiple sessions using a pattern with wildcards.
This action cannot be undone.

Examples:
gllm session remove chat_123
gllm session remove "chat_*" --force
gllm session remove 1 --force
gllm session remove 10-20 --force
gllm session remove "2 - 5" --force`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var names []string
		var pattern string
		if len(args) > 0 {
			pattern = args[0]
		} else {
			// Select sessions to remove
			sessions, err := service.ListSortedSessions(false, true)
			if err != nil || len(sessions) == 0 {
				fmt.Println("No sessions found.")
				return nil
			}

			var options []huh.Option[string]
			for _, c := range sessions {
				label := c.Name
				if c.Provider != "" {
					label = fmt.Sprintf("%s [%s]", c.Name, c.Provider)
				}
				options = append(options, huh.NewOption(label, c.Name))
			}
			height := io.GetTermFitHeight(len(options))

			err = huh.NewMultiSelect[string]().
				Title("Select Sessions to Remove").
				Description("Choose one or more sessions to delete permanently").
				Height(height).
				Options(options...).
				Value(&names).
				Run()
			if err != nil {
				return nil
			}
			if len(names) == 0 {
				return nil
			}
		}

		if pattern != "" {
			// Check if pattern is a range
			rangePattern := strings.ReplaceAll(pattern, " ", "")
			rangeParts := strings.Split(rangePattern, "-")
			if len(rangeParts) == 2 {
				// Handle range removal
				start, err1 := strconv.Atoi(rangeParts[0])
				end, err2 := strconv.Atoi(rangeParts[1])

				if err1 == nil && err2 == nil {
					sessions, err := service.ListSortedSessions(false, false)
					if err != nil {
						fmt.Println(err)
						return nil
					}
					if len(sessions) == 0 {
						fmt.Println("No sessions found.")
						return nil
					}

					// Validate range
					if start < 1 || end < 1 || start > len(sessions) || end > len(sessions) {
						return fmt.Errorf("range %d-%d out of range (1-%d)", start, end, len(sessions))
					}
					if start > end {
						return fmt.Errorf("invalid range: start (%d) cannot be greater than end (%d)", start, end)
					}

					// Collect matching directories in range
					for i := start; i <= end; i++ {
						names = append(names, sessions[i-1].Name)
					}
				} else {
					// Not a valid range, treat as regular pattern
					var err error
					names, err = service.FindSessionsByPattern(pattern)
					if err != nil {
						fmt.Printf("Error finding sessions: %v\n", err)
						return nil
					}
				}
			} else {
				// Regular pattern handling
				var err error
				names, err = service.FindSessionsByPattern(pattern)
				if err != nil {
					fmt.Printf("Error finding sessions: %v\n", err)
					return nil
				}
			}
		}

		if len(names) == 0 {
			if pattern != "" {
				fmt.Printf("No sessions found matching '%s'.\n", pattern)
			}
			return nil
		}

		// Ask for confirmation if not forced
		force, _ := cmd.Flags().GetBool("force")
		if !force {
			fmt.Printf("The following sessions will be removed:\n")
			for _, name := range names {
				fmt.Printf("  - %s\n", name)
			}

			var confirm bool
			err := huh.NewConfirm().
				Title(fmt.Sprintf("Are you sure you want to remove %d sessions?", len(names))).
				Value(&confirm).
				Run()
			if err != nil || !confirm {
				fmt.Println("Operation cancelled.")
				return nil
			}
		}

		// Remove the matching sessions
		for _, name := range names {
			if err := service.RemoveSession(name); err != nil {
				fmt.Printf("Failed to remove '%s': %v\n", name, err)
			} else {
				fmt.Printf("Session '%s' removed successfully.\n", name)
			}
		}

		return nil
	},
}

var sessionClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all sessions",
	Long: `Remove all saved sessions.
This action cannot be undone.

Example:
gllm session clear
gllm session clear --force`,
	RunE: func(cmd *cobra.Command, args []string) error {
		sessions, err := service.ListSortedSessions(false, false)
		if err != nil || len(sessions) == 0 {
			fmt.Println("No sessions found.")
			return nil
		}

		force, _ := cmd.Flags().GetBool("force")

		if !force {
			var confirm bool
			err := huh.NewConfirm().
				Title("Are you sure you want to clear ALL saved sessions?").
				Affirmative("Yes, delete all").
				Value(&confirm).
				Run()
			if err != nil || !confirm {
				fmt.Println("Operation cancelled.")
				return nil
			}
		}

		for _, session := range sessions {
			name := session.Name
			if err := service.RemoveSession(name); err != nil {
				fmt.Printf("failed to remove session directory %s: %v\n", name, err)
			} else {
				fmt.Printf("  - '%s' removed.\n", name)
			}
		}

		fmt.Println("All sessions have been cleared.")
		return nil
	},
}

// sessionInfoCmd represents the session info command
var sessionInfoCmd = &cobra.Command{
	Use:     "info [session|index]",
	Aliases: []string{"in"},
	Short:   "Show session details",
	Long:    `Display detailed information about a specific session.`,
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var sessionName string
		if len(args) > 0 {
			sessionName = args[0]
		} else {
			// Select session
			sessions, err := service.ListSortedSessions(false, true)
			if err != nil || len(sessions) == 0 {
				fmt.Println("No sessions found.")
				return nil
			}

			var options []huh.Option[string]
			for _, c := range sessions {
				label := c.Name
				if c.Provider != "" {
					label = fmt.Sprintf("%s [%s]", c.Name, c.Provider)
				}
				options = append(options, huh.NewOption(label, c.Name))
			}
			height := io.GetTermFitHeight(len(options))

			err = huh.NewSelect[string]().
				Title("Select Session").
				Description("Choose a session to view its logs").
				Height(height).
				Options(options...).
				Value(&sessionName).
				Run()
			if err != nil {
				return nil
			}
		}

		// Try to resolve name if it's an index
		resolvedName, err := service.FindSessionByIndex(sessionName)
		if err != nil {
			return err
		}
		if resolvedName == "" {
			return fmt.Errorf("session '%s' not found", sessionName)
		}
		sessionName = resolvedName

		// Check if session exists
		if !service.SessionExists(sessionName) {
			fmt.Printf("Session '%s' not found.\n", sessionName)
			return nil
		}

		// Read the session file
		data, err := service.ReadSessionContent(sessionName)
		if err != nil {
			return fmt.Errorf("error reading session file: %v", err)
		}

		// Display session details
		fmt.Printf("Name: %s\n", sessionName)

		// Detect provider based on message format
		provider := service.DetectMessageProviderByContent(data)

		// Process and display messages based on provider
		var content string
		switch provider {
		case service.ModelProviderGemini:
			content = service.RenderGeminiSessionLog(data)
		case service.ModelProviderOpenAI, service.ModelProviderOpenAICompatible:
			content = service.RenderOpenAISessionLog(data)
		case service.ModelProviderAnthropic:
			content = service.RenderAnthropicSessionLog(data)
		default:
			fmt.Println("Unknown session format.")
			return nil
		}

		// Show viewport
		m := ui.NewViewportModel(provider, content, func() string {
			return fmt.Sprintf("Session: %s", sessionName)
		})

		p := tea.NewProgram(m, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("error running viewport: %v", err)
		}
		return nil
	},
}

// sessionRenameCmd represents the session rename command
var sessionRenameCmd = &cobra.Command{
	Use:   "rename [oldname|index] [newname]",
	Short: "Rename a session",
	Long:  `Rename an existing session to a new name.`,
	Args:  cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		var oldName, newName string

		if len(args) >= 2 {
			oldName = args[0]
			newName = args[1]
			if err := util.ValidateResourceName("session", newName); err != nil {
				util.Errorf("%v\n", err)
				return err
			}
		} else {
			// Select session to rename
			sessions, err := service.ListSortedSessions(false, true)
			if err != nil || len(sessions) == 0 {
				fmt.Println("No sessions found.")
				return nil
			}

			if len(args) == 1 {
				oldName = args[0]
			} else {
				var options []huh.Option[string]
				for _, c := range sessions {
					label := c.Name
					if c.Provider != "" {
						label = fmt.Sprintf("%s [%s]", c.Name, c.Provider)
					}
					options = append(options, huh.NewOption(label, c.Name))
				}
				height := io.GetTermFitHeight(len(options))

				err = huh.NewSelect[string]().
					Title("Select Session to Rename").
					Description("Choose the session you wish to rename").
					Height(height).
					Options(options...).
					Value(&oldName).
					Run()
				if err != nil {
					return nil
				}
			}

			// Get new name
			newName = oldName // set default value
			err = huh.NewInput().
				Title("New Name").
				Value(&newName).
				Validate(func(s string) error {
					s = strings.TrimSpace(s)
					if s == "" {
						return fmt.Errorf("new name cannot be empty")
					}
					if strings.EqualFold(s, oldName) {
						return fmt.Errorf("name cannot be the same as old name")
					}
					if err := util.ValidateResourceName("session", s); err != nil {
						return err
					}
					return nil
				}).
				Run()
			if err != nil {
				return nil
			}
		}

		// Try to resolve name if it's an index
		resolvedName, err := service.FindSessionByIndex(oldName)
		if err != nil {
			return err
		}
		if resolvedName == "" {
			return fmt.Errorf("session '%s' not found", oldName)
		}
		oldName = resolvedName

		if strings.EqualFold(oldName, newName) {
			fmt.Println("No changes made.")
			return nil
		}

		// Ask for confirmation
		force, _ := cmd.Flags().GetBool("force")
		if !force {
			var confirm bool
			err := huh.NewConfirm().
				Title(fmt.Sprintf("Rename session '%s' to '%s'?", oldName, newName)).
				Value(&confirm).
				Run()
			if err != nil || !confirm {
				fmt.Println("Operation cancelled.")
				return nil
			}
		}

		// Perform the rename
		if err := service.RenameSession(oldName, newName); err != nil {
			return err
		}

		fmt.Printf("Session renamed from '%s' to '%s' successfully.\n", oldName, newName)
		return nil
	},
}

// sessionShareCmd represents the session share command
var sessionShareCmd = &cobra.Command{
	Use:     "share [session|index] [destination]",
	Aliases: []string{"export"},
	Short:   "Share/Export a session",
	Long:    `Export a session to a specified file path.`,
	Args:    cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionName := args[0]
		var destPath string
		if len(args) > 1 {
			destPath = args[1]
		}

		// Try to resolve name if it's an index
		resolvedName, err := service.FindSessionByIndex(sessionName)
		if err != nil {
			return err
		}
		if resolvedName == "" {
			return fmt.Errorf("session '%s' not found", sessionName)
		}
		sessionName = resolvedName

		if err := service.ExportSession(sessionName, destPath); err != nil {
			return err
		}

		fmt.Printf("Session '%s' exported successfully\n", sessionName)
		return nil
	},
}

func init() {
	// Add session command to root command
	rootCmd.AddCommand(sessionCmd)

	// Add subcommands to session command
	sessionCmd.AddCommand(sessionListCmd)
	sessionCmd.AddCommand(sessionRemoveCmd)
	sessionCmd.AddCommand(sessionInfoCmd)
	sessionCmd.AddCommand(sessionClearCmd)
	sessionCmd.AddCommand(sessionRenameCmd)
	sessionCmd.AddCommand(sessionShareCmd)

	// Add flags for other prompt commands if needed in the future
	sessionRemoveCmd.Flags().BoolP("force", "f", false, "Skip confirm")
	sessionClearCmd.Flags().BoolP("force", "f", false, "Force clear all without confirmation")
	sessionRenameCmd.Flags().BoolP("force", "f", false, "Skip confirm")
}
