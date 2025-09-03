package service

import (
	"fmt"
)

const (
	// Terminal colors
	inReasoningColor = "\033[90m" // Bright Black
	inCallingColor   = "\033[36m" // Cyan
	completeColor    = "\033[32m" // Green
)

type StreamDataType int

const (
	DataTypeUnknown   StreamDataType = iota
	DataTypeNormal                   // 1
	DataTypeReasoning                // 2
	DataTypeFinished                 // 3
)

type StreamData struct {
	Text string
	Type StreamDataType
}

type Agent struct {
	ApiKey          string
	EndPoint        string
	ModelName       string
	SystemPrompt    string
	UserPrompt      string
	Temperature     float32
	Files           []*FileData         // Attachment files
	NotifyChan      chan<- StreamNotify // Sub Channel to send notifications
	DataChan        chan<- StreamData   // Sub Channel to receive streamed text data
	ProceedChan     <-chan bool         // Sub Channel to receive proceed signal
	SearchEngine    SearchEngine        // Search engine name
	ToolsUse        ToolsUse            // Use tools
	UseCodeTool     bool                // Use code tool
	ThinkMode       bool                // Think mode
	MCPClient       *MCPClient          // MCP client for MCP tools
	MaxRecursions   int                 // Maximum number of recursions for model calls
	Markdown        *Markdown           // Markdown renderer
	TokenUsage      *TokenUsage         // Token usage metainfo
	Std             *StdRenderer        // Standard renderer
	OutputFile      *FileRenderer       // File renderer
	Status          StatusStack         // Stack to manage streaming status
	Convo           ConversationManager // Conversation manager
	Indicator       *Indicator          // Indicator Spinner
	LastWrittenData string              // Last written data
}

func constructSearchEngine(searchEngine *map[string]any) *SearchEngine {
	se := SearchEngine{}
	se.UseSearch = false
	if searchEngine != nil {
		se.UseSearch = true
		if name, ok := (*searchEngine)["name"]; ok {
			se.Name = name.(string)
		} else {
			se.UseSearch = false
			se.Name = ""
		}
		if keyValue, ok := (*searchEngine)["key"]; ok {
			se.ApiKey = keyValue.(string)
		} else {
			se.UseSearch = false
			se.ApiKey = ""
		}
		if cxValue, ok := (*searchEngine)["cx"]; ok {
			se.CxKey = cxValue.(string)
		} else {
			se.CxKey = ""
		}
		if deepDive, ok := (*searchEngine)["deep_dive"]; ok {
			se.DeepDive = deepDive.(bool)
		} else {
			se.DeepDive = false
		}
		if references, ok := (*searchEngine)["references"]; ok {
			se.MaxReferences = references.(int)
		} else {
			se.MaxReferences = 5
		}
	}

	return &se
}

func ConstructConversationManager(convoName string, provider ModelProvider) (ConversationManager, error) {
	//var convo ConversationManager
	switch provider {
	case ModelOpenChat:
		// Used for Chinese Models
		convo := OpenChatConversation{}
		err := convo.Open(convoName)
		if err != nil {
			return nil, err
		}
		return &convo, nil

	case ModelOpenAI, ModelMistral, ModelOpenAICompatible:
		// Used for OpenAI compatible models
		convo := OpenAIConversation{}
		err := convo.Open(convoName)
		if err != nil {
			return nil, err
		}
		return &convo, nil

	case ModelGemini:
		// Used for Gemini
		convo := Gemini2Conversation{}
		err := convo.Open(convoName)
		if err != nil {
			return nil, err
		}
		return &convo, nil

	default:
		convo := BaseConversation{}
		return &convo, nil
	}
}

type AgentOptions struct {
	Prompt           string
	SysPrompt        string
	Files            []*FileData
	ModelInfo        *map[string]any
	SearchEngine     *map[string]any
	MaxRecursions    int
	ThinkMode        bool
	UseTools         bool
	UseMCP           bool
	SkipToolsConfirm bool
	AppendMarkdown   bool
	AppendUsage      bool
	OutputFile       string
	QuietMode        bool
	ConvoName        string
}

