package service

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"github.com/google/generative-ai-go/genai"
)

func generateGeminiStreamChan(apiKey, modelName, systemPrompt, userPrompt string, temperature float32, images []*ImageData) error {
	// Setup the Gemini client
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		proc <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("Failed to create client: %v", err)}
		return err
	}
	defer client.Close()

	// Create the model and generate content
	model := client.GenerativeModel(modelName)

	// Configure Model Parameters
	// System Instruction (System Prompt)
	if systemPrompt != "" {
		model.SystemInstruction = genai.NewUserContent(genai.Text(systemPrompt))
	}
	model.SetTemperature(temperature)

	parts := []genai.Part{genai.Text(userPrompt)}
	for _, imageData := range images {
		// Check if the image data is empty
		if imageData != nil {
			// Convert the image data to a blob
			parts = append(parts, genai.ImageData(imageData.Format(), imageData.Data()))
		}
	}

	// Signal that streaming has started
	proc <- StreamNotify{Status: StatusStarted}

	// Because gemini wouldn't show reasoning content, so we need to wait here
	proc <- StreamNotify{Status: StatusReasoning, Data: ""}

	iter := model.GenerateContentStream(ctx, parts...)

	proc <- StreamNotify{Status: StatusReasoningOver, Data: ""}

	// Stream the responses
	for {
		resp, err := iter.Next()
		if err == iterator.Done {
			// Signal that streaming is complete
			proc <- StreamNotify{Status: StatusFinished}
			return nil
		}

		if err != nil {
			proc <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("Generation error: %v", err)}
			return err
		}

		// Process and send content
		for _, candidate := range resp.Candidates {
			for _, part := range candidate.Content.Parts {
				if textPart, ok := part.(genai.Text); ok {
					proc <- StreamNotify{Status: StatusData, Data: string(textPart)}
				}
			}
		}
	}
}

/*
 * Search Engine
 *
 */

// Functions that start with lowercase letters (like printSection) are unexported and only visible within the same package.
// Functions that start with uppercase letters (like PrintSection) are exported and can be used by other packages that import your package.
// generateStreamText connects to the Google AI API and streams the generated text.

func GenerateGeminiStreamWithSearchChan(apiKey, modelName, systemPrompt, userPrompt string, temperature float32, images []*ImageData) error {
	ctx := context.Background()

	// Initialize Gemini client
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		proc <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("Failed to create client: %v", err)}
		return err
	}
	defer client.Close()

	// Define the Google Search Tool
	googleSearchTool := getGeminiSearchTool()

	// Set up model with function calling capability
	model := client.GenerativeModel(modelName) // Use a model known for tool use
	model.Tools = []*genai.Tool{googleSearchTool}

	// Configure Model Parameters
	// System Instruction (System Prompt)
	if systemPrompt != "" {
		model.SystemInstruction = genai.NewUserContent(genai.Text(systemPrompt))
	}
	model.SetTemperature(temperature)

	parts := []genai.Part{genai.Text(userPrompt)}
	for _, imageData := range images {
		// Check if the image data is empty
		if imageData != nil {
			// Convert the image data to a blob
			parts = append(parts, genai.ImageData(imageData.Format(), imageData.Data()))
		}
	}

	// --- Conversation Loop ---
	history := []*genai.Content{} // Start with empty history

	// Add initial user prompt to history
	history = append(history, &genai.Content{
		Parts: parts,
		Role:  "user",
	})

	// Signal that streaming has started
	proc <- StreamNotify{Status: StatusStarted}

	// keep track of the references
	var references map[string]interface{}
	// only do 3 times of function calling
	for range 3 {
		resp, err := generateAndProcessStream(ctx, model, history)
		if err != nil {
			proc <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("Generation error: %v", err)}
			return err
		}
		// if len(functionResultData) > 0 {
		// 	printAllLinks(functionResultData)
		// 	functionResultData = nil
		// }
		if resp == nil {
			break
		}
		// First add just the text response (if any)
		if len(resp.modelContent.Parts) > 0 {
			history = append(history, resp.modelContent)
		}
		// Check if a function call was requested in the first response
		if resp.functionCall != nil {
			// Add the complete model response from the first stream to history
			fc := resp.functionCall
			data, err := callSearchFunction(fc)
			if err != nil {
				proc <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("Error calling function: %v", err)}
				return err
			}
			// Link the references
			references = data

			// --- Prepare Function Response ---
			functionResponsePart := &genai.FunctionResponse{
				Name:     fc.Name,
				Response: data,
			}

			// Add function response to history
			history = append(history, &genai.Content{
				Parts: []genai.Part{functionResponsePart},
				Role:  "function", // Role "function" is conventional for tool results
			})

		} else {
			// No function call and no model content
			break
		}
	}
	if references != nil {
		refs := "\n\n" + RetrieveReferences(references)
		proc <- StreamNotify{Status: StatusData, Data: refs}
	}
	// Signal that streaming is complete
	proc <- StreamNotify{Status: StatusFinished}
	return nil
}

