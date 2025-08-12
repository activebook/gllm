package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/volcengine/volcengine-go-sdk/service/arkruntime"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/utils"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
)

type streamState int

const (
	stateNormal streamState = iota
	stateReasoning
)

func (ll *LangLogic) GenerateOpenChatStream() error {
	// if !ll.UseSearchTool {
	// 	return ll.openchatStream()
	// } else {
	// 	return ll.openchatStreamWithSearch()
	// }

	return ll.openchatStreamWithSearch()
}

func (ll *LangLogic) getOpenChatFilePart(file *FileData) *model.ChatCompletionMessageContentPart {

	var part *model.ChatCompletionMessageContentPart
	format := file.Format()
	// Handle based on file type
	if IsImageMIMEType(format) {
		// Create base64 image URL
		base64Data := base64.StdEncoding.EncodeToString(file.Data())
		//imageURL := fmt.Sprintf("data:image/%s;base64,%s", file.Format(), base64Data)
		// data:format;base64,base64Data
		imageURL := fmt.Sprintf("data:%s;base64,%s", file.Format(), base64Data)
		// Create and append image part
		part = &model.ChatCompletionMessageContentPart{
			Type: model.ChatCompletionMessageContentPartTypeImageURL,
			ImageURL: &model.ChatMessageImageURL{
				URL: imageURL,
			},
		}
	} else if IsTextMIMEType(format) {
		// Create and append text part
		part = &model.ChatCompletionMessageContentPart{
			Type: model.ChatCompletionMessageContentPartTypeText,
			Text: string(file.Data()),
		}
	} else {
		// Unknown file type, skip
		// Don't deal with pdf, xls
		// It needs upload to OpenAI's servers first, so we can't include them directly in a message.
	}
	return part
}

func (ll *LangLogic) openchatStreamWithSearch() error {

	// 1. Initialize the Client
	ctx := context.Background()
	// Create a client config with custom base URL
	client := arkruntime.NewClientWithApiKey(
		ll.ApiKey,
		arkruntime.WithTimeout(30*time.Minute),
		arkruntime.WithBaseUrl(ll.EndPoint),
	)

	// Create a tool with the function
	tools := []*model.Tool{}
	if IsExecPluginLoaded() {
		commandTool := ll.getOpenChatCommandTool()
		tools = append(tools, commandTool)
	}
	if ll.UseSearchTool {
		searchTool := ll.getOpenChatSearchTool()
		tools = append(tools, searchTool)
	}

	chat := &OpenChat{
		client:        client,
		ctx:           &ctx,
		model:         ll.ModelName,
		temperature:   ll.Temperature,
		tools:         tools,
		proc:          ll.ProcChan,
		proceed:       ll.ProceedChan,
		references:    make([]*map[string]interface{}, 0, 1),
		maxRecursions: ll.MaxRecursions, // Use configured value from LangLogic
	}

	// 2. Prepare the Messages for Chat Completion
	// Initialize messages slice with proper capacity
	messages := make([]*model.ChatCompletionMessage, 0, 2)

	// Only add system message if not empty
	if ll.SystemPrompt != "" {
		messages = append(messages, &model.ChatCompletionMessage{
			Role: model.ChatMessageRoleSystem,
			Content: &model.ChatCompletionMessageContent{
				StringValue: volcengine.String(ll.SystemPrompt),
			}, Name: Ptr(""),
		})
	}

	var userMessage *model.ChatCompletionMessage
	// Add user message
	userMessage = &model.ChatCompletionMessage{
		Role: model.ChatMessageRoleUser,
		Content: &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(ll.UserPrompt),
		}, Name: Ptr(""),
	}
	messages = append(messages, userMessage)
	// Add image parts if available
	if len(ll.Files) > 0 {
		userMessage = &model.ChatCompletionMessage{
			Role:    model.ChatMessageRoleUser,
			Content: &model.ChatCompletionMessageContent{ListValue: []*model.ChatCompletionMessageContentPart{}},
			Name:    Ptr(""),
		}
		// Add all files
		for _, file := range ll.Files {
			if file != nil {
				part := ll.getOpenChatFilePart(file)
				if part != nil {
					userMessage.Content.ListValue = append(userMessage.Content.ListValue, part)
				}
			}
		}
		messages = append(messages, userMessage)
	}

	// Signal that streaming has started
	ll.ProcChan <- StreamNotify{Status: StatusStarted}
	<-ll.ProceedChan // Wait for the main goroutine to tell sub-goroutine to proceed

	// Load previous messages if any
	convo := GetOpenChatConversation()
	err := convo.Load()
	if err != nil {
		ll.ProcChan <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("failed to load conversation: %v", err)}
		return err
	}
	convo.PushMessages(messages) // Add new messages to the conversation

	// Process the chat with recursive tool call handling
	err = chat.process()
	if err != nil {
		return fmt.Errorf("error processing chat: %v", err)
	}
	return nil
}

