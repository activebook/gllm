package service

import (
	"context"
	"fmt"

	"google.golang.org/genai"
)

func (ag *Agent) getGemini2FilePart(file *FileData) genai.Part {

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

func (ag *Agent) GenerateGemini2Stream() error {
	// Setup the Gemini client
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  ag.ApiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusError, Data: fmt.Sprintf("Failed to create client: %v", err)}, nil)
		return err
	}

	parts := []genai.Part{{Text: ag.UserPrompt}}
	for _, file := range ag.Files {
		// Check if the file data is empty
		if file != nil {
			// Convert the file data to a blob
			part := ag.getGemini2FilePart(file)
			if part.Text != "" || part.InlineData != nil {
				parts = append(parts, part)
			}
		}
	}

	// Load previous messages if any
	convo := GetGemini2Conversation()
	err = convo.Load()
	if err != nil {
		ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusError, Data: fmt.Sprintf("failed to load conversation: %v", err)}, nil)
		return err
	}

	// Create the model and generate content
	// Configure Model Parameters
	config := genai.GenerateContentConfig{
		Temperature: &ag.Temperature,
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
	if ag.SystemPrompt != "" {
		config.SystemInstruction = &genai.Content{Parts: []*genai.Part{{Text: ag.SystemPrompt}}}
	}

	// - If UseTools is true, enable the embedding tools.
	// - Else if UseSearchTool is true, enable Google Search.
	// - Else if UseCodeTool is true, enable code execution.
	// CodeExecution and GoogleSearch cannot be enabled simultaneously.
	if ag.ToolsUse.Enable {
		// load embedding Tools
		config.Tools = append(config.Tools, ag.getGemini2Tools()...)
		if ag.SearchEngine.UseSearch {
			ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusStarted}, ag.ProceedChan)
			Warnf("%s", "Embedding tools are enabled.\n"+
				"Because embedding tools is not compatible with Google Search tool,"+
				" so Google Search is unavailable now.\n"+
				"Please disable embedding tools to use Google Search.")
		}
	} else if ag.SearchEngine.UseSearch {
		// only load search tool
		// **Remember: google doesn't support web_search tool plus function call
		// Function call is not compatible with Google Search tool
		config.Tools = append(config.Tools, ag.getGemini2WebSearchTool())
	} else if ag.UseCodeTool {
		// Remember: CodeExecution and GoogleSearch cannot be enabled at the same time
		config.Tools = append(config.Tools, ag.getGemini2CodeExecTool())
	}

	// Create a chat session
	chat, err := client.Chats.Create(ctx, ag.ModelName, &config, convo.History)
	if err != nil {
		ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusError, Data: fmt.Sprintf("Failed to create chat: %v", err)}, nil)
		return err
	}

	// Signal that streaming has started
	// Wait for the main goroutine to tell sub-goroutine to proceed
	ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusProcessing}, ag.ProceedChan)

	// Stream the responses
	references := make([]map[string]interface{}, 0, 1)
	queries := make([]string, 0, 1)
	streamParts := &parts

	// Use maxRecursions from LangLogic
	maxRecursions := ag.MaxRecursions
	var finalResp *genai.GenerateContentResponse

	for i := 0; i < maxRecursions; i++ {
		funcCalls, resp, err := ag.processGemini2Stream(ctx, chat, streamParts, &references, &queries)
		if err != nil {
			return err
		}
		// No furtheer calls
		if len(*funcCalls) == 0 {
			finalResp = resp
			break
		}
		// reconstruct the function call
		// Although i think this is a bug in the gemini2 api
		// we can safely reconstruct the function call part, because it's a funcCall part
		lastContent := chat.History(false)[len(chat.History(false))-1]
		if lastContent != nil && len(lastContent.Parts) == 0 {
			// ** If we dont' keep the funcCall Name, the function call part would be disposed in chat history **
			// I think it's a bug in the gemini2 api
			// ** It will generate a invalid empty parameter erro in the chat history **
			// So we must reconstruct the function call part
			lastContent.Parts = []*genai.Part{}
			for _, funcCall := range *funcCalls {
				callPart := genai.Part{FunctionCall: funcCall}
				lastContent.Parts = append(lastContent.Parts, &callPart)
			}
		}

		streamParts = &[]genai.Part{}
		for _, funcCall := range *funcCalls {

			// Skip if not our expected function
			// Because some model made up function name
			if funcCall.Name != "" && !AvailableEmbeddingTool(funcCall.Name) {
				continue
			}
			// Handle tool call
			funcResp, err := ag.processGemini2ToolCall(funcCall)
			if err != nil {
				Warnf("Processing tool call: %v\n", err)
				// Send error info to user but continue processing other tool calls
				continue
			}
			// Send function response back through the chat session
			respPart := genai.Part{FunctionResponse: funcResp}
			*streamParts = append(*streamParts, respPart)
		}
	}

	// Add queries to the output if any
	if len(queries) > 0 {
		q := "\n\n" + ag.SearchEngine.RetrieveQueries(queries)
		ag.DataChan <- StreamData{Text: q, Type: DataTypeNormal}
	}

	// Add references to the output if any
	if len(references) > 0 {
		refs := "\n\n" + ag.SearchEngine.RetrieveReferences(references) + "\n"
		ag.DataChan <- StreamData{Text: refs, Type: DataTypeNormal}
	}

	// Record token usage
	if finalResp != nil && finalResp.UsageMetadata != nil {
		ag.TokenUsage.RecordTokenUsage(int(finalResp.UsageMetadata.PromptTokenCount),
			int(finalResp.UsageMetadata.CandidatesTokenCount),
			int(finalResp.UsageMetadata.CachedContentTokenCount),
			int(finalResp.UsageMetadata.ThoughtsTokenCount))
	}

	// Save the conversation history
	convo.History = chat.History(false)
	err = convo.Save()
	if err != nil {
		return fmt.Errorf("failed to save conversation: %v", err)
	}

	// Flush all data to the channel
	ag.DataChan <- StreamData{Type: DataTypeFinished}
	<-ag.ProceedChan
	// Signal that streaming is finished
	ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusFinished}, nil)
	return err
}

