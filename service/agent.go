package service

import (
	"context"
	"fmt"
	"math"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/internal/event"
	"github.com/activebook/gllm/io"
	"github.com/activebook/gllm/util"
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
	ApiKey          string
	EndPoint        string
	Model           string
	Provider        string
	Temperature     float32
	TopP            float32 // Top-p sampling parameter
	Seed            *int32  // Seed for deterministic generation
	ContextLength   int32   // Model context length limit
	MaxOutputTokens int32   // Model max output tokens
}

type Agent struct {
	Ctx             context.Context // Optional completion cancellation context
	Model           *ModelInfo
	SystemPrompt    string
	UserPrompt      string
	Files           []*FileData         // Attachment files
	NotifyChan      chan<- StreamNotify // Sub Channel to send notifications
	DataChan        chan<- StreamData   // Sub Channel to receive streamed text data
	ProceedChan     <-chan bool         // Sub Channel to receive proceed signal
	SearchEngine    *SearchEngine       // Search engine name
	ToolsUse        data.ToolsUse       // Use tools
	EnabledTools    []string            // List of enabled embedding tools
	UseCodeTool     bool                // Use code tool
	ThinkingLevel   ThinkingLevel       // Thinking level: off, low, medium, high
	MCPClient       *MCPClient          // MCP client for MCP tools
	MaxRecursions   int                 // Maximum number of recursions for model calls
	Markdown        *Markdown           // Markdown renderer
	TokenUsage      *TokenUsage         // Token usage metainfo
	StdOutput       io.Output           // Standard I/O
	FileOutput      io.Output           // File I/O
	SSEOutput       *io.SSEOutput       // Network I/O for Server-Sent Events
	Status          StatusStack         // Stack to manage streaming status
	Session         Session             // Session
	Context         ContextManager      // Context manager
	LastWrittenData string              // Last written data

	// Sub-agent orchestration
	SharedState *data.SharedState // Shared state for inter-agent communication
	AgentName   string            // Current agent name for metadata tracking
	ModelName   string            // Current model name of current agent (agent model key)
	Verbose     bool              // Whether verbose output mode is enabled
}

func constructModelInfo(model *data.Model) *ModelInfo {
	mi := ModelInfo{}
	provider := model.Provider
	if provider == "" {
		// Auto-detect provider if not set
		util.LogDebugf("Auto-detecting provider for %s\n", model.Model)
		provider = DetectModelProvider(model.Endpoint, model.Model)
	} else {
		util.LogDebugf("Provider: [%s]\n", provider)
	}
	mi.Model = model.Model
	mi.Provider = provider
	mi.EndPoint = model.Endpoint
	mi.ApiKey = model.Key
	mi.Temperature = model.Temp
	mi.TopP = model.TopP
	mi.Seed = model.Seed
	mi.ContextLength = model.ContextLength
	mi.MaxOutputTokens = model.MaxOutputTokens
	return &mi
}

func constructSearchEngine(capabilities []string) *SearchEngine {
	se := SearchEngine{}
	se.Name = GetNoneSearchEngineName()
	se.UseSearch = false

	if IsWebSearchEnabled(capabilities) {
		// Get allowed search engine from settings
		store := data.GetSettingsStore()
		engineName := store.GetAllowedSearchEngine()

		// If no engine set, try to default to Google if available, or just keep none
		if engineName == "" {
			engineName = GetDefaultSearchEngineName()
		}

		// Get engine config from config store
		configStore := data.NewConfigStore()
		engineConfig := configStore.GetSearchEngine(engineName)

		if engineConfig != nil {
			se.UseSearch = true
			se.Name = engineConfig.Name
			se.ApiKey = engineConfig.Config["key"]
			se.CxKey = engineConfig.Config["cx"]
			se.DeepDive = engineConfig.DeepDive
			se.MaxReferences = engineConfig.Reference
		}
	}

	util.LogDebugf("Search engine: %v, %v\n", se.Name, se.UseSearch)
	return &se
}

