package service

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/activebook/gllm/data"
	"github.com/ergochat/readline"
)

type WorkflowAgentType string

const (
	WorkflowAgentTypeMaster WorkflowAgentType = "master"
	WorkflowAgentTypeWorker WorkflowAgentType = "worker"
)

const (
	WokflowConfirmPrompt = "\033[96mDo you want to proceed with this agent? (y/N):\033[0m "         // use for wait for user confirm
	WokflowProceedPrompt = "\033[96mDoes that work for you? Proceed with next step? (y/N):\033[0m " // use for wait for proceed prompt
	WokflowModifyPrompt  = "\033[96mPlease specify any changes you would like to make:\033[0m "     // use for wait for user modify prompt
)

// WorkflowAgent defines the structure for a single agent in the workflow.
type WorkflowAgent struct {
	Name          string
	Role          WorkflowAgentType
	Model         *data.Model
	Search        *data.SearchEngine
	Template      string
	SystemPrompt  string
	EnabledTools  []string
	Think         bool
	MCP           bool
	Usage         bool
	Markdown      bool
	InputDir      string
	OutputDir     string
	MaxRecursions int
	OutputFile    string
	PassThrough   bool   // pass through current agent, only for debugging
	ConvoName     string // conversation name, for iterate prompt
}

// WorkflowConfig defines the structure for the entire workflow.
type WorkflowConfig struct {
	Agents          []WorkflowAgent
	InterActiveMode bool // Allow user confirm at each agent
}

/*
clearupOutputDir removes the specified output directory and all its contents,
then recreates an empty directory at the same path. Returns an error if the
directory path is empty or if any operation fails.
*/
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

// Terminal colors for workflow confirmation

// promptUserForConfirmation asks the user to confirm before proceeding with an agent
func promptUserForConfirmation(agent *WorkflowAgent) bool {
	fmt.Printf("\n %sAgent:%s %s%s (%s%s%s)\n", agentNameColor, workflowResetColor, agentNameColor, agent.Name, agentRoleColor, agent.Role, workflowResetColor)
	fmt.Printf("   %sModel:%s %s%s%s\n", modelColor, workflowResetColor, modelColor, agent.Model.Name, workflowResetColor)
	fmt.Printf("   %sInput directory:%s %s%s%s\n", directoryColor, workflowResetColor, directoryColor, agent.InputDir, workflowResetColor)
	fmt.Printf("   %sOutput directory:%s %s%s%s\n", directoryColor, workflowResetColor, directoryColor, agent.OutputDir, workflowResetColor)
	fmt.Printf("   %sSystem prompt:%s %s%s\n", promptColor, workflowResetColor, agent.SystemPrompt, workflowResetColor)
	fmt.Printf("   %sTemplate:%s %s%s\n", promptColor, workflowResetColor, agent.Template, workflowResetColor)

	// Format search status
	searchStatus := "false"
	searchColor := booleanFalseColor
	if agent.Search != nil {
		searchStatus = "true"
		searchColor = booleanTrueColor
	}
	fmt.Printf("   %sSearch:%s %s%s%s\n", searchColor, workflowResetColor, searchColor, searchStatus, workflowResetColor)

	// Format tools status
	toolsStatus := "false"
	toolsColor := booleanFalseColor
	if len(agent.EnabledTools) > 0 {
		toolsStatus = "true"
		toolsColor = booleanTrueColor
	}
	fmt.Printf("   %sTools enabled:%s %s%s%s\n", toolsColor, workflowResetColor, toolsColor, toolsStatus, workflowResetColor)

	// MCP  status
	mcpStatus := "false"
	mcpColor := booleanFalseColor
	if agent.MCP {
		mcpStatus = "true"
		mcpColor = booleanTrueColor
	}
	fmt.Printf("   %sMCP enabled:%s %s%s%s\n", mcpColor, workflowResetColor, mcpColor, mcpStatus, workflowResetColor)

	// Format think mode status
	thinkStatus := "false"
	thinkColor := booleanFalseColor
	if agent.Think {
		thinkStatus = "true"
		thinkColor = booleanTrueColor
	}
	fmt.Printf("   %sThink mode:%s %s%s%s\n", thinkColor, workflowResetColor, thinkColor, thinkStatus, workflowResetColor)

	// Format usage tracking status
	usageStatus := "false"
	usageColor := booleanFalseColor
	if agent.Usage {
		usageStatus = "true"
		usageColor = booleanTrueColor
	}
	fmt.Printf("   %sUsage tracking:%s %s%s%s\n", usageColor, workflowResetColor, usageColor, usageStatus, workflowResetColor)

	// Format markdown output status
	markdownStatus := "false"
	markdownColor := booleanFalseColor
	if agent.Markdown {
		markdownStatus = "true"
		markdownColor = booleanTrueColor
	}
	fmt.Printf("   %sMarkdown output:%s %s%s%s\n", markdownColor, workflowResetColor, markdownColor, markdownStatus, workflowResetColor)

	// Format pass through status
	passThroughStatus := "false"
	passThroughColor := booleanFalseColor
	if agent.PassThrough {
		passThroughStatus = "true"
		passThroughColor = booleanTrueColor
	}
	fmt.Printf("   %sPass through:%s %s%s%s\n", passThroughColor, workflowResetColor, passThroughColor, passThroughStatus, workflowResetColor)

	// Set the prompt question
	rl, err := readline.New(WokflowConfirmPrompt)
	if err != nil {
		fmt.Printf("Error initializing readline: %v. Skipping agent.\n", err)
		return false
	}
	defer rl.Close()

	input, err := rl.Readline()
	if err != nil {
		fmt.Printf("Error reading input: %v. Skipping agent.\n", err)
		return false
	}

	response := strings.ToLower(strings.TrimSpace(input))
	return response == "y" || response == "yes"
}

