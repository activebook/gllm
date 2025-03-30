// File: cmd/root.go
package cmd

import (
	"fmt"
	"os"
	"path/filepath" // Import filepath
	"strings"

	"github.com/activebook/gllm/service"
	homedir "github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus" // Import logrus
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// Hardcode the version string here
	version     = "v1.0.0" // <<< Set your desired version
	versionFlag bool       // To hold the version flag value

	cfgFile           string // To hold the path to the config file if specified via flag
	appConfigDir      string // Store the calculated config directory path
	appConfigFilePath string // Store the calculated config file path
	debugMode         bool   // Flag to enable debug logging

	// Global logger instance, configured by setupLogging
	logger = log.New()

	modelFlag     string   // gllm "What is Go?" -model(-m) gpt4o
	attachments   []string // gllm "Summarize this" --attachment(-a) report.txt
	sysPromptFlag string   // gllm "Act as shell" --system-prompt(-s) @shell-assistant
	templateFlag  string   // gllm --template(-t) @coder

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
			// You could put setupLogging() here if it depended on flags directly
			// in ways not handled by OnInitialize. For now, initConfig is fine.
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			// Your main command logic goes here
			// For example, you can print a message or perform some action
			logger.Debugln("Start processing...")
			// If no arguments and no relevant flags are set, show help instead
			// Args: cobra.ArbitraryArgs: This tells Cobra that receiving any number of positional arguments (including zero arguments) is perfectly valid.
			// It won't trigger an error or the help message based on the argument count alone.
			if len(args) == 0 &&
				!cmd.Flags().Changed("model") &&
				!cmd.Flags().Changed("system-prompt") &&
				!cmd.Flags().Changed("template") &&
				!cmd.Flags().Changed("attachment") &&
				!cmd.Flags().Changed("version") {
				cmd.Help()
				return
			}

			// print version
			if len(args) == 0 && versionFlag {
				fmt.Printf("%s %s\n", cmd.CommandPath(), version)
				return
			}

			prompt := ""
			// If prompt is provided, append it to the full prompt
			if len(args) > 0 {
				prompt = args[0]
			}

			// If model flag is provided, update the default model
			if cmd.Flags().Changed("model") {
				if StartsWith(modelFlag, "@") {
					modelFlag = RemoveFirst(modelFlag, "@")
					if err := SetEffectiveModel(modelFlag); err != nil {
						fmt.Printf("%v\n", err)
						fmt.Println("Using default model instead")
					}
				} else {
					fmt.Printf("model[%s] should start with @\n", modelFlag)
					fmt.Println("Using default model instead")
				}
			}

			// If system prompt is provided, update the default system prompt
			if sysPromptFlag != "" {
				if StartsWith(sysPromptFlag, "@") {
					// Using set system prompt
					sysPromptFlag = RemoveFirst(sysPromptFlag, "@")
					if err := SetEffectiveSystemPrompt(sysPromptFlag); err != nil {
						fmt.Printf("%v\n", err)
						fmt.Println("Using default system prompt instead")
					}
				} else {
					// Using plain adhoc system prompt
					SetPlainSystemPrompt(sysPromptFlag)
				}
			}

			// If template is provided, update the default template
			if templateFlag != "" {
				if StartsWith(templateFlag, "@") {
					// Using set template
					templateFlag = RemoveFirst(templateFlag, "@")
					if err := SetEffectiveTemplate(templateFlag); err != nil {
						fmt.Printf("%v\n", err)
						fmt.Println("Using default template instead")
					}
				} else {
					fmt.Printf("template[%s] should start with @\n", templateFlag)
					fmt.Println("Using default template instead")
				}
			}

			// Here you would interact with the specified model
			logger.Debugf("Using model: %s\n", modelFlag)
			logger.Debugf("Using template: %s\n", templateFlag)
			logger.Debugf("Prompt: %s\n", prompt)

			// Call your LLM service here
			var images []*service.ImageData
			isThereAttachment := cmd.Flags().Changed("attachment")
			prompt, images = buildPrompt(prompt, isThereAttachment)
			processQuery(prompt, images)
		},
	}
)

// Appends text to builder with proper newline handling
func appendText(builder *strings.Builder, text string) {
	if text == "" {
		return
	}
	builder.WriteString(text)
	if !strings.HasSuffix(text, "\n") {
		builder.WriteString("\n\n")
	}
}

// Processes a single attachment (file or stdin marker)
func processAttachment(path string) (string, *service.ImageData) {
	// Handle stdin or regular file
	content, err := readContentFromPath(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file[%s]: %v\n", path, err)
		return "", nil
	}

	// Check if content is an image
	isImage, format, err := service.CheckIfImageFromBytes(content)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error checking content type: %v\n", err)
		return "", nil
	}

	if isImage {
		return "", service.NewImageData(format, content)
	} else {
		return string(content), nil
	}
}

func buildPrompt(prompt string, isThereAttachment bool) (string, []*service.ImageData) {
	var finalPrompt strings.Builder
	images := []*service.ImageData{}

	// Add user prompt and template
	appendText(&finalPrompt, prompt)
	appendText(&finalPrompt, GetEffectiveTemplate())

	if isThereAttachment {
		// Process attachments
		for _, attachment := range attachments {
			textContent, imageData := processAttachment(attachment)
			if imageData != nil {
				images = append(images, imageData)
			}
			if textContent != "" {
				appendText(&finalPrompt, textContent)
			}
		}
	} else {
		// No attachments specified, try stdin
		stdinContent := readStdin()
		appendText(&finalPrompt, stdinContent)
	}

	return finalPrompt.String(), images
}

