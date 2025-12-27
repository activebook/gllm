package service

import (
	"context"
	"encoding/json"
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
	case IsVideoMIMEType(mimeType):
		// Handle video files.
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
		TopP:        &ag.TopP,
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

	// Add seed if provided
	if ag.Seed != nil {
		seedInt32 := int32(*ag.Seed)
		config.Seed = &seedInt32
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
	if len(ag.EnabledTools) > 0 {
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
	// it will only consume tokens on new input
	messages, _ := ag.Convo.GetMessages().([]*genai.Content)

	// Context Management
	truncated := false
	cm := NewContextManagerForModel(ag.ModelName, StrategyTruncateOldest)

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
		// Construct Input Content from streamParts
		inputParts := make([]*genai.Part, len(*streamParts))
		hasFuncResponse := false
		for idx, p := range *streamParts {
			partCopy := p
			inputParts[idx] = &partCopy
			if p.FunctionResponse != nil {
				hasFuncResponse = true
			}
		}

		role := genai.RoleUser
		if hasFuncResponse {
			// function response
			// This is the crucial part
			// there is no function role, but it's a function response
			// The producer of the content.
			// Must be either 'user' or 'model'.
			// Useful to set for multi-turn conversations,
			// otherwise can be left blank or unset.
			role = genai.RoleUser
		}

		inputContent := &genai.Content{
			Role:  role,
			Parts: inputParts,
		}

		// Prepare messages for this call
		// We need to pass the full history including the current input
		messages = append(messages, inputContent)

		// Context Management
		// Directly truncate on the messages
		Debugf("Context messages: [%d]", len(messages))
		messages, truncated = cm.PrepareGeminiMessages(messages, ag.SystemPrompt, config.Tools)
		if truncated {
			// Notify user or log that truncation happened
			ag.Warn("Context trimmed to fit model limits")
			Debugf("Context messages after truncation: [%d]", len(messages))
		}

		// Call API
		modelContent, resp, err := ag.processGemini2Stream(ga.ctx, ga.client, &config, messages, &references, &queries)
		if err != nil {
			return err
		}
		// Record token usage
		ag.addUpGemini2TokenUsage(resp)

		// Update History
		// messages already has inputContent from pre-API append
		messages = append(messages, modelContent)

		// Check for function calls in the model content
		funcCalls := []*genai.FunctionCall{}
		for _, part := range modelContent.Parts {
			if part.FunctionCall != nil {
				funcCalls = append(funcCalls, part.FunctionCall)
			}
		}

		// No further calls
		if len(funcCalls) == 0 {
			break
		}

		streamParts = &[]genai.Part{}
		for _, funcCall := range funcCalls {

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
	ag.Convo.SetMessages(messages)
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
	client *genai.Client, config *genai.GenerateContentConfig,
	messages []*genai.Content,
	refs *[]map[string]interface{},
	queries *[]string) (*genai.Content, *genai.GenerateContentResponse, error) {

	// Stream the response
	ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusProcessing}, ag.ProceedChan)
	iter := client.Models.GenerateContentStream(ctx, ag.ModelName, messages, config)
	// Wait for the main goroutine to tell sub-goroutine to proceed
	ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusStarted}, ag.ProceedChan)

	modelContent := &genai.Content{
		Role:  genai.RoleModel,
		Parts: []*genai.Part{},
	}
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
				// Accumulate parts for history ONLY if they have content
				// The API returns Error 400 if we send back a part with no initialized 'data' field
				if part.Text != "" || part.FunctionCall != nil || part.FunctionResponse != nil ||
					part.InlineData != nil || part.FileData != nil ||
					part.ExecutableCode != nil || part.CodeExecutionResult != nil {
					modelContent.Parts = append(modelContent.Parts, part)
				}

				// Record function call, but don't process here
				if part.FunctionCall != nil {
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
	return modelContent, finalResp, nil
}

func (ag *Agent) processGemini2ToolCall(call *genai.FunctionCall) (*genai.FunctionResponse, error) {

	var filteredArgs map[string]interface{}
	if call.Name == "edit_file" || call.Name == "write_file" {
		// Don't show content(the modified content could be too long)
		filteredArgs = FilterToolArguments(call.Args, []string{"content", "edits"})
	} else {
		filteredArgs = FilterToolArguments(call.Args, []string{})
	}

	// Call function
	// Create structured data for the UI
	toolCallData := map[string]interface{}{
		"function": call.Name,
		"args":     filteredArgs,
	}
	jsonData, _ := json.Marshal(toolCallData)
	ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusFunctionCalling, Data: string(jsonData)}, ag.ProceedChan)

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
		"edit_file":           ag.Gemini2EditFileToolCall,
		"list_memory":         ag.Gemini2ListMemoryToolCall,
		"save_memory":         ag.Gemini2SaveMemoryToolCall,
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
