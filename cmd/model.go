// File: cmd/model.go
package cmd

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/service"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
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
	// e.g. ./gllm model add --name minimax2 --provider openai --endpoint "https://api.openai.com/v1" --key "bbcc" --model gpt-4o --temp 0.5 --top_p 0.8 --seed 0
	modelAddCmd.Flags().StringP("name", "n", "", "Model name (required)")
	modelAddCmd.Flags().StringP("provider", "p", "", "Model provider (required)")
	modelAddCmd.Flags().StringP("endpoint", "e", "", "API endpoint URL (required)")
	modelAddCmd.Flags().StringP("key", "k", "", "API key (required)")
	modelAddCmd.Flags().StringP("model", "m", "", "Model ID (required)")
	modelAddCmd.Flags().Float32P("temp", "t", 1.0, "Temperature for generation")
	modelAddCmd.Flags().Float32P("top_p", "o", 1.0, "Top-p sampling parameter")
	modelAddCmd.Flags().IntP("seed", "s", 0, "Seed for deterministic generation (default 0, use 0 for random)")

	// Add optional flags to the set command
	// e.g. ./gllm model set minimax2 --provider openai --endpoint "https://api.openai.com/v1" --key "bbcc" --model gpt-5 --temp 0.75 --top_p 0.9 --seed 1010
	modelSetCmd.Flags().StringP("provider", "p", "", "Model provider (required)")
	modelSetCmd.Flags().StringP("endpoint", "e", "", "API endpoint URL")
	modelSetCmd.Flags().StringP("key", "k", "", "API key")
	modelSetCmd.Flags().StringP("model", "m", "", "Model ID")
	modelSetCmd.Flags().Float32P("temp", "t", 1.0, "Temperature for generation")
	modelSetCmd.Flags().Float32P("top_p", "o", 1.0, "Top-p sampling parameter")
	modelSetCmd.Flags().IntP("seed", "s", 0, "Seed for deterministic generation (default 0, use 0 for random)")

	// Add the force flag to the remove command
	modelRemoveCmd.Flags().BoolP("force", "f", false, "Skip error when model doesn't exist")
	modelClearCmd.Flags().BoolP("force", "f", false, "Clear all models without confirmation prompt")
}

// modelCmd represents the base command when called without any subcommands
var modelCmd = &cobra.Command{
	Use:     "model",
	Aliases: []string{"md"}, // Optional alias
	Short:   "Manage gllm model configuration",
	Long:    `The 'gllm model' command allows you to manage your configured large language models(llms).`,
	Run: func(cmd *cobra.Command, args []string) {
		// Simply delegate to the list command for consistency
		modelListCmd.Run(modelListCmd, args)
	},
}

var modelListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all set models",
	Run: func(cmd *cobra.Command, args []string) {
		store := data.NewConfigStore()
		modelsMap := store.GetModels()

		// Get default model name from active agent
		defaultModelName := ""
		activeAgent := store.GetActiveAgent()
		if activeAgent != nil {
			defaultModelName = activeAgent.Model.Name
		}

		if len(modelsMap) == 0 {
			fmt.Println("No model defined yet. Use 'gllm model add'.")
			return
		}

		verbose, _ := cmd.Flags().GetBool("verbose")

		fmt.Println("Available models:")
		// Sort keys for consistent output
		names := make([]string, 0, len(modelsMap))
		for name := range modelsMap {
			// In data layer, keys might be encoded or raw. Display decoded.
			// Ideally we don't need encoding anymore if not using viper dot notation,
			// but for safety with existing config files:
			names = append(names, name)
		}
		sort.Strings(names)

		// Map from display name to actual key
		displayToKey := make(map[string]string)
		for k := range modelsMap {
			displayToKey[k] = k
		}

		for _, name := range names {
			indicator := " "
			pname := ""

			modelName := displayToKey[name]

			// Check if this model is default (compare to defaultModel string)
			if modelName == defaultModelName {
				indicator = highlightColor("*") // Mark the default model
				pname = highlightColor(name)
			} else {
				indicator = " "
				pname = name
			}
			fmt.Printf(" %s %s\n", indicator, pname)
			if verbose {
				if modelConfig := modelsMap[modelName]; modelConfig != nil {
					fmt.Printf("\tProvider: %s\n", modelConfig.Provider)
					fmt.Printf("\tEndpoint: %s\n", modelConfig.Endpoint)
					fmt.Printf("\tModel: %s\n", modelConfig.Model)
					fmt.Printf("\tTemp: %v\n", modelConfig.Temp)
					fmt.Printf("\tTopP: %v\n", modelConfig.TopP)
					if modelConfig.Seed != nil {
						fmt.Printf("\tSeed: %d\n", *modelConfig.Seed)
					}
				}
			}
		}
		if defaultModelName != "" {
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
  gllm model add --name gpt4 --endpoint "..." --key $OPENAI_KEY --model gpt-4o --temp 1.0`,
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		provider, _ := cmd.Flags().GetString("provider")
		endpoint, _ := cmd.Flags().GetString("endpoint")
		key, _ := cmd.Flags().GetString("key")
		model, _ := cmd.Flags().GetString("model")
		temp, _ := cmd.Flags().GetFloat32("temp")
		topP, _ := cmd.Flags().GetFloat32("top_p")
		seed, _ := cmd.Flags().GetInt("seed")

		store := data.NewConfigStore()

		// Interactive mode if critical flags are missing
		if name == "" || provider == "" || endpoint == "" || key == "" || model == "" {

			// 1. Name
			if name == "" {
				err := huh.NewForm(
					huh.NewGroup(
						huh.NewInput().
							Title("Model Name").
							Value(&name).
							Validate(func(s string) error {
								s = strings.TrimSpace(s)
								if s == "" {
									return fmt.Errorf("name is required")
								}
								if err := CheckModelName(s); err != nil {
									return err
								}
								// Check existence
								if _, exists := store.GetModels()[s]; exists {
									return fmt.Errorf("model '%s' already exists", s)
								}
								return nil
							}),
					),
				).Run()
				if err != nil {
					return nil
				}
			}

			// 2. Provider (to help with defaults)
			err := huh.NewForm(
				huh.NewGroup(
					huh.NewSelect[string]().
						Title("Provider").
						Description("Select the provider for the model.\nps. OpenAI Compatible means the model is compatible with OpenAI API, e.g. OpenRouter, Chinese Models, etc.").
						Options(
							huh.NewOption("OpenAI", service.ModelProviderOpenAI),
							huh.NewOption("Anthropic", service.ModelProviderAnthropic),
							huh.NewOption("Google Gemini", service.ModelProviderGemini),
							huh.NewOption("Other (OpenAI Compatible)", service.ModelProviderOpenAICompatible),
						).
						Value(&provider),
				),
			).Run()
			if err != nil {
				return nil
			}

			defaultEndpoint := ""
			defaultModel := ""
			switch provider {
			case service.ModelProviderOpenAI:
				defaultEndpoint = "https://api.openai.com/v1"
				defaultModel = "gpt-5.2"
			case service.ModelProviderAnthropic:
				defaultEndpoint = "https://api.anthropic.com/v1"
				defaultModel = "claude-4-5-sonnet"
			case service.ModelProviderGemini:
				defaultEndpoint = "https://generativelanguage.googleapis.com"
				defaultModel = "gemini-flash-latest"
			case service.ModelProviderOpenAICompatible:
				defaultEndpoint = "https://openrouter.ai/api/v1"
				defaultModel = "deepseek-r1"
			}

			// 3. Endpoint, Key, Model ID
			err = huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Endpoint").
						Value(&endpoint).
						Placeholder(defaultEndpoint).
						Suggestions([]string{defaultEndpoint}).
						Validate(func(s string) error {
							if !strings.HasPrefix(s, "https://") {
								return fmt.Errorf("endpoint must start with 'https://'")
							}
							return nil
						}),
					huh.NewInput().
						Title("API Key").
						Value(&key).
						EchoMode(huh.EchoModePassword),
					huh.NewInput().
						Title("Model ID").
						Value(&model).
						Placeholder(defaultModel).
						Suggestions([]string{defaultModel}),
				),
			).Run()
			if err != nil {
				return nil
			}

			// Fill empty values with defaults if user just hit enter?
			if endpoint == "" && defaultEndpoint != "" {
				endpoint = defaultEndpoint
			}
			if model == "" && defaultModel != "" {
				model = defaultModel
			}

			if endpoint == "" || key == "" || model == "" {
				return fmt.Errorf("endpoint, key, and model are required")
			}

			// 4. Advanced Settings
			var tempStr = "1.0"
			var topPStr = "1.0"
			var seedStr = ""

			err = huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Temperature").
						Description("Controls randomness (0.0 - 2.0). Lower is more deterministic.").
						Value(&tempStr).
						Validate(ValidateTemperature),
					huh.NewInput().
						Title("Top P").
						Description("Nucleus sampling (0.0 - 1.0). Limits token choices to top probability mass.").
						Value(&topPStr).
						Validate(ValidateTopP),
					huh.NewInput().
						Title("Seed").
						Description("Integer for deterministic generation. Leave empty for random.").
						Value(&seedStr).
						Validate(ValidateSeed),
				).Title("Advanced Settings"),
			).Run()
			if err != nil {
				return nil
			}

			// Parse advanced settings
			if t, err := strconv.ParseFloat(tempStr, 32); err == nil {
				temp = float32(t)
			}
			if p, err := strconv.ParseFloat(topPStr, 32); err == nil {
				topP = float32(p)
			}
			if s, err := strconv.Atoi(seedStr); err == nil {
				seed = s
			}
		}

		modelsMap := store.GetModels()

		// Check existence again (redundant with form validation but safe for flags)
		if _, exists := modelsMap[name]; exists {
			return fmt.Errorf("model named '%s' already exists. Use 'remove' first or use 'set' to change its config or choose a different name", name)
		}

		// Create new model config
		newModel := data.Model{
			Provider: provider,
			Endpoint: endpoint,
			Key:      key,
			Model:    model,
			Temp:     temp,
			TopP:     topP,
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
			s := int32(seed)
			newModel.Seed = &s
		}

		// Add the new model via data layer
		if err := store.SetModel(name, &newModel); err != nil {
			return fmt.Errorf("failed to save model: %w", err)
		}

		fmt.Printf("Model '%s' added successfully.\n", name)
		return nil
	},
}

var modelSetCmd = &cobra.Command{
	Use:   "set [NAME]",
	Short: "Set a named model",
	Long: `Sets a named model with a specific configuration.
Example:
gllm model set gpt4 --endpoint "..." --key $OPENAI_KEY --model gpt-4o --temp 1.0`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var name string
		store := data.NewConfigStore()

		if len(args) > 0 {
			name = args[0]
		} else {
			// Default to current model
			model := GetEffectModel()
			if model != nil {
				name = model.Name
			}

			// Interactive select
			modelsMap := store.GetModels()
			if len(modelsMap) == 0 {
				return fmt.Errorf("no models found")
			}
			var options []huh.Option[string]
			for m := range modelsMap {
				options = append(options, huh.NewOption(m, m))
			}
			sort.Slice(options, func(i, j int) bool { return options[i].Key < options[j].Key })

			err := huh.NewSelect[string]().
				Title("Select Model to Edit").
				Options(options...).
				Value(&name).
				Run()
			if err != nil {
				return nil
			}
		}

		// Find model
		modelConfig := store.GetModel(name)
		if modelConfig == nil {
			return fmt.Errorf("model named '%s' not found", name)
		}

		// Update fields if flags are provided, OR if interactive
		// If NO flags provided, run interactive edit
		if cmd.Flags().NFlag() == 0 {
			var provider, endpoint, key, model string
			var tempStr = "1.0"
			var topPStr = "1.0"
			var seedStr = ""

			// Populate from existing
			provider = modelConfig.Provider
			endpoint = modelConfig.Endpoint
			key = modelConfig.Key
			model = modelConfig.Model
			tempStr = fmt.Sprintf("%v", modelConfig.Temp)
			topPStr = fmt.Sprintf("%v", modelConfig.TopP)
			if modelConfig.Seed != nil {
				seedStr = fmt.Sprintf("%v", *modelConfig.Seed)
			}

			err := huh.NewForm(
				huh.NewGroup(
					huh.NewSelect[string]().
						Title("Provider").
						Options(
							huh.NewOption("OpenAI", service.ModelProviderOpenAI),
							huh.NewOption("Anthropic", service.ModelProviderAnthropic),
							huh.NewOption("Google Gemini", service.ModelProviderGemini),
							huh.NewOption("Other (OpenAI Compatible)", service.ModelProviderOpenAICompatible),
						).
						Value(&provider),
				),
				huh.NewGroup(
					huh.NewInput().
						Title("Endpoint").
						Value(&endpoint).
						Validate(func(s string) error {
							if !strings.HasPrefix(s, "https://") {
								return fmt.Errorf("endpoint must start with 'https://'")
							}
							return nil
						}),
					huh.NewInput().Title("API Key").Value(&key).EchoMode(huh.EchoModePassword),
					huh.NewInput().Title("Model ID").Value(&model),
				),
				huh.NewGroup(
					huh.NewInput().
						Title("Temperature").
						Description("Controls randomness (0.0 - 2.0).").
						Value(&tempStr).
						Validate(ValidateTemperature),
					huh.NewInput().
						Title("Top P").
						Description("Nucleus sampling (0.0 - 1.0).").
						Value(&topPStr).
						Validate(ValidateTopP),
					huh.NewInput().
						Title("Seed").
						Description("Deterministic generation (Integer).").
						Value(&seedStr).
						Validate(ValidateSeed),
				).Title("Advanced Settings"),
			).Run()
			if err != nil {
				return nil
			}

			modelConfig.Endpoint = endpoint
			modelConfig.Key = key
			modelConfig.Model = model
			modelConfig.Provider = provider

			if t, err := strconv.ParseFloat(tempStr, 32); err == nil {
				modelConfig.Temp = float32(t)
			}
			if p, err := strconv.ParseFloat(topPStr, 32); err == nil {
				modelConfig.TopP = float32(p)
			}
			if s, err := strconv.Atoi(seedStr); err == nil {
				i32 := int32(s)
				modelConfig.Seed = &i32
			} else {
				modelConfig.Seed = nil
			}
		} else {
			// Update from flags
			if cmd.Flags().Changed("provider") {
				if v, err := cmd.Flags().GetString("provider"); err == nil {
					modelConfig.Provider = v
				}
			}
			if cmd.Flags().Changed("endpoint") {
				if v, err := cmd.Flags().GetString("endpoint"); err == nil {
					modelConfig.Endpoint = v
				}
			}
			if cmd.Flags().Changed("key") {
				if v, err := cmd.Flags().GetString("key"); err == nil {
					modelConfig.Key = v
				}
			}
			if cmd.Flags().Changed("model") {
				if v, err := cmd.Flags().GetString("model"); err == nil {
					modelConfig.Model = v
				}
			}
			if cmd.Flags().Changed("temp") {
				if v, err := cmd.Flags().GetFloat32("temp"); err == nil {
					modelConfig.Temp = v
				}
			}
			if cmd.Flags().Changed("top_p") {
				if v, err := cmd.Flags().GetFloat32("top_p"); err == nil {
					modelConfig.TopP = v
				}
			}
			if cmd.Flags().Changed("seed") {
				if v, err := cmd.Flags().GetInt("seed"); err == nil {
					if v == 0 {
						modelConfig.Seed = nil
					} else {
						i32 := int32(v)
						modelConfig.Seed = &i32
					}
				}
			}
		}

		// Update the entry via data layer
		if err := store.SetModel(name, modelConfig); err != nil {
			return fmt.Errorf("failed to save model: %w", err)
		}

		fmt.Printf("Model '%s' set successfully.\n", name)
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
		store := data.NewConfigStore()
		modelsMap := store.GetModels()

		// Try finding key
		var modelName string
		if _, exists := modelsMap[name]; exists {
			modelName = name
		} else {
			return fmt.Errorf("model named '%s' not found", name)
		}

		modelConfig := modelsMap[modelName]

		if modelConfig != nil {
			fmt.Printf("Model '%s':\n---\n", name)
			fmt.Printf("Provider: %s\n", modelConfig.Provider)
			fmt.Printf("Endpoint: %s\n", modelConfig.Endpoint)
			fmt.Printf("Model ID: %s\n", modelConfig.Model)
			fmt.Printf("Temp: %v\n", modelConfig.Temp)
			fmt.Printf("TopP: %v\n", modelConfig.TopP)
			if modelConfig.Seed != nil {
				fmt.Printf("Seed: %d\n", *modelConfig.Seed)
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
	// Args: cobra.ExactArgs(1), // Removed for interactive mode
	RunE: func(cmd *cobra.Command, args []string) error {
		store := data.NewConfigStore()
		modelsMap := store.GetModels()

		var name string
		if len(args) > 0 {
			name = args[0]
		} else {
			if len(modelsMap) == 0 {
				fmt.Println("No models to remove.")
				return nil
			}
			var options []huh.Option[string]
			for m := range modelsMap {
				options = append(options, huh.NewOption(m, m))
			}
			sort.Slice(options, func(i, j int) bool { return options[i].Key < options[j].Key })

			err := huh.NewSelect[string]().
				Title("Select Model to Remove").
				Options(options...).
				Value(&name).
				Run()
			if err != nil {
				return nil
			}
		}

		// Find key
		var modelName string
		if _, exists := modelsMap[name]; exists {
			modelName = name
		} else {
			cmd.SilenceUsage = true // Don't show usage for this error
			if force, _ := cmd.Flags().GetBool("force"); force {
				fmt.Printf("Model '%s' does not exist, nothing to remove.\n", name)
				return nil
			}
			return fmt.Errorf("model named '%s' not found", name)
		}

		// Optional: Confirm if not forced
		var confirm bool
		if force, _ := cmd.Flags().GetBool("force"); !force {
			err := huh.NewConfirm().
				Title(fmt.Sprintf("Are you sure you want to remove model '%s'?", name)).
				Value(&confirm).
				Run()
			if err != nil || !confirm {
				fmt.Println("Operation cancelled.")
				return nil
			}
		}

		// Delete the model
		if err := store.DeleteModel(modelName); err != nil {
			return fmt.Errorf("failed to remove model: %w", err)
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
			var confirm bool
			err := huh.NewConfirm().
				Title("Are you sure you want to clear all models? This cannot be undone.").
				Affirmative("Yes").
				Negative("No").
				Value(&confirm).
				Run()

			if err != nil || !confirm {
				fmt.Println("Operation cancelled.")
				return nil
			}
		}

		// Clear all models
		store := data.NewConfigStore()
		modelsMap := store.GetModels()
		for modelName := range modelsMap {
			if err := store.DeleteModel(modelName); err != nil {
				return fmt.Errorf("failed to delete model %s: %w", modelName, err)
			}
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
	// Args: cobra.ExactArgs(1), // Removed to allow interactive mode
	RunE: func(cmd *cobra.Command, args []string) error {
		var name string
		store := data.NewConfigStore()
		modelsMap := store.GetModels()

		if len(args) > 0 {
			name = args[0]
		} else {
			if len(modelsMap) == 0 {
				return fmt.Errorf("no models found")
			}
			// Default to current model
			model := GetEffectModel()
			if model != nil {
				name = model.Name
			}

			// Sort alphabetically but keep current model on top
			var options []huh.Option[string]
			for m := range modelsMap {
				options = append(options, huh.NewOption(m, m))
			}
			sort.Slice(options, func(i, j int) bool {
				if options[i].Key == name {
					return true
				}
				if options[j].Key == name {
					return false
				}
				return options[i].Key < options[j].Key
			})

			err := huh.NewSelect[string]().
				Title("Select Model").
				Options(options...).
				Value(&name).
				Run()
			if err != nil {
				return nil
			}
		}

		// Find key
		var model *data.Model
		if m, exists := modelsMap[name]; exists {
			model = m
		} else {
			return fmt.Errorf("model named '%s' not found. Use 'gllm model list' to see available models", name)
		}

		agent := store.GetActiveAgent()
		if agent == nil {
			return fmt.Errorf("failed to get active agent")
		}
		agent.Model = *model
		if err := store.SetAgent(agent.Name, agent); err != nil {
			return fmt.Errorf("failed to set active agent: %w", err)
		}

		fmt.Printf("Switched to model '%s' successfully.\n", name)
		fmt.Println("This model will be used for all subsequent operations.")
		return nil
	},
}

func GetEffectModel() (model *data.Model) {
	// Get from active agent
	store := data.NewConfigStore()
	agent := store.GetActiveAgent()
	if agent == nil {
		return nil
	}
	return &agent.Model
}

func CheckModelName(name string) error {
	if strings.Contains(name, ".") {
		return fmt.Errorf("model name '%s' contains a dot, which is not allowed", name)
	}
	return nil
}

func ValidateTemperature(s string) error {
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 32)
	if err != nil {
		return err
	}
	if v < 0 || v > 2.0 {
		return fmt.Errorf("temperature must be between 0.0 and 2.0")
	}
	return nil
}

func ValidateTopP(s string) error {
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 32)
	if err != nil {
		return err
	}
	if v <= 0 || v > 1.0 {
		return fmt.Errorf("top_p must be greater than 0.0 and less than or equal to 1.0")
	}
	return nil
}

func ValidateSeed(s string) error {
	if s == "" {
		return nil
	}
	_, err := strconv.Atoi(s)
	return err
}
