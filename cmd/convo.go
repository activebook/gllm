package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/activebook/gllm/service"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var (
	convoMessageCount  int
	convoMessageLength int
)

// convoCmd represents the convo command
var convoCmd = &cobra.Command{
	Use:     "convo",
	Aliases: []string{"cv", "conversation"},
	Short:   "Manage conversations",
	Long:    `Commands to list, remove, and show details of conversations.`,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return convoListCmd.RunE(cmd, args)
	},
}

// convoListCmd represents the convo list command
var convoListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all conversations",
	Long:    `List all available conversations in sorted order.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		convoDir := service.GetConvoDir()

		// Check if directory exists
		if _, err := os.Stat(convoDir); os.IsNotExist(err) {
			fmt.Println("No conversations found.")
			return nil
		}

		convos, err := service.ListSortedConvos(convoDir)
		if err != nil {
			fmt.Println(err)
			return nil
		}
		if len(convos) == 0 {
			fmt.Println("No conversations found.")
			return nil
		}

		fmt.Println("Available conversations:")
		for index, convo := range convos {
			// Display with title if available
			if convo.Provider != "" {
				fmt.Printf("  - [%d] %s [%s]\n", index+1, convo.Name, convo.Provider)
			} else {
				fmt.Printf("  - [%d] %s\n", index+1, convo.Name)
			}
		}
		return nil
	},
}

// convoRemoveCmd represents the convo remove command
var convoRemoveCmd = &cobra.Command{
	Use:     "remove [conversation|pattern|index]",
	Aliases: []string{"rm"},
	Short:   "Remove a conversation by name or index, or multiple conversations using a pattern",
	Long: `Remove a specific conversation by name or index, or multiple conversations using a pattern with wildcards.
This action cannot be undone.

Examples:
gllm convo remove chat_123
gllm convo remove "chat_*" --force
gllm convo remove 1 --force
gllm convo remove 10-20 --force
gllm convo remove "2 - 5" --force`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		convoDir := service.GetConvoDir()
		var matches []string
		var pattern string
		if len(args) > 0 {
			pattern = args[0]
		} else {
			// Select conversations to remove
			convos, err := service.ListSortedConvos(convoDir)
			if err != nil || len(convos) == 0 {
				fmt.Println("No conversations found.")
				return nil
			}

			var options []huh.Option[string]
			for _, c := range convos {
				label := c.Name
				if c.Provider != "" {
					label = fmt.Sprintf("%s [%s]", c.Name, c.Provider)
				}
				options = append(options, huh.NewOption(label, c.Name))
			}

			var selected []string
			err = huh.NewMultiSelect[string]().
				Title("Select Conversations to Remove").
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
			for _, s := range selected {
				matches = append(matches, filepath.Join(convoDir, s+".json"))
			}

			// Confirm if not forced
			force, _ := cmd.Flags().GetBool("force")
			if !force {
				var confirm bool
				err = huh.NewConfirm().
					Title(fmt.Sprintf("Are you sure you want to remove %d conversations?", len(matches))).
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
					fmt.Printf("Conversation '%s' removed successfully.\n", strings.TrimSuffix(filepath.Base(match), filepath.Ext(match)))
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
				convos, err := service.ListSortedConvos(convoDir)
				if err != nil {
					fmt.Println(err)
					return nil
				}
				if len(convos) == 0 {
					fmt.Println("No conversations found.")
					return nil
				}

				// Validate range
				if start < 1 || end < 1 || start > len(convos) || end > len(convos) {
					return fmt.Errorf("range %d-%d out of range (1-%d)", start, end, len(convos))
				}
				if start > end {
					return fmt.Errorf("invalid range: start (%d) cannot be greater than end (%d)", start, end)
				}

				// Collect matching files in range
				for i := start; i <= end; i++ {
					convoPath := filepath.Join(convoDir, convos[i-1].Name+".json")
					matches = append(matches, convoPath)
				}
			} else {
				// Not a valid range, treat as regular pattern
				matches = handleAsPattern(pattern, convoDir)
			}
		} else {
			// Regular pattern handling
			matches = handleAsPattern(pattern, convoDir)
		}

		if len(matches) == 0 {
			fmt.Printf("No conversations found matching '%s'.\n", pattern)
			return nil
		}

		// Ask for confirmation if not forced
		force, _ := cmd.Flags().GetBool("force")
		if !force {
			fmt.Printf("The following conversations will be removed:\n")
			for _, match := range matches {
				fmt.Printf("  - %s\n", strings.TrimSuffix(filepath.Base(match), filepath.Ext(match)))
			}

			var confirm bool
			err := huh.NewConfirm().
				Title("Are you sure you want to remove these conversations?").
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
				fmt.Printf("Conversation '%s' removed successfully.\n", strings.TrimSuffix(filepath.Base(match), filepath.Ext(match)))
			}
		}

		return nil
	},
}

// handleAsPattern handles the pattern as either an index or a file pattern
func handleAsPattern(pattern string, convoDir string) []string {
	var matches []string

	// Try to parse as index
	index, err := strconv.Atoi(pattern)
	if err == nil {
		convos, err := service.ListSortedConvos(convoDir)
		if err != nil {
			fmt.Println(err)
			return matches
		}
		if len(convos) == 0 {
			return matches
		}
		if index >= 1 && index <= len(convos) {
			// Use the resolved file name as the pattern
			pattern = convos[index-1].Name
		}
	}

	// Now pattern is either a name or a wildcard
	convoPathPattern := filepath.Join(convoDir, pattern+".json")

	// Find matching files using the pattern
	matches, err = filepath.Glob(convoPathPattern)
	if err != nil {
		fmt.Printf("Failed to parse pattern: %v\n", err)
		return []string{}
	}

	return matches
}

var convoClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all conversations",
	Long: `Remove all saved conversations.
