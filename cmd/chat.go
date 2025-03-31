package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Message represents a single message in a conversation
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatCmd represents the chat command
var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive chat session (REPL)",
	Long: `Start an interactive chat session with the configured LLM.
This provides a Read-Eval-Print-Loop (REPL) interface where you can
have a continuous conversation with the model.

Special commands:
/exit, /quit - Exit the chat session
/model <name> - Switch to a different model
/clear, /reset - Clear conversation history
/help - Show available commands
/save <filename> - Save conversation to file
/load <filename> - Load conversation from file
/system "<prompt>" - Change the system prompt
/template <name> - Change the template to use`,
	Run: func(cmd *cobra.Command, args []string) {
		startREPL()
	},
}

func init() {
	rootCmd.AddCommand(chatCmd)

	// Add chat-specific flags
	chatCmd.Flags().StringP("model", "m", "", "Model to use for the chat session")
	chatCmd.Flags().StringP("system", "s", "", "System prompt to use")
	chatCmd.Flags().StringP("template", "t", "", "Template to use for the chat session")
	chatCmd.Flags().StringP("load", "l", "", "Load conversation from a file")
}

func startREPL() {
	fmt.Println("Welcome to GLLM Interactive Chat")
	fmt.Println("Type 'exit' or 'quit' to end the session, or '/help' for commands")
	fmt.Println("Use '\\' at the end of a line for multiline input")

	scanner := bufio.NewScanner(os.Stdin)
	conversationHistory := []Message{}

	for {
		fmt.Print("\ngllm> ")

		// Handle multiline input
		var inputLines []string
		multilineMode := false

		for {
			if !scanner.Scan() {
				return
			}

			line := scanner.Text()

			// Check if line ends with \ for continuation
			if strings.HasSuffix(line, "\\") {
				multilineMode = true
				inputLines = append(inputLines, strings.TrimSuffix(line, "\\"))
				fmt.Print("... ")
				continue
			}

			inputLines = append(inputLines, line)

			// If not in multiline mode or an empty line ends multiline input
			if !multilineMode || line == "" {
				break
			}

			fmt.Print("... ")
		}

		input := strings.Join(inputLines, "\n")
		input = strings.TrimSpace(input)

		if input == "exit" || input == "quit" {
			fmt.Println("Goodbye!")
			break
		}

		if input == "" {
			continue
		}

		handleInput(input, &conversationHistory)
	}
}

func handleInput(input string, history *[]Message) {
	// Check for file attachments
	if strings.Contains(input, "-a ") {
		parts := strings.Split(input, "-a ")
		query := strings.TrimSpace(parts[0])

		var fileContents []string
		for i := 1; i < len(parts); i++ {
			filePath := strings.Split(strings.TrimSpace(parts[i]), " ")[0]
			content, err := readFile(filePath)
			if err != nil {
				fmt.Printf("Error reading file %s: %v\n", filePath, err)
				return
			}
			fileContents = append(fileContents, content)
		}

		// Combine file contents with query
		combinedInput := strings.Join(fileContents, "\n\n")
		if query != "" {
			combinedInput += "\n\n" + query
		}

		input = combinedInput
	}

	// Check if it's a command
	if strings.HasPrefix(input, "/") {
		//handleCommand(input, history)
		return
	}

	// Process as normal LLM query...
}

