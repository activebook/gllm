package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/io"
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
		http.HandleFunc("/v1/interact", interactHandler)

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

	sseOut, err := io.NewSSEOutput(w)
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

// handleWebCommand intercepts REPL commands and maps their state mutations to the requested headless session
// Returns: (handledAndClosed bool, newPrompt string, newGuideline string)
func handleWebCommand(prompt string, sessionName string, sseOut *io.SSEOutput) (bool, string, string) {
	parts := parseCommandArgs(prompt)
	if len(parts) == 0 {
		return false, prompt, ""
	}
	command := parts[0]

	switch command {
	case "/clear":
		runCommandCtx(NewContextWithSession(sessionName), sessionClearCurrentCmd, parts[1:], io.NewSSEWriter(sseOut))
		return true, prompt, ""

	case "/compress":
		runCommandCtx(NewContextWithSession(sessionName), sessionCompressCurrentCmd, parts[1:], io.NewSSEWriter(sseOut))
		return true, prompt, ""

	case "/rename":
		runCommandCtx(NewContextWithSession(sessionName), sessionRenameCurrentCmd, parts[1:], io.NewSSEWriter(sseOut))
		return true, prompt, ""

	case "/model":
		runCommand(modelCmd, parts[1:], io.NewSSEWriter(sseOut))
		return true, prompt, ""

	case "/agent":
		runCommand(agentCmd, parts[1:], io.NewSSEWriter(sseOut))
		return true, prompt, ""

	case "/session":
		runCommand(sessionCmd, parts[1:], io.NewSSEWriter(sseOut))
		return true, prompt, ""

	case "/search":
		runCommand(searchCmd, parts[1:], io.NewSSEWriter(sseOut))
		return true, prompt, ""

	case "/tools":
		runCommand(toolsCmd, parts[1:], io.NewSSEWriter(sseOut))
		return true, prompt, ""

	case "/mcp":
		runCommand(mcpCmd, parts[1:], io.NewSSEWriter(sseOut))
		return true, prompt, ""

	case "/skills":
		runCommand(skillsCmd, parts[1:], io.NewSSEWriter(sseOut))
		return true, prompt, ""

	case "/memory":
		runCommand(memoryCmd, parts[1:], io.NewSSEWriter(sseOut))
		return true, prompt, ""

	case "/think":
		runCommand(thinkCmd, parts[1:], io.NewSSEWriter(sseOut))
		return true, prompt, ""

	case "/features", "/capabilities":
		runCommand(capsCmd, parts[1:], io.NewSSEWriter(sseOut))
		return true, prompt, ""

	case "/theme":
		runCommand(themeCmd, parts[1:], io.NewSSEWriter(sseOut))
		return true, prompt, ""

	case "/verbose":
		runCommand(verboseCmd, parts[1:], io.NewSSEWriter(sseOut))
		return true, prompt, ""

	case "/workflow":
		runCommand(workflowCmd, parts[1:], io.NewSSEWriter(sseOut))
		return true, prompt, ""

	case "/update":
		runCommand(updateCmd, parts[1:], io.NewSSEWriter(sseOut))
		return true, prompt, ""

	default:
		// Attempt to execute as Workflow
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

		// Attempt to execute as Skill
		sm := service.GetSkillManager()
		skills := sm.GetAvailableSkillsMetadata()
		var foundSkill *data.SkillMetadata
		skillName := strings.ToLower(strings.TrimPrefix(command, "/"))
		for _, s := range skills {
			if strings.ToLower(s.Name) == skillName {
				foundSkill = &s
				break
			}
		}

		if foundSkill != nil {
			userArgs := ""
			if len(parts) > 1 {
				userArgs = strings.Join(parts[1:], " ")
			}
			guideline := fmt.Sprintf("You need to activate the skill '%s' and follow its guidelines to answer the user's request. Use tool 'activate_skill' with the skill name '%s'.", foundSkill.Name, foundSkill.Name)
			newPrompt := "/" + foundSkill.Name
			if userArgs != "" {
				newPrompt += "\n" + userArgs
			}
			return false, newPrompt, guideline
		}

		// Unsupported native web command or requires CallAgent resolution
		return false, prompt, ""
	}
}

func runAgentWithSSE(prompt string, guideline string, sessionName string, sseIO *io.SSEOutput, agent *data.AgentConfig, ctx context.Context) error {
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
			func(id string, kind service.InteractionKind, purpose string, toolName *string) {
				sseIO.WriteRequestEvent(id, string(kind), purpose, toolName)
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

// InteractRequest is the body of POST /v1/interact used by the frontend to
// resolve a pending interaction (tool confirm, ask-user, etc.).
type InteractRequest struct {
	ID        string `json:"id"`                  // UUID matching the SSE interaction_request event
	Kind      string `json:"kind"`                // "tool_confirm" | "ask_user"
	Approve   string `json:"approve,omitempty"`   // For tool_confirm ("once", "always", "cancel")
	Answer    string `json:"answer,omitempty"`    // For ask_user
	Cancelled bool   `json:"cancelled,omitempty"` // For ask_user: user dismissed the dialog
}

// interactHandler handles POST /v1/interact.
// It routes the frontend response to the corresponding suspended goroutine via InteractionRegistry.
func interactHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req InteractRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var resolveErr error
	switch req.Kind {
	case string(service.InteractionKindConfirm):
		resolveErr = service.InteractionRegistry.ResolveConfirm(req.ID, req.Approve)
	case string(service.InteractionKindAskUser):
		resolveErr = service.InteractionRegistry.ResolveAskUser(req.ID, req.Answer, req.Cancelled)
	default:
		http.Error(w, fmt.Sprintf("unknown interaction kind: %s", req.Kind), http.StatusBadRequest)
		return
	}

	if resolveErr != nil {
		http.Error(w, resolveErr.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
}