This action cannot be undone.

Example:
gllm convo clear
gllm convo clear --force`,
	RunE: func(cmd *cobra.Command, args []string) error {
		convoDir := service.GetConvoDir()
		// Check if directory exists
		if _, err := os.Stat(convoDir); os.IsNotExist(err) {
			fmt.Println("No conversations found.")
			return nil
		}

		force, _ := cmd.Flags().GetBool("force")

		if !force {
			var confirm bool
			err := huh.NewConfirm().
				Title("Are you sure you want to clear ALL saved conversations?").
				Affirmative("Yes, delete all").
				Value(&confirm).
				Run()
			if err != nil || !confirm {
				fmt.Println("Operation cancelled.")
				return nil
			}
		}

		files, err := os.ReadDir(convoDir)
		if err != nil {
			return fmt.Errorf("fail to read conversation directory: %v", err)
		}

		for _, file := range files {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
				name := strings.TrimSuffix(file.Name(), ".json")
				if err := os.Remove(filepath.Join(convoDir, name+".json")); err != nil {
					return fmt.Errorf("failed to remove conversation: %v", err)
				}
				fmt.Printf("  - '%s' removed.\n", name)
			}
		}

		fmt.Println("All conversations have been cleared.")
		return nil
	},
}

// convoInfoCmd represents the convo info command
var convoInfoCmd = &cobra.Command{
	Use:     "info [conversation|index]",
	Aliases: []string{"in"},
	Short:   "Show conversation details",
	Long: `Display detailed information about a specific conversation.

Using the --message-num (-n) flag, set the number of recent messages to display..
Using the --message-chars (-c) flag, set the maximum length of each message's content.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		convoDir := service.GetConvoDir()
		var convoName string
		if len(args) > 0 {
			convoName = args[0]
		} else {
			// Select conversation
			convos, err := service.ListSortedConvos(convoDir)
			if err != nil || len(convos) == 0 {
				fmt.Println("No conversations found.")
				return nil
			}

			var options []huh.Option[string]
			for _, c := range convos {
				label := c.Name
				if c.Provider != "" {
					label = fmt.Sprintf("%s [%s]", c.Name, c.Provider)
				}
				options = append(options, huh.NewOption(label, c.Name))
			}

			err = huh.NewSelect[string]().
				Title("Select Conversation").
				Options(options...).
				Value(&convoName).
				Run()
			if err != nil {
				return nil
			}
		}

		// If convoName is a number, treat it as an index
		index, err := strconv.Atoi(convoName)
		if err == nil {
			convos, err := service.ListSortedConvos(convoDir)
			if err != nil {
				fmt.Println(err)
				return nil
			}
			if len(convos) == 0 {
				fmt.Println("No conversations found.")
				return nil
			}
			if index < 1 || index > len(convos) {
				return fmt.Errorf("index %d out of range (1-%d)", index, len(convos))
			}
			convoName = convos[index-1].Name
		}

		convoPath := filepath.Join(convoDir, convoName+".json")

		// Check if conversation exists
		if _, err := os.Stat(convoPath); os.IsNotExist(err) {
			fmt.Printf("Conversation '%s' not found.\n", convoName)
			return nil
		}

		// Read and parse the conversation file
		data, err := os.ReadFile(convoPath)
		if err != nil {
			return fmt.Errorf("error reading conversation file: %v", err)
		}

		// Display conversation details
		fmt.Printf("Name: %s\n", convoName)

		// Detect provider based on message format
		provider := service.DetectMessageProvider(data)

		// Process and display messages based on provider
		switch provider {
		case service.ModelProviderGemini:
			service.DisplayGeminiConversationLog(data, convoMessageCount, convoMessageLength)
		case service.ModelProviderOpenAI, service.ModelProviderMistral, service.ModelProviderOpenAICompatible:
			service.DisplayOpenAIConversationLog(data, convoMessageCount, convoMessageLength)
		default:
			fmt.Println("Unknown conversation format.")
		}
		return nil
	},
}

