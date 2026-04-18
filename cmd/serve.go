package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/net/rest"
	"github.com/activebook/gllm/net/sse"
	"github.com/activebook/gllm/service"
	"github.com/activebook/gllm/util"
	"github.com/spf13/cobra"
)

var (
	servePort    int
	serveVerbose bool
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start a headless SSE web server",
	Long:  `Start a Server-Sent Events (SSE) server to expose GLLM as a headless service.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		port := strconv.Itoa(servePort)

		http.HandleFunc("/v1/chat/completions", chatCompletionHandler)

		// Mount standard REST endpoints
		rest.Mount(http.DefaultServeMux)

		util.LogInfof("Starting headless GLLM SSE server on port %s...\n", port)
		return http.ListenAndServe(":"+port, nil)
	},
}

func init() {
	serveCmd.Flags().IntVarP(&servePort, "port", "p", 8080, "Port to listen on")
	serveCmd.Flags().BoolVarP(&serveVerbose, "verbose", "v", false, "Enable verbose output to stdio")
	rootCmd.AddCommand(serveCmd)
}

// Minimal OpenAI-like request struct
type ChatRequest struct {
	Messages []Message `json:"messages"`
	Model    string    `json:"model,omitempty"`
	Stream   bool      `json:"stream,omitempty"`
	Session  string    `json:"session,omitempty"` // custom parameter for GLLM specific sessions
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func chatCompletionHandler(w http.ResponseWriter, r *http.Request) {
	// Add CORS headers for all requests
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle CORS preflight
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Set SSE headers (keep alive)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// For GLLM, we merge user messages as prompt
	var promptBuilder strings.Builder
	for _, m := range req.Messages {
		if m.Role == "user" {
			promptBuilder.WriteString(m.Content)
			promptBuilder.WriteString("\n")
		}
	}
	prompt := strings.TrimSpace(promptBuilder.String())

	sseOut, err := sse.NewSSEOutput(w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sessionName := req.Session
	if sessionName == "" {
		sessionName = GenerateSessionName()
	} else {
		// Resolve index-based names to their canonical file name.
		name, err := service.FindSessionByIndex(sessionName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if name != "" {
			sessionName = name
		}
	}

	agent, err := EnsureActiveAgent()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var guideline string
	if strings.HasPrefix(prompt, "/") {
		var handled bool
		handled, prompt, guideline = handleWebCommand(prompt, sessionName, sseOut)
		if handled {
			sseOut.Close()
			return
		}
	}

	// Create a context that can be cancelled by the HTTP request's Done channel
	ctx := r.Context()
	err = runAgentWithSSE(prompt, guideline, sessionName, sseOut, agent, ctx)
	if err != nil {
		util.LogErrorf("Server agent error: %v\n", err)
		sseOut.WriteErrorEvent(err.Error(), "agent_error")
	}

	sseOut.Close()
}

// commandMode classifies how a web command may execute in headless mode.
type commandMode int

const (
	// cmdModeInfo commands are read-only and safe to run in headless mode.
	// They emit text via the command SSE event and never invoke huh pickers.
	cmdModeInfo commandMode = iota
	// cmdModeSessionOp commands mutate the CURRENT session but accept all
	// required arguments inline (no interactive picker needed).
	cmdModeSessionOp
	// cmdModeBlocked commands always require an interactive TTY sub-menu
	// and are not supported in headless mode. The user is redirected to the
	// REST management panel instead.
	cmdModeBlocked
)

// webCommandRegistry maps a slash-command token to its cobra entry point and
// the headless execution policy that applies.
var webCommandRegistry = []struct {
	token string
	cmd   *cobra.Command
	mode  commandMode
	ctx   bool // if true, inject session context via runCommandCtx
}{
	// Info-only: read-only, no TTY interaction
	{"/agent", agentCmd, cmdModeInfo, false},
	{"/model", modelCmd, cmdModeInfo, false},
	{"/tools", toolsCmd, cmdModeInfo, false},
	{"/mcp", mcpCmd, cmdModeInfo, false},
	{"/skills", skillsCmd, cmdModeInfo, false},
	{"/features", capsCmd, cmdModeInfo, false},
	{"/capabilities", capsCmd, cmdModeInfo, false},
	{"/memory", memoryCmd, cmdModeInfo, false},
	{"/think", thinkCmd, cmdModeInfo, false},
	{"/verbose", verboseCmd, cmdModeInfo, false},
	{"/version", versionCmd, cmdModeInfo, false},

	// Session-ops: mutate current session, args must be explicit (no picker)
	{"/clear", sessionClearCurrentCmd, cmdModeSessionOp, true},
	{"/compress", sessionCompressCurrentCmd, cmdModeSessionOp, true},
	{"/rename", sessionRenameCurrentCmd, cmdModeSessionOp, true},

	// Blocked: require interactive TTY sub-menus
	{"/session", sessionCmd, cmdModeBlocked, false},
	{"/search", searchCmd, cmdModeBlocked, false},
	{"/workflow", workflowCmd, cmdModeBlocked, false},
}

// handleWebCommand intercepts REPL commands and maps their state mutations to
// the requested headless session.
// Returns: (handledAndClosed bool, newPrompt string, newGuideline string)
func handleWebCommand(prompt string, sessionName string, sseOut *sse.SSEOutput) (bool, string, string) {
	parts := parseCommandArgs(prompt)
	if len(parts) == 0 {
		return false, prompt, ""
	}
	command := parts[0]

	// 1. Check the static whitelist registry.
	for _, entry := range webCommandRegistry {
		if command != entry.token {
			continue
		}
		switch entry.mode {
		case cmdModeBlocked:
			sseOut.WriteCommandEvent("", fmt.Sprintf(
				"'%s' requires interactive selection and is not supported in chat. "+
					"Please use the management panel instead.", command))
			return true, prompt, ""
		case cmdModeInfo, cmdModeSessionOp:
			w := sse.NewSSEWriter(sseOut)
			if entry.ctx {
				runCommandCtx(NewContextWithSession(sessionName), entry.cmd, parts[1:], w)
			} else {
				runCommand(entry.cmd, parts[1:], w)
			}
			return true, prompt, ""
		}
	}

	// 2. Attempt to execute as a Workflow.
	wm := service.GetWorkflowManager()
	content, _, err := wm.GetWorkflowByName(strings.TrimPrefix(command, "/"))
	if err == nil {
		userArgs := ""
		if len(parts) > 1 {
			userArgs = strings.Join(parts[1:], " ")
		}
		newPrompt := content
		if userArgs != "" {
			newPrompt += "\n" + userArgs
		}
		return false, newPrompt, ""
	}

	// 3. Attempt to execute as a Skill.
	sm := service.GetSkillManager()
	skills := sm.GetAvailableSkillsMetadata()
	skillName := strings.ToLower(strings.TrimPrefix(command, "/"))
	for _, s := range skills {
		if strings.ToLower(s.Name) == skillName {
			userArgs := ""
			if len(parts) > 1 {
				userArgs = strings.Join(parts[1:], " ")
			}
			guideline := fmt.Sprintf(
				"You need to activate the skill '%s' and follow its guidelines to answer the user's request. "+
					"Use tool 'activate_skill' with the skill name '%s'.",
				s.Name, s.Name)
			newPrompt := "/" + s.Name
			if userArgs != "" {
				newPrompt += "\n" + userArgs
			}
			return false, newPrompt, guideline
		}
	}

	// 4. Unknown command — pass through to the LLM.
	return false, prompt, ""
}

func runAgentWithSSE(prompt string, guideline string, sessionName string, sseIO *sse.SSEOutput, agent *data.AgentConfig, ctx context.Context) error {
	sharedState := data.NewSharedState()
	defer sharedState.Clear() // Clean up on session end

	for {
		// Ensure session compatibility (headless hook)
		hook := service.SessionConvertHook{
			OnStartConvert:    func() { sseIO.WriteStatusEvent("converting_session") },
			OnFinishedConvert: func() { sseIO.WriteStatusEvent("session_ready") },
		}
		if err := service.EnsureSessionCompatibility(agent, sessionName, hook); err != nil {
			return err
		}

		finalPrompt := buildFinalPrompt(prompt, guideline)

		mcpStore := data.NewMCPStore()
		mcpConfig, err := mcpStore.Load()
		if err != nil {
			return err
		}

		// Build an interaction handler that suspends the goroutine and emits SSE events
		// so the frontend can surface approval dialogs and POST back user decisions.
		sseInteraction := service.NewSSEInteractionHandler(
			func(id string, kind service.InteractionKind, purpose string) {
				sseIO.WriteRequestEvent(id, string(kind), purpose)
			},
			func(before, after string) {
				sseIO.WriteDiffEvent(before, after)
			},
			0, // no timeout: block until frontend responds
		)

		op := service.AgentOptions{
			Ctx:           ctx, // Carry HTTP completion cancellation context
			Prompt:        finalPrompt,
			SysPrompt:     agent.SystemPrompt,
			Files:         nil, // Can map attachments later if needed
			ModelInfo:     &agent.Model,
			MaxRecursions: agent.MaxRecursions,
			ThinkingLevel: agent.Think,
			EnabledTools:  agent.Tools,
			Capabilities:  agent.Capabilities,
			YoloMode:      false, // Now user-driven; approval comes via /v1/interact
			OutputFile:    "",
			QuietMode:     !serveVerbose, // True by default unless --verbose is provided
			SSEOutput:     sseIO,         // SSE Output for streaming
			SessionName:   sessionName,
			MCPConfig:     mcpConfig,
			Interaction:   sseInteraction,
			SharedState:   sharedState,
			AgentName:     agent.Name,
			ModelName:     agent.Model.Name,
		}

		err = service.CallAgent(&op)
		if err != nil {
			if service.IsSwitchAgentError(err) {
				switchErr, _ := service.AsSwitchAgentError(err)
				prompt = switchErr.Instruction
				if prompt == "" {
					break
				}
				continue
			} else if service.IsUserCancelError(err) {
				break
			}
			return err
		}
		break
	}
	return nil
}