func constructIO(quiet bool, outputFile string) (io.Output, io.Output) {
	// Provide StdRenderer from options
	var stdIO io.Output
	if !quiet {
		stdIO = io.NewStdOutput()
	}

	// Need to output a file
	var fileIO io.Output
	if outputFile != "" {
		var err error
		fileIO, err = io.NewFileOutput(outputFile)
		if err != nil {
			util.LogWarnf("failed to create output file %s: %v\n", outputFile, err)
			return nil, nil
		}
	}
	return stdIO, fileIO
}

// ConstructSystemPrompt constructs the system prompt by injecting memory and skills into the prompt
func ConstructSystemPrompt(prompt string, capabilities []string) string {
	sysPrompt := prompt

	// Inject memory into system prompt
	if IsAgentMemoryEnabled(capabilities) {
		memStore := data.NewMemoryStore()
		if memoryContent := memStore.GetAll(); memoryContent != "" {
			sysPrompt += "\n\n" + memoryContent
		}
	}

	// Inject skills into system prompt if any are available and enabled
	if IsAgentSkillsEnabled(capabilities) {
		// Load available skills metadata
		sm := GetSkillManager() // Use singleton
		if skillsXML := sm.GetAvailableSkills(); skillsXML != "" {
			sysPrompt += "\n\n" + skillsXML
		}
	}

	// Inject plan mode into system prompt if plan mode is enabled
	if IsPlanModeEnabled(capabilities) {
		if data.GetPlanModeInSession() {
			sysPrompt += "\n\n" + data.PlanModeSystemPrompt
		}
	}

	// Inject global and project instruction files (GLLM.md)
	if instructionContent := data.GetInstructionContent(); instructionContent != "" {
		sysPrompt += "\n\n" + instructionContent
	}

	return sysPrompt
}

// construct all enabled tools including features tools
func constructEnabledTools(tools []string, capabilities []string) []string {
	enabledTools := tools

	// Set up enabled tools list with skill automation
	if IsAgentSkillsEnabled(capabilities) {
		// Automatically add activate_skill if not already there
		enabledTools = AppendSkillTools(enabledTools)
	} else {
		// Automatically remove activate_skill if skills are disabled
		enabledTools = RemoveSkillTools(enabledTools)
	}

	// Memory tool injection
	if IsAgentMemoryEnabled(capabilities) {
		enabledTools = AppendMemoryTools(enabledTools)
	} else {
		enabledTools = RemoveMemoryTools(enabledTools)
	}

	// Subagents tool injection
	if IsSubAgentsEnabled(capabilities) {
		enabledTools = AppendSubagentTools(enabledTools)
	} else {
		enabledTools = RemoveSubagentTools(enabledTools)
	}

	// Web Search tool injection
	if IsWebSearchEnabled(capabilities) {
		enabledTools = AppendSearchTools(enabledTools)
	} else {
		enabledTools = RemoveSearchTools(enabledTools)
	}

	// Plan Mode tool injection
	if IsPlanModeEnabled(capabilities) {
		enabledTools = AppendPlanTools(enabledTools)
	} else {
		enabledTools = RemovePlanTools(enabledTools)
	}
	return enabledTools
}

// ConstructSession constructs a new session based on the provider
func ConstructSession(sessionName string, provider string) (Session, error) {
	switch provider {
	case ModelProviderOpenAICompatible:
		// Used for Chinese Models
		session := OpenChatSession{}
		err := session.Open(sessionName)
		if err != nil {
			return nil, err
		}
		return &session, nil

	case ModelProviderOpenAI:
		// Used for OpenAI compatible models
		session := OpenAISession{}
		err := session.Open(sessionName)
		if err != nil {
			return nil, err
		}
		return &session, nil

	case ModelProviderGemini:
		// Used for Gemini
		session := GeminiSession{}
		err := session.Open(sessionName)
		if err != nil {
			return nil, err
		}
		return &session, nil

	case ModelProviderAnthropic:
		// Used for Anthropic
		session := AnthropicSession{}
		err := session.Open(sessionName)
		if err != nil {
			return nil, err
		}
		return &session, nil

	default:
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}
}

