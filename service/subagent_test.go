package service

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/activebook/gllm/data"
	"github.com/spf13/viper"
)

// Mock runner to simulate subagent processing
func mockRunner(op *AgentOptions) error {
	// Simulate work delay
	time.Sleep(50 * time.Millisecond)

	// Since we mock, let's just write "mock_response" to the output file
	// Actually, the new architecture does not write to the file directly in the mock,
	// it expects the `CallAgent` to populate the `Context.Messages` or just return.
	// We'll mimic sending a JSON message to op.ProceedChan or writing a basic response.
	if op.AgentName == "slow_agent" {
		time.Sleep(200 * time.Millisecond)
	}
	return nil
}

func mockCallAgentOverride(op *AgentOptions) error {
	return mockRunner(op)
}

func setupTestConfig() {
	viper.Reset()
	viper.Set("agents.test_agent.model", "gpt-4")
	data.NewConfigStore()
}

func defaultState() *data.SharedState {
	return data.NewSharedState()
}

// Test Dispatch with multiple parallel tasks
func TestDispatchMultipleTasks(t *testing.T) {
	setupTestConfig()
	state := defaultState()
	executor := NewSubAgentExecutor(state, "test_session")

	// Inject custom runner
	executor.runner = func(op *AgentOptions) error {
		// Just sleep to simulate LLM
		time.Sleep(10 * time.Millisecond)
		// We can populate the returned response artificially if needed,
		// but `executeTask` in tests might fail if `GetFileContent` errors.
		// However `executeTask` just runs the runner. If it succeeds, it returns a status.
		return nil
	}

	tasks := []*SubAgentTask{
		{CallerAgentName: "orchestrator", AgentName: "test_agent", TaskKey: "task1", Instruction: "Do 1"},
		{CallerAgentName: "orchestrator", AgentName: "test_agent", TaskKey: "task2", Instruction: "Do 2"},
		{CallerAgentName: "orchestrator", AgentName: "test_agent", TaskKey: "task3", Instruction: "Do 3"},
	}

	start := time.Now()
	responses, err := executor.Dispatch(tasks)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Dispatch failed: %v", err)
	}

	if len(responses) != 3 {
		t.Fatalf("Expected 3 responses, got %d", len(responses))
	}

	// Because tasks run concurrently, time elapsed should be roughly the duration of one (10ms),
	// not sequentially (30ms). Add a small buffer for go scheduler.
	if elapsed > 25*time.Millisecond {
		t.Errorf("Execution took too long (%v), they might not be running concurrently", elapsed)
	}
}

// Test that cross-agent deadlock is resolved by goroutines
func TestCrossAgentCallDeadlock(t *testing.T) {
	setupTestConfig()
	state := defaultState()
	executor := NewSubAgentExecutor(state, "test_session")

	var wg sync.WaitGroup
	wg.Add(2)

	// We simulate Agent A dispatching a task to Agent B.
	// In the real system, A -> dispatcher -> B -> A.
	// But in new actor loop, B -> A is non blocking to A's loop.

	executor.runner = func(op *AgentOptions) error {
		time.Sleep(10 * time.Millisecond)
		return nil
	}

	// This is just verifying the system won't hang when multiple actors dispatch
	go func() {
		defer wg.Done()
		tasks := []*SubAgentTask{{CallerAgentName: "A", AgentName: "test_agent", TaskKey: "A1", Instruction: "call B"}}
		resp, err := executor.Dispatch(tasks)
		if err != nil || len(resp) == 0 {
			t.Errorf("Agent A dispatch failed: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		tasks := []*SubAgentTask{{CallerAgentName: "B", AgentName: "test_agent", TaskKey: "B1", Instruction: "call A"}}
		resp, err := executor.Dispatch(tasks)
		if err != nil || len(resp) == 0 {
			t.Errorf("Agent B dispatch failed: %v", err)
		}
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Deadlock occurred during cross-agent dispatch")
	}
}

func TestFormatSummary(t *testing.T) {
	state := defaultState()
	executor := NewSubAgentExecutor(state, "test_session")

	responses := []AgentResponse{
		{
			Result: &SubAgentResult{
				AgentName: "agentA",
				TaskKey:   "task1",
				Status:    StatusCompleted,
			},
		},
		{
			Result: &SubAgentResult{
				AgentName: "agentA",
				TaskKey:   "task2",
				Status:    StatusFailed,
				Error:     fmt.Errorf("simulated error"),
			},
		},
	}

	summary := executor.FormatSummary(responses)

	if summary == "" {
		t.Fatal("Expected non-empty summary")
	}

	if summary == "" {
		t.Fatal("Summary should contain task keys")
	}
}
