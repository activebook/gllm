package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

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
		ri.startREPL()
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
	QuitFlag       bool     // for cmd /quit or /exit
	EditorInput    string   // for /e editor edit
	Instruction    string   // for underlying system instructions (e.g. skill activation)
	History        []string // for input history
	outputFile     string
	sharedState    *data.SharedState // Persistent SharedState for the session
	autoRenameOnce sync.Once         // ensures auto-rename fires at most once per REPL session
}

func (ri *ReplInfo) printWelcome() {
	termWidth := io.GetTerminalWidth()
	safeWidth := max(40, termWidth-4)

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(data.BorderHex)).
		Width(safeWidth).
		Margin(0, 1).
		Padding(1, 1)

	innerWidth := safeWidth - borderStyle.GetHorizontalFrameSize()

	// Split into left (~40%) and right (~60%) columns
	leftWidth := innerWidth * 40 / 100
	rightWidth := innerWidth - leftWidth

	// --- Left panel: logo + welcome ---
	logo := ui.GetLogo(data.KeyHex, data.LabelHex, 0.5)
	welcomeText := logo + "\nWelcome back!\n" + data.DetailColor + " (v" + version + ")" + data.ResetSeq

	leftContent := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(data.KeyHex)).
		Width(leftWidth).
		Align(lipgloss.Center).
		Padding(0, 1).
		Render(welcomeText)

	// --- Right panel: instructions ---
	// maxCmdLen := 0
	// for cmd := range replSpecMap {
	// 	if len(cmd) > maxCmdLen {
	// 		maxCmdLen = len(cmd)
	// 	}
	// }
	// format := fmt.Sprintf("• %%-%ds : %%s", maxCmdLen)

	instructions := []string{}
	for cmd, desc := range replSpecMap {
		instructions = append(instructions, fmt.Sprintf("• %s: %s", cmd, desc))
	}
	sort.Strings(instructions)

	rightContent := lipgloss.NewStyle().
		Foreground(lipgloss.Color(data.LabelHex)).
		Width(rightWidth).
		Align(lipgloss.Left).
		Padding(0, 0, 0, 2).
		Render(strings.Join(instructions, "\n"))

	// --- Combine panels horizontally ---
	inner := lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftContent,
		rightContent,
	)

	banner := borderStyle.Render(inner)
	fmt.Println(banner)

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(data.DetailHex)).
		Width(safeWidth).
		Align(lipgloss.Center).
		Italic(true)
	fmt.Println(hintStyle.Padding(0, 2).Render("Type your message below and press Enter to send."))
	fmt.Println()
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
	hooks := ui.ChatInputHooks{
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
	}

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

func (ri *ReplInfo) startREPL() {
	// Initialize SharedState for the session
	ri.sharedState = data.NewSharedState()
	defer ri.sharedState.Clear()

	// Set auto approve for the session
	data.SetYoloModeInSession(yoloFlag)

	// Start the REPL
	ri.printWelcome()

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
			util.Errorf("%v\n", err)
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
			ri.handleCommand(input)
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

// clearContext clears the session context
func (ri *ReplInfo) clearContext() {
	agent, err := EnsureActiveAgent()
	if err != nil {
		util.Errorf("%v\n", err)
		return
	}
	// Construct session manager
	session, err := service.ConstructSession(sessionName, agent.Model.Provider)
	if err != nil {
		util.Errorf("Error constructing session manager: %v\n", err)
		return
	}
	// Clear session history
	err = session.Clear()
	if err != nil {
		util.Errorf("Error clearing context: %v\n", err)
		return
	}
	// Empty attachments
	ri.Files = []*service.FileData{}
	fmt.Printf("Context cleared.\n")
}

// showHistory displays session history using TUI viewport
func (ri *ReplInfo) showHistory() {
	// Get active agent
	agent, err := EnsureActiveAgent()
	if err != nil {
		util.Errorf("%v\n", err)
		return
	}

	// Get session data
	sessionData, err := service.ReadSessionContent(sessionName)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No available session yet.")
			return
		}
		util.Errorf("%v\n", err)
		return
	}

	// Detect provider based on message format
	isCompatible, provider, modelProvider := service.CheckSessionFormat(agent, sessionData)
	if !isCompatible {
		// Warn about potential incompatibility if providers differ
		util.Warnf("Session '%s' [%s] is not compatible with the current model provider [%s].\n", sessionName, provider, modelProvider)
	}

	// Render session log
	var content string
	switch provider {
	case service.ModelProviderGemini:
		content = service.RenderGeminiSessionLog(sessionData)
	case service.ModelProviderOpenAI, service.ModelProviderOpenAICompatible:
		content = service.RenderOpenAISessionLog(sessionData)
	case service.ModelProviderAnthropic:
		content = service.RenderAnthropicSessionLog(sessionData)
	default:
		fmt.Println("No available session yet.")
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
		util.Errorf("Error running viewport: %v\n", err)
	}
}

