// File: cmd/init.go
package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/service"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
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

// buildToolsOptions creates sorted huh.Option list for tools with all selected by default
func buildToolsOptions() []huh.Option[string] {
	toolsList := service.GetAllEmbeddingTools()
	var options []huh.Option[string]
	for _, tool := range toolsList {
		// All tools not selected by default for new agents
		options = append(options, huh.NewOption(tool, tool).Selected(false))
	}
	sort.Slice(options, func(i, j int) bool {
		return options[i].Key < options[j].Key
	})
	return options
}

// RunInitWizard runs the interactive setup
// Exported so it can be called from root.go
func RunInitWizard() error {
	var (
		agentName        string
		provider         string
		endpoint         string
		apiKey           string
		model            string
		confirm          bool
		selectedFeatures []string
		selectedTools    []string
	)

	// Group 1: Provider selection
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Welcome to gllm! Your agent for various LLMs.").
				Description("Let's get you set up with your AI companion."),

			huh.NewInput().
				Title("Agent Name").
				Description("From now on, you can have multiple agents. Give this one a name.").
				Value(&agentName).
				Placeholder("default").
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("agent name is required")
					}
					return nil
				}),

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

	if strings.TrimSpace(agentName) == "" {
		agentName = "default"
	}

	// Determine default model based on provider
	defaultModelName := ""
	defaultEndpoint := ""
	defaultProvider := service.ModelProviderOpenAI
	switch provider {
	case "openai":
		defaultEndpoint = "https://api.openai.com/v1"
		defaultModelName = "gpt-5.2"
		defaultProvider = service.ModelProviderOpenAI
	case "anthropic":
		defaultEndpoint = "https://api.anthropic.com/v1"
		defaultModelName = "claude-4-5-sonnet"
		defaultProvider = service.ModelProviderAnthropic
	case "gemini":
		defaultEndpoint = "https://generativelanguage.googleapis.com"
		defaultModelName = "gemini-flash-latest"
		defaultProvider = service.ModelProviderGemini
	case "xai":
		defaultEndpoint = "https://api.x.ai/v1"
		defaultModelName = "grok-4-1-fast"
		defaultProvider = service.ModelProviderOpenAICompatible
	case "groq":
		defaultEndpoint = "https://api.groq.com/openai/v1"
		defaultModelName = "openai/gpt-oss-120b"
		defaultProvider = service.ModelProviderOpenAICompatible
	case "mistral":
		defaultEndpoint = "https://api.mistral.ai/v1"
		defaultModelName = "mistral-large-latest"
		defaultProvider = service.ModelProviderOpenAICompatible
	case "deepseek":
		defaultEndpoint = "https://api.deepseek.com"
		defaultModelName = "deepseek-chat"
		defaultProvider = service.ModelProviderOpenAICompatible
	case "alibaba":
		defaultEndpoint = "https://dashscope.aliyuncs.com/compatible-mode/v1"
		defaultModelName = "qwen3-max"
		defaultProvider = service.ModelProviderOpenAICompatible
	case "bytedance":
		defaultEndpoint = "https://ark.cn-beijing.volces.com/api/v3"
		defaultModelName = "doubao-seed-1-8-251215"
		defaultProvider = service.ModelProviderOpenAICompatible
	case "zai":
		defaultEndpoint = "https://api.z.ai/api/paas/v4"
		defaultModelName = "glm-4.6"
		defaultProvider = service.ModelProviderOpenAICompatible
	case "moonshot":
		defaultEndpoint = "https://api.moonshot.cn/v1"
		defaultModelName = "kimi-k2-0905-preview"
		defaultProvider = service.ModelProviderOpenAICompatible
	case "minimax":
		defaultEndpoint = "https://api.minimax.io/v1"
		defaultModelName = "minimax-m2"
		defaultProvider = service.ModelProviderOpenAICompatible
	case "meituan":
		defaultEndpoint = "https://api.longcat.chat/openai/v1"
		defaultModelName = "longcat-flash-chat"
		defaultProvider = service.ModelProviderOpenAICompatible
	case "xiaomi":
		defaultEndpoint = "https://api.xiaomimimo.com/v1"
		defaultModelName = "mimo-v2-flash"
		defaultProvider = service.ModelProviderOpenAICompatible
	case "openrouter":
		defaultEndpoint = "https://openrouter.ai/api/v1"
		defaultModelName = "openai/gpt-oss-20b:free"
		defaultProvider = service.ModelProviderOpenAICompatible
	case "other":
		defaultEndpoint = "https://api.tokenfactory.nebius.com/v1"
		defaultModelName = "openai/gpt-oss-120b"
		defaultProvider = service.ModelProviderOpenAICompatible
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
		// Group 3: Tools Selection
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Tools").
				Description("Select which tools to enable for this agent").
				Options(buildToolsOptions()...).
				Value(&selectedTools),
		),
		// Group 4: Capabilities
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Agent Capabilities").
				Description("Select additional features to enable").
				Options(
					huh.NewOption("Thinking Mode", "think").Selected(true),
					huh.NewOption("Token Usage Stats", "usage").Selected(true),
					huh.NewOption("Markdown Output", "markdown").Selected(true),
				).
				Value(&selectedFeatures),
		),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Save Configuration?").
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

	// New model config
	newModel := data.Model{
		Name:     agentName, // Agent name is used as model name
		Provider: defaultProvider,
		Endpoint: endpoint,
		Key:      apiKey,
		Model:    model,
		Temp:     1.0,
		TopP:     1.0,
	}

	// Save Model via data layer
	store := data.NewConfigStore()
	if err := store.SetModel(newModel.Name, &newModel); err != nil {
		return fmt.Errorf("failed to save model: %w", err)
	}

	// Setup agent config
	agentConfig := &data.AgentConfig{
		Model:    newModel,
		Tools:    selectedTools,
		Think:    contains(selectedFeatures, "think"),
		Usage:    contains(selectedFeatures, "usage"),
		Markdown: contains(selectedFeatures, "markdown"),
	}

	// Save Agent via data layer
	if err := store.SetAgent(agentName, agentConfig); err != nil {
		return fmt.Errorf("failed to save agent: %w", err)
	}

	// Set Active Agent
	if err := store.SetActiveAgent(agentName); err != nil {
		return fmt.Errorf("failed to set active agent: %w", err)
	}

	fmt.Printf("\n")
	fmt.Printf("✅ Configuration saved to %s\n", store.ConfigFileUsed())
	fmt.Printf("🎉 You are ready to go! Try running: gllm \"Hello World\"\n")

	return nil
}
