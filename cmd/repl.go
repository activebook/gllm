package cmd

import (
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/internal/ui"
	"github.com/activebook/gllm/io"
	"github.com/activebook/gllm/service"
	"github.com/activebook/gllm/util"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// replCmd represents the repl command
var replCmd = &cobra.Command{
	Use:    "repl",
	Short:  "Start an interactive REPL session",
	Hidden: true, // users invoke this implicitly via `gllm` with no subcommand
	Long: `Start an interactive REPL session with the configured LLM.
This provides a Read-Eval-Print-Loop (REPL) interface where you can
have a continuous session with the model.`,
	// Add completion support
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return []string{"--agent", "--session", "--yolo", "--help"}, cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// Start indeterminate progress bar
		ui.GetIndicator().Start("")

		// Clear empty sessions in background
		// service.ClearEmptySessionsAsync()

		var ri *ReplInfo
		store := data.NewConfigStore()

		// Use the shared package-level sessionName/agentName globals so that flags
		// declared on rootCmd (e.g. -c, -g) are correctly honored when replCmd
		// is invoked programmatically from rootCmd's Run handler.

		// If a session name was not supplied, generate a fresh one.
		if sessionName == "" {
			sessionName = GenerateSessionName()
		} else {
			// Resolve index-based names to their canonical file name.
			name, err := service.FindSessionByIndex(sessionName)
			if err != nil {
				return fmt.Errorf("error finding session: %v\n", err)
			}
			if name != "" {
				sessionName = name
			}
		}

		// If an agent name was supplied, switch to it.
		if agentName != "" {
			if store.GetAgent(agentName) == nil {
				return fmt.Errorf("agent %s does not exist", agentName)
			}
			store.SetActiveAgent(agentName)
		}

		// Build the ReplInfo object
		ri = &ReplInfo{
			QuitFlag: false,
		}

		ui.GetIndicator().Stop()

		// Start the REPL
		ri.startREPL(cmd)
		return nil
	},
}

const (
	editTempFile = ".gllm-edit-*.tmp"
)

// Load when package is initialized
func init() {
	rootCmd.AddCommand(replCmd)

	// Add session specific flags
	// Attach should be used inside session
	// Imagine like using web llm ui, you can attach file to the session and turn search on and off
	replCmd.Flags().StringVarP(&agentName, "agent", "g", "", "Agent to use for this session")
	replCmd.Flags().StringVarP(&sessionName, "session", "s", GenerateSessionName(), "Name for this session")
	replCmd.Flags().BoolVarP(&yoloFlag, "yolo", "y", false, "Enable yolo mode (non-interactive)")
}

type ReplInfo struct {
	Files          []*service.FileData
	QuitFlag       bool              // for cmd /quit or /exit
	EditorInput    string            // for /e editor edit
	Guideline      string            // for underlying guideline (e.g. skill activation)
	History        []string          // for input history
	sharedState    *data.SharedState // Persistent SharedState for the session
	autoRenameOnce sync.Once         // ensures auto-rename fires at most once per REPL session
}

// This is the new awaitInput function, which uses bubbletea, support auto-complete
func (ri *ReplInfo) awaitInput() (string, error) {
	agent, err := EnsureActiveAgent()
	if err != nil {
		return "", err
	}

	// Check whether plan mode is enabled or not
	planMode := service.IsPlanModeEnabled(agent.Capabilities)
	data.EnablePlanModeInSession(planMode)

	// build commands from replCommandMap
	var commands []ui.Suggestion
	for cmd, desc := range replCommandMap {
		commands = append(commands, ui.Suggestion{Command: cmd, Description: desc})
	}

	// Load workflow commands
	_ = service.GetWorkflowManager().LoadMetadata(replCommandMap)

	// Add workflow commands
	wm := service.GetWorkflowManager()
	for cmd, desc := range wm.GetCommands() {
		// Skip if the command already exists in replCommandMap
		if _, ok := replCommandMap[cmd]; ok {
			continue
		}
		commands = append(commands, ui.Suggestion{Command: cmd, Description: desc})
	}

	// Add skill commands
	if service.IsAgentSkillsEnabled(agent.Capabilities) {
		sm := service.GetSkillManager()
		skills := sm.GetAvailableSkillsMetadata()

		for _, skill := range skills {
			cmdName := "/" + strings.ToLower(skill.Name)
			// Skip if the command already exists in replCommandMap or workflow cmds
			if _, ok := replCommandMap[cmdName]; ok {
				continue
			}

			// Add to suggestions with prefix to distinguish it
			desc := skill.Description
			commands = append(commands, ui.Suggestion{Command: cmdName, Description: desc})
		}
	}

	// Sort commands by text
	sort.Slice(commands, func(i, j int) bool {
		return commands[i].Command < commands[j].Command
	})

	// Define hooks for UI
	hooks := ri.getChatInputHooks(agent)

	// Run chat input
	result, err := ui.RunChatInput(commands, ri.EditorInput, ri.History, hooks)
	if err != nil {
		return "", err
	}
	if result.Canceled {
		// Return user cancel error
		return "", service.UserCancelError{Reason: service.UserCancelReasonCancel}
	}

	// Update history
	ri.History = result.History

	return result.Value, nil
}

