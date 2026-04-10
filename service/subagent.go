package service

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/util"
)

// SubAgentStatus represents the execution status of a sub-agent task
type SubAgentStatus int

const (
	StatusPending SubAgentStatus = iota
	StatusRunning
	StatusCompleted
	StatusFailed
	StatusCancelled
)

func (s SubAgentStatus) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusRunning:
		return "running"
	case StatusCompleted:
		return "completed"
	case StatusFailed:
		return "failed"
	case StatusCancelled:
		return "cancelled"
	default:
		return "unknown"
	}
}

// SubAgentTask represents a single sub-agent invocation request
type SubAgentTask struct {
	CallerAgentName string   // Caller agent name
	AgentName       string   // Agent profile to use
	Instruction     string   // Task instruction/prompt
	TaskKey         string   // Key to store result in SharedState (becomes agentName_taskKey)
	InputKeys       []string // Keys to read as input context (virtual files), injected into instruction
}

// SubAgentResult represents the outcome of a sub-agent execution
type SubAgentResult struct {
	AgentName  string         // Agent that executed
	Status     SubAgentStatus // Execution status
	Progress   string         // Human-readable progress description
	OutputFile string         // Path to detailed output
	TaskKey    string         // Original task key
	StateKey   string         // Key where result was stored in SharedState (agentName_taskKey)
	Error      error          // Error if failed
	Duration   time.Duration  // Execution duration
	StartTime  time.Time      // When execution started
	EndTime    time.Time      // When execution ended
}

// AgentMessage is a task delivery envelope sent on an agent's TaskChan.
type AgentMessage struct {
	Task     *SubAgentTask
	RespChan chan<- AgentResponse // caller-owned, per-request
}

// AgentResponse is the signal sent back to the caller when a task finishes.
type AgentResponse struct {
	TaskKey string
	Result  *SubAgentResult
	Err     error
}

// ActiveAgent is a persistent, resident subagent actor.
type ActiveAgent struct {
	Name     string
	Config   *data.AgentConfig
	TaskChan chan AgentMessage // event loop inbox
	executor *SubAgentExecutor // reference to the parent executor to access state/runner
}

// AgentRunner defines the function signature for executing an agent
type AgentRunner func(*AgentOptions) error

// SubAgentExecutor manages sub-agent lifecycle and message routing
type SubAgentExecutor struct {
	state           *data.SharedState
	runner          AgentRunner // Function to execute agent (default: CallAgent)
	mainSessionName string      // Orchestrator session name (e.g., "my project")

	mu           sync.RWMutex
	activeAgents map[string]*ActiveAgent
	mcpStore     *data.MCPStore
}

// NewSubAgentExecutor creates a new SubAgentExecutor
func NewSubAgentExecutor(state *data.SharedState, mainSessionName string) *SubAgentExecutor {
	return &SubAgentExecutor{
		state:           state,
		mainSessionName: mainSessionName,
		activeAgents:    make(map[string]*ActiveAgent),
		mcpStore:        data.NewMCPStore(),
		runner:          CallAgent, // Default runner
	}
}

// startSubAgent returns a running ActiveAgent, launching its event loop if it doesn't exist yet.
func (e *SubAgentExecutor) startSubAgent(agentName string) (*ActiveAgent, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 1. Check if already running
	if agent, exists := e.activeAgents[agentName]; exists {
		return agent, nil
	}

	// 2. Load agent config to verify it exists
	store := data.NewConfigStore()
	agentConfig := store.GetAgent(agentName)
	if agentConfig == nil {
		return nil, fmt.Errorf("agent '%s' not found", agentName)
	}

	// 3. Create active agent structure
	agent := &ActiveAgent{
		Name:     agentName,
		Config:   agentConfig,
		TaskChan: make(chan AgentMessage, 8), // small buffer to absorb bursts
		executor: e,
	}

	// 4. Register
	e.activeAgents[agentName] = agent

	// 5. Start the event loop
	go e.agentLoop(agent)

	return agent, nil
}

// Shutdown closes all agent task channels, allowing their event loops to exit.
func (e *SubAgentExecutor) Shutdown() {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, agent := range e.activeAgents {
		close(agent.TaskChan)
	}
	// Clear the registry
	e.activeAgents = make(map[string]*ActiveAgent)
}

