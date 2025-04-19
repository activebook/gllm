package service

import (
	"context"
	"fmt"

	"google.golang.org/genai"
)

func (ll *LangLogic) GenerateGemini2Stream() error {
	return ll.gemini2Stream()
}

func (ll *LangLogic) getGemini2FilePart(file *FileData) genai.Part {

	mimeType := file.Format()
	data := file.Data()
	// Create appropriate part based on file type
	switch {
	case IsImageMIMEType(mimeType):
		// Handle image files
		return genai.Part{InlineData: &genai.Blob{Data: data, MIMEType: mimeType}}
	case IsPDFMIMEType(mimeType):
		// Handle PDF files.
		return genai.Part{InlineData: &genai.Blob{Data: data, MIMEType: mimeType}}
	case IsExcelMIMEType(mimeType):
		// Handle Excel files.
		return genai.Part{InlineData: &genai.Blob{Data: data, MIMEType: mimeType}}
	case IsAudioMIMEType(mimeType):
		// Handle audio files.
		return genai.Part{InlineData: &genai.Blob{Data: data, MIMEType: mimeType}}
	case IsTextMIMEType(mimeType):
		// Handle plain text files.
		return genai.Part{Text: string(data)}
	default:
		// Default to plain text for other file types.
		text := string(data)
		if len(text) > 0 {
			return genai.Part{Text: text}
		} else {
			return genai.Part{}
		}
	}
}

func (ll *LangLogic) gemini2Stream() error {
	// Setup the Gemini client
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  ll.ApiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		ll.ProcChan <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("Failed to create client: %v", err)}
		return err
	}

	parts := []genai.Part{{Text: ll.UserPrompt}}
	for _, file := range ll.Files {
		// Check if the file data is empty
		if file != nil {
			// Convert the file data to a blob
			part := ll.getGemini2FilePart(file)
			if part.Text != "" || part.InlineData != nil {
				parts = append(parts, part)
			}
		}
	}

	// Load previous messages if any
	convo := GetGemini2Conversation()
	err = convo.Load()
	if err != nil {
		ll.ProcChan <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("failed to load conversation: %v", err)}
		return err
	}

	// Create the model and generate content
	// Configure Model Parameters
	config := genai.GenerateContentConfig{
		Temperature: &ll.Temperature,
		ThinkingConfig: &genai.ThinkingConfig{
			// Let model decide how to allocate tokens
			//ThinkingBudget:  genai.Ptr[int32](8000),
			IncludeThoughts: true,
		},
		Tools: []*genai.Tool{
			// Placeholder
			//{CodeExecution: &genai.ToolCodeExecution{}},
			//{GoogleSearch: &genai.GoogleSearch{}},
		},
	}
	// System Instruction (System Prompt)
	if ll.SystemPrompt != "" {
		config.SystemInstruction = &genai.Content{Parts: []*genai.Part{{Text: ll.SystemPrompt}}}
	}
	// Tools
	// Remember: CodeExecution and GoogleSearch cannot be enabled at the same time
	if ll.UseSearchTool {
		config.Tools = append(config.Tools, &genai.Tool{GoogleSearch: &genai.GoogleSearch{}})
	}
	if ll.UseCodeTool && !ll.UseSearchTool {
		config.Tools = append(config.Tools, &genai.Tool{CodeExecution: &genai.ToolCodeExecution{}})
	}

	// Create a chat session
	chat, err := client.Chats.Create(ctx, ll.ModelName, &config, convo.History)
	if err != nil {
		ll.ProcChan <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("Failed to create chat: %v", err)}
		return err
	}

	// Stream the response
	iter := chat.SendMessageStream(ctx, parts...)

	// Signal that streaming has started
	ll.ProcChan <- StreamNotify{Status: StatusStarted}
	<-ll.ProceedChan // Wait for the main goroutine to tell sub-goroutine to proceed

	// Stream the responses
	references := make([]*map[string]interface{}, 0, 1)
	queries := make([]string, 0, 1)
	state := stateNormal
	for resp, err := range iter {
		if err != nil {
			ll.ProcChan <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("Generation error: %v", err)}
			return err
		}

		// Process and send content
		for _, candidate := range resp.Candidates {
			for _, part := range candidate.Content.Parts {
				// State transitions
				switch state {
				case stateNormal:
					if part.Thought {
						ll.ProcChan <- StreamNotify{Status: StatusReasoning, Data: ""}
						state = stateReasoning
					}
				case stateReasoning:
					if !part.Thought {
						ll.ProcChan <- StreamNotify{Status: StatusReasoningOver, Data: ""}
						state = stateNormal
					}
				}

				// Actual data
				if part.Thought && part.Text != "" {
					ll.ProcChan <- StreamNotify{Status: StatusReasoningData, Data: part.Text}
				} else if part.Text != "" {
					ll.ProcChan <- StreamNotify{Status: StatusData, Data: part.Text}
				}
			}

			// Add references to the output if any
			if candidate.GroundingMetadata != nil {
				appendReferences(candidate.GroundingMetadata, &references)
				queries = append(queries, candidate.GroundingMetadata.WebSearchQueries...)
			}
		}
	}

	// Add queries to the output if any
	if len(queries) > 0 {
		q := "\n\n" + RetrieveQueries(queries)
		ll.ProcChan <- StreamNotify{Status: StatusData, Data: q}
	}
	// Add references to the output if any
	if len(references) > 0 {
		refs := "\n\n" + RetrieveReferences(references)
		ll.ProcChan <- StreamNotify{Status: StatusData, Data: refs}
	}
	ll.ProcChan <- StreamNotify{Status: StatusData, Data: "\n"}

	// Save the conversation history
	convo.History = chat.History(false)
	err = convo.Save()
	if err != nil {
		return fmt.Errorf("failed to save conversation: %v", err)
	}
	ll.ProcChan <- StreamNotify{Status: StatusFinished}
	return err
}

func appendReferences(metadata *genai.GroundingMetadata, refs *[]*map[string]interface{}) {
	if len(metadata.GroundingChunks) > 0 {
		// Build a single map with a "results" key as expected by RetrieveReferences
		results := make([]any, 0, len(metadata.GroundingChunks))
		for _, chunk := range metadata.GroundingChunks {
			results = append(results, map[string]any{
				"title":       chunk.Web.Title,
				"link":        chunk.Web.URI,
				"displayLink": chunk.Web.Title,
			})
		}
		// Track references
		*refs = append(*refs, &map[string]any{"results": results})
	}
}
