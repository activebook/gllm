package service

import (
	"fmt"
)

const (
	// Terminal colors
	inReasoningColor = "\033[90m" // Bright Black
	inCallingColor   = "\033[36m" // Cyan
	completeColor    = "\033[32m" // Green
)

type StreamDataType int

const (
	DataTypeUnknown   StreamDataType = iota
	DataTypeNormal                   // 1
	DataTypeReasoning                // 2
	DataTypeFinished                 // 3
)

type StreamData struct {
	Text string
	Type StreamDataType
}

type Agent struct {
	ApiKey        string
	EndPoint      string
	ModelName     string
	SystemPrompt  string
	UserPrompt    string
	Temperature   float32
	Files         []*FileData         // Attachment files
	NotifyChan    chan<- StreamNotify // Sub Channel to send notifications
	DataChan      chan<- StreamData   // Sub Channel to receive streamed text data
	ProceedChan   <-chan bool         // Sub Channel to receive proceed signal
	SearchEngine  SearchEngine        // Search engine name
	UseSearchTool bool                // Use search tool
	UseTools      bool                // Use tools
	UseCodeTool   bool                // Use code tool
	MaxRecursions int                 // Maximum number of recursions for model calls
	Status        StatusStack         // Stack to manage streaming status
}

func constructSearchEngine(searchEngine *map[string]any) *SearchEngine {
	se := SearchEngine{}
	se.UseSearch = false
	if searchEngine != nil {
		se.UseSearch = true
		if name, ok := (*searchEngine)["name"]; ok {
			se.Name = name.(string)
		} else {
			se.UseSearch = false
			se.Name = ""
		}
		if keyValue, ok := (*searchEngine)["key"]; ok {
			se.ApiKey = keyValue.(string)
		} else {
			se.UseSearch = false
			se.ApiKey = ""
		}
		if cxValue, ok := (*searchEngine)["cx"]; ok {
			se.CxKey = cxValue.(string)
		} else {
			se.CxKey = ""
		}
		if deepDive, ok := (*searchEngine)["deep_dive"]; ok {
			se.DeepDive = deepDive.(bool)
		} else {
			se.DeepDive = false
		}
		if references, ok := (*searchEngine)["references"]; ok {
			se.MaxReferences = references.(int)
		} else {
			se.MaxReferences = 5
		}
	}

	return &se
}

func CallAgent(prompt string, sys_prompt string, files []*FileData, modelInfo map[string]any, searchEngine map[string]any, useTools bool, maxRecursions int) {
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
	se := constructSearchEngine(&searchEngine)

	// Set up code tool settings
	exeCode := IsCodeExecutionEnabled()

	// Create a channel to receive notifications
	notifyCh := make(chan StreamNotify, 10) // Buffer to prevent blocking(used for status updates)
	dataCh := make(chan StreamData, 10)     // Buffer to prevent blocking(used for streamed text data)
	proceedCh := make(chan bool)            // For main -> sub communication

	markdownRenderer := NewMarkdownRenderer()
	ll := Agent{
		ApiKey:        modelInfo["key"].(string),
		EndPoint:      modelInfo["endpoint"].(string),
		ModelName:     modelInfo["model"].(string),
		SystemPrompt:  sys_prompt,
		UserPrompt:    prompt,
		Temperature:   temperature,
		Files:         files,
		NotifyChan:    notifyCh,
		DataChan:      dataCh,
		ProceedChan:   proceedCh,
		SearchEngine:  *se,
		UseSearchTool: se.UseSearch,
		UseTools:      useTools,
		UseCodeTool:   exeCode,
		MaxRecursions: maxRecursions,
		Status:        StatusStack{},
	}

	// Check if the endpoint is compatible with OpenAI
	provider := DetectModelProvider(ll.EndPoint)

	spinner := NewSpinner("Processing...")

	// Start the generation in a goroutine
	go func() {
		defer func() {
			// Recover from panics and convert them to errors
			if r := recover(); r != nil {
				errMsg := fmt.Sprintf("Panic occurred: %v", r)
				notifyCh <- StreamNotify{Status: StatusError, Data: errMsg}
			}
		}()

		switch provider {
		case ModelOpenAICompatible:
			if err := ll.GenerateOpenChatStream(); err != nil {
				//Errorf("Stream error: %v\n", err)
				notifyCh <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("%v", err)}
			}
		case ModelGemini:
			if err := ll.GenerateGemini2Stream(); err != nil {
				//Errorf("Stream error: %v\n", err)
				notifyCh <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("%v", err)}
			}
		default:
			notifyCh <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("Unsupported model provider: %s", provider)}
		}
	}()

	defer close(notifyCh)
	defer close(dataCh)
	defer close(proceedCh)

	// Process notifications in the main thread
	// listen on multiple channels in Go, it listens to them simultaneously.
	// If both data channel and notification channel have something to be read at the same time,
	// it is indeed possible for either one to be selected.
	for {
		select {

		// Handle streamed text data
		case data := <-dataCh:
			switch data.Type {
			case DataTypeNormal:
				// Render the streamed text and save to markdown buffer
				markdownRenderer.RenderString("%s", data.Text)
			case DataTypeReasoning:
				// Reasoning data don't need to be saved to markdown buffer
				fmt.Print(inReasoningColor + data.Text) // Print the streamed text
			case DataTypeFinished:
				// Wait all data to be processed(flush)
				// This is important, otherwise notify will be processed before data finished
				proceedCh <- true
			default:
				// Handle other data types if needed
			}

		// Handle status notifications
		case notify := <-notifyCh:

			switch notify.Status {
			case StatusProcessing:
				StartSpinner(spinner, "Processing...")
			case StatusStarted:
				StopSpinner(spinner)
				proceedCh <- true
			case StatusError:
				StopSpinner(spinner)
				Errorf("Stream error: %v\n", notify.Data)
				return
			case StatusFinished:
				StopSpinner(spinner)
				// Render the markdown
				markdownRenderer.RenderMarkdown()
				// Render the token usage
				RenderTokenUsage()
				return // Exit when stream is done
			case StatusReasoning:
				StopSpinner(spinner)
				// Start with in-progress color
				fmt.Println(completeColor + "Thinking ↓")
			case StatusReasoningOver:
				// Switch to complete color at the end
				fmt.Print(resetColor)
				fmt.Printf(completeColor + "✓" + resetColor)
				fmt.Println()
			case StatusFunctionCalling:
				fmt.Print(resetColor)
				fmt.Println()
				fmt.Print(inCallingColor + notify.Data + resetColor) // Print the function call message
				fmt.Println()
				StartSpinner(spinner, "Function Calling...")
			case StatusFunctionCallingOver:
				StopSpinner(spinner)
				proceedCh <- true
			}
		}
	}

}