// Conversation manages the state of an ongoing conversation with an AI assistant
type OpenChat struct {
	client        *arkruntime.Client
	ctx           *context.Context
	model         string
	temperature   float32
	tools         []*model.Tool
	proc          chan<- StreamNotify       // Sub Channel to send notifications
	proceed       <-chan bool               // Main Channel to receive proceed signal
	maxRecursions int                       // Maximum number of recursions for model calls
	queries       []string                  // List of queries to be sent to the AI assistant
	references    []*map[string]interface{} // keep track of the references
}

func (ll *LangLogic) getOpenChatSearchTool() *model.Tool {

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

	searchFunc := model.FunctionDefinition{
		Name:        "web_search",
		Description: "Retrieve the most relevant and up-to-date information from the web.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "The search term or question to find information about.",
				},
			},
			"required": []string{"query"},
		},
	}
	searchTool := model.Tool{
		Type:     model.ToolTypeFunction,
		Function: &searchFunc,
	}

	return &searchTool
}

func (ll *LangLogic) getOpenChatCommandTool() *model.Tool {
	commandFunc := model.FunctionDefinition{
		Name:        "execute_command",
		Description: "Execute system commands on the user's device with confirmation.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "The command to be executed on the user's system.",
				},
				"description": map[string]interface{}{
					"type":        "string",
					"description": "Explanation of what this command will do.",
				},
				"need_confirm": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether this command requires explicit user confirmation before execution.",
					"default":     true,
				},
			},
			"required": []string{"command", "description", "need_confirm"},
		},
	}

	commandTool := model.Tool{
		Type:     model.ToolTypeFunction,
		Function: &commandFunc,
	}

	return &commandTool
}

