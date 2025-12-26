package cmd

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/activebook/gllm/service"
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
	Run: func(cmd *cobra.Command, args []string) {
		// Create an indeterminate progress bar
		indicator := service.NewIndicator("Processing...")

		files := []*service.FileData{}
		// Start a goroutine for your actual LLM work
		done := make(chan bool)
		go func() {

			// If model flag is provided, update the default model
			if cmd.Flags().Changed("model") {
				if err := SetEffectiveModel(modelFlag); err != nil {
					service.Warnf("%v", err)
					fmt.Println("Using default model instead")
				}
			}

			// If system prompt is provided, update the default system prompt
			if sysPromptFlag != "" {
				if err := SetEffectiveSystemPrompt(sysPromptFlag); err != nil {
					service.Warnf("%v", err)
					fmt.Println("Using default system prompt instead")
				}
			}

			// If template is provided, update the default template
			if templateFlag != "" {
				if err := SetEffectiveTemplate(templateFlag); err != nil {
					service.Warnf("%v", err)
					fmt.Println("Using default template instead")
				}
			}

			// Search
			if !searchFlag {
				// if search flag are not set, check if they are enabled globally
				searchFlag = IsSearchEnabled()
			}

			// Tools
			if !toolsFlag {
				// if tools flag are not set, check if they are enabled globally
				toolsFlag = AreToolsEnabled()
			}

			// Check if think mode is enabled
			if !thinkFlag {
				// if think flag is not set, check if it's enabled globally
				thinkFlag = IsThinkEnabled()
			}

			// MCP
			if !mcpFlag {
				// if mcp flag is not set, check if it's enabled globally
				mcpFlag = AreMCPServersEnabled()
			}

			// Always save a conversation file regardless of the flag
			if !cmd.Flags().Changed("conversation") {
				convoName = GenerateChatFileName()
			}

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
		chatInfo := buildChatInfo(files)
		if chatInfo == nil {
			return
		}

		// Start the REPL
		chatInfo.startREPL()
	},
}

var ()

const (
	_gllmTempFile = ".gllm-edit-*.tmp"
)

func init() {
	rootCmd.AddCommand(chatCmd)

	// Add chat-specific flags
	// In each chat session, it shouldn't change model, because the chat history would be invalid.
	// Attach should be used inside chat session
	// Imagine like using web llm ui, you can attach file to the chat session and turn search on and off
	chatCmd.Flags().StringVarP(&modelFlag, "model", "m", "", "Model to use for the chat session")
	chatCmd.Flags().StringVarP(&sysPromptFlag, "system", "S", "", "System prompt to use for the chat session")
	chatCmd.Flags().StringVarP(&templateFlag, "template", "p", "", "Template to use for the chat session")
	chatCmd.Flags().StringSliceVarP(&attachments, "attachment", "a", []string{}, "Specify file(s) or image(s) to append to the chat sessioin")
	chatCmd.Flags().StringVarP(&convoName, "conversation", "c", GenerateChatFileName(), "Name for this chat session")
	chatCmd.Flags().BoolVarP(&searchFlag, "search", "s", false, "Search engine for the chat session")
	chatCmd.Flags().BoolVarP(&toolsFlag, "tools", "t", true, "Enable or disable tools for the chat session")
	chatCmd.Flags().BoolVarP(&thinkFlag, "think", "T", false, "Enable or disable deep think mode for the chat session")
	chatCmd.Flags().BoolVarP(&mcpFlag, "mcp", "", false, "Enable or disable MCP servers for the chat session")
	chatCmd.Flags().Lookup("search").NoOptDefVal = service.GetDefaultSearchEngineName()
	chatCmd.Flags().BoolVarP(&confirmToolsFlag, "confirm-tools", "", false, "Skip confirmation for tool operations")
}

type ChatInfo struct {
	Model         string
	Provider      service.ModelProvider
	Files         []*service.FileData
	Conversion    service.ConversationManager
	QuitFlag      bool   // for cmd /quit or /exit
	EditorInput   string // for /e editor edit
	maxRecursions int
	outputFile    string
}

func buildChatInfo(files []*service.FileData) *ChatInfo {

	// Bugfix:
	// When convoName is an index number, and use it to find convo file
	// it will return the convo file(sorted by modified time)
	// but if user mannually update other convo file, the modified time will change
	// so the next time using index to load convo file, would load the wrong one
	// How to fix:
	// First, we need convert convoName
	// And, keep that name, the next time, it willn't use the index to load convo file
	name, _ := service.FindConvosByIndex(convoName)
	if name != "" {
		convoName = name
	}

	// Construct conversation manager
	// Use dual detection: endpoint first, then model name patterns
	_, modelInfo := GetEffectiveModel()
	provider := service.DetectModelProvider(modelInfo["endpoint"].(string), modelInfo["model"].(string))
	cm, err := service.ConstructConversationManager(convoName, provider)
	if err != nil {
		service.Errorf("Error constructing conversation manager: %v\n", err)
		return nil
	}
	mr := GetMaxRecursions()
	ci := ChatInfo{
		Model:         modelInfo["model"].(string),
		Provider:      provider,
		Files:         files,
		Conversion:    cm,
		QuitFlag:      false,
		maxRecursions: mr,
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

func (ci *ChatInfo) callAgent(input string) {
	var sb strings.Builder
	appendText(&sb, GetEffectiveTemplate())
	appendText(&sb, input)

	// Process @ references in prompt
	prompt := sb.String()
	atRefProcessor := service.NewAtRefProcessor()
	processedPrompt, err := atRefProcessor.ProcessText(prompt)
	if err != nil {
		service.Warnf("Error processing @ references in prompt: %v", err)
		// Continue with original prompt if processing fails
		processedPrompt = prompt
	}

	_, modelInfo := GetEffectiveModel()
	sys_prompt := GetEffectiveSystemPrompt()

	// must recheck tools flag, because it can be set /tools
	toolsFlag = AreToolsEnabled()
	// If tools are enabled, we will use the search engine
	searchFlag = IsSearchEnabled()
	// If search flag is set, we will use the search engine, too
	var searchEngine map[string]any
	if searchFlag || toolsFlag {
		_, searchEngine = GetEffectiveSearchEngine()
	}

	// check if think flag is set
	thinkFlag = IsThinkEnabled()
	// check if mcp flag is set
	mcpFlag = AreMCPServersEnabled()
	// Include usage metainfo
	includeUsage := IncludeUsageMetainfo()
	// Include markdown
	includeMarkdown := IncludeMarkdown()

	op := service.AgentOptions{
		Prompt:           processedPrompt,
		SysPrompt:        sys_prompt,
		Files:            ci.Files,
		ModelInfo:        &modelInfo,
		SearchEngine:     &searchEngine,
		MaxRecursions:    ci.maxRecursions,
		ThinkMode:        thinkFlag,
		UseTools:         toolsFlag,
		UseMCP:           mcpFlag,
		SkipToolsConfirm: confirmToolsFlag,
		AppendUsage:      includeUsage,
		AppendMarkdown:   includeMarkdown,
		OutputFile:       ci.outputFile,
		QuietMode:        false,
		ConvoName:        convoName,
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
	// Reset the system prompt
	SetEffectiveSystemPrompt("")
	// Reset the template
	SetEffectiveTemplate("")
	// Reset the input lines
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
