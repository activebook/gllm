package service

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"sync"
)

type WorkflowAgentType string

const (
	WorkflowAgentTypeMaster WorkflowAgentType = "master"
	WorkflowAgentTypeWorker WorkflowAgentType = "worker"
)

// WorkflowAgent defines the structure for a single agent in the workflow.
type WorkflowAgent struct {
	Name          string
	Role          WorkflowAgentType
	Model         *map[string]any
	Search        *map[string]any
	Template      string
	SystemPrompt  string
	Tools         bool
	Think         bool
	Usage         bool
	Markdown      bool
	InputDir      string
	OutputDir     string
	MaxRecursions int
	OutputFile    string
}

// WorkflowConfig defines the structure for the entire workflow.
type WorkflowConfig struct {
	Agents []WorkflowAgent
}

func clearupOutputDir(outputDir string) error {
	var err error
	if outputDir == "" {
		err = fmt.Errorf("Agent should set an output directory")
		return err
	}
	if err := os.RemoveAll(outputDir); err != nil {
		err = fmt.Errorf("failed to clear output directory %s: %v", outputDir, err)
		return err
	}
	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		err = fmt.Errorf("failed to create output directory %s: %v", outputDir, err)
		return err
	}
	return nil
}

// Appends text to builder with proper newline handling
func appendText(builder *strings.Builder, text string) {
	if text == "" {
		return
	}
	builder.WriteString(text)
	if !strings.HasSuffix(text, "\n") {
		builder.WriteString("\n")
	}
}

func buildAgentPrompt(agent *WorkflowAgent, prompt string) string {
	var finalPrompt strings.Builder

	// Add user prompt and template
	appendText(&finalPrompt, agent.Template)
	appendText(&finalPrompt, prompt)

	return finalPrompt.String()
}

func runMasterAgent(agent *WorkflowAgent, prompt string) error {
	var finalPrompt string
	agentInfo := fmt.Sprintf("[%s (%s)]", agent.Name, agent.Role)
	if agent.InputDir != "" {
		// Master reads all files from the input directory
		files, err := os.ReadDir(agent.InputDir)
		if err != nil {
			err = fmt.Errorf("%s: Failed to read input directory %s: %v", agentInfo, agent.InputDir, err)
			return err
		}

		var content strings.Builder
		for _, file := range files {
			if !file.IsDir() {
				path := GetFilePath(agent.InputDir, file.Name())
				data, err := GetFileContent(path)
				if err != nil {
					err = fmt.Errorf("%s: Failed to read file %s: %v", agentInfo, path, err)
					return err
				}
				appendText(&content, data)
			}
		}
		finalPrompt = content.String()
	} else {
		// First agent gets the initial prompt
		finalPrompt = prompt
	}

	// Build the prompt with template
	finalPrompt = buildAgentPrompt(agent, finalPrompt)
	// Setup the output file path
	// Master output file name: <agent name>.md
	agent.OutputFile = GetFilePath(agent.OutputDir, agent.Name+".md")
	// Execute master agent
	executeAgent(agent, finalPrompt)
	return nil
}

func runWorkerAgent(agent *WorkflowAgent) error {
	agentInfo := fmt.Sprintf("[%s (%s)]", agent.Name, agent.Role)
	if agent.InputDir == "" {
		err := fmt.Errorf("%s: has no input directory, it must have one", agentInfo)
		return err
	}

	files, err := os.ReadDir(agent.InputDir)
	if err != nil {
		err = fmt.Errorf("%s: Failed to read input directory %s: %v", agentInfo, agent.InputDir, err)
		return err
	}

	var wg sync.WaitGroup
	// Collect all errors (continue all goroutines)
	errChan := make(chan error, len(files)) // Buffered channel
	for _, file := range files {
		if !file.IsDir() {
			wg.Add(1)
			go func(file fs.DirEntry) {
				defer wg.Done()

				path := GetFilePath(agent.InputDir, file.Name())
				data, err := GetFileContent(path)
				if err != nil {
					err = fmt.Errorf("%s: Failed to read file %s: %v", agentInfo, path, err)
					errChan <- err
					return
				}

				// Build the prompt with template
				prompt := buildAgentPrompt(agent, data)
				// Setup the output file path
				// Worker output file name: <agent name>_<file name>.md
				agent.OutputFile = GetFilePath(agent.OutputDir, agent.Name+"_"+file.Name()+".md")
				// Execute worker agent
				executeAgent(agent, prompt)
				errChan <- nil // Send nil for success
			}(file)
		}
	}
	wg.Wait()

	close(errChan)

	// Check for errors
	var allErrors []error
	for err := range errChan {
		if err != nil {
			allErrors = append(allErrors, err)
		}
	}
	if len(allErrors) > 0 {
		return errors.Join(allErrors...) // Combine all errors
	}
	return nil
}

// RunWorkflow executes the defined workflow.
func RunWorkflow(config *WorkflowConfig, prompt string) error {
	var err error

	if len(config.Agents) == 0 {
		err = fmt.Errorf("no agents defined in the workflow")
		return err
	}

	// The initial prompt is passed to the first agent.
	workflowPrompt := prompt

	for _, agent := range config.Agents {
		agentInfo := fmt.Sprintf("[%s (%s)]", agent.Name, agent.Role)

		// Print the agent working flow
		switch agent.Role {
		case WorkflowAgentTypeMaster:
			Infof("Agent %s is working...", agentInfo)
		case WorkflowAgentTypeWorker:
			Infof("\tAgent %s is working...", agentInfo)
		default:
			err = fmt.Errorf("Agent %s has no role defined", agent.Name)
			return err
		}

		// Clear the output directory before running the agent
		if err = clearupOutputDir(agent.OutputDir); err != nil {
			err = fmt.Errorf("%s: %v", agentInfo, err)
			return err
		}

		switch agent.Role {
		case WorkflowAgentTypeMaster:
			err = runMasterAgent(&agent, workflowPrompt)
			if err != nil {
				err = fmt.Errorf("workflow: %v", err)
				return err
			}

		case WorkflowAgentTypeWorker:
			err = runWorkerAgent(&agent)
			if err != nil {
				err = fmt.Errorf("workflow: %v", err)
				return err
			}

		default:
			Warnf("Unknown agent role '%s' for agent %s, skipping.", agent.Role, agent.Name)
		}
	}
	Successf("Workflow finished.")
	return nil
}

func executeAgent(agent *WorkflowAgent, prompt string) {
	// Only Master can output to the console, Worker in quiet mode
	quiet := (agent.Role == WorkflowAgentTypeWorker)

	agentOptions := AgentOptions{
		Prompt:         prompt,
		SysPrompt:      agent.SystemPrompt,
		ModelInfo:      agent.Model,
		SearchEngine:   agent.Search,
		MaxRecursions:  agent.MaxRecursions,
		ThinkMode:      agent.Think,
		UseTools:       agent.Tools,
		AppendMarkdown: agent.Markdown,
		AppendUsage:    agent.Usage,
		OutputFile:     agent.OutputFile, // Write to file
		QuietMode:      quiet,            // Worker in quiet mode
	}

	CallAgent(&agentOptions)
}
