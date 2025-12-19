package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize gllm configuration",
	Long: `Interactive setup wizard to configure gllm with your preferred LLM provider.
It will create or update the 'gllm.yaml' configuration file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := RunInitWizard(); err != nil {
			return fmt.Errorf("Initialization failed: %w\n", err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}

// RunInitWizard runs the interactive setup
// Exported so it can be called from root.go
func RunInitWizard() error {
	var (
		provider string
		endpoint string
		apiKey   string
		model    string
		confirm  bool
	)

	// Group 1: Provider selection
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Welcome to gllm! Your agent for various LLMs.").
				Description("Let's get you set up with your AI companion."),

			huh.NewSelect[string]().
				Title("Choose your AI Provider").
				Description("Select the provider you want to use for your LLM.").
				Options(
					huh.NewOption("OpenAI", "openai"),
					huh.NewOption("Anthropic", "anthropic"),
					huh.NewOption("Google Gemini", "gemini"),
					huh.NewOption("Grok", "xai"),
					huh.NewOption("Groq", "groq"),
					huh.NewOption("Mistral", "mistral"),
					huh.NewOption("DeepSeek", "deepseek"),
					huh.NewOption("Qwen", "alibaba"),
					huh.NewOption("Doubao", "bytedance"),
					huh.NewOption("Glm", "zai"),
					huh.NewOption("Kimi", "moonshot"),
					huh.NewOption("MiniMax", "minimax"),
					huh.NewOption("Longcat", "meituan"),
					huh.NewOption("Mimo", "xiaomi"),
					huh.NewOption("Openrouter", "openrouter"),
					huh.NewOption("Other (OpenAI Compatible)", "other"),
				).
				Value(&provider),
		),
	).Run()

	if err != nil {
		return err
	}

	// Determine default model based on provider
	defaultModelName := ""
	defaultEndpoint := ""
	switch provider {
	case "openai":
		defaultEndpoint = "https://api.openai.com/v1"
		defaultModelName = "gpt-5.2"
	case "anthropic":
		defaultEndpoint = "https://api.anthropic.com/v1"
		defaultModelName = "claude-4-5-sonnet"
	case "gemini":
		defaultEndpoint = "https://generativelanguage.googleapis.com"
		defaultModelName = "gemini-flash-latest"
	case "xai":
		defaultEndpoint = "https://api.x.ai/v1"
		defaultModelName = "grok-4-1-fast"
	case "groq":
		defaultEndpoint = "https://api.groq.com/openai/v1"
		defaultModelName = "openai/gpt-oss-120b"
	case "mistral":
		defaultEndpoint = "https://api.mistral.ai/v1"
		defaultModelName = "mistral-large-latest"
	case "deepseek":
		defaultEndpoint = "https://api.deepseek.com"
		defaultModelName = "deepseek-chat"
	case "alibaba":
		defaultEndpoint = "https://dashscope.aliyuncs.com/compatible-mode/v1"
		defaultModelName = "qwen3-max"
	case "bytedance":
		defaultEndpoint = "https://ark.cn-beijing.volces.com/api/v3"
		defaultModelName = "doubao-seed-1-8-251215"
	case "zai":
		defaultEndpoint = "https://api.z.ai/api/paas/v4"
		defaultModelName = "glm-4.6"
	case "moonshot":
		defaultEndpoint = "https://api.moonshot.cn/v1"
		defaultModelName = "kimi-k2-0905-preview"
	case "minimax":
		defaultEndpoint = "https://api.minimax.io/v1"
		defaultModelName = "minimax-m2"
	case "meituan":
		defaultEndpoint = "https://api.longcat.chat/openai/v1"
		defaultModelName = "longcat-flash-chat"
	case "xiaomi":
		defaultEndpoint = "https://api.xiaomimimo.com/v1"
		defaultModelName = "mimo-v2-flash"
	case "openrouter":
		defaultEndpoint = "https://openrouter.ai/api/v1"
		defaultModelName = "openai/gpt-oss-20b:free"
	case "other":
		defaultEndpoint = "https://api.tokenfactory.nebius.com/v1"
		defaultModelName = "openai/gpt-oss-120b"
	}

	// Group 2: Details
	// We use a dynamic form to set the default model
	err = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Endpoint").
				Description("The endpoint to use").
				Value(&endpoint).
				Placeholder(defaultEndpoint). // Visual hint
				Suggestions([]string{defaultEndpoint}).
				Validate(func(str string) error {
					if len(str) == 0 {
						return fmt.Errorf("endpoint is required")
					} else if !strings.HasPrefix(str, "http") {
						return fmt.Errorf("endpoint must be an url")
					}
					return nil
				}),

			huh.NewInput().
				Title("API Key").
				Description("Enter your API key (will be stored locally)").
				EchoMode(huh.EchoModePassword).
				Value(&apiKey).
				Validate(func(str string) error {
					if len(str) < 3 {
						return fmt.Errorf("api key is too short")
					}
					return nil
				}),

			huh.NewInput().
				Title("Default Model Name").
				Description("The model identifier to use").
				Value(&model).
				Placeholder(defaultModelName). // Visual hint
				Suggestions([]string{defaultModelName}).
				Validate(func(str string) error {
					if len(str) == 0 && defaultModelName == "" {
						return fmt.Errorf("model name is required")
					}
					return nil
				}),
		),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Save Configuration?").
				Description(fmt.Sprintf("This will write to %s", getDefaultConfigFilePath())).
				Value(&confirm),
		),
	).Run()

	if err != nil {
		return err
	}

	if !confirm {
		fmt.Printf("Configuration aborted.\n")
		return nil
	}

	// Apply default if user left model empty but we have a suggestion
	if model == "" && defaultModelName != "" {
		model = defaultModelName
	}

	// Create the configuration structure
	// We create a model named "default" (or use the provider name?)
	// Let's use the provider name as the model alias for simplicity, e.g. "openai"
	// But "default" is clearer for the *first* setup.
	modelAlias := provider

	// Existing models map
	modelsMap := viper.GetStringMap("models")
	if modelsMap == nil {
		modelsMap = make(map[string]interface{})
	}

	// New model config
	newModel := map[string]interface{}{
		"endpoint": endpoint,
		"key":      apiKey,
		"model":    model,
	}

	// Encode the alias
	encodedAlias := encodeModelName(modelAlias)
	modelsMap[encodedAlias] = newModel

	viper.Set("models", modelsMap)
	viper.Set("agent.model", encodedAlias)

	// Save
	if err := writeConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("\n")
	// Use service.Green for success message if available, or just fmt
	fmt.Printf("✅ Configuration saved to %s\n", viper.ConfigFileUsed())
	fmt.Printf("🎉 You are ready to go! Try running: gllm \"Hello World\"\n")

	return nil
}