/*
waitForNewPrompt prompts the user to confirm proceeding with the next step,
returning an empty string on confirmation. If declined, it asks for input on
desired changes and returns the user's response in lowercase. Handles input errors
by printing a message and returning an empty string.
*/
func waitForNewPrompt() string {
	// Wait for user to proceed or modify the prompt
	rl, err := readline.New(WokflowProceedPrompt)
	if err != nil {
		fmt.Printf("Error initializing readline: %v. Skipping agent.\n", err)
		return ""
	}
	defer rl.Close()

	prompt, err := rl.Readline()
	if err != nil {
		fmt.Printf("Error reading input: %v. Skipping agent.\n", err)
		return ""
	}

	response := strings.ToLower(strings.TrimSpace(prompt))
	cont := response == "y" || response == "yes"
	if cont {
		return ""
	}

	// Create a new readline instance for modification prompt
	rl2, err := readline.New(WokflowModifyPrompt)
	if err != nil {
		fmt.Printf("Error initializing readline: %v. Skipping agent.\n", err)
		return ""
	}
	defer rl2.Close()

	prompt, err = rl2.Readline()
	if err != nil {
		fmt.Printf("Error reading input: %v. Skipping agent.\n", err)
		return ""
	}

	response = strings.ToLower(strings.TrimSpace(prompt))
	return response
}

/*
runMasterAgent executes the given WorkflowAgent as a master agent.

It builds the agent’s prompt by reading and concatenating the content of all files in the input directory,
then appending the provided prompt if input files exist. If there is no input directory, it uses the initial prompt.
After preparing the prompt and setting up the output file, it invokes the agent execution.

Parameters:

	agent - pointer to the WorkflowAgent to execute
	prompt - initial prompt for the agent

Returns an error if any step fails during file reading, prompt building, or agent execution.
*/
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
		finalPrompt = content.String() + "\n" + prompt
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
	err := executeAgent(agent, finalPrompt)
	return err
}

/*
runWorkerAgent processes all files in the specified input directory for the given WorkflowAgent.
It launches concurrent goroutines to execute agent tasks on each file, collects errors from all tasks,
and returns a joined error if any tasks fail, or nil on success.
*/
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
				fileName := strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))
				outputName := agent.Name + "_" + fileName + ".md"

				// Bugfix:
				// Create a copy of the agent to avoid race condition on OutputFile field
				// Because each task agent will write to its own output file
				taskAgent := *agent
				taskAgent.OutputFile = GetFilePath(agent.OutputDir, outputName)

				// Format the task info
				taskInfo := fmt.Sprintf("[%s] - %s", agent.Name, fileName)
				Infof("\t%s is working...", taskInfo)
				Infof("\ttask saved to: %s", taskAgent.OutputFile)

				// Execute worker agent
				err = executeAgent(&taskAgent, prompt)
				// If there wasn't error, send nil for success
				errChan <- err
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