// agentLoop is the persistent goroutine that receives tasks and dispatches workers.
func (e *SubAgentExecutor) agentLoop(agent *ActiveAgent) {
	for msg := range agent.TaskChan {
		// Spawn a goroutine to handle the task so the loop never blocks
		go e.handleMsg(agent, msg)
	}
}

// handleMsg performs the work requested by an AgentMessage.
func (e *SubAgentExecutor) handleMsg(agent *ActiveAgent, msg AgentMessage) {
	// Execute the task
	result := e.executeTask(agent, msg.Task)

	// Send the response back to the caller
	msg.RespChan <- AgentResponse{
		TaskKey: msg.Task.TaskKey,
		Result:  result,
		Err:     result.Error,
	}
}

// Dispatch fans out tasks asynchronously to subagents and waits for all responses.
func (e *SubAgentExecutor) Dispatch(tasks []*SubAgentTask) ([]AgentResponse, error) {
	if len(tasks) == 0 {
		return nil, nil
	}

	// Buffered channel avoids goroutine leak if the caller panics or gives up early
	respChan := make(chan AgentResponse, len(tasks))

	// Fan-out Phase: Start tasks concurrently
	for _, task := range tasks {
		agent, err := e.startSubAgent(task.AgentName)
		if err != nil {
			// Fast-fail if agent config is missing before trying to send
			respChan <- AgentResponse{
				TaskKey: task.TaskKey,
				Err:     err,
			}
			continue
		}

		// Capture loop variable
		t := task

		// Send non-blockingly (to the dispatch loop, not the actual receiver)
		// If TaskChan buffer is full, we must use a goroutine to wait
		go func(a *ActiveAgent, t *SubAgentTask) {
			a.TaskChan <- AgentMessage{
				Task:     t,
				RespChan: respChan,
			}
		}(agent, t)
	}

	// Fan-in Phase: Collect all responses
	results := make([]AgentResponse, 0, len(tasks))
	for i := 0; i < len(tasks); i++ {
		resp := <-respChan
		results = append(results, resp)
	}

	return results, nil
}

// executeTask runs the LLM call for a sub-agent task.
func (e *SubAgentExecutor) executeTask(agent *ActiveAgent, task *SubAgentTask) *SubAgentResult {
	result := &SubAgentResult{
		AgentName: agent.Name,
		TaskKey:   task.TaskKey,
	}

	// Both components form the key suffix
	agentTaskKey := ""
	if task.TaskKey != "" {
		// Because agentTaskKey would be used as file name, we use "_" as separator
		agentTaskKey = fmt.Sprintf("%s_%s", agent.Name, task.TaskKey)
	}

	// Build subagent session name as "mainSession::agentName_taskKey"
	sessionName := ""
	if e.mainSessionName != "" && agentTaskKey != "" {
		// Because mainSessionName only as a logic namespace, we use "::" as separator
		sessionName = fmt.Sprintf("%s::%s", e.mainSessionName, agentTaskKey)
	}

	// The key used to store the output in SharedState
	result.StateKey = agentTaskKey

	// Set task start status and print the start message
	e.setTaskStart(result, task, sessionName)

	// Load MCP config
	mcpConfig, _ := e.mcpStore.Load()

	// Prepare input context from SharedState: inject as a hidden inline block
	// so it is preserved for the LLM but stripped from the session history UI.
	finalInstruction := task.Instruction
	if len(task.InputKeys) > 0 && e.state != nil {
		var ctxBlob strings.Builder
		ctxBlob.WriteString("# Context from previous tasks:\n")
		for _, key := range task.InputKeys {
			if val, ok := e.state.Get(key); ok {
				contentStr := fmt.Sprintf("%v", val)
				ctxBlob.WriteString(fmt.Sprintf("\n## Output from '%s':\n%s\n", util.GetSanitizeTitle(key), contentStr))
			} else {
				util.LogWarnf("Sub-agent input key '%s' not found in SharedState, skipping.\n", key)
			}
		}
		finalInstruction = BuildInlineContextBlock([]string{ctxBlob.String()}) + task.Instruction
	}

	// Prepare agent options
	op := AgentOptions{
		Prompt:        finalInstruction,
		SysPrompt:     agent.Config.SystemPrompt,
		Files:         nil,
		ModelInfo:     &agent.Config.Model,
		MaxRecursions: agent.Config.MaxRecursions,
		ThinkingLevel: agent.Config.Think,
		EnabledTools:  agent.Config.Tools,
		Capabilities:  agent.Config.Capabilities,
		YoloMode:      true, // Sub-agents always auto-approve
		QuietMode:     true, // Sub-agents run quietly
		SessionName:   sessionName,
		MCPConfig:     mcpConfig,
		SharedState:   e.state,
		AgentName:     agent.Name,
		ModelName:     agent.Config.Model.Name,
	}

	// Execute the agent (synchronous blocking call within this goroutine)
	err := e.runner(&op)
	if err != nil {
		e.setTaskError(result, task.TaskKey, err)
	}

	// Map-Reduce boundary: Compress session and write to SharedState
	if agentTaskKey != "" && e.state != nil {
		sessionData, readErr := ReadSessionContent(sessionName)
		if readErr == nil {
			summary, compressErr := CompressSession(agent.Config, sessionData)
			if compressErr != nil {
				e.state.Set(agentTaskKey, fmt.Sprintf("[compression failed: %v]", compressErr), agent.Name)
				e.setTaskError(result, task.TaskKey, compressErr)
			} else {
				e.state.Set(agentTaskKey, summary, agent.Name)
				e.setTaskCompleted(result, task.TaskKey)
			}
		} else {
			e.setTaskError(result, task.TaskKey, readErr)
		}
	} else {
		e.setTaskError(result, "", fmt.Errorf("failed to write session to SharedState: no task key or shared state"))
	}

	return result
}

