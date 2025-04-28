// File: cmd/version.go
package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/activebook/gllm/service"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(modelCmd)
	modelCmd.AddCommand(modelListCmd)
	modelCmd.AddCommand(modelAddCmd)
	modelCmd.AddCommand(modelSetCmd)
	modelCmd.AddCommand(modelInfoCmd)
	modelCmd.AddCommand(modelRemoveCmd)
	modelCmd.AddCommand(modelClearCmd)
	modelCmd.AddCommand(modelDefaultCmd)

	// Add required flags to the add command
	modelAddCmd.Flags().StringP("name", "n", "", "Model name (required)")
	modelAddCmd.Flags().StringP("endpoint", "e", "", "API endpoint URL (required)")
	modelAddCmd.Flags().StringP("key", "k", "", "API key (required)")
	modelAddCmd.Flags().StringP("model", "m", "", "Model ID (required)")
	modelAddCmd.Flags().Float32P("temp", "t", 0.7, "Temperature for generation (default 0.7)")

	modelAddCmd.MarkFlagRequired("name")
	modelAddCmd.MarkFlagRequired("endpoint")
	modelAddCmd.MarkFlagRequired("key")
	modelAddCmd.MarkFlagRequired("model")

	// Add optional flags to the set command
	modelSetCmd.Flags().StringP("endpoint", "e", "", "API endpoint URL")
	modelSetCmd.Flags().StringP("key", "k", "", "API key")
	modelSetCmd.Flags().StringP("model", "m", "", "Model ID")
	modelSetCmd.Flags().Float32P("temp", "t", 0.7, "Temperature for generation (default 0.7)")

	// Add the force flag to the remove command
	modelRemoveCmd.Flags().BoolP("force", "f", false, "Skip error when model doesn't exist")
	modelClearCmd.Flags().BoolP("force", "f", false, "Clear all models without confirmation prompt")
}

// configCmd represents the base command when called without any subcommands
var modelCmd = &cobra.Command{
	Use:     "model",
	Aliases: []string{"md"}, // Optional alias
	Short:   "Manage gllm model configuration",
	Long:    `The 'gllm model' command allows you to manage your configured large language models(llms).`,
	// Run: func(cmd *cobra.Command, args []string) {
	//  fmt.Println("Use 'gllm config [subcommand] --help' for more information.")
	// },
	// Suggest showing help if 'gllm config' is run without subcommand
	RunE: func(cmd *cobra.Command, args []string) error {
		// If no subcommand is given, show help
		if len(args) == 0 {
			return cmd.Help()
		}
		return cmd.Help()
	},
}

var modelListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all set models",
	Run: func(cmd *cobra.Command, args []string) {
		models := viper.GetStringMapString("models")
		defaultModel := viper.GetString("default.model")

		if len(models) == 0 {
			fmt.Println("No model defined yet. Use 'gllm model add'.")
			return
		}

		fmt.Println("Available models:")
		// Sort keys for consistent output
		names := make([]string, 0, len(models))
		for name := range models {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			indicator := " "
			if name == defaultModel {
				indicator = "*" // Mark the default model
			}
			fmt.Printf(" %s %s\n", indicator, name)
		}
		if defaultModel != "" {
			fmt.Println("\n(*) Indicates the default model.")
		} else {
			fmt.Println("\nNo default model set. Use 'gllm model default <name>'.")
			fmt.Println("The first available model will be used if needed.")
		}
	},
}

var modelAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new named model",
	Long: `Adds a new model with a specific configuration.
Example:
gllm model add --name gpt4 --endpoint "..." --key $OPENAI_KEY --model gpt-4o --temp 0.7`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// When using MarkFlagRequired, Cobra will:
		// Validate the required flags are provided before your function runs
		// Show appropriate error messages
		// Prevent your function from executing if required flags are missing
		// No error check needed, ignore the error return value
		name, _ := cmd.Flags().GetString("name")
		endpoint, _ := cmd.Flags().GetString("endpoint")
		key, _ := cmd.Flags().GetString("key")
		model, _ := cmd.Flags().GetString("model")
		temp, _ := cmd.Flags().GetFloat32("temp")

		// Get existing models map
		modelsMap := viper.GetStringMap("models")
		if modelsMap == nil {
			modelsMap = make(map[string]interface{})
		}

		// Check if model already exists
		if _, exists := modelsMap[name]; exists {
			return fmt.Errorf("model named '%s' already exists. Use 'remove' first or use 'set' to change its config or choose a different name", name)
		}

		// Create new model config
		newModel := map[string]any{
			"endpoint":    endpoint,
			"key":         key,
			"model":       model,
			"temperature": temp,
		}

		// Add the new model
		modelsMap[name] = newModel
		viper.Set("models", modelsMap)

		// Set default model if none exists
		defaultModel := viper.GetString("default.model")
		if defaultModel == "" {
			viper.Set("default.model", name)
		}

		// Write the config file
		if err := writeConfig(); err != nil {
			return fmt.Errorf("failed to save model: %w", err)
		}

		fmt.Printf("Model '%s' added successfully.\n", name)
		return nil
	},
}