type AgentOptions struct {
	Ctx           context.Context // Context to carry cancellation logic
	Prompt        string
	SysPrompt     string
	Files         []*FileData
	ModelInfo     *data.Model
	MaxRecursions int
	ThinkingLevel string
	EnabledTools  []string      // List of enabled embedding tools
	Capabilities  []string      // List of enabled capabilities
	YoloMode      bool          // Whether to automatically approve tools
	QuietMode     bool          // If Quiet mode then don't print to console
	OutputFile    string        // If OutputFile is set then write to file
	SSEOutput     *io.SSEOutput // SSE networking adapter
	SessionName   string
	MCPConfig     map[string]*data.MCPServer

	// Sub-agent orchestration fields
	SharedState *data.SharedState // Shared state for inter-agent communication
	AgentName   string            // Name of the agent running this task
	ModelName   string            // Current model name of current agent (agent model key)
}

func CallAgent(op *AgentOptions) error {

	// Set up model settings
	mi := constructModelInfo(op.ModelInfo)

	// Set up search engine settings based on capabilities
	se := constructSearchEngine(op.Capabilities)

	// Set up tools use settings
	toolsUse := data.ToolsUse{AutoApprove: op.YoloMode, AgentName: op.AgentName}

	// Set up code tool settings
	exeCode := IsCodeExecutionEnabled()

	// Get verbose setting
	settingsStore := data.GetSettingsStore()
	verboseMode := settingsStore.GetVerboseEnabled()

	// Set up thinking level
	thinkingLevel := ParseThinkingLevel(op.ThinkingLevel)

	// Set max recursions to max int if negative
	if op.MaxRecursions < 0 {
		op.MaxRecursions = math.MaxInt
	}
	util.LogDebugf("Max session turns:%d\n", op.MaxRecursions)

	// Create a channel to receive notifications
	notifyCh := make(chan StreamNotify, 10) // Buffer to prevent blocking(used for status updates)
	dataCh := make(chan StreamData, 10)     // Buffer to prevent blocking(used for streamed text data)
	proceedCh := make(chan bool)            // For main -> sub communication

	// active channels used in select (can be set to nil to disable)
	activeNotifyCh := notifyCh
	activeDataCh := dataCh

	// Provide StdRenderer from options
	stdIO, fileIO := constructIO(op.QuietMode, op.OutputFile)

	if stdIO != nil {
		defer stdIO.Close()
	}
	if fileIO != nil {
		defer fileIO.Close()
	}

	// Set up MCP client
	var mc *MCPClient
	if IsMCPServersEnabled(op.Capabilities) {
		mc = GetMCPClient() // use the shared instance
		if !op.QuietMode {
			event.StartIndicator("")
		}
		err := mc.Init(op.MCPConfig, MCPLoadOption{
			LoadAll:   false,
			LoadTools: true, // only load tools
		}) // Load only allowed servers
		if !op.QuietMode {
			event.StopIndicator()
		}
		if err != nil {
			// MCP load failed, warn but continue without MCP tools
			util.LogWarnf("MCP servers unavailable: %v\n", err)
			mc = nil
		}
		// We shouldn't clean up MCP client resources when agent exits
		// Because next turn in repl mode, it would need to re-init the mcp client, which is wasteful and slow
		// The singleton pattern with cleanup at application exit is the **correct** design pattern for shared resources like MCP client connections.
		// defer mc.Close()
	}

	// Need to append markdown
	var markdown *Markdown
	if IsMarkdownEnabled(op.Capabilities) {
		markdown = NewMarkdown()
	}

	// Need to append token usage
	var tu *TokenUsage
	if IsTokenUsageEnabled(op.Capabilities) {
		tu = NewTokenUsage()
	}

	// Inject memory, skills, plan mode into system prompt
	op.SysPrompt = ConstructSystemPrompt(op.SysPrompt, op.Capabilities)

	// Construct all enabled tools
	enabledTools := constructEnabledTools(op.EnabledTools, op.Capabilities)

	ag := Agent{
		Ctx:           op.Ctx,
		Model:         mi,
		SystemPrompt:  op.SysPrompt,
		UserPrompt:    op.Prompt,
		Files:         op.Files,
		NotifyChan:    notifyCh,
		DataChan:      dataCh,
		ProceedChan:   proceedCh,
		SearchEngine:  se,
		ToolsUse:      toolsUse,
		EnabledTools:  enabledTools,
		UseCodeTool:   exeCode,
		MCPClient:     mc,
		ThinkingLevel: thinkingLevel,
		MaxRecursions: op.MaxRecursions,
		Markdown:      markdown,
		TokenUsage:    tu,
		StdOutput:     stdIO,
		FileOutput:    fileIO,
		SSEOutput:     op.SSEOutput,
		Status:        StatusStack{},
		SharedState:   op.SharedState,
		AgentName:     op.AgentName,
		ModelName:     op.ModelName,
		Verbose:       verboseMode,
	}

	// If no context is provided, use background context
	if ag.Ctx == nil {
		ag.Ctx = context.Background()
	}

	// Construct session manager
	cm, err := ConstructSession(op.SessionName, ag.Model.Provider)
	if err != nil {
		return err
	}
	ag.Session = cm

	// Instantiate correct context manager
	strategy := StrategyTruncateOldest
	if IsAutoCompressionEnabled(op.Capabilities) {
		strategy = StrategySummarize
	}
	ag.Context = NewContextManager(&ag, strategy)

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
					notifyCh <- StreamNotify{Status: StatusSwitchAgent, Extra: err}
				} else if IsUserCancelError(err) {
					notifyCh <- StreamNotify{Status: StatusUserCancel, Extra: err}
				} else {
					notifyCh <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("%v", err)}
				}
			}
		case ModelProviderOpenAI:
			// Used for OpenAI compatible models
			if err := ag.GenerateOpenAIStream(); err != nil {
				// Send error through channel instead of returning
				if IsSwitchAgentError(err) {
					notifyCh <- StreamNotify{Status: StatusSwitchAgent, Extra: err}
				} else if IsUserCancelError(err) {
					notifyCh <- StreamNotify{Status: StatusUserCancel, Extra: err}
				} else {
					notifyCh <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("%v", err)}
				}
			}
		case ModelProviderGemini:
			if err := ag.GenerateGeminiStream(); err != nil {
				// Send error through channel instead of returning
				if IsSwitchAgentError(err) {
					notifyCh <- StreamNotify{Status: StatusSwitchAgent, Extra: err}
				} else if IsUserCancelError(err) {
					notifyCh <- StreamNotify{Status: StatusUserCancel, Extra: err}
				} else {
					notifyCh <- StreamNotify{Status: StatusError, Data: fmt.Sprintf("%v", err)}
				}
			}
		case ModelProviderAnthropic:
			if err := ag.GenerateAnthropicStream(); err != nil {
				// Send error through channel instead of returning
				if IsSwitchAgentError(err) {
					notifyCh <- StreamNotify{Status: StatusSwitchAgent, Extra: err}
				} else if IsUserCancelError(err) {
					notifyCh <- StreamNotify{Status: StatusUserCancel, Extra: err}
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
				// Start indicator (let indicator decide the text)
				ag.StartIndicator("")
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
				// Convert notify.Extra to SwitchAgentError safely
				if err, ok := notify.Extra.(error); ok {
					if switchErr, ok := AsSwitchAgentError(err); ok {
						return switchErr
					}
				}
				return fmt.Errorf("unknown switch agent error type: %v", notify.Extra)
			case StatusUserCancel:
				ag.StopIndicator()
				ag.WriteEnd()
				// Convert notify.Extra to UserCancelError safely
				if err, ok := notify.Extra.(error); ok {
					if cancelErr, ok := AsUserCancelError(err); ok {
						return cancelErr
					}
				}
				return fmt.Errorf("unknown user cancel error type: %v", notify.Extra)
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
				ag.WriteFunctionCallOver()
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
