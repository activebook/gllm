package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/activebook/gllm/service"
	"github.com/chzyer/readline"
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
/help - Show available commands
/history, /h [num] [chars] - Show recent conversation history (default: 20 messages, 200 chars)
/markdown, /mark [on|off|only] - Switch whether to render markdown or not
/system, /S <@name|prompt> - change system prompt
/template, /t <@name|tmpl> - change template
/search, /s <search_engine> - select a search engine to use
/reference. /r <num> - change link reference count
/attach, /a <filename> - Attach a file to the chat session
/detach, /d <filename> - Detach a file to the chat session`,
	Run: func(cmd *cobra.Command, args []string) {
		// Create an indeterminate progress bar
		spinner := service.NewSpinner("Processing...")

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
				if err := SetEffectiveSystemPrompt(sysPromptFlag); err != nil {
					service.Warnf("%v", err)
					fmt.Println("Using default system prompt instead")
				}
			}

			if cmd.Flags().Changed("search") {
				// Search mode
				SetEffectSearchEnginelName(searchFlag)
			} else {
				// Normal mode
				searchFlag = ""
			}
			service.SetMaxReferences(referenceFlag)

			// Always save a conversation file regardless of the flag
			if !cmd.Flags().Changed("conversation") {
				convoName = GenerateChatFilename()
			}
			service.NewOpenChatConversation(convoName, true)
			//service.NewGeminiConversation(convoName, true)
			service.NewGemini2Conversation(convoName, true)

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
		service.StopSpinner(spinner)

		// Build the ChatInfo object
		chatInfo := buildChatInfo(files)

		// Start the REPL
		chatInfo.startREPL()
	},
}

var ()

func init() {
	rootCmd.AddCommand(chatCmd)

	// Add chat-specific flags
	// In each chat session, it shouldn't change model, because the chat history would be invalid.
	// Attach should be used inside chat session
	// Imagine like using web llm ui, you can attach file to the chat session and turn search on and off
	chatCmd.Flags().StringVarP(&modelFlag, "model", "m", "", "Model to use for the chat session")
	chatCmd.Flags().StringVarP(&sysPromptFlag, "system", "S", "", "System prompt to use for the chat session")
	chatCmd.Flags().StringVarP(&templateFlag, "template", "t", "", "Template to use for the chat session")
	chatCmd.Flags().StringSliceVarP(&attachments, "attachment", "a", []string{}, "Specify file(s) or image(s) to append to the chat sessioin")
	chatCmd.Flags().StringVarP(&convoName, "conversation", "c", GenerateChatFilename(), "Name for this chat session")
	chatCmd.Flags().StringVarP(&searchFlag, "search", "s", service.GetDefaultSearchEngineName(), "Search engine for the chat session")
	chatCmd.Flags().Lookup("search").NoOptDefVal = service.GetDefaultSearchEngineName()
	chatCmd.Flags().IntVarP(&referenceFlag, "reference", "r", 5, "Specify the number of reference links to show")
}

func (ci *ChatInfo) startREPL() {
	fmt.Println("Welcome to GLLM Interactive Chat")
	fmt.Println("Type 'exit' or 'quit' to end the session, or '/help' for commands")
	fmt.Println("Use '\\' at the end of a line for multiline input")
	fmt.Println("Use '/' for commands")
	ci.showHelp()
	fmt.Println()

	rl, err := readline.New("gllm> ")
	if err != nil {
		fmt.Printf("Error initializing readline: %v\n", err)
		return
	}
	defer rl.Close()

	var inputLines []string
	multilineMode := false

	for {
		prompt := "gllm> "
		if multilineMode {
			prompt = "... "
		}
		rl.SetPrompt(prompt)

		line, err := rl.Readline()
		if err != nil { // Handle EOF or other errors
			if err == readline.ErrInterrupt { // Handle Ctrl+C
				fmt.Println("\nGoodbye!")
				break
			}
			if err == io.EOF { // Handle Ctrl+D
				fmt.Println("\nGoodbye!")
				break
			}
			fmt.Printf("Error reading line: %v\n", err)
			continue
		}

		line = strings.TrimSpace(line)

		// Check if line ends with '\' for multiline input
		if strings.HasSuffix(line, "\\") {
			multilineMode = true
			inputLines = append(inputLines, strings.TrimSuffix(line, "\\"))
			continue
		}

		// Add the final line and process the input
		inputLines = append(inputLines, line)
		input := strings.Join(inputLines, "\n")
		inputLines = nil // Reset for the next input
		multilineMode = false

		if input == "exit" || input == "quit" {
			fmt.Println("Goodbye!")
			break
		}

		if input == "" {
			continue
		}

		ci.handleInput(input)
		if ci.QuitFlag {
			break
		}
	}
}

type ChatInfo struct {
	Model      string
	Provider   string
	Files      []*service.FileData
	Conversion service.ConversationManager
	QuitFlag   bool
}

func buildChatInfo(files []*service.FileData) *ChatInfo {

	modelInfo := GetEffectiveModel()
	provider := service.DetectModelProvider(modelInfo["endpoint"].(string))
	var cm service.ConversationManager
	switch provider {
	case service.ModelOpenAICompatible:
		cm = service.GetOpenChatConversation()
	case service.ModelGemini:
		//cm = service.GetGeminiConversation()
		cm = service.GetGemini2Conversation()
	}

	ci := ChatInfo{
		Model:      modelInfo["model"].(string),
		Provider:   provider,
		Files:      files,
		Conversion: cm,
		QuitFlag:   false,
	}
	return &ci
}

func (ci *ChatInfo) handleInput(input string) {

	// Check if it's a command
	if strings.HasPrefix(input, "/") {
		ci.handleCommand(input)
		return
	}

	// Process as normal LLM query...
	ci.callLLM(input)
}

func (ci *ChatInfo) clearContext() {
	// Reset all settings
	viper.Set("default.system_prompt", "")
	viper.Set("default.template", "")
	viper.Set("default.search", "")
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
		fmt.Println("Igonre system prompt")
	} else {
		fmt.Printf("Switched to system prompt: %s\n", system)
	}
}

func (ci *ChatInfo) setSearchEngine(engine string) {
	succeed := SetEffectSearchEnginelName(engine)
	if succeed {
		searchFlag = GetEffectSearchEnginelName()
		fmt.Printf("Switched to search engine: %s\n", engine)
	}
}

func (ci *ChatInfo) setReferences(count string) {
	num, err := strconv.Atoi(count)
	if err != nil {
		fmt.Println("Invalid number")
		return
	}
	service.SetMaxReferences(num)
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
	fmt.Println("Current settings:")
	fmt.Printf("  Model: %s\n", ci.Model)
	fmt.Printf("  System Prompt: \n    - %s\n", GetEffectiveSystemPrompt())
	fmt.Printf("  Template: \n    - %s\n", GetEffectiveTemplate())
	fmt.Printf("  Search Engine: %s\n", GetEffectSearchEnginelName())
	fmt.Printf("  Attachment(s): \n")
	for _, file := range ci.Files {
		fmt.Printf("    - [%s]: %s\n", file.Format(), file.Path())
	}
}

func (ci *ChatInfo) showHelp() {
	fmt.Println("Available commands:")
	fmt.Println("  /exit, /quit - Exit the chat session")
	fmt.Println("  /clear, /reset - Clear conversation history")
	fmt.Println("  /help - Show this help message")
	fmt.Println("  /info - Show current settings and conversation stats")
	fmt.Println("  /history /h [num] [chars] - Show recent conversation history (default: 20 messages, 200 chars)")
	fmt.Println("  /markdown, /mark [on|off|only] - Switch whether to render markdown or not")
	fmt.Println("  /attach, /a <filename> - Attach a file to the conversation")
	fmt.Println("  /detach, /d <filename|all> - Detach a file from the conversation")
	fmt.Println("  /template, /t \"<tmpl|name>\" - Change the template")
	fmt.Println("  /system /S \"<prompt|name>\" - Change the system prompt")
	fmt.Println("  /search, /s \"<engine>\" - Change the search engine")
	fmt.Println("  /reference, /r \"<num>\" - Change the search link reference count")
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
	case service.ModelOpenAICompatible:
		service.DisplayOpenAIConversationLog(data, num, chars)
	default:
		fmt.Println("Unknown provider")
	}
}

func (ci *ChatInfo) setMarkdown(mark string) {
	if len(mark) != 0 {
		SwitchMarkdown(mark)
	}
	marked := GetMarkdownSwitch()
	if marked == "on" {
		fmt.Println("Makedown output switched " + switchOnColor + "on" + resetColor)
	} else if marked == "only" {
		fmt.Println("Makedown output switched " + switchOnlyColor + "only" + resetColor)
	} else {
		fmt.Println("Makedown output switched " + switchOffColor + "off" + resetColor)
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

	case "/help":
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

	case "/markdown", "/mark":
		if len(parts) < 2 {
			ci.setMarkdown("")
			return
		}
		mark := strings.TrimSpace(parts[1])
		ci.setMarkdown(mark)

	case "/clear", "/reset":
		ci.clearContext()

	case "/template", "/t":
		if len(parts) < 2 {
			fmt.Println("Please specify a template name")
			return
		}
		tmpl := strings.TrimSpace(parts[1])
		ci.setTemplate(tmpl)

	case "/system", "/S":
		if len(parts) < 2 {
			fmt.Println("Please specify a system prompt")
			return
		}
		sysPrompt := strings.TrimSpace(parts[1])
		ci.setSystem(sysPrompt)

	case "/search", "/s":
		if len(parts) < 2 {
			fmt.Println("Please specify a search engine. Options: google, tavily, bing, none")
			return
		}
		engine := strings.TrimSpace(parts[1])
		ci.setSearchEngine(engine)

	case "/reference", "/r":
		if len(parts) < 2 {
			fmt.Println("Please specify a number")
			return
		}
		count := strings.TrimSpace(parts[1])
		ci.setReferences(count)

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

	case "/info":
		// Show current model and conversation stats
		ci.showInfo()

	default:
		fmt.Printf("Unknown command: %s\n", command)
	}

	// Continue the REPL
}

func (ci *ChatInfo) callLLM(input string) {

	var finalPrompt strings.Builder
	appendText(&finalPrompt, GetEffectiveTemplate())
	appendText(&finalPrompt, input)
	modelInfo := GetEffectiveModel()
	sys_prompt := GetEffectiveSystemPrompt()
	var searchEngine map[string]any
	if searchFlag != "" {
		searchEngine = GetEffectiveSearchEngine()
	}
	service.CallLanguageModel(finalPrompt.String(), sys_prompt, ci.Files, modelInfo, searchEngine)

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
