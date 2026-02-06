package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"
	"github.com/anthropics/anthropic-sdk-go/shared/constant"
)

func (ag *Agent) getAnthropicFilePart(file *FileData) *anthropic.ContentBlockParamUnion {
	format := file.Format()
	// Handle based on file type
	if IsImageMIMEType(format) {
		mediaType := format // Format() returns MIME type for FileData usually
		// Encode base64
		base64Data := base64.StdEncoding.EncodeToString(file.Data())
		// Use NewImageBlockBase64 helper
		v := anthropic.NewImageBlockBase64(mediaType, base64Data)
		return &v
	}

	if IsTextMIMEType(format) {
		v := anthropic.NewTextBlock(string(file.Data()))
		return &v
	}

	return nil
}

// GenerateAnthropicStream generates a streaming response using Anthropic API
func (ag *Agent) GenerateAnthropicStream() error {
	// Initialize the Client
	ctx := context.Background()

	// Set both APIKey and AuthToken to ensure it works on X-Api-Key or Bearer
	opts := []option.RequestOption{
		option.WithAPIKey(ag.Model.ApiKey),
		option.WithAuthToken(ag.Model.ApiKey),
	}

	if ag.Model.EndPoint != "" {
		opts = append(opts, option.WithBaseURL(ag.Model.EndPoint))
	}

	// When we call client.Messages.NewStreaming, inside it, it would set anthropic-version to 2023-06-01 automatically
	client := anthropic.NewClient(opts...)

	// Create tools
	var tools []anthropic.ToolUnionParam
	if len(ag.EnabledTools) > 0 {
		tools = ag.getAnthropicTools()
	}
	if ag.MCPClient != nil {
		mcpTools := ag.getAnthropicMCPTools()
		tools = append(tools, mcpTools...)
	}

	// Initialize sub-agent executor if SharedState is available
	var executor *SubAgentExecutor
	if ag.SharedState != nil {
		executor = NewSubAgentExecutor(ag.SharedState, MaxWorkersParalleled)
	}

	op := OpenProcessor{
		ctx:        ctx,
		notify:     ag.NotifyChan,
		data:       ag.DataChan,
		proceed:    ag.ProceedChan,
		search:     ag.SearchEngine,
		toolsUse:   &ag.ToolsUse,
		queries:    make([]string, 0),
		references: make([]map[string]interface{}, 0),
		status:     &ag.Status,
		mcpClient:  ag.MCPClient,
		// Sub-agent orchestration
		sharedState: ag.SharedState,
		executor:    executor,
		agentName:   ag.AgentName,
	}

	chat := &Anthropic{
		client: client,
		tools:  tools,
		op:     &op,
	}

	// Prepare Messages
	err := ag.SortAnthropicMessagesByOrder()
	if err != nil {
		return fmt.Errorf("error sorting messages: %v", err)
	}

	// Signal started
	ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusStarted}, ag.ProceedChan)

	// Process
	err = chat.process(ag)
	if err != nil {
		// Switch agent signal, pop up
		if IsSwitchAgentError(err) {
			return err
		}
		// User cancel signal, pop up
		if IsUserCancelError(err) {
			return err
		}
		return fmt.Errorf("error processing chat: %v", err)
	}

	return nil
}

type Anthropic struct {
	client anthropic.Client
	tools  []anthropic.ToolUnionParam
	op     *OpenProcessor
}

