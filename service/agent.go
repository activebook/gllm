package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/activebook/gllm/data"
	"github.com/charmbracelet/lipgloss"
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

type ModelInfo struct {
	ApiKey      string
	EndPoint    string
	ModelName   string
	Provider    string
	Temperature float32
	TopP        float32 // Top-p sampling parameter
	Seed        *int32  // Seed for deterministic generation
}

type Agent struct {
	Model           *ModelInfo
	SystemPrompt    string
	UserPrompt      string
	Files           []*FileData         // Attachment files
	NotifyChan      chan<- StreamNotify // Sub Channel to send notifications
	DataChan        chan<- StreamData   // Sub Channel to receive streamed text data
	ProceedChan     <-chan bool         // Sub Channel to receive proceed signal
	SearchEngine    *SearchEngine       // Search engine name
	ToolsUse        ToolsUse            // Use tools
	EnabledTools    []string            // List of enabled embedding tools
	UseCodeTool     bool                // Use code tool
	ThinkingLevel   ThinkingLevel       // Thinking level: off, low, medium, high
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

func constructModelInfo(model *data.Model) *ModelInfo {
	mi := ModelInfo{}
	provider := model.Provider
	if provider == "" {
		// Auto-detect provider if not set
		Debugf("Auto-detecting provider for %s", model.Model)
		provider = DetectModelProvider(model.Endpoint, model.Model)
	} else {
		Debugf("Provider: [%s]", provider)
	}
	mi.ModelName = model.Model
	mi.Provider = provider
	mi.EndPoint = model.Endpoint
	mi.ApiKey = model.Key
	mi.Temperature = model.Temp
	mi.TopP = model.TopP
	mi.Seed = model.Seed
	return &mi
}

func constructSearchEngine(searchEngine *data.SearchEngine) *SearchEngine {
	se := SearchEngine{}
	se.Name = GetNoneSearchEngineName()
	se.UseSearch = false
	if searchEngine != nil && searchEngine.Name != "" && searchEngine.Name != GetNoneSearchEngineName() {
		se.UseSearch = true
		se.Name = searchEngine.Name
		se.ApiKey = searchEngine.Config["key"]
		se.CxKey = searchEngine.Config["cx"]
		se.DeepDive = searchEngine.DeepDive
		se.MaxReferences = searchEngine.Reference
	}
	Debugf("Search engine: %v, %v", se.Name, se.UseSearch)
	return &se
}

func ConstructConversationManager(convoName string, provider string) (ConversationManager, error) {
	//var convo ConversationManager
	switch provider {
	case ModelProviderOpenAICompatible:
		// Used for Chinese Models
		convo := OpenChatConversation{}
		err := convo.Open(convoName)
		if err != nil {
			return nil, err
		}
		return &convo, nil

	case ModelProviderOpenAI:
		// Used for OpenAI compatible models
		convo := OpenAIConversation{}
		err := convo.Open(convoName)
		if err != nil {
			return nil, err
		}
		return &convo, nil

	case ModelProviderGemini:
		// Used for Gemini
		convo := Gemini2Conversation{}
		err := convo.Open(convoName)
		if err != nil {
			return nil, err
		}
		return &convo, nil

	case ModelProviderAnthropic:
		// Used for Anthropic
		convo := AnthropicConversation{}
		err := convo.Open(convoName)
		if err != nil {
			return nil, err
		}
		return &convo, nil

	default:
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}
}

type AgentOptions struct {
	Prompt         string
	SysPrompt      string
	Files          []*FileData
	ModelInfo      *data.Model
	SearchEngine   *data.SearchEngine
	MaxRecursions  int
	ThinkingLevel  string
	EnabledTools   []string // List of enabled embedding tools
	UseMCP         bool
	YoloMode       bool // Whether to automatically approve tools
	AppendMarkdown bool
	AppendUsage    bool
	OutputFile     string
	QuietMode      bool
	ConvoName      string
	MCPConfig      map[string]*data.MCPServer
}

func CallAgent(op *AgentOptions) error {

	// Set up model settings
	mi := constructModelInfo(op.ModelInfo)

	// Set up search engine settings
	se := constructSearchEngine(op.SearchEngine)
	toolsUse := ToolsUse{AutoApprove: op.YoloMode}

	// Set up code tool settings
	exeCode := IsCodeExecutionEnabled()

	// Set up thinking level
	thinkingLevel := ParseThinkingLevel(op.ThinkingLevel)

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

	// Set up MCP client
	var mc *MCPClient
	if op.UseMCP {
		mc = GetMCPClient() // use the shared instance
		if !op.QuietMode {
			indicator.Start("Loading MCP servers...")
		}
		err := mc.Init(op.MCPConfig, MCPLoadOption{
			LoadAll:   false,
			LoadTools: true, // only load tools
		}) // Load only allowed servers
		if !op.QuietMode {
			indicator.Stop()
		}
		if err != nil {
			return fmt.Errorf("failed to load MCPServers: %v", err)
		}
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
		Model:         mi,
		SystemPrompt:  op.SysPrompt,
		UserPrompt:    op.Prompt,
		Files:         op.Files,
		NotifyChan:    notifyCh,
		DataChan:      dataCh,
		ProceedChan:   proceedCh,
		SearchEngine:  se,
		ToolsUse:      toolsUse,
		EnabledTools:  op.EnabledTools,
		UseCodeTool:   exeCode,
		MCPClient:     mc,
		ThinkingLevel: thinkingLevel,
		MaxRecursions: op.MaxRecursions,
		Markdown:      markdown,
		TokenUsage:    tu,
		Std:           std,
		OutputFile:    fileRenderer,
		Status:        StatusStack{},
		Indicator:     indicator,
	}

	// Construct conversation manager
	cm, err := ConstructConversationManager(op.ConvoName, ag.Model.Provider)
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

		switch ag.Model.Provider {
		case ModelProviderOpenAICompatible:
			// Used for Chinese Models, they use "thinking[enable/disable]" as extra_body
			if err := ag.GenerateOpenChatStream(); err != nil {
				// Send error through channel instead of returning
				if IsSwitchAgentError(err) {
					notifyCh <- StreamNotify{Status: StatusSwitchAgent, Data: err.(*SwitchAgentError).TargetAgent}
				} else {
					notifyCh <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("%v", err)}
				}
			}
		case ModelProviderOpenAI:
			// Used for OpenAI compatible models
			if err := ag.GenerateOpenAIStream(); err != nil {
				// Send error through channel instead of returning
				if IsSwitchAgentError(err) {
					notifyCh <- StreamNotify{Status: StatusSwitchAgent, Data: err.(*SwitchAgentError).TargetAgent}
				} else {
					notifyCh <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("%v", err)}
				}
			}
		case ModelProviderGemini:
			if err := ag.GenerateGemini2Stream(); err != nil {
				// Send error through channel instead of returning
				if IsSwitchAgentError(err) {
					notifyCh <- StreamNotify{Status: StatusSwitchAgent, Data: err.(*SwitchAgentError).TargetAgent}
				} else {
					notifyCh <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("%v", err)}
				}
			}
		case ModelProviderAnthropic:
			if err := ag.GenerateAnthropicStream(); err != nil {
				// Send error through channel instead of returning
				if IsSwitchAgentError(err) {
					notifyCh <- StreamNotify{Status: StatusSwitchAgent, Data: err.(*SwitchAgentError).TargetAgent}
				} else {
					notifyCh <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("%v", err)}
				}
			}
		default:
			notifyCh <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("Unsupported model provider: %s", ag.Model.Provider)}
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
			case StatusSwitchAgent:
				// Switch agent signal, pop up
				ag.StopIndicator()
				ag.WriteEnd()
				return &SwitchAgentError{TargetAgent: notify.Data}
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
				ag.WriteEnd() // ensure previous data ends with newline, because function call box starts a new line
				ag.WriteFunctionCall(notify.Data)
				// ag.StartIndicator("Function Calling...")
				proceedCh <- true
			case StatusFunctionCallingOver:
				// ag.StopIndicator()
				proceedCh <- true
			case StatusDiffConfirm:
				ag.WriteDiffConfirm(notify.Data)
				proceedCh <- true
			case StatusDiffConfirmOver:
				ag.WriteDiffConfirm("") // just write a newline
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
		ag.Std.Writeln(reasoningColor + "Thinking ↓")
	}
	if ag.OutputFile != nil {
		ag.OutputFile.Writeln("Thinking ↓")
	}
}

