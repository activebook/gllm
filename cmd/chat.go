package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/activebook/gllm/service"
	"github.com/ergochat/readline"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// chatCmd represents the chat command
var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive chat session (REPL)",
	Long: `Start an interactive chat session with the configured LLM.
This provides a Read-Eval-Print-Loop (REPL) interface where you can
have a continuous conversation with the model.

Special commands:
/exit, /quit - Exit the chat session
/clear, /reset - Clear context
/help, /? - Show available commands
/info, /i - Show current settings
/history, /h [num] [chars] - Show recent conversation history (default: 20 messages, 200 chars)
/markdown, /m [on|off] - Switch whether to render markdown or not
/mode single|multi - Switch input mode (chat(*) single, chat(#) multi)
/system, /S <@name|prompt> - change system prompt
/tools, /t [on|off|skip|confirm] - Switch whether to use embedding tools, skip tools confirmation
/template, /p <@name|tmpl> - change template
/think, /T [on|off] - Switch whether to use deep think mode
/search, /s <search_engine> [on|off] - select a search engine to use, or switch on/off
/mcp [on|off|list] - Switch whether to use MCP servers, list available servers
/reference, /r <num> - change link reference count
/usage, /u [on|off] - Switch whether to show token usage information
/editor, /e <editor>|list - Open external editor for multi-line input
/attach, /a <filename> - Attach a file to the chat session
/detach, /d <filename> - Detach a file to the chat session
/output, /o <filename> [off] - Save to output file for model responses
!<command> - Execute a shell command directly`,
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
	_gllmSinglePrompt   = "\033[96mchat(*)>\033[0m "
	_gllmMultiPrompt    = "\033[96mchat(#)>\033[0m "
	_gllmContinuePrompt = ">> "
	_gllmConfirmPrompt  = "\033[96mEnter\033[0m to send (\033[96mCtrl+C/Ctrl+D\033[0m to discard) "
	_gllmFarewell       = "Session ended."
	_gllmTempFile       = ".gllm-edit-*.tmp"
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
	chatCmd.Flags().IntVarP(&referenceFlag, "reference", "r", 5, "Specify the number of reference links to show")
	chatCmd.Flags().BoolVar(&deepDiveFlag, "deep-dive", false, "Enable deep dive search to fetch all links from search results")
	chatCmd.Flags().BoolVarP(&confirmToolsFlag, "confirm-tools", "", false, "Skip confirmation for tool operations")
}

type ChatInfo struct {
	Model                  string
	Provider               service.ModelProvider
	Files                  []*service.FileData
	Conversion             service.ConversationManager
	QuitFlag               bool
	maxRecursions          int
	outputFile             string
	InputMode              string
	InputLines             []string
	WaitingForConfirmation bool
	pendingEmptyLine       bool      // Track if we're waiting for debounce
	lastEmptyLineTime      time.Time // When the empty line was received
	previewShownTime       time.Time // When preview was shown (for buffered paste detection)
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
		Model:                  modelInfo["model"].(string),
		Provider:               provider,
		Files:                  files,
		Conversion:             cm,
		QuitFlag:               false,
		maxRecursions:          mr,
		InputMode:              GetEffectiveChatMode(),
		InputLines:             nil,
		WaitingForConfirmation: false,
	}
	return &ci
}

func (ci *ChatInfo) printWelcome() {
	fmt.Println("Welcome to GLLM Interactive Chat")
	fmt.Println("Type '/exit' or '/quit' to end the session, or '/help' for commands")
	fmt.Println("Use '/single' or '/multi' to switch input mode")
	fmt.Println("Use '/' for commands")
	fmt.Println("Use '!' for exec local commands")
	fmt.Println("Use Ctrl+C/Ctrl+D to exit")
	fmt.Println()
	ci.showHelp()
	fmt.Println()
	ci.printModeStatus()
	fmt.Println()
}