func (a *Anthropic) process(ag *Agent) error {
	// Context Management
	truncated := false
	cm := NewContextManagerForModel(ag.Model.ModelName, StrategyTruncateOldest)

	// Recursion loop
	i := 0
	for range ag.MaxRecursions {
		i++
		a.op.status.ChangeTo(a.op.notify, StreamNotify{Status: StatusProcessing}, a.op.proceed)

		messages, _ := ag.Convo.GetMessages().([]anthropic.MessageParam)

		// Apply context window management
		messages, truncated = cm.PrepareAnthropicMessages(messages, ag.SystemPrompt, a.tools)
		if truncated {
			ag.Warn("Context trimmed to fit model limits")
			Debugf("Context messages after truncation: [%d]", len(messages))
			// Update the conversation with truncated messages
			ag.Convo.SetMessages(messages)
			ag.Convo.Save()
		}

		// Create params
		params := anthropic.MessageNewParams{
			Model:     anthropic.Model(ag.Model.ModelName),
			Messages:  messages,
			MaxTokens: int64(cm.MaxOutputTokens), // Use ContextManager limit
			System: []anthropic.TextBlockParam{{
				Text: ag.SystemPrompt,
				Type: constant.Text("text"),
			}},
			Tools: a.tools, // []ToolUnionParam
		}

		// Enable Thinking if requested, with budget based on level
		params.Thinking = ag.ThinkingLevel.ToAnthropicParams()

		// Temperature/TopP
		// Bug: `temperature` and `top_p` cannot both be specified for this model. Please use only one.
		if ag.Model.Temperature > 0 {
			params.Temperature = param.NewOpt(float64(ag.Model.Temperature))
		} else if ag.Model.TopP > 0 {
			params.TopP = param.NewOpt(float64(ag.Model.TopP))
		}

		stream := a.client.Messages.NewStreaming(a.op.ctx, params)
		a.op.status.ChangeTo(a.op.notify, StreamNotify{Status: StatusStarted}, a.op.proceed)

		// Process stream
		msg, toolCalls, usage, err := a.processStream(stream)
		if err != nil {
			return err
		}

		// Record token usage
		addUpAnthropicTokenUsage(ag, usage)

		// Push assistant message
		err = a.saveToConvo(ag, msg)
		if err != nil {
			return err
		}

		if len(toolCalls) > 0 {
			// Process tool calls
			for _, tc := range toolCalls {
				// Execute tool
				toolMsg, err := a.processToolCall(tc)
				if err != nil {
					// Switch agent signal, pop up
					if IsSwitchAgentError(err) {
						// Bugfix: left an "orphan" tool_call that had no matching tool result.
						// Add tool message to conversation to fix this.
						a.saveToConvo(ag, toolMsg)
						return err
					}
					if IsUserCancelError(err) {
						// User cancel signal, pop up
						a.saveToConvo(ag, toolMsg)
						return err
					}
					ag.Status.ChangeTo(ag.NotifyChan, StreamNotify{Status: StatusWarn, Data: fmt.Sprintf("Tool call failed: %v", err)}, nil)
				}
				// IMPORTANT: Even error happened still add an error response message to maintain conversation integrity
				// The API requires every tool_call to have a corresponding tool response
				err = a.saveToConvo(ag, toolMsg)
				if err != nil {
					return err
				}
			}
		} else {
			break
		}
	}

	// Retrieve queries/references similar to openai.go
	if len(a.op.queries) > 0 {
		q := "\n\n" + ag.SearchEngine.RetrieveQueries(a.op.queries)
		a.op.data <- StreamData{Text: q, Type: DataTypeNormal}
	}
	if len(a.op.references) > 0 {
		refs := "\n\n" + ag.SearchEngine.RetrieveReferences(a.op.references)
		a.op.data <- StreamData{Text: refs, Type: DataTypeNormal}
	}

	a.op.data <- StreamData{Type: DataTypeFinished}
	<-a.op.proceed
	a.op.status.ChangeTo(a.op.notify, StreamNotify{Status: StatusFinished}, nil)

	return nil
}

// saveToConvo processes the conversation save
// We need to save the conversation after each message is sent to the client
// Because model supports interleaved tool calls and responses, aka ReAct
// If error happened or user cancelled, in order to maintain conversation integrity, we need to save the conversation
// So that we can resume the conversation from the last saved state
func (a *Anthropic) saveToConvo(ag *Agent, message anthropic.MessageParam) error {
	// Add the assistant's message to the conversation
	err := ag.Convo.Push(message)
	if err != nil {
		return fmt.Errorf("failed to save conversation: %v", err)
	}
	return nil
}