func CallAgent(op *AgentOptions) error {
	var temperature float32
	switch temp := (*op.ModelInfo)["temperature"].(type) {
	case float64:
		temperature = float32(temp)
	case int:
		temperature = float32(temp)
	case int64:
		temperature = float32(temp)
	case float32:
		temperature = temp
	default:
		// Set a default value if type is unexpected
		temperature = 0.7 // or whatever default makes sense
	}

	// Set up search engine settings
	se := constructSearchEngine(op.SearchEngine)
	toolsUse := ToolsUse{Enable: op.UseTools, AutoApprove: op.SkipToolsConfirm}

	// Set up code tool settings
	exeCode := IsCodeExecutionEnabled()

	// Set up MCP client
	var mc *MCPClient
	if op.UseMCP {
		mc = &MCPClient{}
		err := mc.Init(false)
		if err != nil {
			err := fmt.Errorf("failed to init MCPServers: %v", err)
			return err
		}
	}

	// Create a channel to receive notifications
	notifyCh := make(chan StreamNotify, 10) // Buffer to prevent blocking(used for status updates)
	dataCh := make(chan StreamData, 10)     // Buffer to prevent blocking(used for streamed text data)
	proceedCh := make(chan bool)            // For main -> sub communication

	// active channels used in select (can be set to nil to disable)
	activeNotifyCh := notifyCh
	activeDataCh := dataCh

	// Only create StdRenderer if not in quiet mode
	var indicator *Indicator
	var std *StdRenderer
	if !op.QuietMode {
		std = NewStdRenderer()
		indicator = NewIndicator("Processing...")
	}

	// Need to append markdown
	var markdown *Markdown
	if op.AppendMarkdown {
		markdown = NewMarkdown()
	}

	// Need to append token usage
	var tu *TokenUsage
	if op.AppendUsage {
		tu = NewTokenUsage()
	}

	// Need to output a file
	var fileRenderer *FileRenderer
	if op.OutputFile != "" {
		var err error
		fileRenderer, err = NewFileRenderer(op.OutputFile)
		if err != nil {
			err := fmt.Errorf("failed to create output file %s: %v", op.OutputFile, err)
			return err
		}
		defer fileRenderer.Close()
	}

	ag := Agent{
		ApiKey:        (*op.ModelInfo)["key"].(string),
		EndPoint:      (*op.ModelInfo)["endpoint"].(string),
		ModelName:     (*op.ModelInfo)["model"].(string),
		SystemPrompt:  op.SysPrompt,
		UserPrompt:    op.Prompt,
		Temperature:   temperature,
		Files:         op.Files,
		NotifyChan:    notifyCh,
		DataChan:      dataCh,
		ProceedChan:   proceedCh,
		SearchEngine:  *se,
		ToolsUse:      toolsUse,
		UseCodeTool:   exeCode,
		MCPClient:     mc,
		ThinkMode:     op.ThinkMode,
		MaxRecursions: op.MaxRecursions,
		Markdown:      markdown,
		TokenUsage:    tu,
		Std:           std,
		OutputFile:    fileRenderer,
		Status:        StatusStack{},
		Indicator:     indicator,
	}

	// Check if the endpoint is compatible with OpenAI
	provider := DetectModelProvider(ag.EndPoint)

	// Construct conversation manager
	cm, err := ConstructConversationManager(op.ConvoName, provider)
	if err != nil {
		return err
	}
	ag.Convo = cm

	// Start the generation in a goroutine
	go func() {
		defer func() {
			// Recover from panics and convert them to errors
			if r := recover(); r != nil {
				errMsg := fmt.Sprintf("Panic occurred: %v", r)
				notifyCh <- StreamNotify{Status: StatusError, Data: errMsg}
			}
		}()

		switch provider {
		case ModelOpenChat:
			// Used for Chinese Models, they use "thinking[enable/disable]" as extra_body
			if err := ag.GenerateOpenChatStream(); err != nil {
				// Send error through channel instead of returning
				notifyCh <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("%v", err)}
			}
		case ModelOpenAI, ModelMistral, ModelOpenAICompatible:
			// Used for OpenAI compatible models
			if err := ag.GenerateOpenAIStream(); err != nil {
				// Send error through channel instead of returning
				notifyCh <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("%v", err)}
			}
		case ModelGemini:
			if err := ag.GenerateGemini2Stream(); err != nil {
				// Send error through channel instead of returning
				notifyCh <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("%v", err)}
			}
		default:
			notifyCh <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("Unsupported model provider: %s", provider)}
		}
	}()

	defer close(notifyCh)
	defer close(dataCh)
	defer close(proceedCh)

	// Error variable to store any error from the goroutine
	var processingErr error

	// Process notifications in the main thread
	// listen on multiple channels in Go, it listens to them simultaneously.
	// If both data channel and notification channel have something to be read at the same time,
	// it is indeed possible for either one to be selected.
	for {
		select {

		// Handle streamed text data
		case data := <-activeDataCh:

			// disable notify channel while processing data
			activeNotifyCh = nil

			switch data.Type {
			case DataTypeNormal:
				// Render the streamed text and save to markdown buffer
				ag.WriteText(data.Text)

			case DataTypeReasoning:
				// Reasoning data don't need to be saved to markdown buffer
				ag.WriteReasoning(data.Text)

			case DataTypeFinished:
				// Wait all data to be processed(flush)
				// This is important, otherwise notify will be processed before data finished
				ag.WriteEnd()
				proceedCh <- true
			default:
				// Handle other data types if needed
			}

			// check the number of buffered elements currently in data channel.
			// If there are more data, we can't proceed with notify channel
			// Because data is continuously being processed, aka consequent data
			if len(activeDataCh) == 0 {
				// If there are no more data, we can proceed
				// Re-enable notify channel
				activeNotifyCh = notifyCh
			}

		// Handle status notifications
		// Remember in order to process status notifications,
		// we need to proceedCh to confirm
		case notify := <-activeNotifyCh:

			// disable data channel while processing notify
			activeDataCh = nil

			switch notify.Status {
			case StatusProcessing:
				ag.StartIndicator("Processing...")
				proceedCh <- true
			case StatusStarted:
				ag.StopIndicator()
				proceedCh <- true
			case StatusWarn:
				// Just show warning
				ag.Warn(notify.Data)
			case StatusError:
				// Error happened, stop
				ag.StopIndicator()
				ag.Error(notify.Data)
				processingErr = fmt.Errorf("%s", notify.Data)
				return processingErr
			case StatusFinished:
				ag.StopIndicator()
				// Render the markdown
				ag.WriteMarkdown()
				// Render the token usage
				ag.WriteUsage()
				// Return any error that might have occurred
				// If there wasn't any error, return nil
				return processingErr
			case StatusReasoning:
				ag.StopIndicator()
				// Start with Thinking color
				ag.StartReasoning()
				proceedCh <- true
			case StatusReasoningOver:
				// Complete Thinking color at the end
				ag.CompleteReasoning()
				proceedCh <- true
			case StatusFunctionCalling:
				ag.WriteFunctionCall(notify.Data)
				ag.StartIndicator("Function Calling...")
				proceedCh <- true
			case StatusFunctionCallingOver:
				ag.StopIndicator()
				proceedCh <- true
			}

			// Re-enable data channel
			activeDataCh = dataCh

			// Without an explicit default clause in a select statement,
			// the select will block until at least one of its case statements can proceed.
			// This can lead to a costly loop, which would spin the CPU
			// So don't add explicit default clause here

			//default:
			// Dont' do it!!!
		}
	}
}