func (ag *Agent) CompleteReasoning() {
	if ag.Std != nil {
		ag.Std.Writeln(resetColor + reasoningColor + "✓" + resetColor)
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
		ag.LastWrittenData = text
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

func (ag *Agent) WriteDiffConfirm(text string) {
	// Only write to stdout
	if ag.Std != nil {
		ag.Std.Writeln(text)
	}
}

func (ag *Agent) WriteFunctionCall(text string) {
	if ag.Std != nil {
		// Attempt to parse text as JSON
		// The text is expected to be in format "function_name(arguments)" or just raw text
		// But in our new implementation, we will pass a JSON string: {"function": name, "args": args}

		type ToolCallData struct {
			Function string      `json:"function"`
			Args     interface{} `json:"args"`
		}

		var data ToolCallData
		err := json.Unmarshal([]byte(text), &data)

		var output string
		if err == nil {
			// Make sure we have enough space for the border
			tcol := GetTerminalWidth() - 8

			// Structured data available
			// Use lipgloss to render
			style := lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color(hexToolResponse)).
				Padding(0, 1).
				Margin(0, 0)

			titleStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color(hexToolCall)).
				Bold(true)

			argsStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("250")).Width(tcol) // Light Gray

			var content string

			// For built-in tools, we have a map of args
			// We will try to extract purpose/description and command separately
			if argsMap, ok := data.Args.(map[string]interface{}); ok {
				// 1. Identify Purpose
				// MCP tool calls may not have purpose/description
				var purposeVal string
				if v, ok := argsMap["purpose"].(string); ok {
					purposeVal = v
				}

				// 2. Identify Command (everything else)
				var commandParts []string

				// Then grab any args that look like command parts
				// keep the k=v pairs format for command
				for k, v := range argsMap {
					if k == "purpose" {
						continue
					} else if k == "need_confirm" {
						continue
					}
					val := fmt.Sprintf("%s = %v", k, v)
					commandParts = append(commandParts, val)
				}

				commandVal := strings.Join(commandParts, "\n")

				// Render logic
				// Title (Function Name) -> Cyan Bold
				// Command -> White (With keys)
				// Purpose -> Gray, Dim, Wrapped

				cmdStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("7")).Width(tcol)       // White
				purposeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Width(tcol) // Grey, wrapped

				var parts []string
				parts = append(parts, titleStyle.Render(data.Function))

				if commandVal != "" {
					parts = append(parts, cmdStyle.Render(commandVal))
				}

				if purposeVal != "" {
					parts = append(parts, purposeStyle.Render(purposeVal))
				}

				content = strings.Join(parts, "\n")
			}

			// Fallback if content is still empty
			if content == "" {
				// Convert Args back to string for display
				var argsStr string
				if s, ok := data.Args.(string); ok {
					argsStr = s
				} else {
					bytes, _ := json.MarshalIndent(data.Args, "", "  ")
					argsStr = string(bytes)
				}
				content = fmt.Sprintf("%s\n%s", titleStyle.Render(data.Function), argsStyle.Render(argsStr))
			}

			output = style.Render(content)
		} else {
			// Fallback to original text if not JSON
			output = inCallingColor + text + resetColor
		}

		ag.Std.Writeln(output)
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
			ag.Std.Writeln(resetColor)
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