// convoRenameCmd represents the convo rename command
var convoRenameCmd = &cobra.Command{
	Use:   "rename [oldname|index] [newname]",
	Short: "Rename a conversation",
	Long:  `Rename an existing conversation to a new name.`,
	Args:  cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		convoDir := service.GetConvoDir()
		var oldName, newName string

		if len(args) >= 2 {
			oldName = args[0]
			newName = args[1]
		} else {
			// Select conversation to rename
			convos, err := service.ListSortedConvos(convoDir)
			if err != nil || len(convos) == 0 {
				fmt.Println("No conversations found.")
				return nil
			}

			if len(args) == 1 {
				oldName = args[0]
			} else {
				var options []huh.Option[string]
				for _, c := range convos {
					label := c.Name
					if c.Provider != "" {
						label = fmt.Sprintf("%s [%s]", c.Name, c.Provider)
					}
					options = append(options, huh.NewOption(label, c.Name))
				}

				err = huh.NewSelect[string]().
					Title("Select Conversation to Rename").
					Options(options...).
					Value(&oldName).
					Run()
				if err != nil {
					return nil
				}
			}

			// Get new name
			err = huh.NewInput().
				Title("New Name").
				Value(&newName).
				Validate(func(s string) error {
					s = strings.TrimSpace(s)
					if s == "" {
						return fmt.Errorf("new name cannot be empty")
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
			convos, err := service.ListSortedConvos(convoDir)
			if err != nil {
				fmt.Println(err)
				return nil
			}
			if len(convos) == 0 {
				fmt.Println("No conversations found.")
				return nil
			}
			if index < 1 || index > len(convos) {
				return fmt.Errorf("index %d out of range (1-%d)", index, len(convos))
			}
			oldName = convos[index-1].Name
		}

		oldPath := service.GetFilePath(convoDir, oldName+".json")
		newPath := service.GetFilePath(convoDir, newName+".json")

		// Check if source exists
		if _, err := os.Stat(oldPath); os.IsNotExist(err) {
			return fmt.Errorf("conversation '%s' not found", oldName)
		}

		// Check if target exists
		if _, err := os.Stat(newPath); err == nil {
			return fmt.Errorf("conversation '%s' already exists", newName)
		}

		// Ask for confirmation
		force, _ := cmd.Flags().GetBool("force")
		if !force {
			var confirm bool
			err := huh.NewConfirm().
				Title(fmt.Sprintf("Rename conversation '%s' to '%s'?", oldName, newName)).
				Value(&confirm).
				Run()
			if err != nil || !confirm {
				fmt.Println("Operation cancelled.")
				return nil
			}
		}

		// Perform the rename
		if err := os.Rename(oldPath, newPath); err != nil {
			return fmt.Errorf("failed to rename conversation: %v", err)
		}

		fmt.Printf("Conversation renamed from '%s' to '%s' successfully.\n", oldName, newName)
		return nil
	},
}

func init() {
	// Add convo command to root command
	rootCmd.AddCommand(convoCmd)

	// Add subcommands to convo command
	convoCmd.AddCommand(convoListCmd)
	convoCmd.AddCommand(convoRemoveCmd)
	convoCmd.AddCommand(convoInfoCmd)
	convoCmd.AddCommand(convoClearCmd)
	convoCmd.AddCommand(convoRenameCmd)

	// Add flags for other prompt commands if needed in the future
	convoInfoCmd.Flags().IntVarP(&convoMessageCount, "message-num", "n", 20, "Number of messages to display")
	convoInfoCmd.Flags().IntVarP(&convoMessageLength, "message-chars", "c", 200, "Length of messages to display")
	convoRemoveCmd.Flags().BoolP("force", "f", false, "Skip confirm")
	convoClearCmd.Flags().BoolP("force", "f", false, "Force clear all without confirmation")
	convoRenameCmd.Flags().BoolP("force", "f", false, "Skip confirm")
}
