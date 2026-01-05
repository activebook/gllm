package cmd

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/service"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// chatCmd represents the chat command
var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive chat session (REPL)",
	Long: `Start an interactive chat session with the configured LLM.
This provides a Read-Eval-Print-Loop (REPL) interface where you can
have a continuous conversation with the model.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create an indeterminate progress bar
		indicator := service.NewIndicator("Processing...")

		var chatInfo *ChatInfo
		store := data.NewConfigStore()

		// If conversation flag is not provided, generate a new conversation name
		if !cmd.Flags().Changed("conversation") {
			convoName = GenerateChatFileName()
		} else {
			// Bugfix: When convoName is an index number, and use it to find convo file
			// it will return the convo file(sorted by modified time)
			// but if user mannually update other convo file, the modified time will change
			// so the next time using index to load convo file, would load the wrong one
			// How to fix:
			// First, we need convert convoName
			// And, keep that name, the next time, it willn't use the index to load convo file
			name, err := service.FindConvosByIndex(convoName)
			if err != nil {
				return fmt.Errorf("error finding conversation: %v\n", err)
			}
			if name != "" {
				convoName = name
			}
		}

		// If agent flag is provided, update the default agent
		if cmd.Flags().Changed("agent") {
			if store.GetAgent(agentName) == nil {
				return fmt.Errorf("agent %s does not exist", agentName)
			}
			store.SetActiveAgent(agentName)
		}

		// Build the ChatInfo object
		chatInfo = &ChatInfo{
			QuitFlag: false,
		}

		indicator.Stop()

		// Start the REPL
		chatInfo.startREPL()
		return nil
	},
}

var ()

const (
	_gllmTempFile = ".gllm-edit-*.tmp"
)

// Load when package is initialized
func init() {
	rootCmd.AddCommand(chatCmd)

	// Add chat-specific flags
	// In each chat session, it shouldn't change model, because the chat history would be invalid.
	// Attach should be used inside chat session
	// Imagine like using web llm ui, you can attach file to the chat session and turn search on and off
	chatCmd.Flags().StringVarP(&agentName, "agent", "g", "", "Agent to use for the chat session")
	chatCmd.Flags().StringVarP(&convoName, "conversation", "c", GenerateChatFileName(), "Name for this chat session")
	chatCmd.Flags().BoolVarP(&yoloFlag, "yolo", "y", false, "Enable yolo mode (non-interactive)")
}

type ChatInfo struct {
	Files       []*service.FileData
	QuitFlag    bool   // for cmd /quit or /exit
	EditorInput string // for /e editor edit
	outputFile  string
}

func (ci *ChatInfo) printWelcome() {
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("5")). // Purple
		MarginTop(1).
		MarginBottom(1).
		Padding(0, 0)

	contentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("7")). // White/Light gray
		Padding(0, 2)

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")). // Dark gray
		Italic(true)

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("6")). // Cyan
		Padding(1)

	welcomeText := "Welcome to GLLM Interactive Chat"
	instructions := []string{
		"• Type '/exit' or '/quit' to end the session",
		"• Type '/help' for a list of available commands",
		"• Use '/' for commands and '!' for local shell commands",
		"• Use Ctrl+C to exit at any time",
	}

	header := headerStyle.Render(welcomeText)
	content := contentStyle.Render(strings.Join(instructions, "\n"))

	banner := borderStyle.Render(lipgloss.JoinVertical(
		lipgloss.Center,
		header,
		content,
	))

	fmt.Println(banner)
	fmt.Println(hintStyle.Padding(0, 2).Render("Type your message below and press Enter to send."))
	fmt.Println()
}

func (ci *ChatInfo) awaitChat() (string, error) {
	var input string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewText().
				Title("Chat").
				Value(&input).
				Placeholder("Type your message..."),
		),
	).WithKeyMap(GetHuhKeyMap()) // 4. CRITICAL: Apply the keymap to the FORM level

	err := form.Run()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(input), nil
}

func (ci *ChatInfo) startREPL() {
	// Start the REPL
	ci.printWelcome()

	// Define prompt style
	tcol := service.GetTerminalWidth() - 4
	promptStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#282A2C")). // Grey background
		Foreground(lipgloss.Color("#cecece")). // White text
		Padding(1, 2).Margin(0, 0, 1, 0).      // padding and margin
		Bold(false).
		// Align(lipgloss.Right). 	// align would break code formatting
		Width(tcol) // align and width

	for {
		var input string
		var err error

		// Get user input
		input, err = ci.awaitChat()
		if err != nil {
			// Handle user cancellation (Ctrl+C)
			fmt.Println("\nSession ended.")
			break
		}
		if input == "" {
			continue
		}

		// Handle inner commands
		if ci.startWithInnerCommand(input) {
			// Reset editor input
			ci.EditorInput = ""
			// Handle inner command
			ci.handleCommand(input)
			if ci.QuitFlag {
				break
			}
			fmt.Println()
			// If editor input is not empty, use it as input
			if ci.EditorInput != "" {
				input = ci.EditorInput
			} else {
				continue
			}
		}

		// Echo user input with style
		fmt.Println(promptStyle.Render(input))

		// Handle shell commands
		if ci.startWithLocalCommand(input) {
			ci.executeShellCommand(input[1:])
			continue
		}

		// Call agent
		ci.callAgent(input)
		fmt.Println()

		// quit chat
		if ci.QuitFlag {
			break
		}
	}
}

func (ci *ChatInfo) startWithInnerCommand(line string) bool {
	return strings.HasPrefix(line, "/")
}

func (ci *ChatInfo) startWithLocalCommand(line string) bool {
	return strings.HasPrefix(line, "!")
}

// clearContext clears the conversation context
func (ci *ChatInfo) clearContext() {
	agent, err := EnsureActiveAgent()
	if err != nil {
		service.Errorf("%v", err)
		return
	}
	// Construct conversation manager
	cm, err := service.ConstructConversationManager(convoName, agent.Model.Provider)
	if err != nil {
		service.Errorf("Error constructing conversation manager: %v\n", err)
		return
	}
	// Clear conversation history
	err = cm.Clear()
	if err != nil {
		service.Errorf("Error clearing context: %v\n", err)
		return
	}
	// Empty attachments
	ci.Files = []*service.FileData{}
	fmt.Printf("Context cleared.\n")
}

// showHistory displays conversation history using TUI viewport
func (ci *ChatInfo) showHistory() {
	// Get active agent
	agent, err := EnsureActiveAgent()
	if err != nil {
		service.Errorf("%v", err)
		return
	}

	// Get conversation data
	convoData, convoName, err := GetConvoData(convoName, agent.Model.Provider)
	if err != nil {
		service.Errorf("%v", err)
		return
	}

	// Detect provider based on message format
	isCompatible, provider, modelProvider := CheckConvoFormat(agent, convoData)
	if !isCompatible {
		// Warn about potential incompatibility if providers differ
		service.Warnf("Conversation '%s' [%s] is not compatible with the current model provider [%s].\n", convoName, provider, modelProvider)
	}

	// Render conversation log
	var content string
	switch provider {
	case service.ModelProviderGemini:
		content = service.RenderGeminiConversationLog(convoData)
	case service.ModelProviderOpenAI, service.ModelProviderOpenAICompatible:
		content = service.RenderOpenAIConversationLog(convoData)
	case service.ModelProviderAnthropic:
		content = service.RenderAnthropicConversationLog(convoData)
	default:
		fmt.Println("No available conversation yet.")
		return
	}

	// Show viewport in full screen
	m := NewViewportModel(provider, content, func() string {
		return fmt.Sprintf("Conversation: %s", convoName)
	})
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		service.Errorf("Error running viewport: %v", err)
	}
}

func (ci *ChatInfo) callAgent(input string) {
	// Call agent using the shared runner
	err := RunAgent(input, ci.Files, convoName, yoloFlag, ci.outputFile)
	if err != nil {
		service.Errorf("%v", err)
		return
	}

	// Reset the files after processing
	ci.Files = []*service.FileData{}
}

func (ci *ChatInfo) executeShellCommand(command string) {
	command = strings.TrimSpace(command)
	if command == "" {
		fmt.Println("No command provided")
		return
	}

	// Execute the command
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", command)
	} else {
		cmd = exec.Command("sh", "-c", command)
	}

	// Capture both stdout and stderr
	output, err := cmd.CombinedOutput()

	// Display error if command failed
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			fmt.Printf(cmdErrorColor+"Command failed with exit code %d\n"+resetColor, exitError.ExitCode())
		} else {
			fmt.Printf(cmdErrorColor+"Command failed: %v\n"+resetColor, err)
		}
	}

	// Display the output
	if len(output) > 0 {
		fmt.Printf(cmdOutputColor+"%s\n"+resetColor, output)
	}
	fmt.Print(resetColor) // Reset color after command output
}

func GenerateChatFileName() string {
	// Get the current time
	currentTime := time.Now()

	// Format the time as a string in the format "chat_YYYY-MM-DD_HH-MM-SS.json"
	filename := fmt.Sprintf("chat_%s", currentTime.Format("2006-01-02_15-04-05"))

	return filename
}