func (c *OpenChat) process() error {
	convo := GetOpenChatConversation()

	// only allow 5 recursions
	i := 0
	for range c.maxRecursions {
		i++
		//Debugf("Processing conversation at times: %d\n", i)

		c.proc <- StreamNotify{Status: StatusProcessing}

		// Create the request
		req := model.CreateChatCompletionRequest{
			Model:       c.model,
			Temperature: &c.temperature,
			Messages:    convo.Messages,
			Tools:       c.tools,
			// Thinking: &model.Thinking{
			// 	Type: model.ThinkingTypeAuto,
			// },
		}

		// Make the streaming request
		stream, err := c.client.CreateChatCompletionStream(*c.ctx, req)
		if err != nil {
			errMsg := fmt.Sprintf("Failed to create chat completion stream for model %s: %v", c.model, err)
			c.proc <- StreamNotify{Status: StatusError, Data: errMsg}
			return fmt.Errorf("stream creation error: %v", err)
		}
		defer func() {
			if stream != nil {
				stream.Close()
			}
		}()

		c.proc <- StreamNotify{Status: StatusStarted}
		<-c.proceed // Wait for the main goroutine to tell sub-goroutine to proceed

		// Process the stream and collect tool calls
		assistantMessage, toolCalls, err := c.processStream(stream)
		if err != nil {
			errMsg := fmt.Sprintf("Error while processing stream response: %v", err)
			c.proc <- StreamNotify{Status: StatusError, Data: errMsg}
			return fmt.Errorf("error processing stream: %v", err)
		}

		// Add the assistant's message to the conversation
		convo.PushMessage(assistantMessage)

		// If there are tool calls, process them
		if len(*toolCalls) > 0 {
			// Process each tool call
			for _, toolCall := range *toolCalls {
				toolMessage, err := c.processToolCall(toolCall)
				if err != nil {
					Warnf("Processing tool call: %v\n", err)
					// Send error info to user but continue processing other tool calls
					c.proc <- StreamNotify{Status: StatusData, Data: fmt.Sprintf("\n%sWarning:%s Error processing tool call '%s': %v\n", warnColor, resetColor, toolCall.Function.Name, err)}
					continue
				}
				// Add the tool response to the conversation
				convo.PushMessage(toolMessage)
			}
			// Continue the conversation recursively
		} else {
			// No function call and no model content
			break
		}
	}

	// Add queries to the output if any
	if len(c.queries) > 0 {
		q := "\n\n" + RetrieveQueries(c.queries)
		c.proc <- StreamNotify{Status: StatusData, Data: q}
	}
	// Add references to the output if any
	if len(c.references) > 0 {
		refs := "\n\n" + RetrieveReferences(c.references) + "\n"
		c.proc <- StreamNotify{Status: StatusData, Data: refs}
	}
	// No more message
	// Save the conversation
	err := convo.Save()
	if err != nil {
		errMsg := fmt.Sprintf("Failed to save conversation: %v", err)
		c.proc <- StreamNotify{Status: StatusError, Data: errMsg}
		return fmt.Errorf("failed to save conversation: %v", err)
	}
	c.proc <- StreamNotify{Status: StatusFinished}
	return nil
}

// processStream processes the stream and collects tool calls
func (c *OpenChat) processStream(stream *utils.ChatCompletionStreamReader) (*model.ChatCompletionMessage, *map[string]model.ToolCall, error) {
	assistantMessage := model.ChatCompletionMessage{
		Role: model.ChatMessageRoleAssistant,
		Name: Ptr(""),
	}
	toolCalls := make(map[string]model.ToolCall)
	contentBuffer := strings.Builder{}
	reasoningBuffer := strings.Builder{}
	lastCallId := ""

	state := stateNormal
	for {
		response, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("error receiving stream data: %v", err)
		}

		// Handle regular content
		if len(response.Choices) > 0 {
			delta := (response.Choices[0].Delta)

			// State transitions
			switch state {
			case stateNormal:
				if HasContent(delta.ReasoningContent) {
					c.proc <- StreamNotify{Status: StatusReasoning, Data: ""}
					state = stateReasoning
				}
			case stateReasoning:
				// If reasoning content is empty, switch back to normal state
				// This is to handle the case where reasoning content is empty but we already have content
				// Aka, the model is done with reasoning content and starting to output normal content
				if delta.ReasoningContent == nil ||
					(*delta.ReasoningContent == "" && delta.Content != "") {
					c.proc <- StreamNotify{Status: StatusReasoningOver, Data: ""}
					state = stateNormal
				}
			}

			if HasContent(delta.ReasoningContent) {
				// For reasoning model
				text := *delta.ReasoningContent
				reasoningBuffer.WriteString(text)
				c.proc <- StreamNotify{Status: StatusReasoningData, Data: text}
			} else if delta.Content != "" {
				text := delta.Content
				contentBuffer.WriteString(text)
				c.proc <- StreamNotify{Status: StatusData, Data: text}
			}

			// Handle tool calls in the stream
			if len(delta.ToolCalls) > 0 {
				for _, toolCall := range delta.ToolCalls {
					id := toolCall.ID
					functionName := toolCall.Function.Name

					// Skip if not our expected function
					// Because some model made up function name
					if functionName != "" && functionName != "web_search" && functionName != "execute_command" {
						continue
					}

					// Handle streaming tool call parts
					if id == "" && lastCallId != "" {
						// Continue with previous tool call
						if tc, exists := toolCalls[lastCallId]; exists {
							tc.Function.Arguments += toolCall.Function.Arguments
							toolCalls[lastCallId] = tc
						}
					} else if id != "" {
						// Create or update a tool call
						lastCallId = id
						if tc, exists := toolCalls[id]; exists {
							tc.Function.Arguments += toolCall.Function.Arguments
							toolCalls[id] = tc
						} else {
							// Prepare to receive tool call arguments
							c.proc <- StreamNotify{Status: StatusProcessing}
							toolCalls[id] = model.ToolCall{
								ID:   id,
								Type: model.ToolTypeFunction,
								Function: model.FunctionCall{
									Name:      functionName,
									Arguments: toolCall.Function.Arguments,
								},
							}
						}
					}
				}
			}
		}
	}

	// Update the assistant reasoning message
	reasoning_content := reasoningBuffer.String()
	if reasoning_content != "" {
		assistantMessage.ReasoningContent = &reasoning_content
	}
	// Set the content of the assistant message
	content := contentBuffer.String()
	if content != "" {
		if !strings.HasSuffix(content, "\n") {
			content = content + "\n"
		}
		assistantMessage.Content = &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(content),
		}
	}

	// Add tool calls to the assistant message if there are any
	if len(toolCalls) > 0 {
		var assistantToolCalls []*model.ToolCall
		for _, tc := range toolCalls {
			assistantToolCalls = append(assistantToolCalls, &tc)
		}
		assistantMessage.ToolCalls = assistantToolCalls
		// Function is ready to call
		c.proc <- StreamNotify{Status: StatusStarted}
		<-c.proceed
	}

	return &assistantMessage, &toolCalls, nil
}

