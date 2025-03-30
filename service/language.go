package service

import (
	"fmt"
	"strings"
)

func CallLanguageModel(prompt string, sys_prompt string, images []*ImageData, model map[string]any) {

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

	//generateOpenAIStream(apiKey, endPoint, modelName, systemPrompt, userPrompt, temperature)
	//generateGeminiStream(apiKey, modelName, systemPrompt, userPrompt, temperature)

	// Create a channel to receive notifications
	notifyCh := make(chan StreamNotify, 10) // Buffer to prevent blocking
	// Start the generation in a goroutine
	go func() {
		if openaiCompatible {
			if err := generateOpenAIStreamChan(apiKey, endPoint, modelName, systemPrompt, userPrompt, temperature, images, notifyCh); err != nil {
				fmt.Printf("Stream error: %v\n", err)
			}
		} else {
			if err := generateGeminiStreamChan(apiKey, modelName, systemPrompt, userPrompt, temperature, images, notifyCh); err != nil {
				fmt.Printf("Stream error: %v\n", err)
			}
		}
	}()
	// Process notifications in the main thread
	for notify := range notifyCh {
		switch notify.Status {
		case StatusStarted:
			//fmt.Println("Stream started")
		case StatusData:
			fmt.Print(notify.Data) // Print the streamed text
		case StatusError:
			//fmt.Printf("Error: %s\n", notify.Data)
			return
		case StatusFinished:
			//fmt.Println("\nStream finished")
			return // Exit when stream is done
		}
	}
}