// setTaskStart sets the task to running status and prints the start message.
func (e *SubAgentExecutor) setTaskStart(result *SubAgentResult, task *SubAgentTask, sessionName string) {
	result.Status = StatusRunning
	result.StartTime = time.Now()
	result.Error = nil

	// Determine whether this is a new or resumed session
	mode := "Executing"
	if SessionExists(sessionName, true) {
		mode = "Resuming"
	}
	fmt.Printf("==> %s task: %s %s[%s -> %s]%s ...\n", mode, task.TaskKey, data.AgentRoleColor, task.CallerAgentName, task.AgentName, data.ResetSeq)
}

// setTaskCompleted sets the task to completed status and prints the success message.
func (e *SubAgentExecutor) setTaskCompleted(result *SubAgentResult, taskKey string) {
	result.Status = StatusCompleted
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.Error = nil
	result.Progress = fmt.Sprintf("Completed in %s", result.Duration.Round(time.Millisecond))
	fmt.Printf("%s✓ > Task completed: %s%s\n", data.StatusSuccessColor, taskKey, data.ResetSeq)
}

// setTaskError sets the task to failed status and prints the error message.
func (e *SubAgentExecutor) setTaskError(result *SubAgentResult, taskKey string, err error) {
	result.Status = StatusFailed
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.Error = err
	result.Progress = fmt.Sprintf("Failed after %s: %v", result.Duration.Round(time.Millisecond), err)
	fmt.Printf("%s✗ > Task failed: %s - %v%s\n", data.StatusErrorColor, taskKey, err, data.ResetSeq)
}

// FormatSummary returns a brief summary of task execution
func (e *SubAgentExecutor) FormatSummary(responses []AgentResponse) string {
	if len(responses) == 0 {
		return "No tasks were executed."
	}

	completed := 0
	failed := 0
	var outputs []string

	for _, r := range responses {
		if r.Err != nil || (r.Result != nil && r.Result.Status == StatusFailed) {
			failed++
		} else {
			completed++
			if r.Result != nil && r.Result.StateKey != "" {
				outputs = append(outputs, r.Result.StateKey)
			}
		}
	}

	summary := fmt.Sprintf("Executed %d sub-agent task(s): %d completed, %d failed.",
		len(responses), completed, failed)

	if len(outputs) > 0 {
		summary += fmt.Sprintf("\nResults stored in SharedState keys: %v", outputs)
		summary += "\nUse get_state tool to retrieve detailed results."
	}

	return summary
}
