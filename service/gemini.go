package service

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"github.com/google/generative-ai-go/genai"
)

func (ll *LangLogic) GenerateGeminiStream() error {
	if !ll.UseSearchTool {
		return ll.geminiStream()
	} else {
		return ll.geminiStreamWithSearch()
	}
}

func (ll *LangLogic) getGeminiFilePart(file *FileData) genai.Part {

	mimeType := file.Format()
	data := file.Data()
	// Create appropriate part based on file type
	switch {
	case IsImageMIMEType(mimeType):
		// Handle image files
		return genai.ImageData(mimeType, data)
	case IsPDFMIMEType(mimeType):
		// Handle PDF files.
		return genai.Blob{MIMEType: mimeType, Data: data}
	case IsExcelMIMEType(mimeType):
		// Handle Excel files.
		return genai.Blob{MIMEType: mimeType, Data: data}
	default:
		// Default to plain text for other file types.
		return genai.Text(string(data))
	}
}

func (ll *LangLogic) geminiStream() error {
	// Setup the Gemini client
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(ll.ApiKey))
	if err != nil {
		ll.ProcChan <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("Failed to create client: %v", err)}
		return err
	}
	defer client.Close()

	// Create the model and generate content
	model := client.GenerativeModel(ll.ModelName)

	// Configure Model Parameters
	// System Instruction (System Prompt)
	if ll.SystemPrompt != "" {
		// For gemini, the system prompt is set as a user content
		model.SystemInstruction = genai.NewUserContent(genai.Text(ll.SystemPrompt))
	}
	model.SetTemperature(ll.Temperature)

	parts := []genai.Part{genai.Text(ll.UserPrompt)}
	for _, file := range ll.Files {
		// Check if the file data is empty
		if file != nil {
			// Convert the file data to a blob
			parts = append(parts, ll.getGeminiFilePart(file))
		}
	}

	// Signal that streaming has started
	ll.ProcChan <- StreamNotify{Status: StatusStarted}
	<-ll.ProceedChan // Wait for the main goroutine to tell sub-goroutine to proceed

	// Start a chat session
	cs := model.StartChat()

	// Load previous messages if any
	convo := GetGeminiConversation()
	err = convo.Load()
	if err != nil {
		ll.ProcChan <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("failed to load conversation: %v", err)}
		return err
	}
	cs.History = convo.History

	// Because gemini wouldn't show reasoning content, so we need to wait here
	ll.ProcChan <- StreamNotify{Status: StatusReasoning, Data: ""}

	iter := cs.SendMessageStream(ctx, parts...)
	//iter := model.GenerateContentStream(ctx, parts...)

	ll.ProcChan <- StreamNotify{Status: StatusReasoningOver, Data: ""}

	// Stream the responses
	for {
		resp, err := iter.Next()
		if err == iterator.Done {
			// Signal that streaming is complete
			break
		}

		if err != nil {
			ll.ProcChan <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("Generation error: %v", err)}
			return err
		}

		// Process and send content
		for _, candidate := range resp.Candidates {
			for _, part := range candidate.Content.Parts {
				if textPart, ok := part.(genai.Text); ok {
					ll.ProcChan <- StreamNotify{Status: StatusData, Data: string(textPart)}
				}
			}
		}
	}

	// Save the conversation history
	convo.History = cs.History
	err = convo.Save()
	if err != nil {
		return fmt.Errorf("failed to save conversation: %v", err)
	}
	ll.ProcChan <- StreamNotify{Status: StatusFinished}
	return err
}

/*
 * Search Engine
 *
 */

// Functions that start with lowercase letters (like printSection) are unexported and only visible within the same package.
// Functions that start with uppercase letters (like PrintSection) are exported and can be used by other packages that import your package.
// generateStreamText connects to the Google AI API and streams the generated text.

