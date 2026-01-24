package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/service"
)

func EnsureActiveAgent() (*data.AgentConfig, error) {
	// Get Active Agent
	store := data.NewConfigStore()
	agent := store.GetActiveAgent()
	if agent == nil {
		return nil, fmt.Errorf("no active agent found")
	}

	// Auto-detect provider if not set
	if agent.Model.Provider == "" {
		service.Debugf("Auto-detecting provider for %s", agent.Model.Model)
		agent.Model.Provider = service.DetectModelProvider(agent.Model.Endpoint, agent.Model.Model)
		store.SetModel(agent.Model.Name, &agent.Model)
	} else {
		service.Debugf("Provider: [%s]", agent.Model.Provider)
	}

	// Validate Model
	if agent.Model.Name == "" {
		return nil, fmt.Errorf("no model specified")
	}
	model := store.GetModel(agent.Model.Name)
	if model == nil {
		return nil, fmt.Errorf("model %s not found", agent.Model.Name)
	}
	return agent, nil
}

// RunAgent executes the agent with the given parameters, handling all setup and compatibility checks.
func RunAgent(prompt string, files []*service.FileData, convoName string, yolo bool, outputFile string, inputState *data.SharedState) error {
	// Initialize SharedState for this session (for sub-agent orchestration)
	// If inputState is provided, use it (lifecycle managed by caller)
	// If not, create a new one and manage lifecycle here
	var sharedState *data.SharedState
	if inputState != nil {
		sharedState = inputState
	} else {
		sharedState = data.NewSharedState()
		defer sharedState.Clear() // Clean up on session end
	}

	for {
		// Create an indeterminate progress bar
		indicator := service.NewIndicator("Processing...")

		// Ensure Active Agent
		agent, err := EnsureActiveAgent()
		if err != nil {
			return err
		}

		// Ensure conversation compatibility
		if convoName != "" {
			if err := EnsureConversationCompatibility(agent, convoName); err != nil {
				return err
			}
		}

		// Build Final Prompt (Template + Input + @ Processing)
		finalPrompt := buildFinalPrompt(agent, prompt)

		// Get system prompt
		store := data.NewConfigStore()
		sysPrompt := store.GetSystemPrompt(agent.SystemPrompt)

		// Load MCP config
		mcpStore := data.NewMCPStore()
		mcpConfig, _, _ := mcpStore.Load()

		// Stop indicator
		indicator.Stop()

		// Prepare Agent Options
		op := service.AgentOptions{
			Prompt:        finalPrompt,
			SysPrompt:     sysPrompt,
			Files:         files,
			ModelInfo:     &agent.Model,
			SearchEngine:  &agent.Search,
			MaxRecursions: agent.MaxRecursions,
			ThinkingLevel: agent.Think,
			EnabledTools:  agent.Tools,
			Capabilities:  agent.Capabilities,
			YoloMode:      yolo,
			OutputFile:    outputFile,
			QuietMode:     false,
			ConvoName:     convoName,
			MCPConfig:     mcpConfig,
			// Sub-agent orchestration
			SharedState: sharedState,
			AgentName:   agent.Name,
		}

		// Execute
		err = service.CallAgent(&op)
		if err != nil {
			// Switch agent signal
			if service.IsSwitchAgentError(err) {
				switchErr := err.(*service.SwitchAgentError)
				service.Infof("Already switched to agent [%s].", switchErr.TargetAgent)
				// Set instruction, shouldn't use the old prompt
				prompt = switchErr.Instruction
				service.Debugf("Switch agent instruction: %s", prompt)
				// Clearup files
				files = nil
				if prompt == "" {
					// If no instruction, then no more task, exit
					break
				} else {
					// Switch agent, continue to next loop
					continue
				}
			} else {
				// Other error, return
				return err
			}
		} else {
			// No error, this turn is done, break
			break
		}
	}
	return nil
}

// buildFinalPrompt combines template and user input, and processes @ references
func buildFinalPrompt(agent *data.AgentConfig, input string) string {
	tb := TextBuilder{}
	store := data.NewConfigStore()
	templateContent := store.GetTemplate(agent.Template)
	tb.appendText(templateContent)
	tb.appendText(input)

	rawPrompt := tb.String()
	atRefProcessor := service.NewAtRefProcessor()
	processedPrompt, err := atRefProcessor.ProcessText(rawPrompt)
	if err != nil {
		service.Warnf("Skip processing @ references in prompt: %v", err)
		return rawPrompt
	}
	return processedPrompt
}

// BatchAttachments processes multiple attachments concurrently and adds the resulting
// FileData objects to the provided files slice.
// It uses a WaitGroup to manage goroutines and a channel to collect results safely.
func BatchAttachments(attachments []string) (files []*service.FileData) {
	var wg sync.WaitGroup
	filesCh := make(chan *service.FileData, len(attachments))
	for _, attachment := range attachments {
		wg.Add(1)
		go func(att string) {
			defer wg.Done()
			fileData := ProcessAttachment(att)
			if fileData != nil {
				filesCh <- fileData
			}
		}(attachment)
	}
	wg.Wait()
	close(filesCh)
	for fileData := range filesCh {
		files = append(files, fileData)
	}
	return files
}

