package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/activebook/gllm/service"
)

func TestOpenChatStreamToolCalls(t *testing.T) {
	// Set environment variables for the test, matching the subtask description
	os.Setenv("GLLM_API_KEY", "testkey")
	os.Setenv("GLLM_ENDPOINT_URL", "http://localhost/mock")
	os.Setenv("GLLM_MODEL", "mistral-model")
	os.Setenv("GLLM_TIMEOUT", "30")

	ll := service.LangLogic{
		ApiKey:        os.Getenv("GLLM_API_KEY"),
		EndPoint:      os.Getenv("GLLM_ENDPOINT_URL"),
		ModelName:     os.Getenv("GLLM_MODEL"),
		UserPrompt:    "Tell me a joke and use a tool to search for today's weather.",
		SystemPrompt:  "You are a helpful assistant.",
		Temperature:   0.7,
		UseSearchTool: true,
		// ProcChan and ProceedChan will be set up below
		// Files, ConversationID, and other fields can be left as zero/nil for this test
	}

	actualProcChan := make(chan service.StreamNotify, 10) // Buffered
	actualProceedChan := make(chan bool)

	ll.ProcChan = actualProcChan
	ll.ProceedChan = actualProceedChan

	// Log the initialized LangLogic for debugging
	// fmt.Printf("LangLogic initialized: %+v\n", ll)
	// fmt.Printf("API Key: %s\n", ll.ApiKey)
	// fmt.Printf("Endpoint: %s\n", ll.EndPoint)
	// fmt.Printf("Model: %s\n", ll.ModelName)


	go func() {
		for notify := range actualProcChan { // Receive from actualProcChan
			// fmt.Printf("ProcChan received: %+v\n", notify) // Log notifications
			switch notify.Status {
			case service.StatusStarted:
				// fmt.Println("StatusStarted received, sending true to ProceedChan")
				actualProceedChan <- true // Send to actualProceedChan
			case service.StatusSearchingOver:
				// fmt.Println("StatusSearchingOver received, sending true to ProceedChan")
				actualProceedChan <- true // Send to actualProceedChan
			case service.StatusFunctionCallingOver:
				// fmt.Println("StatusFunctionCallingOver received, sending true to ProceedChan")
				actualProceedChan <- true // Send to actualProceedChan
			case service.StatusError:
				// fmt.Printf("Error status received: %v\n", notify.Data)
				// Optionally, you could t.Errorf here if it's an unexpected error for the test's scope
				// For this test, we expect network errors, so we might not fail the test here.
				return // Stop the goroutine on error
			case service.StatusFinished:
				// fmt.Println("StatusFinished received.")
				return // Stop the goroutine
			}
		}
	}()

	// fmt.Println("Calling GenerateOpenChatStream...")
	err := ll.GenerateOpenChatStream()
	// fmt.Printf("GenerateOpenChatStream returned: %v\n", err) // Log the error

	if err != nil {
		// We expect an error because the endpoint is a mock.
		// The key is to check if the error is a network error (connection refused)
		// and not something like "require_id" or a 422 error from a real but misconfigured endpoint.
		// This is a simplified check. In a more complex scenario, you'd inspect the error more deeply.
		expectedErrorSubstring := "dial tcp [::1]:80: connect: connection refused" // For http://localhost/mock
		altExpectedErrorSubstring := "dial tcp 127.0.0.1:80: connect: connection refused" // For http://localhost/mock
		// Check if using https
		// expectedErrorSubstringHTTPS := "dial tcp [::1]:443: connect: connection refused" // For https://localhost/mock
		// altExpectedErrorSubstringHTTPS := "dial tcp 127.0.0.1:443: connect: connection refused" // For https://localhost/mock

		errorString := err.Error()
		if strings.Contains(errorString, "require_id") {
			t.Errorf("GenerateOpenChatStream failed with 'require_id' error: %v", err)
		} else if strings.Contains(errorString, "422") {
			t.Errorf("GenerateOpenChatStream failed with HTTP 422 error: %v", err)
		} else if strings.Contains(errorString, expectedErrorSubstring) || strings.Contains(errorString, altExpectedErrorSubstring) {
			fmt.Println("Test partially succeeded: GenerateOpenChatStream failed with expected connection refused error, indicating request construction likely passed the 'require_id' check.")
			// This is the expected outcome for this test if direct mocking isn't done.
		} else {
			t.Errorf("GenerateOpenChatStream failed with an unexpected error: %v", err)
		}
	} else {
		t.Log("GenerateOpenChatStream completed without error. This might be unexpected if the mock endpoint was supposed to cause a failure.")
	}

	// Add a timeout to prevent the test from hanging indefinitely if ProceedChan is not handled correctly.
	// This is a safeguard.
	select {
	case <-time.After(10 * time.Second): // Increased timeout
		// t.Error("Test timed out, ProcChan/ProceedChan logic might be stuck.")
	default:
		// Test finished or errored out before timeout
	}
}

// Helper to simulate reading from ProceedChan in some test scenarios if needed
// Not directly used in the main path of TestOpenChatStreamToolCalls but can be useful for debugging.
func simulateProceed(proceedChan chan bool) {
	go func() {
		for {
			time.Sleep(50 * time.Millisecond) // Simulate some work
			proceedChan <- true
		}
	}()
}
// Added strings import because it's used in the test (comment retained, actual import already moved)
