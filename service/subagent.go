package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/activebook/gllm/data"
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
	ID          string // Unique task ID
	AgentName   string // Agent profile to use
	Instruction string // Task instruction/prompt
	TaskKey     string // Key to store result in SharedState
}

// SubAgentResult represents the outcome of a sub-agent execution
type SubAgentResult struct {
	TaskID     string         // Task ID
	AgentName  string         // Agent that executed
	Status     SubAgentStatus // Execution status
	Progress   string         // Human-readable progress description
	OutputFile string         // Path to detailed output
	TaskKey    string         // Key where result was stored in SharedState
	Error      error          // Error if failed
	Duration   time.Duration  // Execution duration
	StartTime  time.Time      // When execution started
	EndTime    time.Time      // When execution ended
}

// AgentRunner defines the function signature for executing an agent
type AgentRunner func(*AgentOptions) error

// SubAgentExecutor manages sub-agent lifecycle and execution
type SubAgentExecutor struct {
	state      *data.SharedState
	maxWorkers int
	taskID     atomic.Int64
	runner     AgentRunner // Function to execute agent (default: CallAgent)

	// Task tracking
	mu       sync.RWMutex
	tasks    map[string]*taskEntry
	mcpStore *data.MCPStore
}

type taskEntry struct {
	task   *SubAgentTask
	result *SubAgentResult
	cancel context.CancelFunc
}

// NewSubAgentExecutor creates a new SubAgentExecutor
func NewSubAgentExecutor(state *data.SharedState, maxWorkers int) *SubAgentExecutor {
	if maxWorkers <= 0 {
		maxWorkers = 5
	}
	return &SubAgentExecutor{
		state:      state,
		maxWorkers: maxWorkers,
		tasks:      make(map[string]*taskEntry),
		mcpStore:   data.NewMCPStore(),
		runner:     CallAgent, // Default runner
	}
}

// generateTaskID generates a unique task ID
func (e *SubAgentExecutor) generateTaskID() string {
	id := e.taskID.Add(1)
	return fmt.Sprintf("task-%d-%d", time.Now().UnixNano(), id)
}

// Submit submits a single task for execution and returns the task ID
func (e *SubAgentExecutor) Submit(task *SubAgentTask) string {
	if task.ID == "" {
		task.ID = e.generateTaskID()
	}

	e.mu.Lock()
	e.tasks[task.ID] = &taskEntry{
		task: task,
		result: &SubAgentResult{
			TaskID:    task.ID,
			AgentName: task.AgentName,
			Status:    StatusPending,
			TaskKey:   task.TaskKey,
		},
	}
	e.mu.Unlock()

	return task.ID
}

// SubmitBatch submits multiple tasks and returns their IDs
func (e *SubAgentExecutor) SubmitBatch(tasks []*SubAgentTask) []string {
	ids := make([]string, len(tasks))
	for i, task := range tasks {
		ids[i] = e.Submit(task)
	}
	return ids
}

// Execute runs all pending tasks and waits for completion
func (e *SubAgentExecutor) Execute(timeout time.Duration) []SubAgentResult {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Collect pending tasks
	e.mu.RLock()
	var pendingTasks []*taskEntry
	for _, entry := range e.tasks {
		if entry.result.Status == StatusPending {
			pendingTasks = append(pendingTasks, entry)
		}
	}
	e.mu.RUnlock()

	if len(pendingTasks) == 0 {
		return nil
	}

	// Use semaphore to limit concurrent workers
	sem := make(chan struct{}, e.maxWorkers)
	var wg sync.WaitGroup

	for _, entry := range pendingTasks {
		wg.Add(1)
		go func(te *taskEntry) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				te.result.Status = StatusCancelled
				te.result.Error = ctx.Err()
				return
			}

			// Create cancellable context for this task
			taskCtx, taskCancel := context.WithCancel(ctx)
			e.mu.Lock()
			te.cancel = taskCancel
			e.mu.Unlock()

			// Execute the task
			e.executeTask(taskCtx, te)
		}(entry)
	}

	wg.Wait()

	// Collect results
	e.mu.RLock()
	results := make([]SubAgentResult, 0, len(pendingTasks))
	for _, entry := range pendingTasks {
		results = append(results, *entry.result)
	}
	e.mu.RUnlock()

	return results
}

