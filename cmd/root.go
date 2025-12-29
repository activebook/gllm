// File: cmd/root.go
package cmd

import (
	"fmt"
	"os" // Import filepath
	"strings"
	"sync"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/service"
	log "github.com/sirupsen/logrus" // Import logrus
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	versionFlag bool // To hold the version flag value
	debugMode   bool // Flag to enable debug logging

	// Global logger instance, configured by setupLogging
	logger = service.GetLogger()

	agentName   string   // gllm "What is Go?" -agent(-g) plan
	attachments []string // gllm "Summarize this" --attachment(-a) report.txt
	convoName   string   // gllm --conversation(-c) "My Conversation" "What is the stock price of Tesla right now?"
	yoloFlag    bool     // gllm -y, --yolo enable yolo mode (non-interactive)

	// Global cmd instance, to be used by subcommands
	rootCmd = &cobra.Command{
		Use:   "gllm [prompt]",
		Short: "A CLI tool to interact with Large Language Models (LLMs)",
		Long: `gllm is your command-line companion for interacting with various LLMs.
Configure your API keys and preferred models, then start chatting or executing commands.`,
		// Accept arbitrary arguments as prompts
		Args: cobra.ArbitraryArgs,
		// Uncomment the following line if your bare application
		// has an action associated with it:
		// Run: func(cmd *cobra.Command, args []string) { },
		// This ensures setupLogging runs *after* flags are parsed and *after* initConfig
		// PersistentPreRunE is usually a good place for things that need flags/config
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// This ensures setupLogging runs *after* flags are parsed and *after* initConfig

			// Check if we are running a help/version command or init itself
			if cmd.Name() == "help" || cmd.Name() == "init" || cmd.Name() == "version" || versionFlag {
				return nil
			}

			// Check if config file is loaded
			store := data.NewConfigStore()
			if store.ConfigFileUsed() == "" {
				// Config missing!
				// Ask user if they want to setup
				fmt.Println("Configuration file not found.")
				// We can't use 'huh' here easily without importing it, but since we are in 'cmd', we can call RunInitWizard
				// But we need to ask permission first?
				// Let's call RunInitWizard directly which has a "Welcome" note.
				// Or ask for confirmation first.

				// Standard simple confirm before launching full TUI
				fmt.Print("Would you like to run the setup wizard now? [Y/n]: ")
				var response string
				fmt.Scanln(&response) // Simple scan
				response = strings.ToLower(strings.TrimSpace(response))

				if response == "" || response == "y" || response == "yes" {
					return RunInitWizard()
				}

				return fmt.Errorf("configuration required to proceed. Run 'gllm init' to setup")
			}

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			// Your main command logic goes here
			// For example, you can print a message or perform some action
			service.Debugf("Start processing...\n")
			//service.Debugf("Arguments received: %#v\n", args)

			// If no arguments and no relevant flags are set, show help instead
			// Args: cobra.ArbitraryArgs: This tells Cobra that receiving any number of positional arguments (including zero arguments) is perfectly valid.
			// It won't trigger an error or the help message based on the argument count alone.
			if len(args) == 0 &&
				!cmd.Flags().Changed("agent") &&
				!cmd.Flags().Changed("attachment") &&
				!cmd.Flags().Changed("version") &&
				!cmd.Flags().Changed("conversation") &&
				!hasStdinData() {
				cmd.Help()
				return
			}

			// print version
			if len(args) == 0 && versionFlag {
				fmt.Printf("%s\n", version)
				return
			}

			prompt := ""
			// If prompt is provided, append it to the full prompt
			if len(args) > 0 {
				prompt = args[0]
			} else {
				// Read from stdin if no prompt is provided
				prompt = readStdin()
			}

			store := data.NewConfigStore()
			// If agent flag is provided, update the default agent
			if cmd.Flags().Changed("agent") {
				// Check if agent exists
				if store.GetAgent(agentName) == nil {
					service.Errorf("Agent %s does not exist", agentName)
					return
				}
				store.SetActiveAgent(agentName)
			}
			// Get active agent
			activeAgent := store.GetActiveAgent()
			if activeAgent == nil {
				service.Errorf("No active agent found")
				return
			}

			// Create an indeterminate progress bar
			indicator := service.NewIndicator("Processing...")

			var files []*service.FileData
			// Start a goroutine for your actual LLM work
			done := make(chan bool)
			go func() {
				// Process all prompt building
				isThereAttachment := cmd.Flags().Changed("attachment")
				prompt, files = buildPrompt(activeAgent, prompt, isThereAttachment)
				done <- true
			}()
			// Update the spinner until work is done
			<-done
			indicator.Stop()

			// Call your LLM service here
			processQuery(prompt, files)
		},
	}
)

