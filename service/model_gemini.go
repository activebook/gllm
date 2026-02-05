package service

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/genai"
)

func (ag *Agent) getGeminiFilePart(file *FileData) genai.Part {

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

type GeminiAgent struct {
	// With *Agent embedded, GeminiAgent automatically has access to all of Agent's fields and methods.
	*Agent   // Embedded pointer to Agent
	ctx      context.Context
	client   *genai.Client
	executor *SubAgentExecutor
}

func (ag *Agent) initGeminiAgent() (*GeminiAgent, error) {
	// Setup the Gemini client
	// In multi-turn conversation, even though we create it each time
	// it can still be cached for advanced gemini models such as 2.5 flash/pro
	// so it's a server side job
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  ag.Model.ApiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusError, Data: fmt.Sprintf("Failed to create client: %v", err)}, nil)
		return nil, err
	}
	return &GeminiAgent{
		Agent:  ag,
		ctx:    ctx,
		client: client,
	}, nil
}

func (ag *Agent) SortGeminiMessagesByOrder() error {
	// Load previous messages if any
	err := ag.Convo.Load()
	if err != nil {
		ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusError, Data: fmt.Sprintf("failed to load conversation: %v", err)}, nil)
		return err
	}

	messages, _ := ag.Convo.GetMessages().([]*genai.Content)

	var parts []*genai.Part

	if ag.UserPrompt != "" {
		parts = append(parts, &genai.Part{Text: ag.UserPrompt})
	}
	for _, file := range ag.Files {
		// Check if the file data is empty
		if file != nil {
			// Convert the file data to a blob
			part := ag.getGeminiFilePart(file)
			if part.Text != "" || part.InlineData != nil {
				parts = append(parts, &part)
			}
		}
	}

	if len(parts) > 0 {
		// Construct Input Content from streamParts
		content := &genai.Content{
			Role:  genai.RoleUser,
			Parts: parts,
		}
		messages = append(messages, content)
	}

	// Save messages to conversation
	ag.Convo.SetMessages(messages)
	return nil
}

func (ag *Agent) GenerateGeminiStream() error {
	var err error
	// Check the setup of Gemini client
	ga, err := ag.initGeminiAgent()
	if err != nil {
		return err
	}

	// Initialize sub-agent executor if SharedState is available
	if ag.SharedState != nil {
		ga.executor = NewSubAgentExecutor(ag.SharedState, MaxWorkersParalleled)
	}

	// Configure Model Parameters
	thinking := ag.ThinkingLevel.ToGeminiConfig(ag.Model.ModelName)

	// Create the model and generate content
	config := genai.GenerateContentConfig{
		Temperature:    &ag.Model.Temperature,
		TopP:           &ag.Model.TopP,
		ThinkingConfig: thinking,
		Tools:          []*genai.Tool{
			// Placeholder
			//{CodeExecution: &genai.ToolCodeExecution{}},
			//{GoogleSearch: &genai.GoogleSearch{}},
		},
	}

	// Add seed if provided
	if ag.Model.Seed != nil {
		config.Seed = ag.Model.Seed
	}
	// System Instruction (System Prompt)
	if ag.SystemPrompt != "" {
		config.SystemInstruction = &genai.Content{Parts: []*genai.Part{{Text: ag.SystemPrompt}}}
	}

	var tool *genai.Tool
	if len(ag.EnabledTools) > 0 {
		// get tools(functions)
		tool = ga.getGeminiTools()
	}
	if ag.MCPClient != nil {
		// Append MCP tools(functions) to the existing tools
		if mcpTool := getGeminiMCPTools(ag.MCPClient); mcpTool != nil {
			tool = appendGeminiTool(tool, mcpTool)
		}
	}
	// Add function tools to config
	if tool != nil {
		config.Tools = append(config.Tools, tool)
	}
	if ag.SearchEngine.UseSearch {
		// Add web search tool to config
		tool = ga.getGeminiWebSearchTool()
		config.Tools = append(config.Tools, tool)
	}
	// // Incompatible tools yet
	// if ag.UseCodeTool {
	// 	// Remember: CodeExecution and GoogleSearch cannot be enabled at the same time
	// 	config.Tools = append(config.Tools, ga.getGeminiCodeExecTool())
	// }

	// Check whether to show warning message
	if len(config.Tools) > 1 {
		// Function tools and Google Search cannot be enabled simultaneously.
		// Function call is not compatible with Google Search tool
		// Only keep the first one
		config.Tools = config.Tools[:1]
		ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusStarted}, ag.ProceedChan)
		ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusWarn,
			Data: fmt.Sprint("Function tools are not compatible with Google Search tool, so Google Search is unavailable now.\n" +
				"Please disable tools if you want to use Google Search.")}, nil)
	}

	// Prepare the Messages for Chat Completion
	err = ag.SortGeminiMessagesByOrder()
	if err != nil {
		return fmt.Errorf("error sorting messages: %v", err)
	}

	// Signal that streaming has started
	// Wait for the main goroutine to tell sub-goroutine to proceed
	ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusStarted}, ag.ProceedChan)

	// Process the chat with recursive tool call handling
	err = ga.process(ag, &config)
	if err != nil {
		// Switch agent signal
		if IsSwitchAgentError(err) {
			return err
		}
		// User cancel signal
		if IsUserCancelError(err) {
			return err
		}
		return fmt.Errorf("error processing chat: %v", err)
	}
	return nil
}