// executeTask runs a single sub-agent task
func (e *SubAgentExecutor) executeTask(ctx context.Context, entry *taskEntry) {
	task := entry.task
	result := entry.result

	result.StartTime = time.Now()
	result.Status = StatusRunning

	// Load agent configuration
	store := data.NewConfigStore()
	agentConfig := store.GetAgent(task.AgentName)
	if agentConfig == nil {
		result.Status = StatusFailed
		result.Error = fmt.Errorf("agent '%s' not found", task.AgentName)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		result.Progress = fmt.Sprintf("Failed: %v", result.Error)
		return
	}

	// Build system prompt with memory
	sysPrompt := store.GetSystemPrompt(agentConfig.SystemPrompt)
	memStore := data.NewMemoryStore()
	memoryContent := memStore.GetFormatted()
	if memoryContent != "" {
		sysPrompt += "\n\n" + memoryContent
	}

	// Load MCP config
	mcpConfig, _, _ := e.mcpStore.Load()

	// Generate output file path (persistent)
	// Use TaskKey in filename for better traceability
	keyPart := ""
	if task.TaskKey != "" {
		keyPart = "_" + GetSanitizeTitle(task.TaskKey)
	}
	outputFile, err := GenerateTaskFilePath(fmt.Sprintf("subagent_%s%s", task.AgentName, keyPart), ".md")
	if err != nil {
		// Fallback to simpler path or handle error
		outputFile = GenerateTempFilePath(fmt.Sprintf("subagent_%s%s", task.AgentName, keyPart), ".md")
		Warnf("Failed to create persistent output file, using temp: %v\n", err)
	}
	result.OutputFile = outputFile

	// Prepare agent options
	op := AgentOptions{
		Prompt:         task.Instruction,
		SysPrompt:      sysPrompt,
		Files:          nil,
		ModelInfo:      &agentConfig.Model,
		SearchEngine:   &agentConfig.Search,
		MaxRecursions:  agentConfig.MaxRecursions,
		ThinkingLevel:  agentConfig.Think,
		EnabledTools:   agentConfig.Tools,
		UseMCP:         agentConfig.MCP,
		YoloMode:       true, // Sub-agents always auto-approve
		AppendUsage:    agentConfig.Usage,
		AppendMarkdown: agentConfig.Markdown,
		OutputFile:     outputFile,
		QuietMode:      true, // Sub-agents run quietly
		ConvoName:      "",   // No conversation persistence for sub-agents
		MCPConfig:      mcpConfig,
		SharedState:    e.state,
		AgentName:      task.AgentName,
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		result.Status = StatusCancelled
		result.Error = ctx.Err()
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		result.Progress = "Cancelled"
		return
	default:
	}

	// Execute the agent
	err = e.runner(&op)

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	if err != nil {
		result.Status = StatusFailed
		result.Error = err
		result.Progress = fmt.Sprintf("Failed after %s: %v", result.Duration.Round(time.Millisecond), err)
	} else {
		result.Status = StatusCompleted
		result.Progress = fmt.Sprintf("Completed in %s", result.Duration.Round(time.Millisecond))

		// Store output in SharedState if TaskKey is specified
		if task.TaskKey != "" && e.state != nil {
			content, err := GetFileContent(outputFile)
			if err == nil {
				e.state.Set(task.TaskKey, content, task.AgentName)
			}
		}
	}
}

// GetProgress returns the current result for a task
func (e *SubAgentExecutor) GetProgress(taskID string) *SubAgentResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if entry, exists := e.tasks[taskID]; exists {
		resultCopy := *entry.result
		return &resultCopy
	}
	return nil
}

// GetAllProgress returns progress for all tasks
func (e *SubAgentExecutor) GetAllProgress() []SubAgentResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	results := make([]SubAgentResult, 0, len(e.tasks))
	for _, entry := range e.tasks {
		results = append(results, *entry.result)
	}
	return results
}

// Cancel cancels a running task
func (e *SubAgentExecutor) Cancel(taskID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	entry, exists := e.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	if entry.cancel != nil {
		entry.cancel()
		entry.result.Status = StatusCancelled
		return nil
	}

	if entry.result.Status == StatusPending {
		entry.result.Status = StatusCancelled
		return nil
	}

	return fmt.Errorf("task %s cannot be cancelled (status: %s)", taskID, entry.result.Status)
}

// CancelAll cancels all running tasks
func (e *SubAgentExecutor) CancelAll() {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, entry := range e.tasks {
		if entry.cancel != nil {
			entry.cancel()
			entry.result.Status = StatusCancelled
		} else if entry.result.Status == StatusPending {
			entry.result.Status = StatusCancelled
		}
	}
}

// Clear removes all completed/failed/cancelled tasks
func (e *SubAgentExecutor) Clear() {
	e.mu.Lock()
	defer e.mu.Unlock()

	for id, entry := range e.tasks {
		status := entry.result.Status
		if status == StatusCompleted || status == StatusFailed || status == StatusCancelled {
			delete(e.tasks, id)
		}
	}
}

// ClearAll removes all tasks
func (e *SubAgentExecutor) ClearAll() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.tasks = make(map[string]*taskEntry)
}

// FormatProgress returns a formatted string of all task progress
func (e *SubAgentExecutor) FormatProgress() string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if len(e.tasks) == 0 {
		return "No sub-agent tasks have been submitted."
	}

	var result string
	result = fmt.Sprintf("Sub-agent tasks (%d total):\n\n", len(e.tasks))

	for _, entry := range e.tasks {
		r := entry.result
		result += fmt.Sprintf("Task: %s\n", r.TaskID)
		result += fmt.Sprintf("  Agent: %s\n", r.AgentName)
		result += fmt.Sprintf("  Status: %s\n", r.Status)
		result += fmt.Sprintf("  Progress: %s\n", r.Progress)
		if r.TaskKey != "" {
			result += fmt.Sprintf("  Task Key: %s\n", r.TaskKey)
		}
		if r.OutputFile != "" {
			result += fmt.Sprintf("  Output File: %s\n", r.OutputFile)
		}
		if r.Duration > 0 {
			result += fmt.Sprintf("  Duration: %s\n", r.Duration.Round(time.Millisecond))
		}
		result += "\n"
	}

	return result
}

// FormatSummary returns a brief summary of task execution
func (e *SubAgentExecutor) FormatSummary(results []SubAgentResult) string {
	if len(results) == 0 {
		return "No tasks were executed."
	}

	completed := 0
	failed := 0
	cancelled := 0
	var outputs []string

	for _, r := range results {
		switch r.Status {
		case StatusCompleted:
			completed++
			if r.TaskKey != "" {
				outputs = append(outputs, r.TaskKey)
			}
		case StatusFailed:
			failed++
		case StatusCancelled:
			cancelled++
		}
	}

	summary := fmt.Sprintf("Executed %d sub-agent task(s): %d completed, %d failed, %d cancelled.",
		len(results), completed, failed, cancelled)

	if len(outputs) > 0 {
		summary += fmt.Sprintf("\nResults stored in SharedState keys: %v", outputs)
		summary += "\nUse get_state tool to retrieve detailed results."
	}

	return summary
}