func (a *Anthropic) processStream(stream *ssestream.Stream[anthropic.MessageStreamEventUnion]) (anthropic.MessageParam, []anthropic.ToolUseBlockParam, *TokenUsage, error) {
	var contentBuilder strings.Builder
	var thinkingBuilder strings.Builder
	var thinkingSignature string
	var currentToolUse *anthropic.ToolUseBlockParam
	var toolCalls []anthropic.ToolUseBlockParam
	var currentInputBuilder strings.Builder // For accumulating JSON input
	usage := NewTokenUsage()

	var contentBlocks []anthropic.ContentBlockParamUnion
	var currentBlockType string

	message := anthropic.NewAssistantMessage() // Helper

	for stream.Next() {
		event := stream.Current()

		// Event types are strings: "content_block_start", "content_block_delta", "content_block_stop", "message_delta"
		switch event.Type {
		case "message_start":
			evt := event.AsMessageStart()
			// For anthropic model, cache read tokens are included in the message usage
			// Because cached tokens are not in the prompt tokens, so we need to count them
			usage.RecordTokenUsage(
				int(evt.Message.Usage.InputTokens),
				int(evt.Message.Usage.OutputTokens),
				int(evt.Message.Usage.CacheReadInputTokens),
				0,
				int(evt.Message.Usage.InputTokens+evt.Message.Usage.OutputTokens+evt.Message.Usage.CacheReadInputTokens),
			)
			// Debugf("Anthropic Usage(message start): %v", evt.Message.Usage)
		case "content_block_start":
			evt := event.AsContentBlockStart()
			block := evt.ContentBlock
			// ContentBlockUnion.Type is string const, check values
			// Block types: "tool_use", "text"
			currentBlockType = block.Type
			switch block.Type {
			case "tool_use": // "tool_use"
				// Start Tool Use
				// block.ToolUse is not a field. It's flat.
				// Skip if not our expected function
				// Because some model made up function name
				functionID := block.ID
				functionName := block.Name
				if functionName != "" && !IsAvailableOpenTool(functionName) && !IsAvailableMCPTool(functionName, a.op.mcpClient) {
					Warnf("Skipping tool call with unknown function name: %s", functionName)
					continue
				}
				// ContentBlockStartEventContentBlockUnion fields are embedded.
				// We can access ID, Name directly.
				currentToolUse = &anthropic.ToolUseBlockParam{
					ID:   functionID,
					Name: functionName,
					Type: constant.ToolUse("tool_use"),
				}
				currentInputBuilder.Reset()

			case "text":
			case "thinking":
				// Start Thinking (signature is typically empty at start)
				a.op.status.ChangeTo(a.op.notify, StreamNotify{Status: StatusReasoning}, a.op.proceed)
			}

		case "content_block_delta":
			evt := event.AsContentBlockDelta()
			delta := evt.Delta
			// Delta types: "text_delta", "input_json_delta", "thinking_delta", "signature_delta"
			switch delta.Type {
			case "text_delta":
				text := delta.Text
				contentBuilder.WriteString(text)
				a.op.data <- StreamData{Text: text, Type: DataTypeNormal}
			case "thinking_delta":
				text := delta.Thinking
				thinkingBuilder.WriteString(text)
				a.op.data <- StreamData{Text: text, Type: DataTypeReasoning}
			case "signature_delta":
				// Bugfix: messages.1.content.0.thinking.signature.str: Input should be a valid string
				// CRITICAL: This is where the signature is actually streamed!
				// Anthropic require thinking signature!
				signature := delta.Signature
				thinkingSignature = thinkingSignature + signature
				// Debugf("Signature delta received: [%s], accumulated: [%s]", signature, thinkingSignature)
			case "input_json_delta":
				currentInputBuilder.WriteString(delta.PartialJSON)
			}

		case "content_block_stop":
			if currentBlockType == "thinking" {
				a.op.status.ChangeTo(a.op.notify, StreamNotify{Status: StatusReasoningOver}, a.op.proceed)
			}
			if currentToolUse != nil {
				var input interface{}
				if err := json.Unmarshal([]byte(currentInputBuilder.String()), &input); err == nil {
					currentToolUse.Input = input
				} else {
					currentToolUse.Input = interface{}(currentInputBuilder.String())
				}
				toolCalls = append(toolCalls, *currentToolUse)

				cb := anthropic.NewToolUseBlock(
					currentToolUse.ID,
					input,
					currentToolUse.Name,
				)
				contentBlocks = append(contentBlocks, cb)

				currentToolUse = nil
			}

		case "message_delta":
			evt := event.AsMessageDelta()
			// For anthropic model, cache read tokens are included in the message usage
			// Because cached tokens are not in the prompt tokens, so we need to count them
			usage.RecordTokenUsage(
				int(evt.Usage.InputTokens),
				int(evt.Usage.OutputTokens),
				int(evt.Usage.CacheReadInputTokens),
				0,
				int(evt.Usage.InputTokens+evt.Usage.OutputTokens+evt.Usage.CacheReadInputTokens),
			)
			// Debugf("Anthropic Usage(message delta): %v", evt.Usage)

		case "message_stop":
			// Finished
		}
	}

	if err := stream.Err(); err != nil {
		return anthropic.MessageParam{}, nil, usage, err
	}

	// Finalize message construction
	textContent := contentBuilder.String()
	thinkingContent := thinkingBuilder.String()

	finalBlocks := []anthropic.ContentBlockParamUnion{}

	// 1. Add Thinking Block first if present
	if thinkingContent != "" {
		// Debugf("Creating thinking block with signature: [%s], content length: %d", thinkingSignature, len(thinkingContent))
		thinkingBlock := anthropic.NewThinkingBlock(thinkingSignature, thinkingContent)
		finalBlocks = append(finalBlocks, thinkingBlock)
	}

	// 2. Add Text Block if present
	if textContent != "" {
		textBlock := anthropic.NewTextBlock(textContent)
		finalBlocks = append(finalBlocks, textBlock)
	}

	// 3. Add Tool blocks (which are already in contentBlocks)
	// Note: contentBlocks currently holds tool use blocks accumulated during the stream
	finalBlocks = append(finalBlocks, contentBlocks...)

	message.Content = finalBlocks

	return message, toolCalls, usage, nil
}