func (ga *GeminiAgent) process(ag *Agent, config *genai.GenerateContentConfig) error {
	// Context Management
	truncated := false
	cm := NewContextManagerForModel(ag.Model.ModelName, StrategyTruncateOldest)

	// Signal that streaming has started
	// Wait for the main goroutine to tell sub-goroutine to proceed
	ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusProcessing}, ag.ProceedChan)

	// Stream the responses
	references := make([]map[string]interface{}, 0, 1)
	queries := make([]string, 0, 1)

	// Use maxRecursions from LangLogic
	maxRecursions := ag.MaxRecursions
	for i := 0; i < maxRecursions; i++ {

		// Create a chat session - this is the important part
		// Get all history messages - MUST be inside loop to pick up newly pushed messages
		messages, _ := ag.Convo.GetMessages().([]*genai.Content)

		// Context Management
		// Directly truncate on the messages
		Debugf("Context messages: [%d]", len(messages))
		messages, truncated = cm.PrepareGeminiMessages(messages, ag.SystemPrompt, config.Tools)
		if truncated {
			// Notify user or log that truncation happened
			ag.Warn("Context trimmed to fit model limits")
			Debugf("Context messages after truncation: [%d]", len(messages))
			ag.Convo.SetMessages(messages)
		}

		// Call API
		modelContent, resp, err := ga.processGeminiStream(ga.ctx, ga.client, config, messages, &references, &queries)
		if err != nil {
			return err
		}

		// Update History
		err = ga.processConvoSave(ag, modelContent)
		if err != nil {
			return err
		}

		// Record token usage
		ga.addUpGeminiTokenUsage(resp)

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

		for _, funcCall := range funcCalls {

			// Skip if not our expected function
			// Because some model made up function name
			if funcCall.Name != "" && !IsAvailableOpenTool(funcCall.Name) && !IsAvailableMCPTool(funcCall.Name, ag.MCPClient) {
				Warnf("Skipping tool call with unknown function name: %s", funcCall.Name)
				continue
			}
			// Handle tool call
			funcResp, err := ga.processGeminiToolCall(funcCall)
			if err != nil {
				// Switch agent signal, pop up
				if IsSwitchAgentError(err) {
					// Add the response part to satisfy history integrity
					ga.processConvoSave(ag, funcResp)
					return err
				}
				if IsUserCancelError(err) {
					// Add the response part to satisfy history integrity
					ga.processConvoSave(ag, funcResp)
					return err
				}
				ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusWarn, Data: fmt.Sprintf("Failed to process tool call: %v", err)}, nil)
			}
			// Bugfix: Even error happened, we still need to send the function response back through the chat session
			// Send function response back through the chat session
			err = ga.processConvoSave(ag, funcResp)
			if err != nil {
				return err
			}
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

	// Flush all data to the channel
	ag.DataChan <- StreamData{Type: DataTypeFinished}
	<-ag.ProceedChan
	// Signal that streaming is finished
	ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusFinished}, nil)
	return nil
}

