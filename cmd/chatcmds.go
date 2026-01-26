package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/service"
	"github.com/spf13/cobra"
)

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func switchYoloMode() {
	yoloFlag = !yoloFlag
	if yoloFlag {
		fmt.Printf("YOLO mode: %s\n", data.SwitchOnColor+"on"+data.ResetSeq)
	} else {
		fmt.Printf("YOLO mode: %s\n", data.SwitchOffColor+"off"+data.ResetSeq)
	}
}

// runCommand executes a command with arguments
func runCommand(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		// No arguments, call the command directly
		if cmd.RunE != nil {
			if err := cmd.RunE(cmd, args); err != nil {
				service.Errorf("%v\n", err)
			}
		} else if cmd.Run != nil {
			cmd.Run(cmd, args)
		}
		return
	}

	// Find subcommand
	subName := args[0]
	for _, sub := range cmd.Commands() {
		if sub.Name() == subName || (len(sub.Aliases) > 0 && contains(sub.Aliases, subName)) {
			// Recurse with the subcommand and remaining args
			runCommand(sub, args[1:])
			return
		}
	}

	// No subcommand found, call on current cmd with all args
	if cmd.RunE != nil {
		if err := cmd.RunE(cmd, args); err != nil {
			service.Errorf("%v\n", err)
		}
	} else if cmd.Run != nil {
		cmd.Run(cmd, args)
	}
}

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
		// Arguments (num, chars) are deprecated/ignored in viewport mode
		// We could implement "--raw" here later
		ci.showHistory()

	case "/clear", "/reset":
		ci.clearContext()

	case "/model", "/m":
		runCommand(modelCmd, parts[1:])

	case "/agent", "/g":
		runCommand(agentCmd, parts[1:])

	case "/template", "/p":
		runCommand(templateCmd, parts[1:])

	case "/system", "/S":
		runCommand(systemCmd, parts[1:])

	case "/search", "/s":
		runCommand(searchCmd, parts[1:])

	case "/tools", "/t":
		runCommand(toolsCmd, parts[1:])

	case "/mcp":
		runCommand(mcpCmd, parts[1:])

	case "/skills":
		runCommand(skillsCmd, parts[1:])

	case "/memory", "/r":
		runCommand(memoryCmd, parts[1:])

	case "/yolo", "/y":
		switchYoloMode()

	case "/convo", "/c":
		runCommand(convoCmd, parts[1:])

	case "/think", "/T":
		runCommand(thinkCmd, parts[1:])

	case "/features", "/f", "/capabilities", "/caps":
		runCommand(capsCmd, parts[1:])

	case "/editor", "/e":
		if len(parts) < 2 {
			ci.handleEditor()
			return
		}
		runCommand(editorCmd, parts[1:])

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

	case "/info", "/i":
		ci.showInfo()

	case "/theme":
		runCommand(themeCmd, parts[1:])

	default:
		fmt.Printf("Unknown command: %s\n", command)
	}
}

// showHelp displays available chat commands
func (ci *ChatInfo) showHelp() {
	fmt.Println("Available commands:")
	fmt.Println("  /exit, /quit - Exit the chat session")
	fmt.Println("  /clear, /reset - Clear conversation history")
	fmt.Println("  /help, /? - Show this help message")
	fmt.Println("  /info, /i - Show current settings")
	fmt.Println("  /history, /h - Show recent conversation history")
	fmt.Println("  /model, /m [subcmd] - Manage models (list, switch, add, etc.)")
	fmt.Println("  /agent, /g [subcmd] - Manage agents (list, switch, add, etc.)")
	fmt.Println("  /template, /p [subcmd] - Manage templates (list, switch, add, etc.)")
	fmt.Println("  /system, /S [subcmd] - Manage system prompts (list, switch, add, etc.)")
	fmt.Println("  /search, /s [subcmd] - Manage search engines (list, switch, etc.)")
	fmt.Println("  /memory, /r [subcmd] - Manage memory (list, add, clear)")
	fmt.Println("  /think, /T [off|low|medium|high|sw] - Set thinking level (sw for interactive)")
	fmt.Println("  /tools, /t [on|off] - Manage embedding tools")
	fmt.Println("  /mcp [subcmd] - Manage MCP servers (list, switch, etc.)")
	fmt.Println("  /skills [subcmd] - Manage agent skills (list, switch, install, etc.)")
	fmt.Println("  /features, /f [switch] - Switch agent features and capabilities (usage, markdown, etc.)")
	fmt.Println("  /yolo, /y - Toggle YOLO mode (non-interactive tool execution)")
	fmt.Println("  /convo, /c [subcmd] - Manage conversations (list, info, remove, etc.)")
	fmt.Println("  /editor, /e [subcmd] - Manage editor or open for multi-line input")
	fmt.Println("  /attach, /a <file> - Attach a file")
	fmt.Println("  /detach, /d <file|all> - Detach a file")
	fmt.Println("  /theme [subcmd] - Manage and switch themes")
	fmt.Println("  !<command> - Execute a shell command")
}

// showInfo displays current chat settings and information
func (ci *ChatInfo) showInfo() {

	printSection := func(title string) {
		fmt.Println()
		fullTitle := fmt.Sprintf("=== %s ===", strings.ToUpper(title))
		fmt.Printf("%s%s%s\n", data.SectionColor, fullTitle, data.ResetSeq)
	}

	printSection("CURRENT SETTINGS")

	// System prompt
	printSection("SYSTEM PROMPT")
	systemCmd.Run(systemCmd, []string{})

	// Template
	printSection("TEMPLATE")
	templateCmd.Run(templateCmd, []string{})

	// Memory section (included in system prompt)
	// printSection("Memory")
	// memoryListCmd.Run(memoryListCmd, []string{})
	// w.Flush()

	// Search Engines section
	printSection("Search Engines")
	searchListCmd.Run(searchListCmd, []string{})

	// Plugins section
	printSection("Tools")
	ListEmbeddingTools()

	// Current Agent section
	printSection("Agents")
	agentCmd.Run(agentCmd, []string{})

	// Attachments
	printSection("ATTACHMENTS")
	if len(ci.Files) > 0 {
		fmt.Printf("%s%s%s (%d):\n", data.KeyColor, "Attachments", data.ResetSeq, len(ci.Files))
		for _, file := range ci.Files {
			fmt.Printf("  - [%s]: %s\n", file.Format(), file.Path())
		}
	} else {
		fmt.Println("Attachments: None")
	}

	fmt.Println()
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
					file := ProcessAttachment(filePath)
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
