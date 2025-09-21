// File: cmd/version.go
package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/activebook/gllm/service"
	"github.com/fatih/color"
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
	modelCmd.AddCommand(modelSwitchCmd)

	// Add flags to the list command
	modelListCmd.Flags().BoolP("verbose", "v", false, "Show model names and their content")

	// Add required flags to the add command
	modelAddCmd.Flags().StringP("name", "n", "", "Model name (required)")
	modelAddCmd.Flags().StringP("endpoint", "e", "", "API endpoint URL (required)")
	modelAddCmd.Flags().StringP("key", "k", "", "API key (required)")
	modelAddCmd.Flags().StringP("model", "m", "", "Model ID (required)")
	modelAddCmd.Flags().Float32P("temp", "t", 0.7, "Temperature for generation (default 0.7)")
	modelAddCmd.Flags().Float32P("top_p", "p", 1.0, "Top-p sampling parameter (default 1.0)")
	modelAddCmd.Flags().IntP("seed", "s", 0, "Seed for deterministic generation (default 0, use 0 for random)")

	modelAddCmd.MarkFlagRequired("name")
	modelAddCmd.MarkFlagRequired("endpoint")
	modelAddCmd.MarkFlagRequired("key")
	modelAddCmd.MarkFlagRequired("model")

	// Add optional flags to the set command
	modelSetCmd.Flags().StringP("endpoint", "e", "", "API endpoint URL")
	modelSetCmd.Flags().StringP("key", "k", "", "API key")
	modelSetCmd.Flags().StringP("model", "m", "", "Model ID")
	modelSetCmd.Flags().Float32P("temp", "t", 0.7, "Temperature for generation (default 0.7)")
	modelSetCmd.Flags().Float32P("top_p", "p", 1.0, "Top-p sampling parameter (default 1.0)")
	modelSetCmd.Flags().IntP("seed", "s", 0, "Seed for deterministic generation (default 0, use 0 for random)")

	// Add the force flag to the remove command
	modelRemoveCmd.Flags().BoolP("force", "f", false, "Skip error when model doesn't exist")
	modelClearCmd.Flags().BoolP("force", "f", false, "Clear all models without confirmation prompt")
}

// encodeModelName encodes model names that contain dots to avoid Viper path interpretation issues
func encodeModelName(name string) string {
	return strings.ReplaceAll(name, ".", "#dot#")
}

// decodeModelName decodes model names that were encoded to avoid Viper path interpretation issues
func decodeModelName(name string) string {
	return strings.ReplaceAll(name, "#dot#", ".")
}

// modelCmd represents the base command when called without any subcommands
var modelCmd = &cobra.Command{
	Use:     "model",
	Aliases: []string{"md"}, // Optional alias
	Short:   "Manage gllm model configuration",
	Long:    `The 'gllm model' command allows you to manage your configured large language models(llms).`,
	Run: func(cmd *cobra.Command, args []string) {
		// Simply delegate to the list command for consistency
		modelListCmd.Run(cmd, args)
	},
}

var modelListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all set models",
	Run: func(cmd *cobra.Command, args []string) {
		highlightColor := color.New(color.FgGreen, color.Bold).SprintFunc()
		modelsMap := viper.GetStringMap("models")
		defaultModel := viper.GetString("agent.model")

		if len(modelsMap) == 0 {
			fmt.Println("No model defined yet. Use 'gllm model add'.")
			return
		}

		verbose, _ := cmd.Flags().GetBool("verbose")

		fmt.Println("Available models:")
		// Sort keys for consistent output
		names := make([]string, 0, len(modelsMap))
		for name := range modelsMap {
			names = append(names, decodeModelName(name))
		}
		sort.Strings(names)

		for _, name := range names {
			indicator := " "
			pname := ""
			encodedName := encodeModelName(name)
			if encodedName == defaultModel {
				indicator = highlightColor("*") // Mark the default model
				pname = highlightColor(name)
			} else {
				indicator = " "
				pname = name
			}
			fmt.Printf(" %s %s\n", indicator, pname)
			if verbose {
				if configMap, ok := modelsMap[encodedName].(map[string]interface{}); ok {
					for key, value := range configMap {
						fmt.Printf("\t%s: %v\n", key, value)
					}
				}
			}
		}
		if defaultModel != "" {
			fmt.Println("\n(*) Indicates the current model.")
		} else {
			fmt.Println("\nNo model selected. Use 'gllm model switch <name>' to select one.")
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
		topP, _ := cmd.Flags().GetFloat32("top_p")
		seed, _ := cmd.Flags().GetInt("seed")

		// Get existing models map
		modelsMap := viper.GetStringMap("models")
		if modelsMap == nil {
			modelsMap = make(map[string]interface{})
		}

		encodedName := encodeModelName(name)

		// Check if model already exists
		if _, exists := modelsMap[encodedName]; exists {
			return fmt.Errorf("model named '%s' already exists. Use 'remove' first or use 'set' to change its config or choose a different name", name)
		}

		// Create new model config
		newModel := map[string]any{
			"endpoint":    endpoint,
			"key":         key,
			"model":       model,
			"temperature": temp,
			"top_p":       topP,
		}

		// Validate temperature value (should be between 0 and 2.0)
		if temp < 0 || temp > 2.0 {
			return fmt.Errorf("temperature must be between 0 and 2.0, got: %f", temp)
		}

		// Validate top_p value (should be between 0 and 1, exclusive of 0)
		if topP <= 0 || topP > 1.0 {
			return fmt.Errorf("top_p must be greater than 0 and less than or equal to 1.0, got: %f", topP)
		}

		// Only add seed if it's not 0 (0 means random)
		if seed != 0 {
			newModel["seed"] = seed
		}

		// Add the new model
		modelsMap[encodedName] = newModel
		viper.Set("models", modelsMap)

		// Set default model if none exists
		defaultModel := viper.GetString("agent.model")
		if defaultModel == "" {
			viper.Set("agent.model", encodedName)
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
		encodedName := encodeModelName(name)

		// Get or create model configuration
		modelsMap := viper.GetStringMap("models")
		if modelsMap == nil {
			return fmt.Errorf("there is no model yet, use 'add' first")
		}

		// Get or create model entry
		var modelConfig map[string]interface{}
		if existingConfig, exists := modelsMap[encodedName]; exists {
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

		// Only update temperature if the flag was explicitly provided
		if cmd.Flags().Changed("temp") {
			if temp, err := cmd.Flags().GetFloat32("temp"); err == nil {
				// Validate temperature value (should be between 0 and 2.0)
				if temp < 0 || temp > 2.0 {
					return fmt.Errorf("temperature must be between 0 and 2.0, got: %f", temp)
				}
				modelConfig["temperature"] = temp
			}
		}

		// Only update top_p if the flag was explicitly provided
		if cmd.Flags().Changed("top_p") {
			if topP, err := cmd.Flags().GetFloat32("top_p"); err == nil {
				// Validate top_p value (should be between 0 and 1, exclusive of 0)
				if topP <= 0 || topP > 1.0 {
					return fmt.Errorf("top_p must be greater than 0 and less than or equal to 1.0, got: %f", topP)
				}
				modelConfig["top_p"] = topP
			}
		}

		// Only update seed if the flag was explicitly provided
		if cmd.Flags().Changed("seed") {
			if seed, err := cmd.Flags().GetInt("seed"); err == nil {
				// Only add seed if it's not 0 (0 means random)
				if seed != 0 {
					modelConfig["seed"] = seed
				} else {
					// Remove seed if set to 0 (random)
					delete(modelConfig, "seed")
				}
			}
		}

		// Update the entry
		modelsMap[encodedName] = modelConfig
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
		encodedName := encodeModelName(name)

		// Get the models map with nested structure
		modelsMap := viper.GetStringMap("models")

		modelConfig, exists := modelsMap[encodedName]
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
		encodedName := encodeModelName(name)
		modelsMap := viper.GetStringMap("models")

		if _, exists := modelsMap[encodedName]; !exists {
			cmd.SilenceUsage = true // Don't show usage for this error
			if force, _ := cmd.Flags().GetBool("force"); force {
				fmt.Printf("Model '%s' does not exist, nothing to remove.\n", name)
				return nil
			}
			return fmt.Errorf("model named '%s' not found", name)
		}

		// Delete the prompt
		delete(modelsMap, encodedName)
		viper.Set("models", modelsMap)

		// Check if the removed model was the default
		defaultPrompt := viper.GetString("agent.model")
		if encodedName == defaultPrompt {
			viper.Set("agent.model", "") // Clear the default
			fmt.Printf("Note: Removed model '%s' was the agent. Default model cleared.\n", name)
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
		if viper.IsSet("agent.model") {
			viper.Set("agent.model", "")
		}

		// Write config file
		if err := writeConfig(); err != nil {
			return fmt.Errorf("failed to update configuration: %w", err)
		}

		fmt.Println("All models have been cleared.")
		return nil
	},
}

var modelSwitchCmd = &cobra.Command{
	Use:     "switch NAME",
	Aliases: []string{"sw", "select"},
	Short:   "Switch to a different model",
	Long: `Switch to a different model configuration. This will change your current model
to the specified one for all subsequent operations.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		encodedName := encodeModelName(name)
		models := viper.GetStringMap("models")

		// Check if the model exists before switching to it
		if _, exists := models[encodedName]; !exists {
			return fmt.Errorf("model named '%s' not found. Use 'gllm model list' to see available models", name)
		}

		viper.Set("agent.model", encodedName)

		// Write the config file
		if err := writeConfig(); err != nil {
			return fmt.Errorf("failed to save model setting: %w", err)
		}

		fmt.Printf("Switched to model '%s' successfully.\n", name)
		fmt.Println("This model will be used for all subsequent operations.")
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
		decodedName := decodeModelName(k)
		v := modelsMap[k]
		if configMap, ok := v.(map[string]interface{}); ok {
			// Convert the inner map to a string representation
			var pairs []string
			for k, v := range configMap {
				pairs = append(pairs, fmt.Sprintf("\t%s: %v", k, v))
			}
			sort.Strings(pairs)
			models[decodedName] = strings.Join(pairs, "\n")
		} else {
			return nil, fmt.Errorf("invalid model configuration for '%s'", decodedName)
		}
	}
	return models, nil
}

func SetEffectiveModel(name string) error {
	encodedName := encodeModelName(name)
	modelsMap := viper.GetStringMap("models")
	if _, ok := modelsMap[encodedName]; !ok {
		return fmt.Errorf("model named '%s' not found", name)
	}

	viper.Set("agent.model", encodedName)
	if err := writeConfig(); err != nil {
		return fmt.Errorf("failed to update configuration: %w", err)
	}
	return nil
}

func GetEffectModelName() string {
	defaultName := viper.GetString("agent.model")
	return decodeModelName(defaultName)
}

func GetModelInfo(name string) (details map[string]any) {
	encodedName := encodeModelName(name)
	modelsMap := viper.GetStringMap("models")

	if modelsMap == nil {
		return nil
	}

	modelConfig, exists := modelsMap[encodedName]
	if !exists {
		return nil
	}

	// Type assert to get the nested map
	if configMap, ok := modelConfig.(map[string]interface{}); ok {
		return configMap
	}

	return nil
}

// GetEffectiveModel returns the configuration for the model to use
func GetEffectiveModel() (name string, details map[string]any) {
	defaultName := viper.GetString("agent.model")
	modelsMap := viper.GetStringMap("models")

	// 1. Try to use default model
	if defaultName != "" {
		if modelConfig, ok := modelsMap[defaultName]; ok {
			// Convert the map[string]interface{} to map[string]string
			if configMap, ok := modelConfig.(map[string]interface{}); ok {
				return decodeModelName(defaultName), configMap
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
				return decodeModelName(firstModelName), configMap
			}
		}
	}

	// 3. No models available
	logger.Debugln("No model to use!")
	return "", nil
}
