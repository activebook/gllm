package service

import (
	"context"
	"fmt"

	"google.golang.org/genai"
)

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

func (ll *LangLogic) GenerateGemini2Stream() error {
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

	// - If UseTools is true, enable the embedding tools.
	// - Else if UseSearchTool is true, enable Google Search.
	// - Else if UseCodeTool is true, enable code execution.
	// CodeExecution and GoogleSearch cannot be enabled simultaneously.
	if ll.UseTools {
		// load embedding Tools
		config.Tools = append(config.Tools, ll.getGemini2Tools()...)
		if ll.UseSearchTool {
			Warnf("%s", "Embedding tools are enabled.\n"+
				"Because embedding tools is not compatible with Google Search tool, so Google Search is unavailable now.\n"+
				"Please disable embedding tools to use Google Search.\n")
		}
	} else if ll.UseSearchTool {
		// only load search tool
		// **Remember: google doesn't support web_search tool plus function call
		// Function call is not compatible with Google Search tool
		config.Tools = append(config.Tools, ll.getGemini2WebSearchTool())
	} else if ll.UseCodeTool {
		// Remember: CodeExecution and GoogleSearch cannot be enabled at the same time
		config.Tools = append(config.Tools, ll.getGemini2CodeExecTool())
	}

	// Create a chat session
	chat, err := client.Chats.Create(ctx, ll.ModelName, &config, convo.History)
	if err != nil {
		ll.ProcChan <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("Failed to create chat: %v", err)}
		return err
	}

	// Signal that streaming has started
	ll.ProcChan <- StreamNotify{Status: StatusStarted}
	<-ll.ProceedChan // Wait for the main goroutine to tell sub-goroutine to proceed

	// Stream the responses
	references := make([]*map[string]interface{}, 0, 1)
	queries := make([]string, 0, 1)
	streamParts := &parts

	// Use maxRecursions from LangLogic
	maxRecursions := ll.MaxRecursions

	for i := 0; i < maxRecursions; i++ {
		funcCalls, err := ll.processGemini2Stream(ctx, chat, streamParts, &references, &queries)
		if err != nil {
			return err
		}
		// No furtheer calls
		if len(*funcCalls) == 0 {
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
			funcResp, err := ll.processGemini2ToolCall(funcCall)
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
		q := "\n\n" + RetrieveQueries(queries)
		ll.ProcChan <- StreamNotify{Status: StatusData, Data: q}
	}
	// Add references to the output if any
	if len(references) > 0 {
		refs := "\n\n" + RetrieveReferences(references) + "\n"
		ll.ProcChan <- StreamNotify{Status: StatusData, Data: refs}
	}

	// Save the conversation history
	convo.History = chat.History(false)
	err = convo.Save()
	if err != nil {
		return fmt.Errorf("failed to save conversation: %v", err)
	}
	ll.ProcChan <- StreamNotify{Status: StatusFinished}
	return err
}

func (ll *LangLogic) processGemini2Stream(ctx context.Context,
	chat *genai.Chat, parts *[]genai.Part,
	refs *[]*map[string]interface{},
	queries *[]string) (*[]*genai.FunctionCall, error) {

	// Stream the response
	ll.ProcChan <- StreamNotify{Status: StatusProcessing}
	iter := chat.SendMessageStream(ctx, *parts...)
	ll.ProcChan <- StreamNotify{Status: StatusStarted}
	<-ll.ProceedChan // Wait for the main goroutine to tell sub-goroutine to proceed

	state := stateNormal
	funcCalls := []*genai.FunctionCall{}
	for resp, err := range iter {
		if err != nil {
			ll.ProcChan <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("Generation error: %v", err)}
			return nil, err
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
				// Actual text data
				if part.Thought && part.Text != "" {
					ll.ProcChan <- StreamNotify{Status: StatusReasoningData, Data: part.Text}
				} else if part.Text != "" {
					ll.ProcChan <- StreamNotify{Status: StatusData, Data: part.Text}
				}
			}

			// Add references to the output if any
			if candidate.GroundingMetadata != nil {
				appendReferences(candidate.GroundingMetadata, refs)
				*queries = append(*queries, candidate.GroundingMetadata.WebSearchQueries...)
			}
		}
	}
	return &funcCalls, nil
}

func (ll *LangLogic) processGemini2ToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {

	// Call function
	ll.ProcChan <- StreamNotify{
		Status: StatusFunctionCalling,
		Data:   fmt.Sprintf("%s(%s)\n", call.Name, formatToolCallArguments(call.Args)),
	}

	var resp *genai.FunctionResponse
	var err error
	switch call.Name {
	case "shell":
		resp, err = ll.processGemini2ShellToolCall(call)
	case "read_file":
		resp, err = ll.processGemini2ReadFileToolCall(call)
	case "write_file":
		resp, err = ll.processGemini2WriteFileToolCall(call)
	case "create_directory":
		resp, err = ll.processGemini2CreateDirectoryToolCall(call)
	case "list_directory":
		resp, err = ll.processGemini2ListDirectoryToolCall(call)
	case "delete_file":
		resp, err = ll.processGemini2DeleteFileToolCall(call)
	case "delete_directory":
		resp, err = ll.processGemini2DeleteDirectoryToolCall(call)
	case "move":
		resp, err = ll.processGemini2MoveToolCall(call)
	case "search_files":
		resp, err = ll.processGemini2SearchFilesToolCall(call)
	case "search_text_in_file":
		resp, err = ll.processGemini2SearchTextInFileToolCall(call)
	case "read_multiple_files":
		resp, err = ll.processGemini2ReadMultipleFilesToolCall(call)
	case "web_fetch":
		resp, err = ll.processGemini2WebFetchToolCall(call)
	case "edit_file":
		resp, err = ll.processGemini2EditFileToolCall(call)
	default:
		// For web_search and other Google Search/CodeExecution tools that don't need special processing
		resp, err = &genai.FunctionResponse{
			ID:   call.ID,
			Name: call.Name,
			Response: map[string]any{
				"content": nil,
				"error":   fmt.Sprintf("unknown tool call: %v", call.Name),
			},
		}, nil
	}

	// Function call is done
	ll.ProcChan <- StreamNotify{Status: StatusFunctionCallingOver}
	<-ll.ProceedChan
	return resp, err
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
