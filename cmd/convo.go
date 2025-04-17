package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/activebook/gllm/service"
	"github.com/spf13/cobra"
)

var (
	convoMessageCount  int
	convoMessageLength int
)

// convoCmd represents the convo command
var convoCmd = &cobra.Command{
	Use:   "convo",
	Short: "Manage conversations",
	Long:  `Commands to list, remove, and show details of conversations.`,
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
gllm convo remove 1 --force`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pattern := args[0]
		convoDir := service.GetConvoDir()
		var matches []string

		// Try to parse as index
		index, err := strconv.Atoi(pattern)
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
			// Use the resolved file name as the pattern
			pattern = convos[index-1].Name
		}

		// Now pattern is either a name or a wildcard
		convoPathPattern := filepath.Join(convoDir, pattern+".json")

		// Find matching files using the pattern
		matches, err = filepath.Glob(convoPathPattern)
		if err != nil {
			return fmt.Errorf("failed to parse pattern: %v", err)
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
			fmt.Print("Are you sure? (y/N): ")
			var response string
			fmt.Scanln(&response)

			response = strings.ToLower(strings.TrimSpace(response))
			if response != "y" && response != "yes" {
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
			fmt.Print("Are you sure you want to clear all saved conversations? This cannot be undone. [y/N]: ")
			var response string
			fmt.Scanln(&response)

			response = strings.ToLower(strings.TrimSpace(response))
			if response != "y" && response != "yes" {
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
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		convoName := args[0]
		convoDir := service.GetConvoDir()

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
		case service.ModelGemini:
			service.DisplayGeminiConversationLog(data, convoMessageCount, convoMessageLength)
		case service.ModelOpenAI, service.ModelOpenAICompatible:
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
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		oldName := args[0]
		newName := args[1]
		newName = service.GetSanitizeTitle(newName)
		convoDir := service.GetConvoDir()

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
			fmt.Printf("Rename conversation '%s' to '%s'? (y/N): ", oldName, newName)
			var response string
			fmt.Scanln(&response)

			response = strings.ToLower(strings.TrimSpace(response))
			if response != "y" && response != "yes" {
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
