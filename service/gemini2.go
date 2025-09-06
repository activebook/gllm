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

type Gemini2Agent struct {
	ctx    context.Context
	client *genai.Client
}

func (ag *Agent) initGemini2Agent() (*Gemini2Agent, error) {
	// Setup the Gemini client
	// In multi-turn conversation, even though we create it each time
	// it can still be cached for advanced gemini models such as 2.5 flash/pro
	// so it's a server side job
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  ag.ApiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusError, Data: fmt.Sprintf("Failed to create client: %v", err)}, nil)
		return nil, err
	}
	return &Gemini2Agent{
		ctx:    ctx,
		client: client,
	}, nil
}

func (ag *Agent) GenerateGemini2Stream() error {
	var err error
	// Check the setup of Gemini client
	ga, err := ag.initGemini2Agent()
	if err != nil {
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
	err = ag.Convo.Load()
	if err != nil {
		ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusError, Data: fmt.Sprintf("failed to load conversation: %v", err)}, nil)
		return err
	}

	// Configure Model Parameters
	thinkingBudgetVal := int32(-1)
	if ag.ThinkMode {
		// Turn on dynamic thinking:
		thinkingBudgetVal = int32(-1)
	} else {
		// Turn off thinking:
		thinkingBudgetVal = int32(0)
	}
	// Create the model and generate content
	config := genai.GenerateContentConfig{
		Temperature: &ag.Temperature,
		ThinkingConfig: &genai.ThinkingConfig{
			// Let model decide how to allocate tokens
			ThinkingBudget:  &thinkingBudgetVal,
			IncludeThoughts: ag.ThinkMode,
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

	// - If UseTools is true, enable embedding tools (include MCP if client exists).
	// - If UseSearch is true, enable Google Search (disable function tools).
	// - If UseCodeTool is true, enable code execution.
	// - If UseTools is false but MCP client exists, enable MCP-only tools.
	// Function tools and Google Search cannot be enabled simultaneously.
	if ag.ToolsUse.Enable {
		// load embedding tools (include MCP if available)
		includeMCP := ag.MCPClient != nil
		config.Tools = append(config.Tools, ag.getGemini2EmbeddingTools(includeMCP))
		if ag.SearchEngine.UseSearch {
			ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusStarted}, ag.ProceedChan)
			ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusWarn,
				Data: fmt.Sprint("Function tools are enabled.\n" +
					"Because function tools are not compatible with Google Search tool," +
					" so Google Search is unavailable now.\n" +
					"Please disable tools if you want to use Google Search.")}, nil)
		}
	} else if ag.SearchEngine.UseSearch {
		// only load search tool
		// **Remember: google doesn't support web_search tool plus function call
		// Function call is not compatible with Google Search tool
		config.Tools = append(config.Tools, ag.getGemini2WebSearchTool())
	} else if ag.UseCodeTool {
		// Remember: CodeExecution and GoogleSearch cannot be enabled at the same time
		config.Tools = append(config.Tools, ag.getGemini2CodeExecTool())
	} else if ag.MCPClient != nil {
		// Load MCP-only tools when embedding tools are disabled but MCP client exists
		if mcpTool := ag.getGemini2MCPTools(); mcpTool != nil {
			config.Tools = append(config.Tools, mcpTool)
		}
	}

	// Create a chat session - this is the important part
	// Within the chat session(multi-turn conversation)
	// we still need to sent the entire history to the model
	// but it won't consume tokens, because it was cached in local and remote server
	// so gemini model would fast load the cached history KV
	// it will only consume tokens on new input
	messages, _ := ag.Convo.GetMessages().([]*genai.Content)
	chat, err := ga.client.Chats.Create(ga.ctx, ag.ModelName, &config, messages)
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

	for i := 0; i < maxRecursions; i++ {
		funcCalls, resp, err := ag.processGemini2Stream(ga.ctx, chat, streamParts, &references, &queries)
		if err != nil {
			return err
		}
		// Record token usage
		// The final response contains the token usage metainfo
		ag.addUpGemini2TokenUsage(resp)

		// No furtheer calls
		if len(*funcCalls) == 0 {
			break
		}
		// reconstruct the function call
		// Although i think this is a bug in the gemini2 api
		// we can safely reconstruct the function call part, because it's a funcCall part
		lastContent := chat.History(true)[len(chat.History(true))-1]
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
			if funcCall.Name != "" && !AvailableEmbeddingTool(funcCall.Name) && !AvailableSearchTool(funcCall.Name) && !AvailableMCPTool(funcCall.Name, ag.MCPClient) {
				continue
			}
			// Handle tool call
			funcResp, err := ag.processGemini2ToolCall(funcCall)
			if err != nil {
				ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusWarn, Data: fmt.Sprintf("Failed to process tool call: %v", err)}, nil)
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
		refs := "\n\n" + ag.SearchEngine.RetrieveReferences(references)
		ag.DataChan <- StreamData{Text: refs, Type: DataTypeNormal}
	}

	// Save the conversation history(curated)
	ag.Convo.SetMessages(chat.History(true))
	err = ag.Convo.Save()
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

	var args string
	if call.Name == "edit_file" {
		// Don't show content(the modified content could be too long)
		args = formatToolCallArguments(call.Args, []string{"content"})
	} else {
		args = formatToolCallArguments(call.Args, []string{})
	}

	// Call function
	ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusFunctionCalling, Data: fmt.Sprintf("%s(%s)\n", call.Name, args)}, ag.ProceedChan)

	var resp *genai.FunctionResponse
	var err error

	// Using a map for dispatch is cleaner and more extensible than a large switch statement.
	toolHandlers := map[string]func(*genai.FunctionCall) (*genai.FunctionResponse, error){
		"shell":               ag.Gemini2ShellToolCall,
		"read_file":           ag.Gemini2ReadFileToolCall,
		"write_file":          ag.Gemini2WriteFileToolCall,
		"create_directory":    ag.Gemini2CreateDirectoryToolCall,
		"list_directory":      ag.Gemini2ListDirectoryToolCall,
		"delete_file":         ag.Gemini2DeleteFileToolCall,
		"delete_directory":    ag.Gemini2DeleteDirectoryToolCall,
		"move":                ag.Gemini2MoveToolCall,
		"copy":                ag.Gemini2CopyToolCall,
		"search_files":        ag.Gemini2SearchFilesToolCall,
		"search_text_in_file": ag.Gemini2SearchTextInFileToolCall,
		"read_multiple_files": ag.Gemini2ReadMultipleFilesToolCall,
		"web_fetch":           ag.Gemini2WebFetchToolCall,
		//"edit_file":           ag.Gemini2EditFileToolCall,
		"edit_file": ag.Gemini2ModifyFileToolCall,
	}

	if handler, ok := toolHandlers[call.Name]; ok {
		resp, err = handler(call)
	} else if ag.MCPClient != nil && ag.MCPClient.FindTool(call.Name) != nil {
		// Handle MCP tool calls
		resp, err = ag.Gemini2MCPToolCall(call)
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

// In an agentic workflow with multi-turn interactions:
// Each turn involves streaming responses from the LLM
// Each response may contain tool calls that trigger additional processing
// New responses are generated based on tool call results
// Each of these interactions consumes tokens that should be tracked
func (ag *Agent) addUpGemini2TokenUsage(resp *genai.GenerateContentResponse) {
	if resp != nil && resp.UsageMetadata != nil && ag.TokenUsage != nil {
		// Warnf("addUpTokenUsage - PromptTokenCount: %d, CandidatesTokenCount: %d, CachedContentTokenCount: %d, ThoughtsTokenCount: %d, TotalTokenCount: %d",
		// 	resp.UsageMetadata.PromptTokenCount,
		// 	resp.UsageMetadata.CandidatesTokenCount,
		// 	resp.UsageMetadata.CachedContentTokenCount,
		// 	resp.UsageMetadata.ThoughtsTokenCount,
		// 	resp.UsageMetadata.TotalTokenCount)
		ag.TokenUsage.RecordTokenUsage(int(resp.UsageMetadata.PromptTokenCount),
			int(resp.UsageMetadata.CandidatesTokenCount),
			int(resp.UsageMetadata.CachedContentTokenCount),
			int(resp.UsageMetadata.ThoughtsTokenCount),
			int(resp.UsageMetadata.TotalTokenCount))
	}
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