var modelSetCmd = &cobra.Command{
	Use:   "set Name",
	Short: "Set a named model",
	Long: `Sets a named model with a specific configuration.
Example:
gllm model set gpt4 --endpoint "..." --key $OPENAI_KEY --model gpt-4o --temp 0.7`,
	Args: cobra.ExactArgs(1), // Requires exactly one argument (the name)
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// Get or create model configuration
		modelsMap := viper.GetStringMap("models")
		if modelsMap == nil {
			return fmt.Errorf("there is no model yet, use 'add' first")
		}

		// Get or create model entry
		var modelConfig map[string]interface{}
		if existingConfig, exists := modelsMap[name]; exists {
			var ok bool
			if modelConfig, ok = existingConfig.(map[string]interface{}); !ok {
				modelConfig = make(map[string]interface{})
			}
		} else {
			return fmt.Errorf("model named '%s' not found", name)
		}

		// Update fields if flags are provided
		if endpoint, err := cmd.Flags().GetString("endpoint"); err == nil && endpoint != "" {
			modelConfig["endpoint"] = endpoint
		}

		if key, err := cmd.Flags().GetString("key"); err == nil && key != "" {
			modelConfig["key"] = key
		}

		if model, err := cmd.Flags().GetString("model"); err == nil && model != "" {
			modelConfig["model"] = model
		}

		if temp, err := cmd.Flags().GetFloat32("temp"); err == nil {
			modelConfig["temperature"] = temp // Note: May want to convert to float
		}

		// Update the entry
		modelsMap[name] = modelConfig
		viper.Set("models", modelsMap)

		// Write the config file
		if err := writeConfig(); err != nil {
			return fmt.Errorf("failed to save model: %w", err)
		}

		fmt.Printf("Model '%s' set successfully.\n", name)
		fmt.Println("---")
		for key, value := range modelConfig {
			fmt.Printf("%s: %v\n", key, value)
		}
		fmt.Println("---")
		return nil
	},
}

var modelInfoCmd = &cobra.Command{
	Use:     "info NAME",
	Aliases: []string{"in"},
	Short:   "Show the detail of a specific model",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// Get the models map with nested structure
		modelsMap := viper.GetStringMap("models")

		modelConfig, exists := modelsMap[name]
		if !exists {
			return fmt.Errorf("model named '%s' not found", name)
		}

		// Type assert to get the nested map
		if configMap, ok := modelConfig.(map[string]interface{}); ok {
			fmt.Printf("Model '%s':\n---\n", name)
			for key, value := range configMap {
				fmt.Printf("%s: %v\n", key, value)
			}
			fmt.Println("---")
			return nil
		}

		return fmt.Errorf("invalid configuration format for model '%s'", name)
	},
}

var modelRemoveCmd = &cobra.Command{
	Use:     "remove NAME",
	Aliases: []string{"rm"},
	Short:   "Remove a named model",
	Long: `Removes a named model.
	
Example:
gllm model remove gpt4
gllm model remove gpt4 --force`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		modelsMap := viper.GetStringMap("models")

		if _, exists := modelsMap[name]; !exists {
			cmd.SilenceUsage = true // Don't show usage for this error
			if force, _ := cmd.Flags().GetBool("force"); force {
				fmt.Printf("Model '%s' does not exist, nothing to remove.\n", name)
				return nil
			}
			return fmt.Errorf("model named '%s' not found", name)
		}

		// Delete the prompt
		delete(modelsMap, name)
		viper.Set("models", modelsMap)

		// Check if the removed model was the default
		defaultPrompt := viper.GetString("default.model")
		if name == defaultPrompt {
			viper.Set("default.model", "") // Clear the default
			fmt.Printf("Note: Removed model '%s' was the default. Default model cleared.\n", name)
		}

		// Write the config file
		if err := writeConfig(); err != nil {
			return fmt.Errorf("failed to save configuration after removing model: %w", err)
		}

		fmt.Printf("Model '%s' removed successfully.\n", name)
		return nil
	},
}

var modelClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all saved models",
	Long: `Remove all saved models from configuration.
This action cannot be undone.

Example:
gllm model clear
gllm model clear --force`,
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")

		if !force {
			fmt.Print("Are you sure you want to clear all models? This cannot be undone. [y/N]: ")
			var response string
			fmt.Scanln(&response)

			response = strings.ToLower(strings.TrimSpace(response))
			if response != "y" && response != "yes" {
				fmt.Println("Operation cancelled.")
				return nil
			}
		}

		// Clear all models
		modelsMap := viper.GetStringMap("models")
		for modelConfig := range modelsMap {
			delete(modelsMap, modelConfig)
		}
		viper.Set("models", modelsMap)

		// Clear default model if set
		if viper.IsSet("default.model") {
			viper.Set("default.model", "")
		}

		// Write config file
		if err := writeConfig(); err != nil {
			return fmt.Errorf("failed to update configuration: %w", err)
		}

		fmt.Println("All models have been cleared.")
		return nil
	},
}

var modelDefaultCmd = &cobra.Command{
	Use:     "default NAME",
	Aliases: []string{"def"},
	Short:   "Set the default model to use",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		models := viper.GetStringMapString("models")

		// Check if the model exists before setting it as default
		if _, exists := models[name]; !exists {
			return fmt.Errorf("model named '%s' not found. Cannot set as default", name)
		}

		viper.Set("default.model", name)

		// Write the config file
		if err := writeConfig(); err != nil {
			return fmt.Errorf("failed to save default model setting: %w", err)
		}

		fmt.Printf("Default model set to '%s' successfully.\n", name)
		return nil
	},
}

func GetAllModels() (map[string]string, error) {
	modelsMap := viper.GetStringMap("models")
	if modelsMap == nil {
		return nil, fmt.Errorf("no models found")
	}

	// Get keys and sort them
	keys := make([]string, 0, len(modelsMap))
	for k := range modelsMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	models := make(map[string]string)
	for _, k := range keys {
		v := modelsMap[k]
		if configMap, ok := v.(map[string]interface{}); ok {
			// Convert the inner map to a string representation
			var pairs []string
			for k, v := range configMap {
				pairs = append(pairs, fmt.Sprintf("\t%s: %v", k, v))
			}
			models[k] = strings.Join(pairs, "\n")
		} else {
			return nil, fmt.Errorf("invalid model configuration for '%s'", k)
		}
	}
	return models, nil
}

func SetEffectiveModel(name string) error {
	modelsMap := viper.GetStringMap("models")
	if _, ok := modelsMap[name]; !ok {
		return fmt.Errorf("model named '%s' not found", name)
	}

	viper.Set("default.model", name)
	if err := writeConfig(); err != nil {
		return fmt.Errorf("failed to update configuration: %w", err)
	}
	return nil
}

func GetEffectModelName() string {
	defaultName := viper.GetString("default.model")
	return defaultName
}

// GetEffectiveModel returns the configuration for the model to use
func GetEffectiveModel() (name string, details map[string]any) {
	defaultName := viper.GetString("default.model")
	modelsMap := viper.GetStringMap("models")

	// 1. Try to use default model
	if defaultName != "" {
		if modelConfig, ok := modelsMap[defaultName]; ok {
			// Convert the map[string]interface{} to map[string]string
			if configMap, ok := modelConfig.(map[string]interface{}); ok {
				return defaultName, configMap
			}
			service.Warnf("Warning: Default model '%s' has invalid configuration format", defaultName)
		} else {
			service.Warnf("Warning: Default model '%s' not found in configuration. Falling back...", defaultName)
		}
	}

	// 2. Fall back to the first model alphabetically
	if len(modelsMap) > 0 {
		names := make([]string, 0, len(modelsMap))
		for name := range modelsMap {
			names = append(names, name)
		}
		sort.Strings(names)

		firstModelName := names[0]
		logger.Debugf("Using first available model '%s' as fallback", firstModelName)

		if modelConfig, ok := modelsMap[firstModelName]; ok {
			if configMap, ok := modelConfig.(map[string]interface{}); ok {
				return firstModelName, configMap
			}
		}
	}

	// 3. No models available
	logger.Debugln("No model to use!")
	return "", nil
}