// compressContext compresses the session context by replacing it with a summary
func (ri *ReplInfo) compressContext() {
	// Get active agent
	agent, err := EnsureActiveAgent()
	if err != nil {
		util.Errorf("%v\n", err)
		return
	}

	// Get session data
	sessionData, err := service.ReadSessionContent(sessionName)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No available session yet.")
			return
		}
		util.Errorf("%v\n", err)
		return
	}

	// Compress the session context
	ui.GetIndicator().Start(ui.IndicatorCompressingContext)
	summary, err := service.CompressSession(agent, sessionData)
	ui.GetIndicator().Stop()

	if err != nil {
		util.Errorf("Failed to compress session: %v\n", err)
		return
	}

	// Build the new compressed session
	newData, err := service.BuildCompressedSession(summary, agent.Model.Provider)
	if err != nil {
		util.Errorf("Failed to build compressed session: %v\n", err)
		return
	}

	// Save back to the file format
	err = service.WriteSessionContent(sessionName, newData)
	if err != nil {
		util.Errorf("Failed to save compressed session: %v\n", err)
		return
	}

	util.Successln("Compressed successfully!\nUse /history to view the compressed session.")
}

// renameSession uses the model synchronously to infer a meaningful name for
// the current session and renames the session directory on disk.
// It mirrors the /compress UX: a spinner is shown during the model call,
// and the package-level sessionName variable is updated on success.
func (ri *ReplInfo) renameSession() {
	agent, err := EnsureActiveAgent()
	if err != nil {
		util.Errorf("%v\n", err)
		return
	}

	sessionData, err := service.ReadSessionContent(sessionName)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No session history yet — nothing to rename.")
			return
		}
		util.Errorf("%v\n", err)
		return
	}
	if len(sessionData) == 0 {
		fmt.Println("Session is empty — nothing to rename.")
		return
	}

	ui.GetIndicator().Start(ui.IndicatorRenamingSession)
	newName, err := service.GenerateSessionName(agent, sessionData)
	ui.GetIndicator().Stop()

	if err != nil {
		util.Errorf("Failed to generate session name: %v\n", err)
		return
	}
	if newName == sessionName {
		util.Successln("Session name is already optimal: " + sessionName)
		return
	}

	if err := service.RenameSession(sessionName, newName); err != nil {
		util.Errorf("Failed to rename session: %v\n", err)
		return
	}

	oldName := sessionName
	sessionName = newName
	util.Successln(fmt.Sprintf("Session renamed: %s → %s", oldName, newName))
}

func (ri *ReplInfo) autoRenameSessionOnce() {
	// Auto-rename: fire exactly once, asynchronously, on the first successful
	// turn of a default-named session. The sync.Once on ReplInfo guarantees
	// that a rapid second turn cannot trigger a duplicate rename.
	ri.autoRenameOnce.Do(func() {
		if !isDefaultSessionName(sessionName) {
			return
		}
		agent, err := EnsureActiveAgent()
		if err != nil {
			util.Errorf("%v\n", err)
			return
		}
		if !service.IsAutoRenameEnabled(agent.Capabilities) {
			return
		}

		go func() {
			// Do it at background
			ri.renameSession()
		}()
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
		util.Errorf("Failed to copy to clipboard: %v\n", err)
	}
	util.Successln("Copied the last response to clipboard.")
}

func (ri *ReplInfo) callAgent(input string) {
	prompt := input
	if ri.Instruction != "" {
		prompt = fmt.Sprintf("<instruction>\n%s\n</instruction>\n\n<user-request>%s</user-request>", ri.Instruction, input)
		ri.Instruction = "" // Clear it after use
	}

	// Call agent using the shared runner, passing persisted SharedState
	err := RunAgent(prompt, ri.Files, sessionName, ri.outputFile, ri.sharedState)
	if err != nil {
		util.Errorf("%v\n", err)
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
			util.Errorf("Command failed with exit code %d\n", exitError.ExitCode())
		} else {
			util.Errorf("Command failed: %v\n", err)
		}
	}

	// Display the output
	if len(output) > 0 {
		// shell output color
		fmt.Printf(data.ShellOutputColor+"%s\n"+data.ResetSeq, output)
	}
}

// isDefaultSessionName returns true when the session name is the auto-generated
// timestamp form produced by GenerateSessionName() in repl.go: "session-YYYY-MM-DD_HH-MM-SS".
func isDefaultSessionName(name string) bool {
	return strings.HasPrefix(name, "session-")
}

func GenerateSessionName() string {
	// Get the current time
	currentTime := time.Now()

	// Format the time as a string in the format "chat_YYYY-MM-DD_HH-MM-SS.json"
	filename := fmt.Sprintf("session-%s", currentTime.Format("2006-01-02_15-04-05"))

	return filename
}
