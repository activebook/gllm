// File: cmd/memory.go
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/service"
	"github.com/charmbracelet/huh"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// Color definitions for memory command output
var (
	memoryHeaderColor = color.New(color.FgCyan, color.Bold).SprintFunc()
	memoryItemColor   = color.New(color.FgGreen).SprintFunc()
)

var memoryCmd = &cobra.Command{
	Use:     "memory",
	Aliases: []string{"mem", "ctx"},
	Short:   "Manage gllm memory/context",
	Long: `Memory allows gllm to remember important facts about you across sessions.

These memories are injected into the system prompt to personalize responses.
Use subcommands to list, add, or clear memories,
or use 'memory path' to see where the memory file is located.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Show current memory status
		store := data.NewMemoryStore()
		memories, err := store.Load()
		if err != nil {
			service.Errorf("Error loading memory: %v\n", err)
			return
		}

		fmt.Println(cmd.Long)
		fmt.Println()
		fmt.Printf("Saved memories: %s\n", memoryHeaderColor(fmt.Sprintf("%d", len(memories))))

		if len(memories) > 0 {
			fmt.Println("\nRecent memories:")
			// Show up to 3 recent memories
			showCount := min(3, len(memories))
			for i := 0; i < showCount; i++ {
				fmt.Printf("  • %s\n", memoryItemColor(memories[i]))
			}
			if len(memories) > 3 {
				fmt.Printf("  ... and %d more (use 'gllm memory list' to see all)\n", len(memories)-3)
			}
		}
	},
}

var memoryListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls", "show", "pr"},
	Short:   "List all saved memories",
	Long: `Display all memories currently saved in the memory file.

Example:
  gllm memory list
  gllm memory list --verbose`,
	Run: func(cmd *cobra.Command, args []string) {
		store := data.NewMemoryStore()
		memories, err := store.Load()
		if err != nil {
			service.Errorf("Error loading memories: %v\n", err)
			return
		}

		if len(memories) == 0 {
			fmt.Println("No memories saved yet.")
			fmt.Println("Use 'gllm memory add \"your memory\"' to add one.")
			return
		}

		verbose, _ := cmd.Flags().GetBool("verbose")

		fmt.Printf("%s (%d items):\n", memoryHeaderColor("Saved Memories"), len(memories))
		fmt.Println()

		for i, memory := range memories {
			if verbose {
				fmt.Printf("%d. %s\n", i+1, memoryItemColor(memory))
			} else {
				// Truncate long memories for display
				displayMemory := memory
				if !verbose && len(memory) > 80 {
					displayMemory = memory[:77] + "..."
				} else {
					displayMemory = memory
				}
				fmt.Printf("%d. %s\n", i+1, memoryItemColor(displayMemory))
			}
		}
	},
}

var memoryAddCmd = &cobra.Command{
	Use:   "add \"memory content\"",
	Short: "Add a new memory",
	Long: `Add a new memory to be remembered across sessions.

Examples:
  gllm memory add "I prefer Go over Python"
  gllm memory add "Always use dark mode themes"
  gllm memory add "My project uses PostgreSQL"`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var memory string
		if len(args) > 0 {
			memory = args[0]
		} else {
			// Interactive mode
			err := huh.NewForm(
				huh.NewGroup(
					huh.NewText().
						Title("New Memory").
						Description("Enter information you want gllm to remember across sessions.").
						Value(&memory).
						Lines(5).
						Validate(func(s string) error {
							if strings.TrimSpace(s) == "" {
								return fmt.Errorf("memory content cannot be empty")
							}
							return nil
						}),
				),
			).WithKeyMap(GetHuhKeyMap()).Run()
			if err != nil {
				return
			}
		}

		if strings.TrimSpace(memory) == "" {
			service.Errorf("Memory content cannot be empty\n")
			return
		}

		store := data.NewMemoryStore()
		err := store.Add(memory)
		if err != nil {
			service.Errorf("Error adding memory: %v\n", err)
			return
		}

		fmt.Printf("✓ Memory added: %s\n", memoryItemColor(memory))
	},
}

var memoryClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all memories",
	Long: `Remove all saved memories from the memory file.
This action cannot be undone.

Example:
  gllm memory clear
  gllm memory clear --force`,
	Run: func(cmd *cobra.Command, args []string) {
		force, _ := cmd.Flags().GetBool("force")

		if !force {
			store := data.NewMemoryStore()
			memories, err := store.Load()
			if err != nil {
				service.Errorf("Error loading memories: %v\n", err)
				return
			}

			if len(memories) == 0 {
				fmt.Println("No memories to clear.")
				return
			}

			fmt.Printf("This will delete %d memories. This cannot be undone.\n", len(memories))

			var confirm bool
			err = huh.NewConfirm().
				Title("Are you sure you want to clear all memories?").
				Affirmative("Yes, delete all").
				Value(&confirm).
				Run()
			if err != nil {
				return
			}
			if !confirm {
				fmt.Println("Operation cancelled.")
				return
			}
		}

		store := data.NewMemoryStore()
		err := store.Clear()
		if err != nil {
			service.Errorf("Error clearing memories: %v\n", err)
			return
		}

		fmt.Println("✓ All memories have been cleared.")
	},
}

var memoryPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show the location of the memory file",
	Long:  `Display the full path to the memory file. You can manually edit this file.`,
	Run: func(cmd *cobra.Command, args []string) {
		store := data.NewMemoryStore()
		memoryPath := store.GetPath()

		// Check if file exists
		if _, err := os.Stat(memoryPath); os.IsNotExist(err) {
			// Create the file if it doesn't exist
			err := store.Save([]string{})
			if err != nil {
				service.Errorf("Error initializing memory file: %v\n", err)
				return
			}
			fmt.Printf("Memory file initialized at: %s\n", memoryPath)
		} else {
			fmt.Printf("Memory file location: %s\n", memoryPath)
		}
	},
}

func init() {
	// Add flags
	memoryListCmd.Flags().BoolP("verbose", "v", false, "Show full memory content without truncation")
	memoryClearCmd.Flags().BoolP("force", "f", false, "Force clear without confirmation")

	// Add subcommands
	memoryCmd.AddCommand(memoryListCmd)
	memoryCmd.AddCommand(memoryAddCmd)
	memoryCmd.AddCommand(memoryClearCmd)
	memoryCmd.AddCommand(memoryPathCmd)

	// Add to root command
	rootCmd.AddCommand(memoryCmd)
}