// processToolCall processes a single tool call and returns a tool response message
func (c *OpenChat) processToolCall(toolCall model.ToolCall) (*model.ChatCompletionMessage, error) {
	// Parse the query from the arguments
	var argsMap map[string]interface{}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &argsMap); err != nil {
		return nil, fmt.Errorf("error parsing arguments: %v", err)
	}

	switch toolCall.Function.Name {
	case "execute_command":
		return c.processCommandToolCalls(&toolCall, &argsMap)
	case "web_search":
		return c.processSearchToolCalls(&toolCall, &argsMap)
	default:
		return nil, fmt.Errorf("unknown function name: %s", toolCall.Function.Name)
	}
}

func (c *OpenChat) processCommandToolCalls(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	// Create a tool message
	// Tool Message's Content wouldn't be serialized in the response
	// That's not a problem, because each time, the tool result could be different!
	toolMessage := model.ChatCompletionMessage{
		Role:       model.ChatMessageRoleTool,
		ToolCallID: toolCall.ID,
		Name:       Ptr(""),
	}

	cmdStr, ok := (*argsMap)["command"].(string)
	if !ok {
		return nil, fmt.Errorf("command not found in arguments for tool call ID %s", toolCall.ID)
	}
	needConfirm, ok := (*argsMap)["need_confirm"].(bool)
	if !ok {
		needConfirm = true
	}
	if needConfirm {
		// Response with a prompt to let user confirm
		descStr, ok := (*argsMap)["description"].(string)
		if !ok {
			return nil, fmt.Errorf("description not found in arguments for tool call ID %s", toolCall.ID)
		}
		outStr := fmt.Sprintf(ExecRespTmplConfirm, cmdStr, descStr)
		toolMessage.Content = &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(outStr),
		}
		return &toolMessage, nil
	}

	// Call function
	c.proc <- StreamNotify{Status: StatusData, Data: fmt.Sprintf("Function call: %v\n", toolCall.Function.Name)}

	// Log that we're executing the command
	c.proc <- StreamNotify{Status: StatusData, Data: fmt.Sprintf("%s\n", cmdStr)}
	var errStr string

	// Do the real command
	c.proc <- StreamNotify{Status: StatusFunctionCalling, Data: ""}
	var out []byte
	var err error
	if runtime.GOOS == "windows" {
		out, err = exec.Command("cmd", "/C", cmdStr).CombinedOutput()
	} else {
		out, err = exec.Command("sh", "-c", cmdStr).CombinedOutput()
	}
	c.proc <- StreamNotify{Status: StatusFunctionCallingOver, Data: ""}
	<-c.proceed

	// Handle command exec failed
	if err != nil {
		var exitCode int
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		}
		errStr = fmt.Sprintf("Command failed with exit code %d: %v", exitCode, err)
		// Send error details to user
		c.proc <- StreamNotify{Status: StatusData, Data: fmt.Sprintf("\n%sError:%s %s\n", errorColor, resetColor, errStr)}
	}

	// Output the result
	outStr := string(out)
	if outStr != "" {
		outStr = outStr + "\n"
		c.proc <- StreamNotify{Status: StatusData, Data: outStr}
	}

	// Format error info if present
	errorInfo := ""
	if errStr != "" {
		errorInfo = fmt.Sprintf("Error: %s", errStr)
	}
	// Format output info
	outputInfo := ""
	if outStr != "" {
		outputInfo = fmt.Sprintf("Output:\n%s", outStr)
	} else {
		outputInfo = "Output: <no output>"
	}
	// Create a response that prompts the LLM to provide insightful analysis of the command output
	finalResponse := fmt.Sprintf(ExecRespTmplOutput, cmdStr, errorInfo, outputInfo)

	// Create and return the tool response message
	toolMessage.Content = &model.ChatCompletionMessageContent{
		StringValue: volcengine.String(finalResponse),
	}
	return &toolMessage, nil
}