// getChatInputHooks returns the hooks required for the chat input UI.
func (ri *ReplInfo) getChatInputHooks(agent *data.AgentConfig) ui.ChatInputHooks {
	return ui.ChatInputHooks{
		// Start load MCP server when chat input is ready
		EventChatInputReady: func() {
			StartLoadMCPServer(agent)
		},
		IsPlanModeActive: func() bool {
			return data.IsPlanModeInSessionEnabled() && data.GetPlanModeInSession()
		},
		IsYoloModeActive: func() bool {
			return data.GetYoloModeInSession()
		},
		ToggleSessionMode: func() {
			// here we do a cycle [normal->plan->yolo->normal]
			switchSessionMode()
		},
		// OnPasteRequested handles Ctrl+V: reads clipboard, updates session files,
		// and sends the appropriate PasteResultMsg back to the UI.
		OnPasteRequested: func() {
			img, err := data.ReadClipboardImage()
			if err == nil && img != nil {
				fileName := fmt.Sprintf("pasted_image_%d%s", len(ri.Files)+1, img.Ext)
				ri.Files = append(ri.Files, service.NewFileData(img.Mime, img.Data, fileName))
				attStr := fmt.Sprintf("Attached: %d image(s)", len(ri.Files))
				ui.SendEvent(ui.PasteResultMsg{PastedAttachments: attStr})
				return
			}
			// Fallback: read text from clipboard and instruct UI to insert it
			text, _ := data.ReadClipboardText()
			ui.SendEvent(ui.PasteResultMsg{PastedText: text})
		},
	}
}

func (ri *ReplInfo) startREPL(cmd *cobra.Command) {
	// Initialize SharedState for the session
	ri.sharedState = data.NewSharedState()
	defer ri.sharedState.Clear()

	// Set auto approve for the session
	data.SetYoloModeInSession(yoloFlag)

	// Print welcome banner
	printReplWelcome()

	// Print session history if available
	ri.printSessionHistory()

	// Launch background update check (non-blocking).
	// Only check once repl started
	StartBackgroundUpdateCheck()

	// Define prompt style
	tcol := io.GetTerminalWidth()
	promptStyle := lipgloss.NewStyle().
		Background(lipgloss.Color(data.CurrentTheme.Background)).
		Foreground(lipgloss.Color(data.CurrentTheme.Foreground)).
		Padding(1, 2).Margin(0, 0, 1, 0). // padding and margin
		Bold(false).
		// Align(lipgloss.Right). 	// align would break code formatting
		Width(tcol) // align and width

	for !ri.QuitFlag {
		var input string
		var err error

		// Get user input
		input, err = ri.awaitInput()
		if err != nil {
			if service.IsUserCancelError(err) {
				// Handle user cancellation (Ctrl+C)
				fmt.Println("\nSession ended.")
				break
			}
			util.LogErrorf("%v\n", err)
			break
		}
		if input == "" {
			continue
		}

		// Handle inner commands
		if ri.startWithInnerCommand(input) {
			// Reset editor input
			ri.EditorInput = ""
			// Handle inner command
			ri.handleCommand(cmd, input)
			if ri.QuitFlag {
				break
			}
			fmt.Println()
			// If editor input is not empty, use it as input
			if ri.EditorInput != "" {
				input = ri.EditorInput
				// Reset editor input
				ri.EditorInput = ""
			} else {
				continue
			}
		}

		// Echo user input with style
		fmt.Println(promptStyle.Render(input))

		// Handle shell commands
		if ri.startWithLocalCommand(input) {
			ri.executeShellCommand(input[1:])
			continue
		}

		// Call agent
		ri.callAgent(input)
		fmt.Println()
	}
}

func (ri *ReplInfo) startWithInnerCommand(line string) bool {
	return strings.HasPrefix(line, "/")
}

func (ri *ReplInfo) startWithLocalCommand(line string) bool {
	return strings.HasPrefix(line, "!")
}

