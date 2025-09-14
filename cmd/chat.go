package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"text/tabwriter"

	"github.com/activebook/gllm/service"
	"github.com/chzyer/readline"
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
	_gllmConfirmPrompt  = "Press \033[96mEnter\033[0m to send, or use \033[96mCtrl+C/Ctrl+D\033[0m to discard"
	_gllmFarewell       = "Session ended."
	_gllmTempFile       = "gllm-edit-*.tmp"
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

func (ci *ChatInfo) startREPL() {
	fmt.Println("Welcome to GLLM Interactive Chat")
	fmt.Println("Type '/exit' or '/quit' to end the session, or '/help' for commands")
	fmt.Println("Use '/mode single|multi' to switch chat mode")
	fmt.Println("Use '/' for commands")
	fmt.Println("Use '!' for exec local commands")
	fmt.Println("Use Ctrl+C/Ctrl+D to exit")
	fmt.Println()
	ci.showHelp()
	fmt.Println()
	ci.printModeStatus()
	fmt.Println()

	cfg := &readline.Config{
		Prompt: _gllmMultiPrompt,
	}
	rl, err := readline.NewEx(cfg)
	if err != nil {
		fmt.Printf("Error initializing readline: %v\n", err)
		return
	}
	defer rl.Close()

	for {

		var prompt string
		if ci.WaitingForConfirmation {
			prompt = _gllmConfirmPrompt
		} else if ci.InputMode == "single" {
			if len(ci.InputLines) == 0 {
				prompt = _gllmSinglePrompt
			} else {
				prompt = _gllmContinuePrompt
			}
		} else {
			if len(ci.InputLines) == 0 {
				prompt = _gllmMultiPrompt
			} else {
				prompt = _gllmContinuePrompt
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
				if ci.WaitingForConfirmation {
					// Discard editor content
					ci.InputLines = nil
					ci.WaitingForConfirmation = false
					continue
				} else if haveMultiInputs {
					// Break out multi input
					ci.InputLines = nil
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

		// Handle editor confirmation workflow
		if ci.WaitingForConfirmation {
			ci.handleEditorConfirmation()
			continue
		}

		// Handle special prefixes
		// ! for shell commands
		// Don't process commands/shell commands in the middle multi inputs
		if !haveMultiInputs && (strings.HasPrefix(line, "/") || strings.HasPrefix(line, "!")) {
			// Execute commands/shell commands
			if strings.HasPrefix(line, "/") {
				ci.handleCommand(line)
				if ci.QuitFlag {
					break
				}
			} else {
				ci.executeShellCommand(line[1:])
			}
			continue
		}

		// Process input based on mode
		switch ci.InputMode {
		case "single":
			ci.processSingleModeInput(line)
		case "multi":
			ci.processMultiModeInput(line)
		}
		if ci.QuitFlag {
			break
		}
	}
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
	PreferredEditor        string
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
	_, modelInfo := GetEffectiveModel()
	provider := service.DetectModelProvider(modelInfo["endpoint"].(string))
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
		InputMode:     GetEffectiveChatMode(),
		InputLines:    []string{},
	}
	return &ci
}

func (ci *ChatInfo) clearContext() {
	// Reset all settings
	viper.Set("agent.system_prompt", "")
	viper.Set("agent.template", "")
	viper.Set("agent.search", "")
	err := viper.WriteConfig()
	if err != nil {
		service.Errorf("Error clearing context: %v\n", err)
		return
	}
	// Empty the conversation history
	err = ci.Conversion.Clear()
	if err != nil {
		service.Errorf("Error clearing context: %v\n", err)
		return
	}
	// Empty attachments
	ci.Files = []*service.FileData{}
	fmt.Printf("Context cleared.\n")
}

func (ci *ChatInfo) setTemplate(template string) {
	if err := SetEffectiveTemplate(template); err != nil {
		service.Warnf("%v", err)
		fmt.Println("Ignore template prompt")
	} else {
		fmt.Printf("Switched to template: %s\n", template)
	}
}

func (ci *ChatInfo) setSystem(system string) {
	if err := SetEffectiveSystemPrompt(system); err != nil {
		service.Warnf("%v", err)
		fmt.Println("Ignore system prompt")
	} else {
		fmt.Printf("Switched to system prompt: %s\n", system)
	}
}

func (ci *ChatInfo) setSearchEngine(engine string) {
	switch engine {
	case "on":
		searchOnCmd.Run(searchCmd, []string{})
	case "off":
		searchOffCmd.Run(searchCmd, []string{})
	default:
		succeed := SetEffectSearchEngineName(engine)
		if succeed {
			fmt.Printf("Switched to search engine: %s\n", GetEffectSearchEngineName())
		}
	}
}

func (ci *ChatInfo) setReferences(count string) {
	num, err := strconv.Atoi(count)
	if err != nil {
		fmt.Println("Invalid number")
		return
	}
	referenceFlag = num
	fmt.Printf("Reference count set to %d\n", num)
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

func (ci *ChatInfo) showInfo() {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	sectionColor := color.New(color.FgCyan, color.Bold).SprintFunc()
	headerColor := color.New(color.FgYellow, color.Bold).SprintFunc()
	highlightColor := color.New(color.FgGreen, color.Bold).SprintFunc()
	keyColor := color.New(color.FgMagenta, color.Bold).SprintFunc()

	printSection := func(title string) {
		fmt.Println()
		fullTitle := fmt.Sprintf("=== %s ===", strings.ToUpper(title))
		lineWidth := 50
		padding := (lineWidth - len(fullTitle)) / 2
		if padding < 0 {
			padding = 0
		}
		fmt.Printf("%s%s\n", strings.Repeat(" ", padding), sectionColor(fullTitle))
		fmt.Println(color.New(color.FgCyan).Sprint(strings.Repeat("-", lineWidth)))
	}

	printSection("CURRENT SETTINGS")

	// Basic settings
	fmt.Fprintln(w, headerColor(" SETTING ")+"\t"+headerColor(" VALUE "))
	fmt.Fprintln(w, headerColor("---------")+"\t"+headerColor("-------"))
	fmt.Fprintf(w, "%s\t%s\n", keyColor("Model"), highlightColor(ci.Model))
	fmt.Fprintf(w, "%s\t%s\n", keyColor("Search Engine"), highlightColor(GetEffectSearchEngineName()))
	fmt.Fprintf(w, "%s\t%t\n", keyColor("Deep Think"), IsThinkEnabled())
	fmt.Fprintf(w, "%s\t%t\n", keyColor("Embedding Tools"), AreToolsEnabled())
	fmt.Fprintf(w, "%s\t%t\n", keyColor("MCP Servers"), AreMCPServersEnabled())
	fmt.Fprintf(w, "%s\t%t\n", keyColor("Markdown"), IncludeMarkdown())
	fmt.Fprintf(w, "%s\t%t\n", keyColor("Usage Metainfo"), IncludeUsageMetainfo())
	fmt.Fprintf(w, "%s\t%s\n", keyColor("Output File"), ci.outputFile)
	w.Flush()

	// System prompt
	printSection("SYSTEM PROMPT")
	fmt.Printf("%s\n", GetEffectiveSystemPrompt())

	// Template
	printSection("TEMPLATE")
	fmt.Printf("%s\n", GetEffectiveTemplate())

	// Attachments
	printSection("ATTACHMENTS")
	if len(ci.Files) > 0 {
		fmt.Printf("%s (%d):\n", keyColor("Attachments"), len(ci.Files))
		for _, file := range ci.Files {
			fmt.Printf("  - [%s]: %s\n", file.Format(), file.Path())
		}
	} else {
		fmt.Println("Attachments: None")
	}

	fmt.Println(color.New(color.FgCyan, color.Bold).Sprint(strings.Repeat("=", 50)))
}

func (ci *ChatInfo) showHelp() {
	fmt.Println("Available commands:")
	fmt.Println("  /exit, /quit - Exit the chat session")
	fmt.Println("  /clear, /reset - Clear conversation history")
	fmt.Println("  /help, /? - Show this help message")
	fmt.Println("  /info, /i - Show current settings")
	fmt.Println("  /history, /h [num] [chars] - Show recent conversation history (default: 20 messages, 200 chars)")
	fmt.Println("  /markdown, /m [on|off] - Switch whether to render markdown or not")
	fmt.Println("  /mode single|multi - Switch input mode (chat(*) single, chat(#) multi - press Enter on empty line to submit)")
	fmt.Println("  /attach, /a <filename> - Attach a file to the conversation")
	fmt.Println("  /detach, /d <filename|all> - Detach a file from the conversation")
	fmt.Println("  /template, /p \"<tmpl|name>\" - Change the template")
	fmt.Println("  /system /S \"<prompt|name>\" - Change the system prompt")
	fmt.Println("  /think, /T \"[on|off]\" - Switch whether to use deep think mode")
	fmt.Println("  /search, /s \"<engine>[on|off]\" - Change the search engine, or switch on/off")
	fmt.Println("  /tools, /t \"[on|off|skip|confirm]\" - Switch whether to use embedding tools, skip tools confirmation")
	fmt.Println("  /mcp \"[on|off|list]\" - Switch whether to use MCP servers, or list available servers")
	fmt.Println("  /reference, /r \"<num>\" - Change the search link reference count")
	fmt.Println("  /usage, /u \"[on|off]\" - Switch whether to show token usage information")
	fmt.Println("  /editor, /e <editor>|list - Open external editor for multi-line input")
	fmt.Println("  /output, /o <filename> [off] - Save to output file for model responses")
	fmt.Println("  !<command> - Execute a shell command directly (e.g. !ls -la)")
}

func (ci *ChatInfo) showHistory(num int, chars int) {

	convoPath := ci.Conversion.GetPath()

	// Check if conversation exists
	if _, err := os.Stat(convoPath); os.IsNotExist(err) {
		service.Errorf("Conversation '%s' not found.\n", convoPath)
		return
	}

	// Read and parse the conversation file
	data, err := os.ReadFile(convoPath)
	if err != nil {
		service.Errorf("error reading conversation file: %v", err)
		return
	}

	convoName := strings.TrimSuffix(filepath.Base(convoPath), filepath.Ext(convoPath))
	// Display conversation details
	fmt.Printf("Name: %s\n", convoName)

	switch ci.Provider {
	case service.ModelGemini:
		service.DisplayGeminiConversationLog(data, num, chars)
	case service.ModelOpenChat:
		service.DisplayOpenAIConversationLog(data, num, chars)
	case service.ModelOpenAI, service.ModelMistral, service.ModelOpenAICompatible:
		service.DisplayOpenAIConversationLog(data, num, chars)
	default:
		fmt.Println("Unknown provider")
	}
}

func (ci *ChatInfo) setUseTools(useTools string) {

	switch useTools {
	// Set useTools on or off
	case "on":
		SwitchUseTools(useTools)
	case "off":
		SwitchUseTools(useTools)

		// Set whether or not to skip tools confirmation
	case "confirm":
		confirmToolsFlag = false
		fmt.Print("Tool operations would need confirmation\n")
	case "skip":
		confirmToolsFlag = true
		fmt.Print("Tool confirmation would skip\n")

	default:
		toolsCmd.Run(toolsCmd, nil)
	}
}

func (ci *ChatInfo) setUseMCP(useMCP string) {
	switch useMCP {
	case "on":
		SwitchMCP(useMCP)
	case "off":
		SwitchMCP(useMCP)
	case "list":
		mcpListCmd.Run(mcpListCmd, []string{})
	default:
		mcpCmd.Run(mcpCmd, []string{})
	}
}

func (ci *ChatInfo) setOutputFile(path string) {
	switch path {
	case "":
		if ci.outputFile == "" {
			fmt.Println("No output file is currently set")
		} else {
			fmt.Printf("Current output file: %s\n", ci.outputFile)
		}
	case "off":
		ci.outputFile = ""
		fmt.Println("No output file")
	default:
		filename := strings.TrimSpace(path)
		err := validFilePath(filename, false)
		if err != nil {
			service.Warnf("%v", err)
			return
		}
		// If we get here, the file can be created/overwritten
		ci.outputFile = filename
		fmt.Printf("Output file set to: %s\n", filename)
	}
}

func (ci *ChatInfo) processSingleModeInput(line string) {
	// Single mode: process immediately, but support \ for line continuation
	if strings.HasSuffix(line, "\\") {
		// Line continuation: accumulate until no \
		ci.InputLines = append(ci.InputLines, strings.TrimSuffix(line, "\\"))
		return
	}

	// End of input: combine accumulated lines if any
	if len(ci.InputLines) > 0 {
		ci.InputLines = append(ci.InputLines, line)
		input := strings.Join(ci.InputLines, "\n")
		ci.InputLines = nil
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
	// Multi mode: accumulate lines, submit on empty line
	if line == "" {
		if len(ci.InputLines) > 0 {
			// Submit accumulated input
			input := strings.Join(ci.InputLines, "\n")
			ci.InputLines = nil
			ci.callAgent(input)
			return
		}
	} else {
		// Non-empty line: add to accumulation
		ci.InputLines = append(ci.InputLines, line)
	}
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
		ci.InputLines = nil
		ci.WaitingForConfirmation = false
		fmt.Println("No content.")
		return
	}

	lines := strings.Split(content, "\n")

	ci.InputLines = lines
	ci.WaitingForConfirmation = true

	// Display edited content
	separator := strings.Repeat("═", (service.GetTerminalWidth()*3)/4)
	fmt.Println(separator)
	for i, line := range lines {
		fmt.Printf("%d: %s\n", i+1, line)
	}
	fmt.Println()
}

func (ci *ChatInfo) handleEditorConfirmation() {
	// User pressed Enter - send content
	input := strings.Join(ci.InputLines, "\n")
	ci.InputLines = nil
	ci.WaitingForConfirmation = false
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

func (ci *ChatInfo) printModeStatus() {
	if ci.InputMode == "single" {
		color.New(color.FgGreen, color.Bold).Printf("Single-line mode: ")
		color.New(color.FgCyan).Printf("chat(*)> ")
		color.New(color.FgYellow).Println("- Press Enter to submit immediately")
	} else {
		color.New(color.FgGreen, color.Bold).Printf("Multi-line mode: ")
		color.New(color.FgCyan).Printf("chat(#)> ")
		color.New(color.FgYellow).Println("- Press Enter to add lines, press Enter on empty line to submit")
	}
}

func (ci *ChatInfo) handleCommand(cmd string) {
	// Split the command into parts
	parts := strings.SplitN(cmd, " ", 3)
	command := parts[0]
	switch command {
	case "/exit", "/quit":
		ci.QuitFlag = true
		fmt.Println("Exiting chat session")
		return

	case "/help", "/?":
		ci.showHelp()

	case "/history", "/h":
		num := 20
		chars := 200
		// Parse arguments
		if len(parts) > 1 {
			if n, err := strconv.Atoi(parts[1]); err == nil && n > 0 {
				num = n
			}
		}
		if len(parts) > 2 {
			if c, err := strconv.Atoi(parts[2]); err == nil && c > 0 {
				chars = c
			}
		}
		ci.showHistory(num, chars)

	case "/markdown", "/m":
		if len(parts) < 2 {
			markdownCmd.Run(markdownCmd, []string{})
			return
		}
		mark := strings.TrimSpace(parts[1])
		SwitchMarkdown(mark)

	case "/clear", "/reset":
		ci.clearContext()

	case "/template", "/p":
		if len(parts) < 2 {
			templateListCmd.Run(templateListCmd, []string{})
			return
		}
		// Join all remaining parts as they might contain spaces
		tmpl := strings.Join(parts[1:], " ")
		tmpl = strings.TrimSpace(tmpl)
		ci.setTemplate(tmpl)

	case "/system", "/S":
		if len(parts) < 2 {
			systemListCmd.Run(systemListCmd, []string{})
			return
		}
		sysPrompt := strings.Join(parts[1:], " ")
		sysPrompt = strings.TrimSpace(sysPrompt)
		ci.setSystem(sysPrompt)

	case "/search", "/s":
		if len(parts) < 2 {
			searchCmd.Run(searchCmd, []string{})
			return
		}
		engine := strings.TrimSpace(parts[1])
		ci.setSearchEngine(engine)

	case "/tools", "/t":
		if len(parts) < 2 {
			toolsCmd.Run(toolsCmd, []string{})
			return
		}
		tools := strings.TrimSpace(parts[1])
		ci.setUseTools(tools)

	case "/mcp":
		if len(parts) < 2 {
			mcpCmd.Run(mcpCmd, []string{})
			return
		}
		mcp := strings.TrimSpace(parts[1])
		ci.setUseMCP(mcp)

	case "/think", "/T":
		if len(parts) < 2 {
			thinkCmd.Run(thinkCmd, []string{})
			return
		} else {
			mode := strings.TrimSpace(parts[1])
			SwitchThinkMode(mode)
		}

	case "/reference", "/r":
		if len(parts) < 2 {
			fmt.Println("Please specify a number")
			return
		}
		count := strings.TrimSpace(parts[1])
		ci.setReferences(count)

	case "/usage", "/u":
		if len(parts) < 2 {
			usageCmd.Run(usageCmd, []string{})
			return
		}
		usage := strings.TrimSpace(parts[1])
		SwitchUsageMetainfo(usage)

	case "/attach", "/a":
		if len(parts) < 2 {
			fmt.Println("Please specify a file path")
			return
		}
		ci.addAttachFiles(cmd)

	case "/detach", "/d":
		if len(parts) < 2 {
			fmt.Println("Please specify a file path")
			return
		}
		ci.detachFiles(cmd)

	case "/editor", "/e":
		if len(parts) < 2 {
			ci.handleEditor()
			return
		}
		arg := strings.TrimSpace(parts[1])
		editorCmd.Run(editorCmd, []string{arg})

	case "/output", "/o":
		if len(parts) < 2 {
			ci.setOutputFile("")
		} else {
			filename := strings.TrimSpace(parts[1])
			ci.setOutputFile(filename)
		}

	case "/mode":
		if len(parts) < 2 {
			fmt.Printf("Current input mode: %s\n", ci.InputMode)
			return
		}
		mode := strings.TrimSpace(parts[1])
		if mode == "single" || mode == "multi" {
			if err := SetEffectiveChatMode(mode); err != nil {
				service.Errorf("Failed to save chat mode: %v\n", err)
				return
			}
			ci.InputMode = mode
			fmt.Printf("Switched to %s mode\n", mode)
			ci.printModeStatus()
		} else {
			fmt.Println("Invalid mode. Use 'single' or 'multi'")
		}

	case "/info", "/i":
		// Show current model and conversation stats
		ci.showInfo()

	default:
		fmt.Printf("Unknown command: %s\n", command)
	}

	// Continue the REPL
}

func (ci *ChatInfo) callAgent(input string) {

	var finalPrompt strings.Builder
	appendText(&finalPrompt, GetEffectiveTemplate())
	appendText(&finalPrompt, input)
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
		Prompt:           finalPrompt.String(),
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

	err := service.CallAgent(&op)
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
