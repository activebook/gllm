package service

import "fmt"

const (
	// Terminal colors
	inProgressColor = "\033[90m" // Bright Black
	completeColor   = "\033[32m" // Green
)

type LangLogic struct {
	ApiKey        string
	EndPoint      string
	ModelName     string
	SystemPrompt  string
	UserPrompt    string
	Temperature   float32
	Files         []*FileData         // Attachment files
	ProcChan      chan<- StreamNotify // Sub Channel to send notifications
	ProceedChan   <-chan bool         // Sub Channel to receive proceed signal
	UseSearchTool bool                // Use search tool
	UseCodeTool   bool                // Use code tool
}

func CallLanguageModel(prompt string, sys_prompt string, files []*FileData, modelInfo map[string]any, searchEngine map[string]any) {
	var temperature float32
	switch temp := modelInfo["temperature"].(type) {
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

	// Set up search engine settings
	useSearch := false
	if searchEngine != nil {
		useSearch = true
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
	}

	// Set up code tool settings
	exeCode := IsCodeExecutionEnabled()

	// Create a channel to receive notifications
	notifyCh := make(chan StreamNotify, 10) // Buffer to prevent blocking
	proceedCh := make(chan bool)            // For main -> sub communication

	markdownRenderer := NewMarkdownRenderer()
	ll := LangLogic{
		ApiKey:        modelInfo["key"].(string),
		EndPoint:      modelInfo["endpoint"].(string),
		ModelName:     modelInfo["model"].(string),
		SystemPrompt:  sys_prompt,
		UserPrompt:    prompt,
		Temperature:   temperature,
		Files:         files,
		ProcChan:      notifyCh,
		ProceedChan:   proceedCh,
		UseSearchTool: useSearch,
		UseCodeTool:   exeCode,
	}

	// Check if the endpoint is compatible with OpenAI
	provider := DetectModelProvider(ll.EndPoint)

	spinner := NewSpinner("Processing...")

	// Start the generation in a goroutine
	go func() {
		switch provider {
		case ModelOpenAICompatible:
			if err := ll.GenerateOpenChatStream(); err != nil {
				Errorf("Stream error: %v\n", err)
			}
		case ModelGemini:
			if err := ll.GenerateGemini2Stream(); err != nil {
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
			proceedCh <- true
		case StatusData:
			// Render the streamed text and save to markdown buffer
			markdownRenderer.RenderString("%s", notify.Data)
		case StatusReasoningData:
			// Reasoning data don't need to be saved to markdown buffer
			fmt.Print(notify.Data) // Print the streamed text
		case StatusError:
			StopSpinner(spinner)
			Errorf("Stream error: %v\n", notify.Data)
			return
		case StatusFinished:
			StopSpinner(spinner)
			// Render the markdown
			markdownRenderer.RenderMarkdown()
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
		case StatusSearching:
			StartSpinner(spinner, "Searching...")
		case StatusSearchingOver:
			StopSpinner(spinner)
			proceedCh <- true
		case StatusFunctionCalling:
			StartSpinner(spinner, "Fucntion Calling...")
		case StatusFunctionCallingOver:
			StopSpinner(spinner)
			proceedCh <- true
		}
	}
}