func readFile(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func handleCommand(cmd string, history *[]Message, modelName *string) bool {
	// Split the command into parts
	parts := strings.SplitN(cmd, " ", 2)
	command := parts[0]

	switch command {
	case "/exit", "/quit":
		fmt.Println("Exiting chat session")
		return false

	case "/help":
		fmt.Println("Available commands:")
		fmt.Println("  /exit, /quit - Exit the chat session")
		fmt.Println("  /model <name> - Switch to a different model")
		fmt.Println("  /clear, /reset - Clear conversation history")
		fmt.Println("  /help - Show this help message")
		fmt.Println("  /save <filename> - Save conversation to file")
		fmt.Println("  /load <filename> - Load conversation from file")
		fmt.Println("  /system \"<prompt>\" - Change the system prompt")
		fmt.Println("  /info - Show current settings and conversation stats")

	case "/clear", "/reset":
		*history = nil
		systemPrompt := viper.GetString("system_prompt")
		if systemPrompt != "" {
			*history = append(*history, Message{Role: "system", Content: systemPrompt})
		}
		fmt.Println("Conversation history cleared")

	case "/model":
		if len(parts) < 2 {
			fmt.Println("Please specify a model name")
			return true
		}
		newModel := strings.TrimSpace(parts[1])
		*modelName = newModel
		fmt.Printf("Switched to model: %s\n", newModel)

	case "/system":
		if len(parts) < 2 {
			fmt.Println("Please specify a system prompt")
			return true
		}
		newPrompt := strings.TrimSpace(parts[1])

		// Update or add system message
		systemFound := false
		for i, msg := range *history {
			if msg.Role == "system" {
				(*history)[i].Content = newPrompt
				systemFound = true
				break
			}
		}

		if !systemFound {
			// Insert system message at beginning
			*history = append([]Message{{Role: "system", Content: newPrompt}}, *history...)
		}

		fmt.Println("System prompt updated")

	case "/save":
		if len(parts) < 2 {
			fmt.Println("Please specify a filename")
			return true
		}
		filename := strings.TrimSpace(parts[1])
		err := saveConversation(filename, *history)
		if err != nil {
			fmt.Printf("Error saving conversation: %v\n", err)
		} else {
			fmt.Printf("Conversation saved to %s\n", filename)
		}

	case "/load":
		if len(parts) < 2 {
			fmt.Println("Please specify a filename")
			return true
		}
		filename := strings.TrimSpace(parts[1])
		loadedHistory, err := loadConversation(filename)
		if err != nil {
			fmt.Printf("Error loading conversation: %v\n", err)
		} else {
			*history = loadedHistory
			fmt.Printf("Conversation loaded from %s\n", filename)
		}

	case "/info":
		// Show current model and conversation stats
		fmt.Printf("Current model: %s\n", *modelName)

		// Find system prompt
		for _, msg := range *history {
			if msg.Role == "system" {
				fmt.Printf("System prompt: %s\n", msg.Content)
				break
			}
		}

		// Count messages
		userCount := 0
		assistantCount := 0
		for _, msg := range *history {
			if msg.Role == "user" {
				userCount++
			} else if msg.Role == "assistant" {
				assistantCount++
			}
		}
		fmt.Printf("Conversation: %d user messages, %d assistant responses\n",
			userCount, assistantCount)

	default:
		fmt.Printf("Unknown command: %s\n", command)
	}

	return true // Continue the REPL
}

func saveConversation(filename string, history []Message) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(history)
}

func loadConversation(filename string) ([]Message, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var history []Message
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&history)
	return history, err
}

func showHistory(history []Message) {
	if len(history) == 0 {
		fmt.Println("No conversation history")
		return
	}

	fmt.Println("Conversation history:")
	for i, msg := range history {
		switch msg.Role {
		case "user":
			fmt.Printf("\n[%d] User: %s\n", i/2+1, msg.Content)
		case "assistant":
			fmt.Printf("    Assistant: %s\n", truncateForDisplay(msg.Content))
		}
	}
}

func truncateForDisplay(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) > 3 {
		return strings.Join(lines[:3], "\n") + "...\n[content truncated]"
	}
	return content
}

func callLLM(modelName string, history []Message) (string, error) {
	// This function should implement the actual call to your LLM API
	// You'll need to replace this with your specific implementation

	// Example implementation structure:
	// 1. Get model configuration from viper
	// modelConfig := viper.GetStringMap(fmt.Sprintf("models.%s", modelName))

	// 2. Create API request based on the model type
	// (OpenAI, Anthropic, etc.)

	// 3. Send request to API and get response

	// For now, return a placeholder
	return "This is a placeholder response. Replace this function with your actual LLM API integration.", nil
}