// Processes a single attachment (file or stdin marker)
func ProcessAttachment(path string) *service.FileData {
	// Handle stdin or regular file
	data, err := readContentFromPath(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file[%s]: %v\n", path, err)
		return nil
	}

	// Check if content is an image
	isImage, format, err := service.CheckIfImageFromBytes(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error checking content type: %v\n", err)
		return nil
	}

	// If not an image, try to get MIME type from file extension
	if !isImage {
		format = service.GetMIMEType(path)
		if service.IsUnknownMIMEType(format) {
			// try to guess MIME type by content
			format = service.GetMIMETypeByContent(data)
		}
	}
	return service.NewFileData(format, data, path)
}

// EnsureConversationCompatibility checks if the existing conversation is compatible with the current agent's provider.
// If not, it attempts to convert the conversation history.
func EnsureConversationCompatibility(agent *data.AgentConfig, convoName string) error {
	// 1. Get Conversation Data
	convoData, _, err := GetConvoData(convoName, agent.Model.Provider)
	if err != nil {
		// If conversation doesn't exist, that's fine, nothing to check/convert
		// We should differentiate "not found" from other errors
		if strings.Contains(err.Error(), "not found") {
			return nil
		}
		return err
	}

	// 2. Check Compatibility
	isCompatible, provider, modelProvider := CheckConvoFormat(agent, convoData)
	if !isCompatible {
		service.Debugf("Conversation '%s' [%s] is not compatible with the current model provider [%s].\n", convoName, provider, modelProvider)

		// 3. Convert Data
		convertData, err := service.ConvertMessages(convoData, provider, modelProvider)
		if err != nil {
			return fmt.Errorf("error converting conversation: %v", err)
		}

		// 4. Write Back
		if err := WriteConvoData(convoName, convertData, modelProvider); err != nil {
			return err
		}
		service.Debugf("Conversation '%s' converted to compatible format [%s].\n", convoName, modelProvider)
	}

	return nil
}

// GetConvoData retrieves conversation data.
func GetConvoData(convoName string, provider string) (data []byte, name string, err error) {
	cm, err := service.ConstructConversationManager(convoName, provider)
	if err != nil {
		return nil, "", fmt.Errorf("error constructing conversation manager: %v", err)
	}

	convoPath := cm.GetPath()
	if _, err := os.Stat(convoPath); os.IsNotExist(err) {
		return nil, "", fmt.Errorf("conversation '%s' not found", convoPath)
	}

	data, err = os.ReadFile(convoPath)
	if err != nil {
		return nil, "", fmt.Errorf("error reading conversation file: %v", err)
	}

	name = strings.TrimSuffix(filepath.Base(convoPath), filepath.Ext(convoPath))
	return data, name, nil
}

// WriteConvoData writes conversation data for a specific provider.
func WriteConvoData(convoName string, data []byte, provider string) error {
	cm, err := service.ConstructConversationManager(convoName, provider)
	if err != nil {
		return fmt.Errorf("error constructing conversation manager: %v", err)
	}

	convoPath := cm.GetPath()
	// Preserving original file mode if it exists, roughly
	// But os.WriteFile will create if not exists with 0666 before umask
	// We can check stat first if we want to be strict, but standard WriteFile is usually fine for this app

	// Check if conversation exists to get mode, though checking existence to write might be overkill
	// unless we want to preserve permissions strictly.
	// Copied logic from chat.go for safety
	var fi os.FileInfo
	if fi, err = os.Stat(convoPath); os.IsNotExist(err) {
		// If not exist, write with default perm
		return os.WriteFile(convoPath, data, 0644)
	}

	return os.WriteFile(convoPath, data, fi.Mode())
}

// CheckConvoFormat verifies if the conversation data is compatible with the agent's provider.
func CheckConvoFormat(agent *data.AgentConfig, convoData []byte) (isCompatible bool, provider string, modelProvider string) {
	modelProvider = agent.Model.Provider

	// Detect provider based on message format
	provider = service.DetectMessageProvider(convoData)

	// Check compatibility
	isCompatible = provider == modelProvider
	if !isCompatible {
		isCompatible = provider == service.ModelProviderUnknown ||
			(provider == service.ModelProviderOpenAI && modelProvider == service.ModelProviderOpenAICompatible) ||
			(provider == service.ModelProviderOpenAICompatible && modelProvider == service.ModelProviderOpenAI) ||
			(provider == service.ModelProviderOpenAICompatible && modelProvider == service.ModelProviderAnthropic)
	}

	return isCompatible, provider, modelProvider
}