// printSessionHistory loads and renders existing messages when resuming a session.
// It is a no-op when the session is brand-new (no data on disk) or when
// the session name is empty (anonymous single-turn mode).
func (ri *ReplInfo) printSessionHistory() {
	if sessionName == "" {
		return
	}
	agent, err := EnsureActiveAgent()
	if err != nil {
		return
	}

	rendered, notice, err := service.RenderSessionHistory(agent, sessionName)
	if err != nil {
		util.LogErrorf("%v\n", err)
		return
	}
	if util.IsEmpty(rendered) {
		return
	}

	// Print a subtle divider so the user understands they are seeing history
	dividerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(data.LabelHex)).
		Italic(true)
	fmt.Println(dividerStyle.Render("── Resuming session: " + sessionName + " ──"))
	fmt.Println()
	fmt.Print(rendered)
	fmt.Println()

	// Print notice at the end if any
	if !util.IsEmpty(notice) {
		util.LogWarnf("%v\n", notice)
	}
}

// viewSessionHistory displays session history using TUI viewport
func (ri *ReplInfo) viewSessionHistory() {
	agent, err := EnsureActiveAgent()
	if err != nil {
		util.LogErrorf("%v\n", err)
		return
	}

	provider, content, notice, err := service.RenderSessionForViewport(agent, sessionName)
	if err != nil {
		util.LogErrorf("%v\n", err)
		return
	}
	// If content is empty, return
	if util.IsEmpty(content) {
		return
	}

	// Show viewport in full screen
	m := ui.NewViewportModel(provider, content, func() string {
		return fmt.Sprintf("Session: %s", sessionName)
	})
	// Bugfix: we don't need to run history in alt screen
	// because it will break the ChatInput view
	// p := tea.NewProgram(m, tea.WithAltScreen())
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		util.LogErrorf("Error running viewport: %v\n", err)
	}

	// Print notice after close the view if any
	if !util.IsEmpty(notice) {
		util.LogWarnf("%v\n", notice)
	}
}

// compressContext has been refactored to sessionCompressCmd.
// renameSession has been refactored to sessionAutoRenameCmd.

func (ri *ReplInfo) autoRenameSessionOnce() {
	// Auto-rename: fire exactly once, asynchronously, on the first successful
	// turn of a default-named session. The sync.Once on ReplInfo guarantees
	// that a rapid second turn cannot trigger a duplicate rename.
	ri.autoRenameOnce.Do(func() {
		if !IsDefaultSessionName(sessionName) {
			return
		}
		agent, err := EnsureActiveAgent()
		if err != nil {
			util.LogErrorf("%v\n", err)
			return
		}
		if !service.IsAutoRenameEnabled(agent.Capabilities) {
			return
		}

		// Bugfix:
		// Don't run it at background
		// At backgound the output will break the input frame
		runCommand(sessionRenameCurrentCmd, []string{})
	})
}

// copyLastMessage copies the last assistant response or its latest code block to the clipboard.
func (ri *ReplInfo) copyLastMessage() {
	lastAssistantMessage := data.GetClipboardText()

	if lastAssistantMessage == "" {
		fmt.Println("No assistant message found to copy.")
		return
	}

	// Actually copy to clipboard using atotto/clipboard
	err := data.WriteClipboardText(lastAssistantMessage)
	if err != nil {
		util.LogErrorf("Failed to copy to clipboard: %v\n", err)
	}
	util.LogSuccessln("Copied the last response to clipboard.")
}

func (ri *ReplInfo) callAgent(input string) {
	prompt := input
	guideline := ""
	if ri.Guideline != "" {
		guideline = fmt.Sprintf("<guideline>%s</guideline>", ri.Guideline)
		ri.Guideline = "" // Clear it after use
	}

	// Call agent using the shared runner, passing persisted SharedState
	err := RunAgent(prompt, guideline, ri.Files, sessionName, "", ri.sharedState)
	if err != nil {
		util.LogErrorf("%v\n", err)
		return
	}

	// Auto-rename session once
	ri.autoRenameSessionOnce()

	// Reset the files after processing
	ri.Files = []*service.FileData{}
}

func (ri *ReplInfo) executeShellCommand(command string) {
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
			util.LogErrorf("Command failed with exit code %d\n", exitError.ExitCode())
		} else {
			util.LogErrorf("Command failed: %v\n", err)
		}
	}

	// Display the output
	if len(output) > 0 {
		// shell output color
		fmt.Printf(data.ShellOutputColor+"%s\n"+data.ResetSeq, output)
	}
}
