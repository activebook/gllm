package cmd

import (
	"fmt"
	"os"
	"sync"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/internal/ui"
	"github.com/activebook/gllm/service"
	"github.com/activebook/gllm/util"
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
		agent.Model.Provider = service.DetectModelProvider(agent.Model.Endpoint, agent.Model.Model)
		store.SetModel(agent.Model.Name, &agent.Model)
	}

	// Validate Model
	if agent.Model.Name == "" {
		return nil, fmt.Errorf("no model specified")
	}
	model := store.GetModel(agent.Model.Name)
	if model == nil {
		return nil, fmt.Errorf("model %s not found", agent.Model.Name)
	}
	// Auto-detect model limits if not set
	if model.ContextLength == 0 {
		// Trigger background sync to cache it for next time
		go service.SyncModelLimits(agent.Model.Name, agent.Model.Model)
	}
	return agent, nil
}

// RunAgent executes the agent with the given parameters, handling all setup and compatibility checks.
func RunAgent(prompt string, files []*service.FileData, sessionName string, outputFile string, inputState *data.SharedState) error {
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
		// Start indeterminate progress bar
		ui.GetIndicator().Start("")

		// Get YOLO mode
		yolo := data.GetYoloModeInSession()

		// Ensure Active Agent
		agent, err := EnsureActiveAgent()
		if err != nil {
			return err
		}

		// Ensure session compatibility
		if sessionName != "" {
			if err := service.EnsureSessionCompatibility(agent, sessionName); err != nil {
				return err
			}
		}

		// Build Final Prompt (Input + @ Processing)
		finalPrompt := buildFinalPrompt(prompt)

		// Load MCP config
		mcpStore := data.NewMCPStore()
		mcpConfig, err := mcpStore.Load()
		if err != nil {
			return err
		}

		// Stop indicator
		ui.GetIndicator().Stop()

		// Prepare Agent Options
		op := service.AgentOptions{
			Prompt:        finalPrompt,
			SysPrompt:     agent.SystemPrompt,
			Files:         files,
			ModelInfo:     &agent.Model,
			MaxRecursions: agent.MaxRecursions,
			ThinkingLevel: agent.Think,
			EnabledTools:  agent.Tools,
			Capabilities:  agent.Capabilities,
			YoloMode:      yolo,
			OutputFile:    outputFile,
			QuietMode:     false,
			SessionName:   sessionName,
			MCPConfig:     mcpConfig,
			// Sub-agent orchestration
			SharedState: sharedState,
			AgentName:   agent.Name,
			ModelName:   agent.Model.Name,
		}

		// Execute
		err = service.CallAgent(&op)
		if err != nil {
			// Switch agent signal
			if service.IsSwitchAgentError(err) {
				switchErr, _ := service.AsSwitchAgentError(err)
				util.Infof("Already switched to agent [%s].\n", switchErr.TargetAgent)
				// Set instruction, shouldn't use the old prompt
				prompt = switchErr.Instruction
				util.Debugf("Switch agent instruction: %s\n", prompt)
				// Clearup files
				files = nil
				if prompt == "" {
					// If no instruction, then no more task, exit
					break
				} else {
					// Switch agent, continue to next loop
					continue
				}
			} else if service.IsUserCancelError(err) {
				// User cancelled operation
				userCancelErr, _ := service.AsUserCancelError(err)
				util.Infof("%v\n", userCancelErr)
				break
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

// buildFinalPrompt combines user input and processes @ references
func buildFinalPrompt(input string) string {
	tb := TextBuilder{}
	tb.appendText(input)

	rawPrompt := tb.String()
	atRefProcessor := service.NewAtRefProcessor()
	processedPrompt, err := atRefProcessor.ProcessText(rawPrompt)
	if err != nil {
		util.Warnf("Skip processing @ references in prompt: %v\n", err)
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



// StartLoadMCPServer launches background MCP preloading (non-blocking).
func StartLoadMCPServer(agent *data.AgentConfig) {
	go func() {
		if !service.IsMCPServersEnabled(agent.Capabilities) {
			return
		}

		mcpStore := data.NewMCPStore()
		mcpConfig, err := mcpStore.Load()
		if err != nil {
			return
		}

		mc := service.GetMCPClient()
		mc.PreloadAsync(mcpConfig, service.MCPLoadOption{
			LoadAll:   false,
			LoadTools: true,
		})
	}()
}