// Processes a single attachment (file or stdin marker)
func processAttachment(path string) *service.FileData {
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

// batchAttachments processes multiple attachments concurrently and adds the resulting
// FileData objects to the provided files slice. It uses a WaitGroup to manage goroutines
// and a channel to collect results safely.
func batchAttachments(files *[]*service.FileData) {
	var wg sync.WaitGroup
	filesCh := make(chan *service.FileData, len(attachments))
	for _, attachment := range attachments {
		wg.Add(1)
		go func(att string) {
			defer wg.Done()
			fileData := processAttachment(att)
			if fileData != nil {
				filesCh <- fileData
			}
		}(attachment)
	}
	wg.Wait()
	close(filesCh)
	for fileData := range filesCh {
		*files = append(*files, fileData)
	}
}

func buildPrompt(agent *data.AgentConfig, prompt string, isThereAttachment bool) (string, []*service.FileData) {
	var sb strings.Builder
	files := []*service.FileData{}

	// Get template content
	store := data.NewConfigStore()
	templateContent := store.GetTemplate(agent.Template)
	appendText(&sb, templateContent)
	appendText(&sb, prompt)

	if isThereAttachment {
		// Process attachments
		batchAttachments(&files)
	} else {
		// No attachments specified, try stdin
		stdinContent := readStdin()
		if len(stdinContent) > 0 {
			appendText(&sb, stdinContent)
		}
	}

	// Process @ references in prompt
	finalPrompt := sb.String()
	atRefProcessor := service.NewAtRefProcessor()
	processedPrompt, err := atRefProcessor.ProcessText(finalPrompt)
	if err != nil {
		service.Warnf("Error processing @ references in prompt: %v", err)
		// Continue with original prompt if processing fails
		processedPrompt = finalPrompt
	}

	return processedPrompt, files
}

func processQuery(prompt string, files []*service.FileData) {
	// Get Active Agent
	store := data.NewConfigStore()
	activeAgent := store.GetActiveAgent()
	if activeAgent == nil {
		service.Errorf("No active agent found")
		return
	}

	// Yolo mode(non-interactive mode)
	yolo := false
	if yoloFlag {
		yolo = true
	}

	// Get system prompt
	sys_prompt := store.GetSystemPrompt(activeAgent.SystemPrompt)

	// Get memory content
	memStore := data.NewMemoryStore()
	memoryContent := memStore.GetFormatted()
	if memoryContent != "" {
		sys_prompt += "\n\n" + memoryContent
	}

	// Load MCP config
	mcpStore := data.NewMCPStore()
	mcpConfig, _, _ := mcpStore.Load()

	// Check whether model is valid
	if activeAgent.Model.Name == "" {
		service.Errorf("No model specified")
		return
	} else {
		model := store.GetModel(activeAgent.Model.Name)
		if model == nil {
			service.Errorf("Model %s not found", activeAgent.Model.Name)
			return
		}
	}

	// Call your LLM service here
	op := service.AgentOptions{
		Prompt:         prompt,
		SysPrompt:      sys_prompt,
		Files:          files,
		ModelInfo:      &activeAgent.Model,
		SearchEngine:   &activeAgent.Search,
		MaxRecursions:  activeAgent.MaxRecursions,
		ThinkMode:      activeAgent.Think,
		EnabledTools:   activeAgent.Tools,
		UseMCP:         activeAgent.MCP,
		YoloMode:       yolo,
		AppendUsage:    activeAgent.Usage,
		AppendMarkdown: activeAgent.Markdown,
		OutputFile:     "",
		QuietMode:      false,
		ConvoName:      convoName,
		MCPConfig:      mcpConfig,
	}
	err := service.CallAgent(&op)
	if err != nil {
		service.Errorf("%v", err)
		return
	}
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	// Ensure MCPClient resources are cleaned up on exit
	// This is a safeguard; the shared instance should ideally be closed only once
	// when the application is truly exiting, not after every command execution.
	// Hence, we place it here in Execute() which is called once in main.
	// If you create separate MCPClient instances elsewhere, ensure they are closed too.
	// If the application grows more complex (e.g., with subcommands that run indefinitely),
	// consider a more robust lifecycle management strategy.
	defer service.GetMCPClient().Close()
	if err := rootCmd.Execute(); err != nil {
		service.Errorf("'%s'\n", err)
		os.Exit(1)
	}
}

// This function runs when the package is initialized.
func init() {
	// Initialize Viper configuration
	cobra.OnInitialize(initConfig)

	// Define persistent flags (available to this command and all subcommands)
	// Define flags
	//rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", fmt.Sprintf("config file (default is %s)", appConfigFilePath))
	rootCmd.PersistentFlags().BoolVarP(&debugMode, "debug", "d", false, "Enable debug logging (overrides config file level)")

	// Disable the default completion command
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	// Define the flags
	rootCmd.Flags().StringVarP(&agentName, "agent", "g", "", "Switch to the agent to use")
	rootCmd.Flags().StringSliceVarP(&attachments, "attachment", "a", []string{}, "Specify file(s), image(s), url(s) to append to the prompt")
	rootCmd.Flags().StringVarP(&convoName, "conversation", "c", "", "Specify a conversation name to track chat session")
	rootCmd.Flags().BoolVarP(&yoloFlag, "yolo", "y", false, "Enable yolo mode (non-interactive)")
	rootCmd.Flags().BoolVarP(&versionFlag, "version", "v", false, "Print the version number of gllm")

	// Set logrus defaults before configuration is loaded
	// This ensures basic logging works even if config fails
	service.InitLogger()
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	// Ensure the config directory exists before Cobra/Viper try to read from it
	if err := data.EnsureConfigDir(); err != nil {
		cobra.CheckErr(err)
		return
	}

	// Initialize the config store, and read the config file
	// It only needs to be done once, and viper will handle the rest
	store := data.NewConfigStore()
	err := store.SetConfigFile(data.GetConfigFilePath())
	if err != nil {
		cobra.CheckErr(err)
		return
	}

	// *** Placeholder for Log Configuration ***
	// We will add log setup based on Viper settings later.
	setupLogging()
}

// setupLogging configures the global logger based on Viper settings and flags.
func setupLogging() {
	logLevelStr := viper.GetString("log.level")

	// --- Determine Log Level ---
	// Flag overrides config
	level := log.InfoLevel // Default
	if debugMode {
		// override config log level if debug flag is set
		level = log.DebugLevel
		logLevelStr = "debug" // For logging confirmation
	} else {
		var err error
		level, err = log.ParseLevel(logLevelStr)
		if err != nil {
			service.Warnf("Invalid log level '%s' in config, using 'info': %v", logLevelStr, err)
			level = log.InfoLevel
			logLevelStr = "info (due to invalid config value)"
		} else {
		}
	}
	logger.SetLevel(level)

	// Log the final configuration being used (at Debug level)
	service.Debugf("Logger initialized: level=%s ", logLevelStr)
}