func (a *Anthropic) processToolCall(toolCall anthropic.ToolUseBlockParam) (anthropic.MessageParam, error) {
	// Parse the query from the arguments
	var argsMap map[string]interface{}
	inputVal := toolCall.Input
	if m, ok := inputVal.(map[string]interface{}); ok {
		argsMap = m
	} else {
		return anthropic.MessageParam{}, fmt.Errorf("invalid tool input arguments: %v", inputVal)
	}

	var filteredArgs map[string]interface{}
	if toolCall.Name == ToolEditFile || toolCall.Name == ToolWriteFile {
		// Don't show content(the modified content could be too long)
		filteredArgs = FilterOpenToolArguments(argsMap, []string{"content", "edits"})
	} else {
		filteredArgs = FilterOpenToolArguments(argsMap, []string{})
	}

	// Notify UI
	toolCallData := map[string]interface{}{
		"function": toolCall.Name,
		"args":     filteredArgs,
	}
	jsonData, _ := json.Marshal(toolCallData)
	a.op.status.ChangeTo(a.op.notify, StreamNotify{Status: StatusFunctionCalling, Data: string(jsonData)}, a.op.proceed)

	var msg anthropic.MessageParam
	var err error

	// Define tool handlers map
	toolHandlers := map[string]func(anthropic.ToolUseBlockParam, *map[string]interface{}) (anthropic.MessageParam, error){
		ToolShell:             a.op.AnthropicShellToolCall,
		ToolWebFetch:          a.op.AnthropicWebFetchToolCall,
		ToolWebSearch:         a.op.AnthropicWebSearchToolCall,
		ToolReadFile:          a.op.AnthropicReadFileToolCall,
		ToolWriteFile:         a.op.AnthropicWriteFileToolCall,
		ToolEditFile:          a.op.AnthropicEditFileToolCall,
		ToolCreateDirectory:   a.op.AnthropicCreateDirectoryToolCall,
		ToolListDirectory:     a.op.AnthropicListDirectoryToolCall,
		ToolDeleteFile:        a.op.AnthropicDeleteFileToolCall,
		ToolDeleteDirectory:   a.op.AnthropicDeleteDirectoryToolCall,
		ToolMove:              a.op.AnthropicMoveToolCall,
		ToolCopy:              a.op.AnthropicCopyToolCall,
		ToolSearchFiles:       a.op.AnthropicSearchFilesToolCall,
		ToolSearchTextInFile:  a.op.AnthropicSearchTextInFileToolCall,
		ToolReadMultipleFiles: a.op.AnthropicReadMultipleFilesToolCall,
		ToolListMemory:        a.op.AnthropicListMemoryToolCall,
		ToolSaveMemory:        a.op.AnthropicSaveMemoryToolCall,
		ToolSwitchAgent:       a.op.AnthropicSwitchAgentToolCall,
		ToolListAgent:         a.op.AnthropicListAgentToolCall,
		ToolSpawnSubAgents:    a.op.AnthropicSpawnSubAgentsToolCall,
		ToolGetState:          a.op.AnthropicGetStateToolCall,
		ToolSetState:          a.op.AnthropicSetStateToolCall,
		ToolListState:         a.op.AnthropicListStateToolCall,
		ToolActivateSkill:     a.op.AnthropicActivateSkillToolCall,
	}

	if handler, ok := toolHandlers[toolCall.Name]; ok {
		msg, err = handler(toolCall, &argsMap)
	} else if a.op.mcpClient != nil && a.op.mcpClient.FindTool(toolCall.Name) != nil {
		// Handle MCP tool calls
		msg, err = a.op.AnthropicMCPToolCall(toolCall, &argsMap)
	} else {
		errorMsg := fmt.Sprintf("Error: Unknown function '%s'. This function is not available. Please use one of the available functions from the tool list.", toolCall.Name)
		toolResult := anthropic.NewToolResultBlock(toolCall.ID, errorMsg, true)
		msg = anthropic.NewUserMessage(toolResult)
		// Warn the user
		a.op.status.ChangeTo(a.op.notify, StreamNotify{Status: StatusWarn, Data: fmt.Sprintf("Model attempted to call unknown function: %s", toolCall.Name)}, nil)
		err = nil // Error is captured in the tool result
	}

	// Function call is done
	a.op.status.ChangeTo(a.op.notify, StreamNotify{Status: StatusFunctionCallingOver}, a.op.proceed)
	return msg, err
}

