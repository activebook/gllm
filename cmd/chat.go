package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/internal/ui"
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
	// Add completion support
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return []string{"--agent", "--conversation", "--yolo", "--help"}, cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// Start indeterminate progress bar
		ui.GetIndicator().Start("")

		// Clear empty conversations in background
		// service.ClearEmptyConvosAsync()

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

		ui.GetIndicator().Stop()

		// Start the REPL
		chatInfo.startREPL()
		return nil
	},
}

var ()

const (
	chatEidtTempFile = ".gllm-edit-*.tmp"
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
	QuitFlag    bool     // for cmd /quit or /exit
	EditorInput string   // for /e editor edit
	History     []string // for chat input history
	outputFile  string
	sharedState *data.SharedState // Persistent SharedState for the session
}

func (ci *ChatInfo) printWelcome() {
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(data.KeyHex)).
		Width(ui.GetTerminalWidth()-4).
		Align(lipgloss.Center).
		MarginTop(0).
		MarginBottom(1).
		Padding(0, 0)

	contentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(data.LabelHex)).
		Width(ui.GetTerminalWidth()-4).
		Align(lipgloss.Left).
		Padding(0, 2)

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(data.DetailHex)).
		Width(ui.GetTerminalWidth() - 4).
		Align(lipgloss.Center).
		Italic(true)

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(data.BorderHex)).
		Width(ui.GetTerminalWidth()-4).
		Margin(0, 1).
		Padding(1)

	logo := ui.GetLogo(data.KeyHex, data.LabelHex, 0.5)
	welcomeText := logo + "\nWelcome to Chat Mode" + " (v" + version + ")"
	instructions := []string{
		"• '/help' lists all available commands",
		"• '/exit', '/quit' to end current chat session",
		"• Ctrl+C to cancel, Ctrl+D to clear the input",
		"• '/' for commands, '!' for local shell commands",
		"• '@' for files and folders",
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

// This is the legacy awaitChat function, which uses huh, don't support auto-complete
func (ci *ChatInfo) awaitChat_legacy() (string, error) {
	var input string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewText().
				Title("Chat").
				Value(&input).
				Placeholder("Type your message..."),
		),
	).WithKeyMap(ui.GetHuhKeyMap()) // 4. CRITICAL: Apply the keymap to the FORM level

	err := form.Run()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(input), nil
}

// This is the new awaitChat function, which uses bubbletea, support auto-complete
func (ci *ChatInfo) awaitChat() (string, error) {
	var commands []ui.Suggestion
	for cmd, desc := range chatCommandMap {
		commands = append(commands, ui.Suggestion{Command: cmd, Description: desc})
	}

	// Load workflow commands
	_ = service.GetWorkflowManager().LoadMetadata(chatCommandMap)

	// Add workflow commands
	wm := service.GetWorkflowManager()
	for cmd, desc := range wm.GetCommands() {
		// Skip if the command already exists in chatCommandMap
		if _, ok := chatCommandMap[cmd]; ok {
			continue
		}
		commands = append(commands, ui.Suggestion{Command: cmd, Description: desc})
	}

	// Sort commands by text
	sort.Slice(commands, func(i, j int) bool {
		return commands[i].Command < commands[j].Command
	})

	// Run chat input
	result, err := ui.RunChatInput(commands, ci.EditorInput, ci.History)
	if err != nil {
		return "", err
	}
	if result.Canceled {
		return "", fmt.Errorf("user canceled")
	}

	// Update history
	ci.History = result.History

	return result.Value, nil
}

func (ci *ChatInfo) startREPL() {
	// Initialize SharedState for the session
	ci.sharedState = data.NewSharedState()
	defer ci.sharedState.Clear()

	// Start the REPL
	ci.printWelcome()

	// Define prompt style
	tcol := ui.GetTerminalWidth()
	promptStyle := lipgloss.NewStyle().
		Background(lipgloss.Color(data.CurrentTheme.Background)).
		Foreground(lipgloss.Color(data.CurrentTheme.Foreground)).
		Padding(1, 2).Margin(0, 0, 1, 0). // padding and margin
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
				// Reset editor input
				ci.EditorInput = ""
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
		if os.IsNotExist(err) {
			fmt.Println("No available conversation yet.")
			return
		}
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
	m := ui.NewViewportModel(provider, content, func() string {
		return fmt.Sprintf("Conversation: %s", convoName)
	})
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		service.Errorf("Error running viewport: %v", err)
	}
}

func (ci *ChatInfo) callAgent(input string) {
	// Call agent using the shared runner, passing persisted SharedState
	err := RunAgent(input, ci.Files, convoName, yoloFlag, ci.outputFile, ci.sharedState)
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
			service.Errorf("Command failed with exit code %d", exitError.ExitCode())
		} else {
			service.Errorf("Command failed: %v", err)
		}
	}

	// Display the output
	if len(output) > 0 {
		// shell output color
		fmt.Printf(data.ShellOutputColor+"%s\n"+data.ResetSeq, output)
	}
}

func GenerateChatFileName() string {
	// Get the current time
	currentTime := time.Now()

	// Format the time as a string in the format "chat_YYYY-MM-DD_HH-MM-SS.json"
	filename := fmt.Sprintf("chat_%s", currentTime.Format("2006-01-02_15-04-05"))

	return filename
}