func (c *OpenChat) processSearchToolCalls(toolCall *model.ToolCall, argsMap *map[string]interface{}) (*model.ChatCompletionMessage, error) {
	query, ok := (*argsMap)["query"].(string)
	if !ok {
		return nil, fmt.Errorf("query not found in arguments for tool call ID %s", toolCall.ID)
	}

	Debugf("\nFunction Calling: %s(%+v)\n", toolCall.Function.Name, query)
	c.proc <- StreamNotify{Status: StatusSearching, Data: ""}

	// Call the search function
	engine := GetSearchEngine()
	var data map[string]any
	var err error
	switch engine {
	case GoogleSearchEngine:
		// Use Google Search Engine
		data, err = GoogleSearch(query)
	case BingSearchEngine:
		// Use Bing Search Engine
		data, err = BingSearch(query)
	case TavilySearchEngine:
		// Use Tavily Search Engine
		data, err = TavilySearch(query)
	case NoneSearchEngine:
		// Use None Search Engine
		data, err = NoneSearch(query)
	default:
		err = fmt.Errorf("unknown search engine: %s", engine)
	}

	if err != nil {
		c.proc <- StreamNotify{Status: StatusSearchingOver, Data: ""}
		<-c.proceed
		Warnf("Performing search: %v", err)
		errMsg := fmt.Sprintf("Error performing search for query '%s': %v", query, err)
		c.proc <- StreamNotify{Status: StatusData, Data: fmt.Sprintf("\n%sError:%s %s\n", errorColor, resetColor, errMsg)}
		return nil, fmt.Errorf("error performing search: %v", err)
	}
	// keep the search results for references
	c.queries = append(c.queries, query)
	c.references = append(c.references, &data)

	// Convert search results to JSON string
	resultsJSON, err := json.Marshal(data)
	if err != nil {
		// TODO: Potentially send an error FunctionResponse back to the model
		c.proc <- StreamNotify{Status: StatusSearchingOver, Data: ""}
		<-c.proceed
		errMsg := fmt.Sprintf("Error marshaling search results for query '%s': %v", query, err)
		c.proc <- StreamNotify{Status: StatusData, Data: fmt.Sprintf("\n%sError:%s %s\n", errorColor, resetColor, errMsg)}
		return nil, fmt.Errorf("error marshaling results: %v", err)
	}

	c.proc <- StreamNotify{Status: StatusSearchingOver, Data: ""}
	<-c.proceed
	// Create and return the tool response message
	return &model.ChatCompletionMessage{
		Role: model.ChatMessageRoleTool,
		Content: &model.ChatCompletionMessageContent{
			StringValue: volcengine.String(string(resultsJSON)),
		}, Name: Ptr(""),
		ToolCallID: toolCall.ID,
	}, nil
}