func (ag *Agent) SortAnthropicMessagesByOrder() error {
	// Load
	err := ag.Convo.Load()
	if err != nil {
		return err
	}

	history, _ := ag.Convo.GetMessages().([]anthropic.MessageParam)

	// User Message
	var userContent []anthropic.ContentBlockParamUnion

	// Add Text
	if ag.UserPrompt != "" {
		userContent = append(userContent, anthropic.NewTextBlock(ag.UserPrompt))
	}

	// Add Files
	for _, file := range ag.Files {
		if file != nil {
			part := ag.getAnthropicFilePart(file)
			if part != nil {
				userContent = append(userContent, *part) // Dereference pointer
			}
		}
	}

	if len(userContent) > 0 {
		userMsg := anthropic.NewUserMessage(userContent...)
		history = append(history, userMsg)
	}

	ag.Convo.SetMessages(history)
	// Bugfix: save conversation after update messages
	// Although system message wouldn't needed, but it's better to save it for consistency
	return ag.Convo.Save()
}

func (ag *Agent) getAnthropicTools() []anthropic.ToolUnionParam {
	var tools []anthropic.ToolUnionParam
	genericTools := GetOpenToolsFiltered(ag.EnabledTools)
	for _, genericTool := range genericTools {
		tools = append(tools, genericTool.ToAnthropicTool())
	}
	return tools
}

func (ag *Agent) getAnthropicMCPTools() []anthropic.ToolUnionParam {
	var tools []anthropic.ToolUnionParam
	if ag.MCPClient != nil {
		mcpTools := getMCPTools(ag.MCPClient)
		for _, mcpTool := range mcpTools {
			tools = append(tools, mcpTool.ToAnthropicTool())
		}
	}
	return tools
}

func addUpAnthropicTokenUsage(ag *Agent, usage *TokenUsage) {
	// Anthropic doesn't include cached tokens in the prompt tokens
	// So we need to set CachedTokensInPrompt to false
	// Anthropic model doesn't include Thought Tokens (always be 0)
	// and Cached Tokens are not included in the Input Tokens
	// so total tokens = input tokens + output tokens + cached tokens
	if ag.TokenUsage != nil && usage != nil {
		ag.TokenUsage.CachedTokensInPrompt = false
		ag.TokenUsage.RecordTokenUsage(
			usage.InputTokens,
			usage.OutputTokens,
			usage.CachedTokens,
			usage.ThoughtTokens,
			usage.TotalTokens,
		)
	}
}