func processQuery(prompt string, images []*service.ImageData) {
	// Call your LLM service here
	model := GetEffectiveModel()
	sys_prompt := GetEffectiveSystemPrompt()
	service.CallLanguageModel(prompt, sys_prompt, images, model)
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	// Ensure the config directory exists before Cobra/Viper try to read from it
	// We do it here because initConfig might run *after* some Cobra initialization
	// that could potentially depend on the directory existing (though unlikely here).
	// It's generally safer to ensure prerequisites early.
	// Alternatively, put this inside initConfig before viper.ReadInConfig().
	if appConfigDir != "" { // Make sure appConfigDir has been calculated by init()
		if err := os.MkdirAll(appConfigDir, 0750); err != nil { // 0750 permissions: user rwx, group rx, others none
			logger.Errorf("Error creating config directory '%s': %v\n", appConfigDir, err)
			// Decide if this is a fatal error. Maybe just warn? For now, let's warn.
		}
	}

	if err := rootCmd.Execute(); err != nil {
		logger.Errorf("Whoops. There was an error while executing your CLI '%s'\n", err)
		os.Exit(1)
	}
}

func init() {
	// This function runs when the package is initialized.

	// Calculate config paths early
	initConfigPaths() // New function to calculate paths

	// Initialize Viper configuration
	cobra.OnInitialize(initConfig)

	// Define persistent flags (available to this command and all subcommands)
	// Define flags
	//rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", fmt.Sprintf("config file (default is %s)", appConfigFilePath))
	rootCmd.PersistentFlags().BoolVarP(&debugMode, "debug", "d", false, "Enable debug logging (overrides config file level)")

	// Disable the default completion command
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	// Define the flags
	rootCmd.Flags().StringVarP(&modelFlag, "model", "m", "", "Specify the language model to use")
	rootCmd.Flags().StringSliceVarP(&attachments, "attachment", "a", []string{}, "Specify file(s) or image(s) to append to the prompt")
	rootCmd.Flags().StringVarP(&sysPromptFlag, "system-prompt", "s", "", "Specify a system prompt")
	rootCmd.Flags().StringVarP(&templateFlag, "template", "t", "", "Specify a template to use")
	rootCmd.Flags().BoolVarP(&versionFlag, "version", "v", false, "Print the version number of gllm")

	// Add more persistent flags here if needed (e.g., --verbose, --log-file)
	// Set logrus defaults before configuration is loaded
	// This ensures basic logging works even if config fails
	logger.SetOutput(os.Stderr)
	logger.SetLevel(log.InfoLevel)            // Default to Info level initially
	logger.SetFormatter(&log.TextFormatter{}) // Default to TextFormatter
}

// initConfigPaths calculates the application's configuration directory and file path.
func initConfigPaths() {
	var err error
	// Prefer os.UserConfigDir()
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		// Fallback to home directory if UserConfigDir fails
		logger.Warnln("Warning: Could not find user config dir, falling back to home directory.", err)
		userConfigDir, err = homedir.Dir()
		cobra.CheckErr(err) // If home dir also fails, panic
	}

	// App specific directory: e.g., ~/.config/gllm or ~/Library/Application Support/gllm
	appConfigDir = filepath.Join(userConfigDir, "gllm")

	// Default config file path: e.g., ~/.config/gllm/.gllm.yaml
	appConfigFilePath = filepath.Join(appConfigDir, "gllm.yaml")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Search config in home directory with name ".gllm" (without extension).
		viper.AddConfigPath(appConfigDir) // Add home directory to search path
		viper.SetConfigName("gllm")       // Name of config file (without ext)
		viper.SetConfigType("yaml")       // REQUIRED if the config file does not have the extension in the name
	}

	viper.AutomaticEnv() // Read in environment variables that match

	// Set default log settings in Viper *before* reading the config
	// This ensures these keys exist even if not in the file
	viper.SetDefault("log.level", "info")

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		logger.Debugln("Using config file:", viper.ConfigFileUsed())
	} else {
		// Handle errors using the logger
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			logger.Debugf("Config file not found in %s or via --config flag. Using defaults/env vars.", appConfigDir)
		} else if os.IsNotExist(err) {
			logger.Debugf("Config file path %s does not exist. Using defaults/env vars.", viper.ConfigFileUsed())
		} else {
			// Use Warn or Error level for actual reading errors
			logger.Warnf("Error reading config file (%s): %v", viper.ConfigFileUsed(), err)
		}
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
			logger.Warnf("Invalid log level '%s' in config, using 'info': %v", logLevelStr, err)
			level = log.InfoLevel
			logLevelStr = "info (due to invalid config value)"
		} else {
		}
	}
	logger.SetLevel(level)

	// Log the final configuration being used (at Debug level)
	logger.Debugf("Logger initialized: level=%s ", logLevelStr)
}

// Helper function to get the calculated default config file path
// Useful for the 'config path' command.
func getDefaultConfigFilePath() string {
	// Ensure paths are calculated if they haven't been for some reason
	// (though 'init' should have handled this)
	if appConfigFilePath == "" {
		initConfigPaths()
	}
	return appConfigFilePath
}