func getGeminiSearchTool() *genai.Tool {

	var searchTool *genai.Tool
	engine := GetSearchEngine()
	switch engine {
	case GoogleSearchEngine:
		// Use Google Search Engine
	case BingSearchEngine:
		// Use Bing Search Engine
	case TavilySearchEngine:
		// Use Tavily Search Engine
	default:

	}
	// All use the same search tool
	searchTool = &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{{
			Name:        "web_search",
			Description: "Retrieve the most relevant and up-to-date information from the web.",
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"query": {
						Type:        genai.TypeString,
						Description: "The search term or question to find information about.",
					},
				},
				Required: []string{"query"},
			},
		}},
	}
	return searchTool
}

func callSearchFunction(fc *genai.FunctionCall) (map[string]any, error) {
	// Notify the stream of the function call
	Debugf("Function Calling: %s(%+v)\n", fc.Name, fc.Args)
	proc <- StreamNotify{Status: StatusFunctionCalling, Data: ""}

	// --- Execute Local Function ---
	var args struct {
		Query string `json:"query"`
	}
	// Marshal/Unmarshal Args (more robust error handling needed in production)
	argsJSON, _ := json.Marshal(fc.Args)
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		proc <- StreamNotify{Status: StatusFunctionCallingOver, Data: ""}
		Logf("Warning: Could not unmarshal function args: %v. Args: %+v", err, fc.Args)
		return nil, fmt.Errorf("could not unmarshal function args: %v", err)
	}

	// Call your actual function
	engine := GetSearchEngine()
	var data map[string]any
	var err error
	switch engine {
	case GoogleSearchEngine:
		// Use Google Search Engine
		data, err = GoogleSearch(args.Query)
	case BingSearchEngine:
		// Use Bing Search Engine
		data, err = BingSearch(args.Query)
	case TavilySearchEngine:
		// Use Tavily Search Engine
		data, err = TavilySearch(args.Query)
	default:
	}
	if err != nil {
		proc <- StreamNotify{Status: StatusFunctionCallingOver, Data: ""}
		Logf("Error performing search: %v", err)
		// TODO: Potentially send an error FunctionResponse back to the model
		return nil, fmt.Errorf("error performing search: %v", err)
	}
	proc <- StreamNotify{Status: StatusFunctionCallingOver, Data: ""}
	return data, nil
}

func extractAllParts(history []*genai.Content) []genai.Part {
	// Flatten the history []*Content into a single []Part slice
	allParts := []genai.Part{} // Use the actual Part type
	for _, content := range history {
		// Ensure content and content.Parts are not nil before appending
		if content != nil && len(content.Parts) > 0 {
			allParts = append(allParts, content.Parts...) // Append all parts from this content item
		}
	}
	return allParts
}

// Helper struct to return results from stream processing
type streamProcessingResult struct {
	modelContent *genai.Content      // The complete content object from the model for this turn
	functionCall *genai.FunctionCall // Populated if a function call was found
	fullText     string              // Concatenated text parts
}

// Refactored function to handle one stream generation and processing call
func generateAndProcessStream(ctx context.Context, model *genai.GenerativeModel, history []*genai.Content) (*streamProcessingResult, error) {
	allParts := extractAllParts(history)

	// Because gemini wouldn't show reasoning content, so we need to wait here
	proc <- StreamNotify{Status: StatusReasoning, Data: ""}
	iter := model.GenerateContentStream(ctx, allParts...)
	proc <- StreamNotify{Status: StatusReasoningOver, Data: ""}

	result := &streamProcessingResult{
		modelContent: &genai.Content{Role: "model", Parts: []genai.Part{}}, // Initialize model content for this turn
	}
	var accumulatedText string

	for {
		resp, err := iter.Next()
		if err == iterator.Done {
			// stream complete
			break
		}
		if err != nil {
			Logf("Stream error: %v", err.Error())
			return nil, fmt.Errorf("stream error: %v", err)
		}
		if resp == nil || len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
			continue // Skip empty responses
		}

		// Process parts from the first candidate
		candidateContent := resp.Candidates[0].Content
		for _, part := range candidateContent.Parts {
			if textPart, ok := part.(genai.Text); ok {
				// Append part to the turn's content
				if textPart != "" {
					txt := string(textPart)
					proc <- StreamNotify{Status: StatusData, Data: txt}
					accumulatedText += txt
				}
			} else if fcPart, ok := part.(genai.FunctionCall); ok {
				// Store the first function call encountered. Gemini API usually sends one per turn.
				// Must check the function name, some models make up function name
				if result.functionCall == nil && fcPart.Name == "web_search" {
					result.functionCall = &fcPart // Store the pointer directly
					// No need to print details here, we'll do it after the loop
				}
				// We still append the FunctionCall part to modelContent.Parts
				// result.modelContent.Parts = append(result.modelContent.Parts, part)
				// This will pop up bug:
				// Please ensure that function call turn contains at least one function_call part which can not be mixed with function_response parts.
				// This is because we are adding FunctionCall part to modelContent.Parts
				// function_call parts and function_response parts cannot be mixed in the same turn.
			}
			// Handle other part types if necessary
		}
	}
	if accumulatedText != "" {
		// If the part is not empty, add it to the accumulated text
		result.modelContent.Parts = append(result.modelContent.Parts, genai.Text(accumulatedText))
		result.fullText = accumulatedText
	}
	return result, nil // Signal to continue processing
}