func (ll *LangLogic) getGeminiSearchTool() *genai.Tool {

	var searchTool *genai.Tool
	engine := GetSearchEngine()
	switch engine {
	case GoogleSearchEngine:
		// Use Google Search Engine
	case BingSearchEngine:
		// Use Bing Search Engine
	case TavilySearchEngine:
		// Use Tavily Search Engine
	case NoneSearchEngine:
		// Use None Search Engine
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

func (ll *LangLogic) callSearchFunction(fc *genai.FunctionCall) (map[string]any, error) {
	// Notify the stream of the function call
	Debugf("Function Calling: %s(%+v)\n", fc.Name, fc.Args)
	ll.ProcChan <- StreamNotify{Status: StatusFunctionCalling, Data: ""}

	// --- Execute Local Function ---
	var args struct {
		Query string `json:"query"`
	}
	// Marshal/Unmarshal Args (more robust error handling needed in production)
	argsJSON, _ := json.Marshal(fc.Args)
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		ll.ProcChan <- StreamNotify{Status: StatusFunctionCallingOver, Data: ""}
		<-ll.ProceedChan
		Warnf("Could not unmarshal function args: %v. Args: %+v", err, fc.Args)
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
	case NoneSearchEngine:
		// Use None Search Engine
		data, err = NoneSearch(args.Query)
	default:
	}
	if err != nil {
		ll.ProcChan <- StreamNotify{Status: StatusFunctionCallingOver, Data: ""}
		<-ll.ProceedChan
		Warnf("Performing search: %v", err)
		// TODO: Potentially send an error FunctionResponse back to the model
		return nil, fmt.Errorf("error performing search: %v", err)
	}
	ll.ProcChan <- StreamNotify{Status: StatusFunctionCallingOver, Data: ""}
	<-ll.ProceedChan
	return data, nil
}

// Helper struct to return results from stream processing
type streamProcessingResult struct {
	modelContent *genai.Content      // The complete content object from the model for this turn
	functionCall *genai.FunctionCall // Populated if a function call was found
	fullText     string              // Concatenated text parts
}

// A more concise version: Use only the ChatSession's history
func (ll *LangLogic) geminiStreamWithSearch() error {
	ctx := context.Background()

	// Initialize client
	client, err := genai.NewClient(ctx, option.WithAPIKey(ll.ApiKey))
	if err != nil {
		ll.ProcChan <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("Failed to create client: %v", err)}
		return err
	}
	defer client.Close()

	// Setup model & tools
	googleSearchTool := ll.getGeminiSearchTool()
	model := client.GenerativeModel(ll.ModelName)
	model.Tools = []*genai.Tool{googleSearchTool}

	if ll.SystemPrompt != "" {
		model.SystemInstruction = genai.NewUserContent(genai.Text(ll.SystemPrompt))
	}
	model.SetTemperature(ll.Temperature)

	// Prepare user message parts
	parts := []genai.Part{genai.Text(ll.UserPrompt)}
	for _, file := range ll.Files {
		if file != nil {
			parts = append(parts, ll.getGeminiFilePart(file))
		}
	}

	// Signal that streaming has started
	ll.ProcChan <- StreamNotify{Status: StatusStarted}
	<-ll.ProceedChan // Wait for the main goroutine to tell sub-goroutine to proceed

	// Start chat session and load previous history
	cs := model.StartChat()
	convo := GetGeminiConversation()
	err = convo.Load()
	if err != nil {
		ll.ProcChan <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("failed to load conversation: %v", err)}
		return err
	}
	cs.History = convo.History

	// Send the initial message
	references := make([]*map[string]interface{}, 0, 1)

	// Prepare user content
	userContent := &genai.Content{
		Parts: parts,
		Role:  "user",
	}

	contentParts := userContent.Parts
	// Process function calling loop - max 5 iterations
	for i := 0; i < 5; i++ {
		Debugf("Processing conversation at times: %d\n", i+1)

		// Add to chat session (don't need to track separately)
		ll.ProcChan <- StreamNotify{Status: StatusReasoning, Data: ""}
		resp := cs.SendMessageStream(ctx, contentParts...)
		ll.ProcChan <- StreamNotify{Status: StatusReasoningOver, Data: ""}

		// Process the stream
		result, err := ll.processStream(resp)
		if err != nil {
			ll.ProcChan <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("Generation error: %v", err)}
			return err
		}

		// Stread Done and No function call - we're done
		if result.functionCall == nil {
			break
		}

		// Handle function call
		fc := result.functionCall
		data, err := ll.callSearchFunction(fc)
		if err != nil {
			// continue
			//ll.ProcChan <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("Error calling function: %v", err)}
			//return err
		}

		// Track references
		references = append(references, &data)

		// Prepare function response
		functionResponsePart := &genai.FunctionResponse{
			Name:     fc.Name,
			Response: data,
		}

		// Send function response back through the chat session
		functionContent := &genai.Content{
			Parts: []genai.Part{functionResponsePart},
			Role:  "function",
		}
		contentParts = functionContent.Parts
	}

	// Add references to the output if any
	if len(references) > 0 {
		refs := "\n\n" + RetrieveReferences(references)
		ll.ProcChan <- StreamNotify{Status: StatusData, Data: refs}
	}

	// Save conversation history
	convo.History = cs.History
	if err := convo.Save(); err != nil {
		return fmt.Errorf("failed to save conversation: %v", err)
	}

	ll.ProcChan <- StreamNotify{Status: StatusFinished}
	return nil
}

// Helper function to process stream
func (ll *LangLogic) processStream(iter *genai.GenerateContentResponseIterator) (*streamProcessingResult, error) {
	result := &streamProcessingResult{
		modelContent: &genai.Content{Role: "model", Parts: []genai.Part{}},
	}
	var accumulatedText string

	for {
		resp, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			Infof("Stream error: %v", err.Error())
			return nil, fmt.Errorf("stream error: %v", err)
		}
		if resp == nil || len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
			continue
		}

		candidateContent := resp.Candidates[0].Content
		for _, part := range candidateContent.Parts {
			if textPart, ok := part.(genai.Text); ok {
				if textPart != "" {
					txt := string(textPart)
					ll.ProcChan <- StreamNotify{Status: StatusData, Data: txt}
					accumulatedText += txt
				}
			} else if fcPart, ok := part.(genai.FunctionCall); ok {
				if result.functionCall == nil && fcPart.Name == "web_search" {
					result.functionCall = &fcPart
				}
			}
		}
	}

	if accumulatedText != "" {
		result.modelContent.Parts = append(result.modelContent.Parts, genai.Text(accumulatedText))
		result.fullText = accumulatedText
	}
	return result, nil
}
