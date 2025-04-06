package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/activebook/gllm/service"
	"github.com/spf13/cobra"
)

func getConvoDir() string {
	dir := service.MakeUserSubDir("gllm", "convo")
	return dir
}

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
		convoDir := getConvoDir()

		// Check if directory exists
		if _, err := os.Stat(convoDir); os.IsNotExist(err) {
			fmt.Println("No conversations found.")
			return nil
		}

		files, err := os.ReadDir(convoDir)
		if err != nil {
			return fmt.Errorf("fail to read conversation directory: %v", err)
		}

		var convos []string
		for _, file := range files {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
				title := strings.TrimSuffix(file.Name(), ".json")
				fullPath := service.GetFilePath(convoDir, file.Name())
				var convo string
				data, err := os.ReadFile(fullPath)
				if err != nil {
					convo = fmt.Sprintf("  - %s", title)
				} else {
					provider := service.DetectMessageProvider(data)
					convo = fmt.Sprintf("  - %s [%s]", title, provider)
				}
				convos = append(convos, convo)
			}
		}

		if len(convos) == 0 {
			fmt.Println("No conversations found.")
			return nil
		}

		// Sort conversations alphabetically
		sort.Strings(convos)

		fmt.Println("Available conversations:")
		for _, convo := range convos {
			// Display with title if available
			fmt.Println(convo)
		}
		return nil
	},
}

// convoRemoveCmd represents the convo remove command
var convoRemoveCmd = &cobra.Command{
	Use:     "remove [conversation]",
	Aliases: []string{"rm"},
	Short:   "Remove a conversation",
	Long: `Remove a specific conversation with confirmation.
This action cannot be undone.

Example:
gllm convo remove [conversation]
gllm convo remove [conversation] --force`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		convoName := args[0]
		convoDir := getConvoDir()
		convoPath := service.GetFilePath(convoDir, convoName+".json")

		// Check if conversation exists
		if _, err := os.Stat(convoPath); os.IsNotExist(err) {
			fmt.Printf("Conversation '%s' not found.\n", convoName)
			return nil
		}

		// Ask for confirmation
		force, _ := cmd.Flags().GetBool("force")
		if !force {
			fmt.Printf("Are you sure you want to remove conversation '%s'? (y/N): ", convoName)
			var response string
			fmt.Scanln(&response)

			response = strings.ToLower(strings.TrimSpace(response))
			if response != "y" && response != "yes" {
				fmt.Println("Operation cancelled.")
				return nil
			}
		}
		if err := os.Remove(convoPath); err != nil {
			return fmt.Errorf("failed to remove conversation: %v", err)
		}
		fmt.Printf("Conversation '%s' removed successfully.\n", convoName)
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
		convoDir := getConvoDir()
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
	Use:     "info [conversation]",
	Aliases: []string{"in"},
	Short:   "Show conversation details",
	Long:    `Display detailed information about a specific conversation.`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		convoName := args[0]
		convoDir := getConvoDir()
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

		// Detect provider based on message format
		provider := service.DetectMessageProvider(data)

		// Process and display messages based on provider
		switch provider {
		case service.ModelGemini:
			service.DisplayGeminiConversationLog(data)
		case service.ModelOpenAI, service.ModelOpenAICompatible:
			service.DisplayOpenAIConversationLog(data)
		default:
			fmt.Println("Unknown conversation format.")
		}
		return nil
	},
}

// convoRenameCmd represents the convo rename command
var convoRenameCmd = &cobra.Command{
	Use:   "rename [oldname] [newname]",
	Short: "Rename a conversation",
	Long:  `Rename an existing conversation to a new name.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		oldName := args[0]
		newName := args[1]
		newName = service.GetSanitizeTitle(newName)
		convoDir := getConvoDir()
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
	convoRemoveCmd.Flags().BoolP("force", "f", false, "Skip confirm")
	convoClearCmd.Flags().BoolP("force", "f", false, "Force clear all without confirmation")
	convoRenameCmd.Flags().BoolP("force", "f", false, "Skip confirm")
}