/*
WriteText writes the given text to the Agent's Std, Markdown, and OutputFile writers if they are set.
*/
func (ag *Agent) WriteText(text string) {
	if ag.Std != nil {
		ag.Std.Writef("%s", text)
		ag.LastWrittenData = text
	}
	if ag.Markdown != nil {
		ag.Markdown.Writef("%s", text)
	}
	if ag.OutputFile != nil {
		ag.OutputFile.Writef("%s", text)
	}
}

/*
StartReasoning notifies the user and logs to file that the agent has started thinking.
It writes a status message to both Std and OutputFile if they are available.
*/
func (ag *Agent) StartReasoning() {
	if ag.Std != nil {
		ag.Std.Writeln(completeColor + "Thinking ↓")
	}
	if ag.OutputFile != nil {
		ag.OutputFile.Writeln("Thinking ↓")
	}
}

func (ag *Agent) CompleteReasoning() {
	if ag.Std != nil {
		ag.Std.Writeln(resetColor + completeColor + "✓" + resetColor)
	}
	if ag.OutputFile != nil {
		ag.OutputFile.Writeln("✓")
	}
}

/*
WriteReasoning writes the provided reasoning text to both the standard output and an output file, applying specific formatting to each if they are available.
*/
func (ag *Agent) WriteReasoning(text string) {
	if ag.Std != nil {
		ag.Std.Writef("%s%s", inReasoningColor, text)
	}
	if ag.OutputFile != nil {
		ag.OutputFile.Writef("%s", text)
	}
}

func (ag *Agent) WriteMarkdown() {
	// Render the markdown
	if ag.Markdown != nil {
		if ag.Std != nil {
			ag.Markdown.Render(ag.Std)
		}
	}
}

func (ag *Agent) WriteUsage() {
	// Render the token usage
	if ag.TokenUsage != nil {
		if ag.Std != nil {
			ag.TokenUsage.Render(ag.Std)
		}
	}
}

func (ag *Agent) WriteFunctionCall(text string) {
	if ag.Std != nil {
		ag.Std.Writeln(resetColor)
		ag.Std.Writeln(inCallingColor + text + resetColor)
	}
	if ag.OutputFile != nil {
		ag.OutputFile.Writef("\n%s\n", text)
	}
}

func (ag *Agent) WriteEnd() {
	// Ensure output ends with a newline to prevent shell from displaying %
	// the % character in shells like zsh when output doesn't end with newline
	//if ag.Std != nil && ag.Markdown == nil && ag.TokenUsage == nil {
	if ag.Std != nil {
		if !EndWithNewline(ag.LastWrittenData) {
			ag.Std.Writeln()
		}
	}
}

func (ag *Agent) StartIndicator(text string) {
	if ag.Indicator != nil {
		ag.Indicator.Start(text)
	}
}

func (ag *Agent) StopIndicator() {
	if ag.Indicator != nil {
		ag.Indicator.Stop()
	}
}

func (ag *Agent) Error(text string) {
	// ignore stdout, because CallAgent will return the error
	// if ag.Std != nil {
	// 	Errorf("Agent: %v\n", text)
	// }
	if ag.OutputFile != nil {
		ag.OutputFile.Writef("\n%s\n", text)
	}
}

func (ag *Agent) Warn(text string) {
	if ag.Std != nil {
		Warnf("%s", text)
	}
	if ag.OutputFile != nil {
		ag.OutputFile.Writef("\n%s\n", text)
	}
}