func (ga *GeminiAgent) processConvoSave(ag *Agent, content *genai.Content) error {
	// Save the conversation history(curated)
	ag.Convo.Push(content)
	err := ag.Convo.Save()
	if err != nil {
		return fmt.Errorf("failed to save conversation: %v", err)
	}
	return nil
}

func (ga *GeminiAgent) processGeminiStream(ctx context.Context,
	client *genai.Client, config *genai.GenerateContentConfig,
	messages []*genai.Content,
	refs *[]map[string]interface{},
	queries *[]string) (*genai.Content, *genai.GenerateContentResponse, error) {

	// Stream the response
	ga.Status.ChangeTo(ga.NotifyChan, StreamNotify{Status: StatusProcessing}, ga.ProceedChan)
	iter := client.Models.GenerateContentStream(ctx, ga.Model.ModelName, messages, config)
	// Wait for the main goroutine to tell sub-goroutine to proceed
	ga.Status.ChangeTo(ga.NotifyChan, StreamNotify{Status: StatusStarted}, ga.ProceedChan)

	modelContent := &genai.Content{
		Role:  genai.RoleModel,
		Parts: []*genai.Part{},
	}
	var finalResp *genai.GenerateContentResponse

	for resp, err := range iter {
		if err != nil {
			ga.Status.ChangeTo(ga.NotifyChan, StreamNotify{Status: StatusError, Data: fmt.Sprintf("Generation error: %v", err)}, nil)
			return nil, nil, err
		}

		// Process and send content
		for _, candidate := range resp.Candidates {
			if candidate.Content == nil {
				continue // Skip candidates with nil content, bugfix: panic on nil content
			}
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
				switch ga.Status.Peek() {
				case StatusReasoning:
					if !part.Thought {
						ga.Status.ChangeTo(ga.NotifyChan, StreamNotify{Status: StatusReasoningOver}, ga.ProceedChan)
					}
				default:
					if part.Thought {
						ga.Status.ChangeTo(ga.NotifyChan, StreamNotify{Status: StatusReasoning}, ga.ProceedChan)
					}
				}

				// Actual text data (don't trim text, because we need to keep the spaces between them)
				if part.Thought && part.Text != "" {
					// Reasoning data
					ga.DataChan <- StreamData{Text: (part.Text), Type: DataTypeReasoning}
				} else if part.Text != "" {
					// Normal text data
					ga.DataChan <- StreamData{Text: (part.Text), Type: DataTypeNormal}
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

func (ga *GeminiAgent) processGeminiToolCall(call *genai.FunctionCall) (*genai.Content, error) {

	var filteredArgs map[string]interface{}
	if call.Name == ToolEditFile || call.Name == ToolWriteFile {
		// Don't show content(the modified content could be too long)
		filteredArgs = FilterOpenToolArguments(call.Args, []string{"content", "edits"})
	} else {
		filteredArgs = FilterOpenToolArguments(call.Args, []string{})
	}

	// Call function
	// Create structured data for the UI
	toolCallData := map[string]interface{}{
		"function": call.Name,
		"args":     filteredArgs,
	}
	jsonData, _ := json.Marshal(toolCallData)
	ga.Status.ChangeTo(ga.NotifyChan, StreamNotify{Status: StatusFunctionCalling, Data: string(jsonData)}, ga.ProceedChan)

	var resp *genai.FunctionResponse
	var err error

	// Using a map for dispatch is cleaner and more extensible than a large switch statement.
	toolHandlers := map[string]func(*genai.FunctionCall) (*genai.FunctionResponse, error){
		ToolShell:             ga.GeminiShellToolCall,
		ToolReadFile:          ga.GeminiReadFileToolCall,
		ToolWriteFile:         ga.GeminiWriteFileToolCall,
		ToolCreateDirectory:   ga.GeminiCreateDirectoryToolCall,
		ToolListDirectory:     ga.GeminiListDirectoryToolCall,
		ToolDeleteFile:        ga.GeminiDeleteFileToolCall,
		ToolDeleteDirectory:   ga.GeminiDeleteDirectoryToolCall,
		ToolMove:              ga.GeminiMoveToolCall,
		ToolCopy:              ga.GeminiCopyToolCall,
		ToolSearchFiles:       ga.GeminiSearchFilesToolCall,
		ToolSearchTextInFile:  ga.GeminiSearchTextInFileToolCall,
		ToolReadMultipleFiles: ga.GeminiReadMultipleFilesToolCall,
		ToolWebFetch:          ga.GeminiWebFetchToolCall,
		ToolEditFile:          ga.GeminiEditFileToolCall,
		ToolListMemory:        ga.GeminiListMemoryToolCall,
		ToolSaveMemory:        ga.GeminiSaveMemoryToolCall,
		ToolSwitchAgent:       ga.GeminiSwitchAgentToolCall,
		ToolListAgent:         ga.GeminiListAgentToolCall,
		ToolSpawnSubAgents:    ga.GeminiSpawnSubAgentsToolCall,
		ToolGetState:          ga.GeminiGetStateToolCall,
		ToolSetState:          ga.GeminiSetStateToolCall,
		ToolListState:         ga.GeminiListStateToolCall,
		ToolActivateSkill:     ga.GeminiActivateSkillToolCall,
	}

	if handler, ok := toolHandlers[call.Name]; ok {
		resp, err = handler(call)
	} else if ga.MCPClient != nil && ga.MCPClient.FindTool(call.Name) != nil {
		// Handle MCP tool calls
		resp, err = ga.GeminiMCPToolCall(call)
	} else {
		// For web_search and other Google Search/CodeExecution tools that don't need special processing
		resp, err = &genai.FunctionResponse{
			ID:   call.ID,
			Name: call.Name,
			Response: map[string]any{
				"content": nil,
				"error":   fmt.Sprintf("Error: Unknown function '%s'. This function is not available. Please use one of the available functions from the tool list.", call.Name),
			},
		}, nil
		// Warn the user
		ga.Status.ChangeTo(ga.NotifyChan, StreamNotify{Status: StatusWarn, Data: fmt.Sprintf("Model attempted to call unknown function: %s", call.Name)}, nil)
	}

	// Function response only has one part
	respPart := genai.Part{FunctionResponse: resp}
	respContent := &genai.Content{
		Role:  genai.RoleUser, // In Gemini, tool responses are sent as 'user' role
		Parts: []*genai.Part{&respPart},
	}

	// Function call is done
	ga.Status.ChangeTo(ga.NotifyChan, StreamNotify{Status: StatusFunctionCallingOver}, ga.ProceedChan)
	return respContent, err
}

// In an agentic workflow with multi-turn interactions:
// Each turn involves streaming responses from the LLM
// Each response may contain tool calls that trigger additional processing
// New responses are generated based on tool call results
// Each of these interactions consumes tokens that should be tracked
func (ga *GeminiAgent) addUpGeminiTokenUsage(resp *genai.GenerateContentResponse) {
	if resp != nil && resp.UsageMetadata != nil && ga.TokenUsage != nil {
		// For gemini model, cache read tokens are not included in the usage metadata
		// The total number of tokens for the entire request. This is the sum of `prompt_token_count`,
		// `candidates_token_count`, `tool_use_prompt_token_count`, and `thoughts_token_count`.
		ga.TokenUsage.CachedTokensInPrompt = true
		ga.TokenUsage.RecordTokenUsage(int(resp.UsageMetadata.PromptTokenCount),
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
