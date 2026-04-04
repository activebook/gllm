package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/internal/ui"
	"github.com/activebook/gllm/io"
	"github.com/activebook/gllm/service"
	"github.com/activebook/gllm/util"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize gllm configuration and instruction",
	Long: `Interactive setup wizard to configure gllm with your preferred LLM provider.
It will create or update the 'gllm.yaml' configuration file and GLLM.md instruction file.`,
	// Add completion support
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
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
	toolsList := service.GetEmbeddingTools()
	var options []huh.Option[string]
	for _, tool := range toolsList {
		// All tools selected by default for new agents
		options = append(options, huh.NewOption(tool, tool).Selected(true))
	}
	return options
}

// RunInitWizard runs the interactive setup
// Exported so it can be called from root.go
func RunInitWizard() error {
	var (
		agentName             string
		agentDesc             string
		provider              string
		endpoint              string
		apiKey                string
		model                 string
		confirm               bool
		selectedFeatures      []string
		selectedTools         []string
		selectedThinkingLevel string
	)

	height := io.GetTermFitHeight(100) // algo would use term height/2

	store := data.NewConfigStore()

	// --- Dual-entry mode: if gllm.yaml already exists, offer a branching menu ---
	if store.ConfigFileUsed() != "" {
		fmt.Printf("Note: Updating existing configuration at %s\n\n", store.ConfigFileUsed())

		var wizardMode string
		err := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("What would you like to do?").
					Description("An existing configuration was detected.").
					Options(
						huh.NewOption("Reconfigure Agent", "agent"),
						huh.NewOption("Generate / Update GLLM.md", "instruction"),
						huh.NewOption("Both", "both"),
					).
					Value(&wizardMode),
			).WithHeight(height),
		).Run()
		if err != nil {
			return err
		}

		switch wizardMode {
		case "instruction":
			agent := store.GetActiveAgent()
			if agent == nil {
				return fmt.Errorf("no active agent found; run 'gllm init' to create one first")
			}
			return runInstructionWizard(agent)
		case "both":
			// fall through to run the full agent wizard, then append instruction wizard at the end
		case "agent":
			// fall through to run the full agent wizard only
		}
	}

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

			// Description
			huh.NewText().
				Title("Description (optional)").
				Description("A brief summary of what this agent does").
				Lines(5).
				Value(&agentDesc).
				Placeholder("A helpful, reliable AI assistant."),

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
		).Title("Setup").WithHeight(height),
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

	// Pre-populate with defaults or existing values
	endpoint = defaultEndpoint
	model = defaultModelName
	if existing := store.GetAgent(agentName); existing != nil {
		if existing.Model.Endpoint != "" {
			endpoint = existing.Model.Endpoint
		}
		if existing.Model.Key != "" {
			apiKey = existing.Model.Key
		}
		if existing.Model.Model != "" {
			model = existing.Model.Model
		}
		if len(existing.Tools) > 0 {
			selectedTools = existing.Tools
		}
		if existing.Think != "" {
			selectedThinkingLevel = existing.Think
		}
		if len(existing.Capabilities) > 0 {
			selectedFeatures = existing.Capabilities
		}
	}

	// Features/Capabilities Group
	msfeatures := huh.NewMultiSelect[string]().
		Title("Agent Capabilities").
		Description("Select additional features to enable").
		Options(
			huh.NewOption("Show Token Usage Stats", service.CapabilityTokenUsage).Selected(true),
			huh.NewOption("Show Markdown Output", service.CapabilityMarkdown).Selected(true),
			huh.NewOption("Auto Rename Session", service.CapabilityAutoRename).Selected(true),
			huh.NewOption("Auto Compress Context", service.CapabilityAutoCompression).Selected(true),
			huh.NewOption("Enable MCP Servers", service.CapabilityMCPServers).Selected(false),
			huh.NewOption("Enable Agent Skills", service.CapabilityAgentSkills).Selected(false),
			huh.NewOption("Enable Agent Memory", service.CapabilityAgentMemory).Selected(false),
			huh.NewOption("Enable Sub Agents", service.CapabilitySubAgents).Selected(false),
			huh.NewOption("Enable Web Search", service.CapabilityWebSearch).Selected(false),
		).Value(&selectedFeatures)
	featureNote := ui.GetDynamicHuhNote("Feature Details", msfeatures, service.GetCapabilityDescHighlight)

	// Details Group
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
		).Title("Details").WithHeight(height),
		// Group 3: Tools Selection
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select Embedding Tools").
				Description("Choose which tools to enable for this agent. Press space to toggle, enter to confirm.").
				Options(func() []huh.Option[string] {
					opts := buildToolsOptions()
					ui.SortMultiOptions(opts, []string{}) // No tools selected by default
					return opts
				}()...).
				Value(&selectedTools),
			ui.GetStaticHuhNote("Tools Details", EmbeddingToolsDescription),
		).Title("Tools").WithHeight(height),
		// Group 4: Thinking Level
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Thinking Level").
				Description("Select the thinking level for this agent").
				Options(
					huh.NewOption("Off - Disable thinking", "off").Selected(true),
					huh.NewOption("Minimal - Minimal reasoning", "minimal").Selected(false),
					huh.NewOption("Low - Low reasoning", "low").Selected(false),
					huh.NewOption("Medium - Moderate reasoning", "medium").Selected(false),
					huh.NewOption("High - Maximum reasoning", "high").Selected(false),
				).
				Value(&selectedThinkingLevel),
		).Title("Thinking").WithHeight(height),
		// Group 5: Capabilities
		huh.NewGroup(
			msfeatures, featureNote,
		).Title("Capabilities").WithHeight(height),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Save Configuration?").
				Description("You can modify this agent thereafter").
				Value(&confirm),
		).Title("Confirmation").WithHeight(height),
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
	if err := store.SetModel(newModel.Name, &newModel); err != nil {
		return fmt.Errorf("failed to save model: %w", err)
	}

	// Setup agent config
	var agentConfig *data.AgentConfig
	if existing := store.GetAgent(agentName); existing != nil {
		// Update existing agent
		agentConfig = existing
		agentConfig.Description = agentDesc
		agentConfig.Model = newModel
		agentConfig.Tools = selectedTools
		agentConfig.Think = selectedThinkingLevel
		agentConfig.Capabilities = selectedFeatures
	} else {
		// Create new agent
		agentConfig = &data.AgentConfig{
			Description:  agentDesc,
			Model:        newModel,
			Tools:        selectedTools,
			Think:        selectedThinkingLevel,
			Capabilities: selectedFeatures,
			SystemPrompt: defaultSystemPromptContent,
		}
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

	// Offer GLLM.md generation after a successful agent save.
	// We re-fetch the agent so the caller always gets the freshly persisted config.
	newAgentConfig := store.GetAgent(agentName)
	if newAgentConfig == nil {
		return nil // shouldn't happen; fail gracefully without blocking the happy path
	}

	var wantInstruction bool
	_ = huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Generate Instruction File (GLLM.md)?").
				Description("GLLM.md is used to provide project-specific context and rules.").
				Value(&wantInstruction),
		),
	).Run()

	if wantInstruction {
		if err := runInstructionWizard(newAgentConfig); err != nil {
			fmt.Printf("⚠️  Instruction file generation skipped: %v\n", err)
		}
	}

	return nil
}