func (ag *Agent) processGemini2Stream(ctx context.Context,
	chat *genai.Chat, parts *[]genai.Part,
	refs *[]map[string]interface{},
	queries *[]string) (*[]*genai.FunctionCall, *genai.GenerateContentResponse, error) {

	// Stream the response
	ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusProcessing}, ag.ProceedChan)
	iter := chat.SendMessageStream(ctx, *parts...)
	// Wait for the main goroutine to tell sub-goroutine to proceed
	ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusStarted}, ag.ProceedChan)

	funcCalls := []*genai.FunctionCall{}
	var finalResp *genai.GenerateContentResponse
	for resp, err := range iter {
		if err != nil {
			ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusError, Data: fmt.Sprintf("Generation error: %v", err)}, nil)
			return nil, nil, err
		}

		// Process and send content
		for _, candidate := range resp.Candidates {
			// Process content
			for _, part := range candidate.Content.Parts {
				// Record function call, but don't process here
				if part.FunctionCall != nil {
					funcCalls = append(funcCalls, part.FunctionCall)
					// If we keep the name, we could keep the funcCall
					// But we must erase the name when we actually call that function
					// Otherelse it will generate rudandent error
					// But, if we don't keep the funcCall name
					// It will dispose the funcCall, I think this is a bug!!!
					// So we need reconstruct the funcCall
					//part.Text = funcCall.Name

					// function call wouldn't have text
					// so pass here
					continue
				}

				// State transitions
				switch ag.Status.Peek() {
				case StatusReasoning:
					if !part.Thought {
						ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusReasoningOver}, ag.ProceedChan)
					}
				default:
					if part.Thought {
						ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusReasoning}, ag.ProceedChan)
					}
				}

				// Actual text data (don't trim text, because we need to keep the spaces between them)
				if part.Thought && part.Text != "" {
					// Reasoning data
					ag.DataChan <- StreamData{Text: (part.Text), Type: DataTypeReasoning}
				} else if part.Text != "" {
					// Normal text data
					ag.DataChan <- StreamData{Text: (part.Text), Type: DataTypeNormal}
				}
			}

			// Add references to the output if any
			if candidate.GroundingMetadata != nil {
				appendReferences(candidate.GroundingMetadata, refs)
				*queries = append(*queries, candidate.GroundingMetadata.WebSearchQueries...)
			}
		}

		// If we have a final response, save it
		// It has usage metadata
		finalResp = resp
	}
	return &funcCalls, finalResp, nil
}

func (ag *Agent) processGemini2ToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {

	// Call function
	ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusFunctionCalling, Data: fmt.Sprintf("%s(%s)\n", call.Name, formatToolCallArguments(call.Args))}, ag.ProceedChan)

	var resp *genai.FunctionResponse
	var err error

	// Using a map for dispatch is cleaner and more extensible than a large switch statement.
	toolHandlers := map[string]func(*genai.FunctionCall) (*genai.FunctionResponse, error){
		"shell":               ag.processGemini2ShellToolCall,
		"read_file":           ag.processGemini2ReadFileToolCall,
		"write_file":          ag.processGemini2WriteFileToolCall,
		"create_directory":    ag.processGemini2CreateDirectoryToolCall,
		"list_directory":      ag.processGemini2ListDirectoryToolCall,
		"delete_file":         ag.processGemini2DeleteFileToolCall,
		"delete_directory":    ag.processGemini2DeleteDirectoryToolCall,
		"move":                ag.processGemini2MoveToolCall,
		"copy":                ag.processGemini2CopyToolCall,
		"search_files":        ag.processGemini2SearchFilesToolCall,
		"search_text_in_file": ag.processGemini2SearchTextInFileToolCall,
		"read_multiple_files": ag.processGemini2ReadMultipleFilesToolCall,
		"web_fetch":           ag.processGemini2WebFetchToolCall,
		"edit_file":           ag.processGemini2EditFileToolCall,
	}

	if handler, ok := toolHandlers[call.Name]; ok {
		resp, err = handler(call)
	} else {
		// For web_search and other Google Search/CodeExecution tools that don't need special processing
		resp, err = &genai.FunctionResponse{
			ID:   call.ID,
			Name: call.Name,
			Response: map[string]any{
				"content": nil,
				"error":   fmt.Sprintf("unknown or built-in tool call: %v", call.Name),
			},
		}, nil
	}

	// Function call is done
	ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusFunctionCallingOver}, ag.ProceedChan)
	return resp, err
}

func appendReferences(metadata *genai.GroundingMetadata, refs *[]map[string]interface{}) {
	// Process grounding metadata to extract references
	// Check if we have grounding chunks
	if len(metadata.GroundingChunks) > 0 {
		// Build a single map with a "results" key as expected by RetrieveReferences
		results := make([]any, 0, len(metadata.GroundingChunks))
		for _, chunk := range metadata.GroundingChunks {
			// Check if the web chunk exists before accessing its fields
			if chunk.Web != nil {
				// Use URI as the displayLink since that's what users typically see
				results = append(results, map[string]any{
					"title":       chunk.Web.Title,
					"link":        chunk.Web.URI,
					"displayLink": chunk.Web.Title,
				})
			}
		}

		// Only append if we have valid results
		if len(results) > 0 {
			// Track references
			*refs = append(*refs, map[string]any{"results": results})
		}
	}
}
