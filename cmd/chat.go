package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

		files := []*service.FileData{}
		// Start a goroutine for your actual LLM work
		done := make(chan bool)
		go func() {

			// Process all prompt building
			if cmd.Flags().Changed("attachment") {
				// Process attachments
				for _, attachment := range attachments {
					fileData := processAttachment(attachment)
					if fileData != nil {
						files = append(files, fileData)
					}
				}
			}
			done <- true
		}()
		// Update the spinner until work is done
		<-done
		indicator.Stop()

		// Build the ChatInfo object
		chatInfo = &ChatInfo{
			Files:    files,
			QuitFlag: false,
		}

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

func buildChatInfo(files []*service.FileData) *ChatInfo {

	ci := ChatInfo{
		Files:    files,
		QuitFlag: false,
	}
	return &ci
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
	agent := ci.getActiveAgent()
	if agent == nil {
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
	agent := ci.getActiveAgent()
	if agent == nil {
		return
	}

	// Get conversation data
	convoData, convoName, err := ci.getConvoData(agent)
	if err != nil {
		service.Errorf("%v", err)
		return
	}

	// Detect provider based on message format
	isCompatible, provider, modelProvider := ci.checkConvoFormat(agent, convoData)
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

// Get conversation data
// As soon as the function is called, these named returned variables are created and initialized to their zero value
func (ci *ChatInfo) getConvoData(agent *data.AgentConfig) (data []byte, name string, err error) {

	// Construct conversation manager
	cm, err := service.ConstructConversationManager(convoName, agent.Model.Provider)
	if err != nil {
		return nil, "", fmt.Errorf("error constructing conversation manager: %v\n", err)
	}

	convoPath := cm.GetPath()

	// Check if conversation exists
	if _, err := os.Stat(convoPath); os.IsNotExist(err) {
		return nil, "", fmt.Errorf("conversation '%s' not found.\n", convoPath)
	}

	// Read and parse the conversation file
	data, err = os.ReadFile(convoPath)
	if err != nil {
		return nil, "", fmt.Errorf("error reading conversation file: %v", err)
	}

	name = strings.TrimSuffix(filepath.Base(convoPath), filepath.Ext(convoPath))
	return data, name, nil
}

/*
 * Write converted conversation data for provider
 * data: converted conversation data, compatible to provider
 * provider: model provider
 */
func (ci *ChatInfo) writeConvoData(data []byte, provider string) error {
	// Construct conversation manager
	cm, err := service.ConstructConversationManager(convoName, provider)
	if err != nil {
		return fmt.Errorf("error constructing conversation manager: %v\n", err)
	}

	convoPath := cm.GetPath()

	// Check if conversation exists
	var fi os.FileInfo
	if fi, err = os.Stat(convoPath); os.IsNotExist(err) {
		return fmt.Errorf("conversation '%s' not found.\n", convoPath)
	}

	// Write the conversation file
	err = os.WriteFile(convoPath, data, fi.Mode())
	if err != nil {
		return fmt.Errorf("error writing conversation file: %v", err)
	}

	return nil
}

// Check if conversation data is compatible with the current model provider
func (ci *ChatInfo) checkConvoFormat(agent *data.AgentConfig, convoData []byte) (isCompatible bool, provider string, modelProvider string) {

	modelProvider = agent.Model.Provider

	// Detect provider based on message format
	provider = service.DetectMessageProvider(convoData)

	// Check compatibility: OpenAI and OpenAI Compatible are compatible
	isCompatible = provider == modelProvider
	if !isCompatible {
		// OpenAI and OpenAI Compatible are compatible
		// OpenAI Compatible and Anthropic are compatible on pure text contents
		// Unknown provider is no message yet
		isCompatible = provider == service.ModelProviderUnknown ||
			(provider == service.ModelProviderOpenAI && modelProvider == service.ModelProviderOpenAICompatible) ||
			(provider == service.ModelProviderOpenAICompatible && modelProvider == service.ModelProviderOpenAI) ||
			(provider == service.ModelProviderOpenAICompatible && modelProvider == service.ModelProviderAnthropic)
	}

	return isCompatible, provider, modelProvider
}

func (ci *ChatInfo) getActiveAgent() *data.AgentConfig {
	// Get ActiveAgent
	store := data.NewConfigStore()
	agent := store.GetActiveAgent()
	if agent == nil {
		service.Errorf("No active agent found")
		return nil
	}

	// Bugfix: Auto-detect provider if not set
	// Legacy models don't have provider set
	if agent.Model.Provider == "" {
		service.Debugf("Auto-detecting provider for %s", agent.Model.Model)
		agent.Model.Provider = service.DetectModelProvider(agent.Model.Endpoint, agent.Model.Model)
		store.SetModel(agent.Model.Name, &agent.Model)
	} else {
		service.Debugf("Provider: [%s]", agent.Model.Provider)
	}

	return agent
}

func (ci *ChatInfo) callAgent(input string) {
	// Get active agent
	agent := ci.getActiveAgent()
	if agent == nil {
		return
	}

	// Get conversation data
	convoData, convoName, err := ci.getConvoData(agent)
	if err != nil {
		service.Errorf("%v", err)
		return
	}

	// Detect provider based on message format
	isCompatible, provider, modelProvider := ci.checkConvoFormat(agent, convoData)
	if !isCompatible {
		service.Debugf("Conversation '%s' [%s] is not compatible with the current model provider [%s].\n", convoName, provider, modelProvider)
		// Convert conversation data to compatible format
		convertData, err := service.ConvertMessages(convoData, provider, modelProvider)
		if err != nil {
			service.Errorf("%v", err)
			return
		}
		// Write conversation data
		err = ci.writeConvoData(convertData, modelProvider)
		if err != nil {
			service.Errorf("%v", err)
			return
		}
		service.Debugf("Conversation '%s' converted to compatible format [%s].\n", convoName, modelProvider)
	}

	// Yolo flag
	yolo := false
	if yoloFlag {
		yolo = true
	}

	tb := TextBuilder{}

	// Get template content
	store := data.NewConfigStore()
	templateContent := store.GetTemplate(agent.Template)
	tb.appendText(templateContent)
	tb.appendText(input)

	//Process @ references in prompt
	prompt := tb.String()
	atRefProcessor := service.NewAtRefProcessor()
	processedPrompt, err := atRefProcessor.ProcessText(prompt)
	if err != nil {
		service.Warnf("Skip processing @ references in prompt: %v", err)
		// Continue with original prompt if processing fails
		processedPrompt = prompt
	}

	// Get system prompt
	sys_prompt := store.GetSystemPrompt(agent.SystemPrompt)

	// Get memory content
	memStore := data.NewMemoryStore()
	memoryContent := memStore.GetFormatted()
	if memoryContent != "" {
		sys_prompt += "\n\n" + memoryContent
	}

	// Load MCP config
	mcpStore := data.NewMCPStore()
	mcpConfig, _, _ := mcpStore.Load()

	// Check whether model is valid
	if agent.Model.Name == "" {
		service.Errorf("No model specified")
		return
	} else {
		model := store.GetModel(agent.Model.Name)
		if model == nil {
			service.Errorf("Model %s not found", agent.Model.Name)
			return
		}
	}

	// Call agent
	op := service.AgentOptions{
		Prompt:         processedPrompt,
		SysPrompt:      sys_prompt,
		Files:          ci.Files,
		ModelInfo:      &agent.Model,
		SearchEngine:   &agent.Search,
		MaxRecursions:  agent.MaxRecursions,
		ThinkingLevel:  agent.Think,
		EnabledTools:   agent.Tools,
		UseMCP:         agent.MCP,
		YoloMode:       yolo,
		AppendUsage:    agent.Usage,
		AppendMarkdown: agent.Markdown,
		OutputFile:     ci.outputFile,
		QuietMode:      false,
		ConvoName:      convoName,
		MCPConfig:      mcpConfig,
	}

	err = service.CallAgent(&op)
	if err != nil {
		service.Errorf("%v", err)
		return
	}

	// We must reset the files after processing
	// We shouldn't pass the files to the next call each time
	// Because the files are already in the context of this conversation
	// The same to system prompt and template
	// We shouldn't pass the system prompt and template to the next call each time

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
