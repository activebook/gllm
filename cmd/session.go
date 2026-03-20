package cmd

import (
	"fmt"
	"os"
	"path/filepath"
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
		sessionDir := service.GetSessionsDir()

		// Check if directory exists
		if _, err := os.Stat(sessionDir); os.IsNotExist(err) {
			fmt.Println("No sessions found.")
			return nil
		}

		sessions, err := service.ListSortedSessions(sessionDir, false, true)
		if err != nil {
			fmt.Println(err)
			return nil
		}
		if len(sessions) == 0 {
			fmt.Println("No sessions found.")
			return nil
		}

		fmt.Println("Available sessions:")
		for index, session := range sessions {
			// Display with title if available
			if session.Provider != "" {
				fmt.Printf("- [%d] %s [%s]\n", index+1, session.Name, session.Provider)
			} else {
				fmt.Printf("- [%d] %s\n", index+1, session.Name)
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
		sessionDir := service.GetSessionsDir()
		var matches []string
		var pattern string
		if len(args) > 0 {
			pattern = args[0]
		} else {
			// Select sessions to remove
			sessions, err := service.ListSortedSessions(sessionDir, false, true)
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

			var selected []string
			err = huh.NewMultiSelect[string]().
				Title("Select Sessions to Remove").
				Description("Choose one or more sessions to delete permanently").
				Height(height).
				Options(options...).
				Value(&selected).
				Run()
			if err != nil {
				return nil
			}
			if len(selected) == 0 {
				return nil
			}

			// We treat selected names as specific matches
			var matches []string
			var names []string
			for _, s := range selected {
				names = append(names, s+".jsonl")
				matches = append(matches, filepath.Join(sessionDir, s+".jsonl"))
			}

			// Confirm if not forced
			force, _ := cmd.Flags().GetBool("force")
			if !force {
				var confirm bool
				err = huh.NewConfirm().
					Title(fmt.Sprintf("Are you sure you want to remove %d sessions?", len(matches))).
					Description(strings.Join(names, "\n")).
					Value(&confirm).
					Run()
				if err != nil || !confirm {
					fmt.Println("Operation cancelled.")
					return nil
				}
			}

			for _, match := range matches {
				if err := os.Remove(match); err != nil {
					fmt.Printf("Failed to remove '%s': %v\n", strings.TrimSuffix(filepath.Base(match), filepath.Ext(match)), err)
				} else {
					fmt.Printf("Session '%s' removed successfully.\n", strings.TrimSuffix(filepath.Base(match), filepath.Ext(match)))
				}
			}
			return nil
		}

		// Check if pattern is a range
		rangePattern := strings.ReplaceAll(pattern, " ", "")
		rangeParts := strings.Split(rangePattern, "-")
		if len(rangeParts) == 2 {
			// Handle range removal
			start, err1 := strconv.Atoi(rangeParts[0])
			end, err2 := strconv.Atoi(rangeParts[1])

			if err1 == nil && err2 == nil {
				sessions, err := service.ListSortedSessions(sessionDir, false, true)
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

				// Collect matching files in range
				for i := start; i <= end; i++ {
					sessionPath := filepath.Join(sessionDir, sessions[i-1].Name+".jsonl")
					matches = append(matches, sessionPath)
				}
			} else {
				// Not a valid range, treat as regular pattern
				matches = handleAsPattern(pattern, sessionDir)
			}
		} else {
			// Regular pattern handling
			matches = handleAsPattern(pattern, sessionDir)
		}

		if len(matches) == 0 {
			fmt.Printf("No sessions found matching '%s'.\n", pattern)
			return nil
		}

		// Ask for confirmation if not forced
		force, _ := cmd.Flags().GetBool("force")
		if !force {
			fmt.Printf("The following sessions will be removed:\n")
			for _, match := range matches {
				fmt.Printf("  - %s\n", strings.TrimSuffix(filepath.Base(match), filepath.Ext(match)))
			}

			var confirm bool
			err := huh.NewConfirm().
				Title("Are you sure you want to remove these sessions?").
				Value(&confirm).
				Run()
			if err != nil || !confirm {
				fmt.Println("Operation cancelled.")
				return nil
			}
		}

		// Remove the matching files
		for _, match := range matches {
			if err := os.Remove(match); err != nil {
				fmt.Printf("Failed to remove '%s': %v\n", strings.TrimSuffix(filepath.Base(match), filepath.Ext(match)), err)
			} else {
				fmt.Printf("Session '%s' removed successfully.\n", strings.TrimSuffix(filepath.Base(match), filepath.Ext(match)))
			}
		}

		return nil
	},
}

// handleAsPattern handles the pattern as either an index or a file pattern
func handleAsPattern(pattern string, sessionDir string) []string {
	var matches []string

	// Try to parse as index
	index, err := strconv.Atoi(pattern)
	if err == nil {
		sessions, err := service.ListSortedSessions(sessionDir, false, true)
		if err != nil {
			fmt.Println(err)
			return matches
		}
		if len(sessions) == 0 {
			return matches
		}
		if index >= 1 && index <= len(sessions) {
			// Use the resolved file name as the pattern
			pattern = sessions[index-1].Name
		}
	}

	// Now pattern is either a name or a wildcard
	sessionPathPattern := filepath.Join(sessionDir, pattern+".jsonl")

	// Find matching files using the pattern
	matches, err = filepath.Glob(sessionPathPattern)
	if err != nil {
		fmt.Printf("Failed to parse pattern: %v\n", err)
		return []string{}
	}

	return matches
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
		sessionDir := service.GetSessionsDir()
		// Check if directory exists
		if _, err := os.Stat(sessionDir); os.IsNotExist(err) {
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

		files, err := os.ReadDir(sessionDir)
		if err != nil {
			return fmt.Errorf("fail to read session directory: %v", err)
		}

		for _, file := range files {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".jsonl") {
				name := strings.TrimSuffix(file.Name(), ".jsonl")
				if err := os.Remove(filepath.Join(sessionDir, name+".jsonl")); err != nil {
					return fmt.Errorf("failed to remove session: %v", err)
				}
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
		sessionDir := service.GetSessionsDir()
		var sessionName string
		if len(args) > 0 {
			sessionName = args[0]
		} else {
			// Select session
			sessions, err := service.ListSortedSessions(sessionDir, false, true)
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

		// If sessionName is a number, treat it as an index
		index, err := strconv.Atoi(sessionName)
		if err == nil {
			sessions, err := service.ListSortedSessions(sessionDir, false, true)
			if err != nil {
				fmt.Println(err)
				return nil
			}
			if len(sessions) == 0 {
				fmt.Println("No sessions found.")
				return nil
			}
			if index < 1 || index > len(sessions) {
				return fmt.Errorf("index %d out of range (1-%d)", index, len(sessions))
			}
			sessionName = sessions[index-1].Name
		}

		sessionPath := filepath.Join(sessionDir, sessionName+".jsonl")

		// Check if session exists
		if _, err := os.Stat(sessionPath); os.IsNotExist(err) {
			fmt.Printf("Session '%s' not found.\n", sessionName)
			return nil
		}

		// Read and parse the session file
		data, err := os.ReadFile(sessionPath)
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
		sessionDir := service.GetSessionsDir()
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
			sessions, err := service.ListSortedSessions(sessionDir, false, true)
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

		// If oldName is a number, treat it as an index
		index, err := strconv.Atoi(oldName)
		if err == nil {
			sessions, err := service.ListSortedSessions(sessionDir, false, true)
			if err != nil {
				fmt.Println(err)
				return nil
			}
			if len(sessions) == 0 {
				fmt.Println("No sessions found.")
				return nil
			}
			if index < 1 || index > len(sessions) {
				return fmt.Errorf("index %d out of range (1-%d)", index, len(sessions))
			}
			oldName = sessions[index-1].Name
		}

		if strings.EqualFold(oldName, newName) {
			fmt.Println("No changes made.")
			return nil
		}

		oldPath := util.JoinFilePath(sessionDir, oldName+".jsonl")
		newPath := util.JoinFilePath(sessionDir, newName+".jsonl")

		// Check if source exists
		if _, err := os.Stat(oldPath); os.IsNotExist(err) {
			return fmt.Errorf("session '%s' not found", oldName)
		}

		// Check if target exists
		if _, err := os.Stat(newPath); err == nil {
			return fmt.Errorf("session '%s' already exists", newName)
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
		if err := os.Rename(oldPath, newPath); err != nil {
			return fmt.Errorf("failed to rename session: %v", err)
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

		sessionDir := service.GetSessionsDir()
		sourcePath := util.JoinFilePath(sessionDir, sessionName+".jsonl")

		if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
			return fmt.Errorf("session '%s' not found", sessionName)
		}

		data, err := os.ReadFile(sourcePath)
		if err != nil {
			return fmt.Errorf("failed to read session: %v", err)
		}

		// If destPath is not given, use session name in local path
		if destPath == "" {
			destPath = sessionName + ".jsonl"
		} else {
			// If destPath is a directory, append the filename
			if info, err := os.Stat(destPath); err == nil && info.IsDir() {
				destPath = filepath.Join(destPath, sessionName+".jsonl")
			}
		}

		if err := os.WriteFile(destPath, data, 0644); err != nil {
			return fmt.Errorf("failed to export session: %v", err)
		}

		fmt.Printf("Session '%s' exported to '%s'\n", sessionName, destPath)
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