/*
runWorkflowAgent runs the specified workflow agent based on its role.
It clears the agent's output directory, then executes the master or worker routine as appropriate.
Returns an error if setup or execution fails.
*/
func runWorkflowAgent(agent *WorkflowAgent, workflowPrompt string) error {
	// Clear the output directory before running the agent
	if err := clearupOutputDir(agent.OutputDir); err != nil {
		agentInfo := fmt.Sprintf("[%s (%s)]", agent.Name, agent.Role)
		err = fmt.Errorf("%s: %v", agentInfo, err)
		return err
	}

	// run agent based on its role
	switch agent.Role {
	case WorkflowAgentTypeMaster:
		err := runMasterAgent(agent, workflowPrompt)
		if err != nil {
			err = fmt.Errorf("workflow: %v", err)
			return err
		}

	case WorkflowAgentTypeWorker:
		err := runWorkerAgent(agent)
		if err != nil {
			err = fmt.Errorf("workflow: %v", err)
			return err
		}

	default:
		Warnf("Unknown agent role '%s' for agent %s, skipping.", agent.Role, agent.Name)
	}
	return nil
}

func measureWorkflowTime() func() {
	start := time.Now()
	return func() {
		elapsed := time.Since(start)
		formatted := FormatMinutesSeconds(elapsed)
		Infof("Workflow execution time: %v\n", formatted)
	}
}

/*
RunWorkflow executes a workflow using the provided WorkflowConfig and initial prompt.
It processes a series of agents in order, handling agent roles, interactive mode,
and prompt modifications for the first agent as needed. Returns an error if the
workflow encounters issues such as missing agent roles or agent execution errors.
*/
// RunWorkflow executes the defined workflow.
func RunWorkflow(config *WorkflowConfig, prompt string) error {
	var err error

	if len(config.Agents) == 0 {
		err = fmt.Errorf("no agents defined in the workflow")
		return err
	}

	// The initial prompt is passed to the first agent.
	workflowPrompt := prompt

	// Measure workflow time
	// use defer, even error occured it still can measure
	defer measureWorkflowTime()()

	for i, agent := range config.Agents {
		agentInfo := fmt.Sprintf("[%s (%s)]", agent.Name, agent.Role)

		// Print the agent working flow
		switch agent.Role {
		case WorkflowAgentTypeMaster:
			Infof("Agent %s is working...", agentInfo)

		case WorkflowAgentTypeWorker:
			Infof("Agent %s is working...", agentInfo)
		default:
			err = fmt.Errorf("Agent %s has no role defined", agent.Name)
			return err
		}

		// Pass through check
		if agent.PassThrough {
			Infof("Agent %s is passing through ↓", agentInfo)
			continue
		}

		// Interactive mode: ask for user confirmation before running the agent
		if config.InterActiveMode {
			if !promptUserForConfirmation(&agent) {
				Infof("Agent %s skipped by user.", agentInfo)
				continue
			}
		}

		if i == 0 {
			// First agent gets the temp convo name
			if agent.ConvoName == "" {
				convoName := GenerateTempFileName()
				agent.ConvoName = convoName
			}
		}

		// Run the agent
		err = runWorkflowAgent(&agent, workflowPrompt)
		if err != nil {
			return err
		}

		// Need user to confirm the output of the first agent
		// User can change its plan and execute again
		if i == 0 {
			for {
				// Ask user if they want to proceed or modify the prompt
				workflowPrompt = waitForNewPrompt()
				if workflowPrompt == "" {
					break
				}
				// Run the agent again with new prompt
				agent.Template = ""     // Reset template
				agent.SystemPrompt = "" // Reset system prompt
				err = runWorkflowAgent(&agent, workflowPrompt)
				if err != nil {
					return err
				}
			}
		}
	}
	Successf("Workflow finished.")
	return nil
}

/*
executeAgent initializes agent options and executes the workflow agent with the given prompt.
Sets quiet mode based on agent role. Returns an error if execution fails.
*/
func executeAgent(agent *WorkflowAgent, prompt string) error {
	// Only Master can output to the console, Worker in quiet mode
	quiet := (agent.Role == WorkflowAgentTypeWorker)

	agentOptions := AgentOptions{
		Prompt:         prompt,
		SysPrompt:      agent.SystemPrompt,
		Files:          nil,
		ModelInfo:      agent.Model,
		SearchEngine:   agent.Search,
		MaxRecursions:  agent.MaxRecursions,
		ThinkMode:      agent.Think,
		EnabledTools:   agent.EnabledTools,
		UseMCP:         agent.MCP,
		YoloMode:       true, // Always skip tools confirmation
		AppendMarkdown: agent.Markdown,
		AppendUsage:    agent.Usage,
		OutputFile:     agent.OutputFile, // Write to file
		QuietMode:      quiet,            // Worker in quiet mode
		ConvoName:      agent.ConvoName,  // conversation name, for iterate prompt
	}

	err := CallAgent(&agentOptions)
	return err
}