// runInstructionWizard handles the GLLM.md sub-wizard:
//  1. Prompt for storage location (local vs global), annotated with CREATE / UPDATE.
//  2. Generate content via service.GenerateInstructionContent (sync LLM call).
//  3. Show generated content for review.
//  4. On confirmation, write the file using os.WriteFile.
//
// agentConfig must be the fully resolved active agent (used for provider dispatch).
func runInstructionWizard(agentConfig *data.AgentConfig) error {
	height := io.GetTermFitHeight(8)

	var storageChoice string
	localLabel := fmt.Sprintf("Local — ./GLLM.md %s", existsLabel(data.LocalInstructionFileExists()))
	globalLabel := fmt.Sprintf("Global — %s %s", data.GetGlobalInstructionFilePath(), existsLabel(data.GlobalInstructionFileExists()))

	err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Where to store GLLM.md?").
				Description("Local applies to this project only. Global applies to all projects.").
				Options(
					huh.NewOption(localLabel, "local"),
					huh.NewOption(globalLabel, "global"),
				).
				Value(&storageChoice),
		).WithHeight(height),
	).Run()
	if err != nil {
		return err
	}

	var targetPath string
	switch storageChoice {
	case "global":
		targetPath = data.GetGlobalInstructionFilePath()
	default:
		targetPath = data.GetLocalInstructionFilePath()
	}

	// Resolve to an absolute path for clear user-facing output.
	if abs, err := filepath.Abs(targetPath); err == nil {
		targetPath = abs
	}

	// fmt.Printf("\n🔍 Scanning project context...\n")

	// Start spinner — ui.GetIndicator() is standalone; no event bus required.
	indicator := ui.GetIndicator()
	indicator.Start(ui.IndicatorGenInstruction)

	content, genErr := service.GenerateInstructionContent(agentConfig)

	indicator.Stop()

	if genErr != nil {
		return fmt.Errorf("generation failed: %w", genErr)
	}

	// Render the generated Markdown so the user can review it before committing to save.
	fmt.Println()
	fmt.Println(strings.Repeat("─", 60))
	glamourStyle := data.MostSimilarGlamourStyle()
	if tr, err := glamour.NewTermRenderer(glamour.WithStandardStyle(glamourStyle)); err == nil {
		if rendered, err := tr.Render(content); err == nil {
			fmt.Print(rendered)
		} else {
			fmt.Println(content) // graceful fallback
		}
	} else {
		fmt.Println(content) // graceful fallback
	}
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println()

	var saveConfirm bool
	err = huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Save to %s?", targetPath)).
				Affirmative("Save").
				Negative("Discard").
				Value(&saveConfirm),
		),
	).Run()
	if err != nil {
		return err
	}

	if !saveConfirm {
		fmt.Println("Discarded — no file written.")
		return nil
	}

	// WriteInstructionFile writes content to the given instruction file path,
	// creating any missing parent directories along the way.
	// This is the single authoritative write path for all GLLM.md I/O.
	if err := util.WriteFileContent(targetPath, content); err != nil {
		return err
	}

	fmt.Printf("✅ GLLM.md written to %s\n", targetPath)
	return nil
}

// existsLabel returns a short annotation for the storage-location picker,
// indicating whether the file will be created fresh or overwritten.
func existsLabel(exists bool) string {
	if exists {
		return "(exists — will UPDATE)"
	}
	return "(will CREATE)"
}
