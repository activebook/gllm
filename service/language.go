package service

import (
	"fmt"
	"strings"
)

// Create a channel to receive notifications
// Shared by gemini.go and openchat.go
var (
	proc chan<- StreamNotify
)

const (
	// Terminal colors
	inProgressColor = "\033[90m" // Bright Black
	completeColor   = "\033[32m" // Green
)

func CallLanguageModel(prompt string, sys_prompt string, files []*FileData, model map[string]any) {

	apiKey := model["key"].(string)
	endPoint := model["endpoint"].(string)
	modelName := model["model"].(string)
	systemPrompt := sys_prompt
	userPrompt := prompt

	var temperature float32
	switch temp := model["temperature"].(type) {
	case float64:
		temperature = float32(temp)
	case int:
		temperature = float32(temp)
	case int64:
		temperature = float32(temp)
	case float32:
		temperature = temp
	default:
		// Set a default value if type is unexpected
		temperature = 0.7 // or whatever default makes sense
	}

	var openaiCompatible bool = true
	domains := []string{"googleapis.com", "google.com"}
	for _, domain := range domains {
		if strings.Contains(endPoint, domain) {
			openaiCompatible = false
			break
		}
	}

	spinner := NewSpinner("Processing...")

	// Create a channel to receive notifications
	notifyCh := make(chan StreamNotify, 10) // Buffer to prevent blocking
	proc = notifyCh
	// Start the generation in a goroutine
	go func() {
		if openaiCompatible {
			//if err := generateOpenAIStreamChan(apiKey, endPoint, modelName, systemPrompt, userPrompt, temperature, files); err != nil {
			if err := generateVolcStreamChan(apiKey, endPoint, modelName, systemPrompt, userPrompt, temperature, files); err != nil {
				Errorf("Stream error: %v\n", err)
			}
		} else {
			if err := generateGeminiStreamChan(apiKey, modelName, systemPrompt, userPrompt, temperature, files); err != nil {
				Errorf("Stream error: %v\n", err)
			}
		}
	}()

	// Process notifications in the main thread
	for notify := range notifyCh {
		switch notify.Status {
		case StatusProcessing:
			StartSpinner(spinner, "Processing...")
		case StatusStarted:
			StopSpinner(spinner)
		case StatusData:
			fmt.Print(notify.Data) // Print the streamed text
		case StatusError:
			StopSpinner(spinner)
			return
		case StatusFinished:
			StopSpinner(spinner)
			return // Exit when stream is done
		case StatusReasoning:
			StopSpinner(spinner)
			// Start with in-progress color
			fmt.Println(completeColor + "Thinking ↓" + inProgressColor)
		case StatusReasoningOver:
			// Switch to complete color at the end
			fmt.Print(resetColor)
			fmt.Print(completeColor + "✓" + resetColor)
			fmt.Println()
		}
	}
}

func CallLanguageModelRag(prompt string, sys_prompt string, files []*FileData, model map[string]any, searchEngine map[string]any) {

	apiKey := model["key"].(string)
	endPoint := model["endpoint"].(string)
	modelName := model["model"].(string)
	systemPrompt := sys_prompt
	userPrompt := prompt

	if name, ok := searchEngine["name"]; ok {
		SetSearchEngine(name.(string))
	} else {
		SetSearchEngine("")
	}
	if keyValue, ok := searchEngine["key"]; ok {
		SetSearchApiKey(keyValue.(string))
	} else {
		SetSearchApiKey("")
	}
	if cxValue, ok := searchEngine["cx"]; ok {
		SetSearchCxKey(cxValue.(string))
	} else {
		SetSearchCxKey("")
	}

	var temperature float32
	switch temp := model["temperature"].(type) {
	case float64:
		temperature = float32(temp)
	case int:
		temperature = float32(temp)
	case int64:
		temperature = float32(temp)
	case float32:
		temperature = temp
	default:
		// Set a default value if type is unexpected
		temperature = 0.7 // or whatever default makes sense
	}

	var openaiCompatible bool = true
	domains := []string{"googleapis.com", "google.com"}
	for _, domain := range domains {
		if strings.Contains(endPoint, domain) {
			openaiCompatible = false
			break
		}
	}

	spinner := NewSpinner("Processing...")

	// Create a channel to receive notifications
	notifyCh := make(chan StreamNotify, 10) // Buffer to prevent blocking
	proc = notifyCh
	// Start the generation in a goroutine
	go func() {
		if openaiCompatible {
			//if err := generateOpenAIStreamWithSearchChan(apiKey, endPoint, modelName, systemPrompt, userPrompt, temperature, files); err != nil {
			if err := generateVolcStreamWithSearchChan(apiKey, endPoint, modelName, systemPrompt, userPrompt, temperature, files); err != nil {
				Errorf("Stream error: %v\n", err)
			}
		} else {
			if err := GenerateGeminiStreamWithSearchChan(apiKey, modelName, systemPrompt, userPrompt, temperature, files); err != nil {
				Errorf("Stream error: %v\n", err)
			}
		}
	}()
	// Process notifications in the main thread
	for notify := range notifyCh {
		switch notify.Status {
		case StatusProcessing:
			StartSpinner(spinner, "Processing...")
		case StatusStarted:
			StopSpinner(spinner)
		case StatusData:
			fmt.Print(notify.Data) // Print the streamed text
		case StatusError:
			StopSpinner(spinner)
			return
		case StatusFinished:
			StopSpinner(spinner)
			return // Exit when stream is done
		case StatusReasoning:
			StopSpinner(spinner)
			// Start with in-progress color
			fmt.Println(completeColor + "Thinking ↓" + inProgressColor)
		case StatusReasoningOver:
			// Switch to complete color at the end
			fmt.Print(resetColor)
			fmt.Print(completeColor + "✓" + resetColor)
			fmt.Println()
		case StatusFunctionCalling:
			StartSpinner(spinner, "Searching...")
		case StatusFunctionCallingOver:
			StopSpinner(spinner)
		}
	}
}