func (ci *ChatInfo) startREPL() {
	cfg := &readline.Config{
		Prompt: _gllmSinglePrompt,
	}
	rl, err := readline.NewEx(cfg)
	if err != nil {
		fmt.Printf("Error initializing readline: %v\n", err)
		return
	}
	defer rl.Close()

	// Start the REPL
	ci.printWelcome()
	for {

		var prompt string
		if ci.WaitingForConfirmation {
			// Need to confirm
			prompt = _gllmConfirmPrompt
		} else {
			switch ci.InputMode {
			case "single":
				// Single mode
				if len(ci.InputLines) == 0 {
					prompt = _gllmSinglePrompt
				} else {
					prompt = _gllmContinuePrompt
				}
			case "multi":
				// Multi mode
				if len(ci.InputLines) == 0 {
					prompt = _gllmMultiPrompt
				} else {
					prompt = _gllmContinuePrompt
				}
			}
		}
		rl.SetPrompt(prompt)

		// In the middle of processing multi-line input
		// Single("\") or Multi
		haveMultiInputs := (len(ci.InputLines) > 0)

		line, err := rl.Readline()
		if err != nil { // Handle EOF or other errors
			// Handle Ctrl+C and Handle Ctrl+D
			if err == readline.ErrInterrupt || err == io.EOF {
				if ci.WaitingForConfirmation || haveMultiInputs {
					// Discard multiline or editor content
					ci.resetInputState()
					continue
				} else {
					// Quit
					fmt.Println("\n" + _gllmFarewell)
					break
				}
			}
			fmt.Printf("Error reading line: %v\n", err)
			continue
		}

		line = strings.TrimSpace(line)

		// Handle special prefixes
		// Don't process commands/shell commands in the middle multi inputs
		// Exception: /preview and /pv are allowed during multi-input to check accumulated content
		if !ci.WaitingForConfirmation && !haveMultiInputs {
			if ci.startWithInnerCommand(line) {
				// / for inner commands
				ci.handleCommand(line)
				if ci.QuitFlag {
					break
				}
				continue
			} else if ci.startWithLocalCommand(line) {
				// ! for shell commands
				ci.executeShellCommand(line[1:])
				continue
			}
		}

		// Handle editor confirmation workflow
		if ci.WaitingForConfirmation {
			// we just care about the
			ci.processEditorInput()
			continue
		} else {
			switch ci.InputMode {
			case "single":
				ci.processSingleModeInput(line)
			case "multi":
				ci.processMultiModeInput(line)
			}
		}

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

func (ci *ChatInfo) addAttachFiles(input string) {
	// Normalize input by replacing /attach with /a
	input = strings.ReplaceAll(input, "/attach ", "/a ")

	// Split input into tokens
	tokens := strings.Fields(input)

	var wg sync.WaitGroup
	var mu sync.Mutex
	for i := 0; i < len(tokens); i++ {
		if tokens[i] == "/a" {
			if i+1 < len(tokens) {
				// Check if there's a file path after /a
				filePath := tokens[i+1]
				i++ // Skip the file path token

				wg.Add(1)
				go func(filePath string) {
					defer wg.Done()

					// Verify file exists and is not a directory
					if !checkIsLink(filePath) {
						fileInfo, err := os.Stat(filePath)
						if err != nil {
							if os.IsNotExist(err) {
								service.Errorf("File not found: %s\n", filePath)
							} else {
								service.Errorf("Error accessing file %s: %v\n", filePath, err)
							}
							return
						}
						if fileInfo.IsDir() {
							service.Errorf("Cannot attach directory: %s\n", filePath)
							return
						}
					}
					// Check if file is already attached
					mu.Lock()
					found := false
					for _, file := range ci.Files {
						if file.Path() == filePath {
							found = true
							break
						}
					}
					mu.Unlock()
					// If file is already attached, skip processing
					if found {
						service.Warnf("File already attached: %s", filePath)
						return
					}

					// Process the attachment
					file := processAttachment(filePath)
					if file == nil {
						service.Errorf("Error loading attachment: %s\n", filePath)
						return
					}

					// Append the file to the list of attachments
					mu.Lock()
					ci.Files = append(ci.Files, file)
					mu.Unlock()
					fmt.Printf("Attachment loaded: %s\n", filePath)
				}(filePath)
			} else {
				fmt.Println("Please specify a file path after /a")
			}
		}
		// Ignore other tokens
	}
	wg.Wait()

	if len(ci.Files) == 0 {
		fmt.Println("No attachments were loaded")
	}
}

func (ci *ChatInfo) detachFiles(input string) {
	// Normalize input by replacing /detach with /d
	input = strings.ReplaceAll(input, "/detach ", "/d ")

	// Handle "all" case
	if strings.Contains(input, "/d all") || strings.Contains(input, "/detach all") {
		if len(ci.Files) == 0 {
			fmt.Println("No attachments to detach")
			return
		}
		ci.Files = []*service.FileData{}
		fmt.Println("Detached all attachments")
		return
	}

	// Split input into tokens
	tokens := strings.Fields(input)

	// Process detach commands
	detachedAny := false
	for i := 0; i < len(tokens); i++ {
		if tokens[i] == "/d" {
			// Check if there's a file path after /d
			if i+1 >= len(tokens) {
				fmt.Println("Please specify a file path after /d")
				continue
			}

			// Get the file path (next token)
			filePath := tokens[i+1]
			i++ // Skip the file path token

			// Find and detach the file
			found := false
			for j, file := range ci.Files {
				if file.Path() == filePath {
					ci.Files = append(ci.Files[:j], ci.Files[j+1:]...)
					fmt.Printf("Detached: %s\n", filePath)
					detachedAny = true
					found = true
					break
				}
			}

			if !found {
				fmt.Printf("Attachment not found: %s\n", filePath)
			}
		}
	}

	if !detachedAny {
		fmt.Println("No valid detachment")
	}
}

func (ci *ChatInfo) processSingleModeInput(line string) {
	// Single mode: process immediately, but support \ for line continuation
	if strings.HasSuffix(line, "\\") {
		// Line continuation: accumulate until no \
		ci.InputLines = append(ci.InputLines, strings.TrimSuffix(line, "\\"))
		return
	}

	// End of line continuation: combine accumulated lines if any
	if len(ci.InputLines) > 0 {
		ci.InputLines = append(ci.InputLines, line)
		input := strings.Join(ci.InputLines, "\n")
		ci.resetInputState()
		ci.callAgent(input)
		return
	}

	// Single line input
	if line == "" {
		return
	}
	ci.callAgent(line)
}

func (ci *ChatInfo) processMultiModeInput(line string) {
	const debounceDelay = 300 * time.Millisecond

	// Check if we just showed preview and this input arrived instantly (buffered paste)
	if !ci.previewShownTime.IsZero() {
		elapsed := time.Since(ci.previewShownTime)
		// If input arrived < 100ms after preview, it was likely buffered during the sleep
		if elapsed < 100*time.Millisecond {
			// Cancel confirmation mode!
			ci.WaitingForConfirmation = false
			ci.previewShownTime = time.Time{} // Reset

			// Add the missing empty line that triggered the preview
			ci.InputLines = append(ci.InputLines, "")

			// Fall through to add the current line
			fmt.Println("...continuing paste...")
		} else {
			ci.previewShownTime = time.Time{} // Reset
		}
	}

	// Multi mode: accumulate lines
	if line == "" {
		if len(ci.InputLines) > 0 {
			// Empty line with content - sleep to debounce
			time.Sleep(debounceDelay)

			// Show preview
			ci.showPreview()
			ci.WaitingForConfirmation = true
			ci.previewShownTime = time.Now()
		}
		// No content yet, just ignore empty line
		return
	}

	// Non-empty line: add to accumulation
	ci.InputLines = append(ci.InputLines, line)
}

func (ci *ChatInfo) handleEditorCommand() {
	editor := getPreferredEditor()
	tempFile, err := createTempFile(_gllmTempFile)
	if err != nil {
		service.Errorf("Failed to create temp file: %v", err)
		return
	}
	defer os.Remove(tempFile)

	// Open in detected editor
	cmd := exec.Command(editor, tempFile)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Printf("Opening in %s...\n", editor)
	if err := cmd.Run(); err != nil {
		service.Errorf("Editor failed: %v", err)
		return
	}

	// Read back edited content
	recv, err := os.ReadFile(tempFile)
	if err != nil {
		service.Errorf("Failed to read edited content: %v", err)
		return
	}

	content := string(recv)
	content = strings.Trim(content, " \n")
	if len(content) == 0 {
		ci.resetInputState()
		fmt.Println("No content.")
		return
	}

	lines := strings.Split(content, "\n")
	ci.InputLines = lines
	ci.WaitingForConfirmation = true

	// Use shared preview display
	ci.showPreview()
}

func (ci *ChatInfo) processEditorInput() {
	// User pressed Enter - send content
	input := strings.Join(ci.InputLines, "\n")
	ci.resetInputState()
	ci.callAgent(input)
}

func (ci *ChatInfo) handleEditor() {
	// No arguments - check if preferred editor is set
	if getPreferredEditor() == "" {
		// No preferred editor set, show list
		listAvailableEditors()
	} else {
		// Preferred editor set, open it
		ci.handleEditorCommand()
	}
}

func (ci *ChatInfo) resetInputState() {
	ci.InputLines = nil
	ci.WaitingForConfirmation = false
	ci.pendingEmptyLine = false
}

func (ci *ChatInfo) printModeStatus() {
	if ci.InputMode == "single" {
		color.New(color.FgGreen, color.Bold).Printf("Single-line mode: ")
		color.New(color.FgCyan).Printf("chat(*)> ")
		color.New(color.FgYellow).Println("- Press Enter to submit immediately")
	} else {
		color.New(color.FgGreen, color.Bold).Printf("Multi-line mode: ")
		color.New(color.FgCyan).Printf("chat(#)> ")
		color.New(color.FgYellow).Println("- Enter lines, empty line to preview & confirm")
	}
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
		if searchEngine != nil {
			searchEngine["deep_dive"] = deepDiveFlag   // Add deep dive flag to search engine settings
			searchEngine["references"] = referenceFlag // Add references flag to search engine settings
		}
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
	ci.resetInputState()
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

// GetEffectiveChatMode returns the chat input mode to use
func GetEffectiveChatMode() string {
	mode := viper.GetString("chat.mode")
	if mode == "" {
		mode = "single" // Default to multi mode
	}
	return mode
}

// SetEffectiveChatMode sets the chat input mode
func SetEffectiveChatMode(mode string) error {
	if mode != "single" && mode != "multi" {
		return fmt.Errorf("invalid chat mode: %s (must be 'single' or 'multi')", mode)
	}

	viper.Set("chat.mode", mode)
	if err := writeConfig(); err != nil {
		return fmt.Errorf("failed to save chat mode: %w", err)
	}
	return nil
}
