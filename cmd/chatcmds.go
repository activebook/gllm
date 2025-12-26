package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"text/tabwriter"

	"github.com/activebook/gllm/service"
	"github.com/fatih/color"
	"github.com/spf13/viper"
)

// handleCommand processes chat commands
func (ci *ChatInfo) handleCommand(cmd string) {
	// Split the command into parts
	// Robust parsing: find the command (first word) and the raw arguments string
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return
	}
	parts := strings.Fields(cmd)
	command := parts[0]
	// Construct a "parts" slice that mimics the old behavior (cmd, arg1, arg2...)
	// but mostly we just need command and "the rest"
	// To minimize changes, we'll keep 'parts' generally available but also provide robust args parsing where needed.
	switch command {
	case "/exit", "/quit":
		ci.QuitFlag = true
		fmt.Println("Session Ended")
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

	case "/memory", "/y":
		if len(parts) < 2 {
			memoryListCmd.Run(memoryListCmd, []string{})
			return
		}
		args := parts[1:]
		ci.handleMemory(args)

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

	case "/info", "/i":
		// Show current model and conversation stats
		ci.showInfo()

	default:
		fmt.Printf("Unknown command: %s\n", command)
	}

	// Continue the REPL
}

// showHelp displays available chat commands
func (ci *ChatInfo) showHelp() {
	fmt.Println("Available commands:")
	fmt.Println("  /exit, /quit - Exit the chat session")
	fmt.Println("  /clear, /reset - Clear conversation history")
	fmt.Println("  /help, /? - Show this help message")
	fmt.Println("  /info, /i - Show current settings")
	fmt.Println("  /history, /h [num] [chars] - Show recent conversation history (default: 20 messages, 200 chars)")
	fmt.Println("  /markdown, /m [on|off] - Switch whether to render markdown or not")
	fmt.Println("  /attach, /a <filename> - Attach a file to the conversation")
	fmt.Println("  /detach, /d <filename|all> - Detach a file from the conversation")
	fmt.Println("  /template, /p \"<tmpl|name>\" - Change the template")
	fmt.Println("  /system /S \"<prompt|name>\" - Change the system prompt")
	fmt.Println("  /think, /T \"[on|off]\" - Switch whether to use deep think mode")
	fmt.Println("  /search, /s \"<engine>[on|off]\" - Change the search engine, or switch on/off")
	fmt.Println("  /tools, /t \"[on|off|skip|confirm]\" - Switch whether to use embedding tools, skip tools confirmation")
	fmt.Println("  /mcp \"[on|off|list]\" - Switch whether to use MCP servers, or list available servers")
	fmt.Println("  /memory, /y \"[list|add|clear]\" - Manage long-term cross-session memory")
	fmt.Println("  /reference, /r \"<num>\" - Change the search link reference count")
	fmt.Println("  /usage, /u \"[on|off]\" - Switch whether to show token usage information")
	fmt.Println("  /editor, /e <editor>|list - Open external editor for multi-line input")
	fmt.Println("  /output, /o <filename> [off] - Save to output file for model responses")
	fmt.Println("  !<command> - Execute a shell command directly (e.g. !ls -la)")
}

// showInfo displays current chat settings and information
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

// showHistory displays conversation history
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

// clearContext clears the conversation context
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

// setTemplate sets the conversation template
func (ci *ChatInfo) setTemplate(template string) {
	if err := SetEffectiveTemplate(template); err != nil {
		service.Warnf("%v", err)
		fmt.Println("Ignore template prompt")
	} else {
		fmt.Printf("Switched to template: %s\n", template)
	}
}

// setSystem sets the system prompt
func (ci *ChatInfo) setSystem(system string) {
	if err := SetEffectiveSystemPrompt(system); err != nil {
		service.Warnf("%v", err)
		fmt.Println("Ignore system prompt")
	} else {
		fmt.Printf("Switched to system prompt: %s\n", system)
	}
}

// setSearchEngine sets the search engine
func (ci *ChatInfo) setSearchEngine(engine string) {
	switch engine {
	case "off":
		// Disable search
		if err := SetAgentValue("search", service.NoneSearchEngine); err != nil {
			service.Errorf("Error saving configuration: %s\n", err)
		} else {
			fmt.Println("Search engine disabled.")
		}
	case "on":
		// Enable default search (google) if none set
		current := GetEffectSearchEngineName()
		if current == "" || current == service.NoneSearchEngine {
			if err := SetAgentValue("search", service.GoogleSearchEngine); err != nil {
				service.Errorf("Error saving configuration: %s\n", err)
			} else {
				fmt.Printf("Search engine enabled (set to %s).\n", service.GoogleSearchEngine)
			}
		} else {
			fmt.Printf("Search engine is already enabled (%s).\n", current)
		}
	default:
		// Switch to specified engine
		succeed := SetEffectSearchEngineName(engine)
		if succeed {
			fmt.Printf("Switched to search engine: %s\n", GetEffectSearchEngineName())
		}
	}
}

// setReferences sets the reference count
func (ci *ChatInfo) setReferences(count string) {
	num, err := strconv.Atoi(count)
	if err != nil {
		fmt.Println("Invalid number")
		return
	}
	referenceFlag = num
	fmt.Printf("Reference count set to %d\n", num)
}

// setUseTools sets the tools usage mode
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

// setUseMCP sets the MCP usage mode
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

// handleMemory handles memory commands in the REPL
func (ci *ChatInfo) handleMemory(args []string) {
	// Simple argument parsing logic
	// If it starts with "list", "clear", run those commands
	// If it starts with "add", treat the rest as the memory content
	subcmd := strings.TrimSpace(args[0])
	switch subcmd {
	case "list", "ls":
		memoryListCmd.Run(memoryListCmd, []string{})
	case "clear":
		memoryClearCmd.Run(memoryClearCmd, []string{})
	case "add":
		if len(args) < 2 {
			fmt.Println("Usage: memory add <content>")
			return
		}
		content := strings.TrimSpace(strings.Join(args[1:], " "))
		content = strings.Trim(content, "\"") // Remove surrounding quotes if present
		memoryAddCmd.Run(memoryAddCmd, []string{content})
	default:
		// Default to base command which shows help/status
		memoryListCmd.Run(memoryListCmd, []string{})
	}
}

// setOutputFile sets the output file for responses
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
		fmt.Println("No content.")
		return
	}

	// Set editor input
	ci.EditorInput = content
}
