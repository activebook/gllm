package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/activebook/gllm/service"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// searchCmd represents the search command
var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Configure and manage search engines globally",
	Long: `Configure API keys and settings for various search engines used with gllm.
You can switch on/off whether to use search engines.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(cmd.Long)
		defaultEngine := GetAgentString("search")
		fmt.Println()
		if defaultEngine != "" {
			fmt.Printf("Current search engine set to %s\n", switchOnColor+defaultEngine+resetColor)
		} else {
			fmt.Println("No search engine set.")
		}
		fmt.Println()
		ListSearchTools()
	},
}

// searchSwitchCmd represents the command to switch search engine
var searchSwitchCmd = &cobra.Command{
	Use:     "switch [ENGINE]",
	Aliases: []string{"sw"},
	Short:   "Switch the active search engine",
	Long:    `Switch the search engine used by the current agent. Options: google, bing, tavily, none.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var engine string

		// Check if engine name provided as argument
		if len(args) > 0 {
			provided := strings.ToLower(args[0])
			switch provided {
			case service.GoogleSearchEngine, service.BingSearchEngine, service.TavilySearchEngine, service.NoneSearchEngine:
				engine = provided
			case "":
				engine = service.NoneSearchEngine
			default:
				return fmt.Errorf("invalid search engine '%s'. Valid options: google, bing, tavily, none", args[0])
			}
		} else {
			// Map display names to values
			options := []huh.Option[string]{
				huh.NewOption("Google", service.GoogleSearchEngine),
				huh.NewOption("Bing", service.BingSearchEngine),
				huh.NewOption("Tavily", service.TavilySearchEngine),
				huh.NewOption("None (Disable Search)", service.NoneSearchEngine),
			}

			// Default to current
			current := GetAgentString("search")
			if current == "" {
				engine = service.NoneSearchEngine
			} else {
				engine = current
			}

			// Interactive select
			err := huh.NewSelect[string]().
				Title("Switch Search Engine").
				Description("Select the search engine to use for the current agent").
				Options(options...).
				Value(&engine).
				Run()
			if err != nil {
				return nil
			}
		}

		if err := SetAgentValue("search", engine); err != nil {
			return fmt.Errorf("failed to saving configuration: %w", err)
		}

		if engine == service.NoneSearchEngine {
			fmt.Println("Search engine disabled.")
		} else {
			fmt.Printf("Switched search engine to: %s\n", engine)
		}
		return nil
	},
}

// searchSetCmd represents the command to configure a search engine
var searchSetCmd = &cobra.Command{
	Use:   "set [ENGINE]",
	Short: "Configure a search engine",
	Long:  `Configure API keys and settings for a specific search engine (google, bing, tavily).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var engine string
		if len(args) > 0 {
			engine = args[0]
		} else {
			// Select engine to configure
			err := huh.NewSelect[string]().
				Title("Select Search Engine to Configure").
				Options(
					huh.NewOption("Google", service.GoogleSearchEngine),
					huh.NewOption("Bing", service.BingSearchEngine),
					huh.NewOption("Tavily", service.TavilySearchEngine),
				).
				Value(&engine).
				Run()
			if err != nil {
				return nil
			}
		}

		// Configure based on engine
		switch engine {
		case service.GoogleSearchEngine:
			key := viper.GetString("search_engines.google.key")
			cx := viper.GetString("search_engines.google.cx")

			err := huh.NewForm(
				huh.NewGroup(
					huh.NewNote().
						Title("Google Search Engine Configuration").
						Description("Quota: 100 searches per day (free tier)"),
					huh.NewInput().
						Title("Google Search API Key").
						Description("API Key from Google Cloud Console").
						Value(&key).
						EchoMode(huh.EchoModePassword),
					huh.NewInput().
						Title("Search Engine ID (CX)").
						Description("CX ID from Programmable Search Engine").
						Value(&cx),
				),
			).Run()
			if err != nil {
				return nil
			}

			viper.Set("search_engines.google.key", key)
			viper.Set("search_engines.google.cx", cx)

		case service.BingSearchEngine:
			key := viper.GetString("search_engines.bing.key")

			err := huh.NewForm(
				huh.NewGroup(
					huh.NewNote().
						Title("Bing Search Engine Configuration").
						Description("Quota: 100 searches per month (free tier)"),
					huh.NewInput().
						Title("Bing Search API Key").
						Description("API Key for Bing Search (via SerpAPI)").
						Value(&key).
						EchoMode(huh.EchoModePassword),
				),
			).Run()
			if err != nil {
				return nil
			}

			viper.Set("search_engines.bing.key", key)

		case service.TavilySearchEngine:
			key := viper.GetString("search_engines.tavily.key")

			err := huh.NewForm(
				huh.NewGroup(
					huh.NewNote().
						Title("Tavily Search Engine Configuration").
						Description("Quota: 1000 searches per month (free tier)"),
					huh.NewInput().
						Title("Tavily API Key").
						Description("API Key from Tavily").
						Value(&key).
						EchoMode(huh.EchoModePassword),
				),
			).Run()
			if err != nil {
				return nil
			}

			viper.Set("search_engines.tavily.key", key)

		default:
			return fmt.Errorf("unknown search engine: %s", engine)
		}

		if err := viper.WriteConfig(); err != nil {
			return fmt.Errorf("failed to save configuration: %w", err)
		}

		fmt.Printf("Configuration for '%s' saved successfully.\n", engine)
		return nil
	},
}

// listCmd represents the command to list all search engines
var searchListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all configured search engines",
	Aliases: []string{"ls"},
	Long:    `Display details for all configured search engines.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Configured Search Engines:")
		fmt.Println("-------------------------")

		// Google
		googleKey := viper.GetString("search_engines.google.key")
		googleCx := viper.GetString("search_engines.google.cx")
		if googleKey != "" {
			fmt.Println("Google Custom Search:")
			fmt.Printf("  API Key: %s\n", maskAPIKey(googleKey))
			fmt.Printf("  CX: %s\n", maskAPIKey(googleCx))
			fmt.Println("  Quota: 100 searches per day (free tier)")
		}

		// Tavily
		tavilyKey := viper.GetString("search_engines.tavily.key")
		if tavilyKey != "" {
			fmt.Println("Tavily Search:")
			fmt.Printf("  API Key: %s\n", maskAPIKey(tavilyKey))
			fmt.Println("  Quota: 1000 searches per month (free tier)")
		}

		// Bing
		bingKey := viper.GetString("search_engines.bing.key")
		if bingKey != "" {
			fmt.Println("Bing Search:")
			fmt.Printf("  API Key: %s\n", maskAPIKey(bingKey))
			fmt.Println("  Quota: 100 searches per month (free tier) - SerpAPI")
		}

		if googleKey == "" && tavilyKey == "" && bingKey == "" {
			fmt.Println("No search engines are currently configured.")
			fmt.Println("Use 'gllm search [engine] --key YOUR_KEY' to configure.")
		}

		fmt.Println("-------------------------")

		// Update the list command to show default status
		// In the listCmd.Run function, add:
		defaultEngine := GetAgentString("search")
		fmt.Println()
		if defaultEngine != "" {
			fmt.Printf("Current search engine set to %s\n", switchOnColor+defaultEngine+resetColor)
		} else {
			fmt.Println("No search engine set.")
		}
	},
}

var searchSaveCmd = &cobra.Command{
	Use:    "save [on|off]",
	Hidden: true,
	Short:  "Enable or disable saving search results",
	Long: `Enable or disable saving search results to conversation history.
Keep in mind:
  When set on, the search result is saved into the conversation context before continuing with the LLM step,
  it could consume more tokens and could potentially exceed the maximum context length of the LLM.
  If you want to keep them for future LLM turns or debugging or you know exactly what you want, set it on.
  Otherelse, you should set it to off.
`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			fmt.Println("Usage: gllm search save [on|off]")
			var res string
			rs := viper.GetBool("search_engines.results.save")
			switch rs {
			case true:
				res = switchOnColor + "on" + resetColor
			case false:
				res = switchOffColor + "off" + resetColor
			}
			fmt.Printf("Current search result saving status: %s\n", res)
			return
		}
		mode := strings.ToLower(args[0])
		switch mode {
		case "on":
			viper.Set("search_engines.results.save", true)
			if err := viper.WriteConfig(); err != nil {
				service.Errorf("Error saving configuration: %s\n", err)
				return
			}
			fmt.Println("Saving of search results is: " + switchOnColor + "on" + resetColor)
		case "off":
			viper.Set("search_engines.results.save", false)
			if err := viper.WriteConfig(); err != nil {
				service.Errorf("Error saving configuration: %s\n", err)
				return
			}
			fmt.Println("Saving of search results is: " + switchOffColor + "off" + resetColor)
		default:
			fmt.Println("Usage: gllm search save [on|off]")
		}
	},
}

// maskAPIKey returns a masked version of the API key for display
func maskAPIKey(key string) string {
	return key
	/*
		if len(key) <= 8 {
			return "********"
		}
		visible := 4
		return key[:visible] + strings.Repeat("*", len(key)-visible)
	*/
}

func IsSearchEnabled() bool {
	engine := GetEffectSearchEngineName()
	switch engine {
	case service.GoogleSearchEngine, service.TavilySearchEngine, service.BingSearchEngine:
		return true
	case service.NoneSearchEngine:
		return false
	default:
		return false
	}
}

func GetEffectSearchEngineName() string {
	defaultName := GetAgentString("search")
	return defaultName
}

func SetEffectSearchEngineName(name string) bool {
	var err error
	switch name {
	case service.GoogleSearchEngine:
		err = SetAgentValue("search", service.GoogleSearchEngine)
	case service.TavilySearchEngine:
		err = SetAgentValue("search", service.TavilySearchEngine)
	case service.BingSearchEngine:
		err = SetAgentValue("search", service.BingSearchEngine)
	case service.NoneSearchEngine:
		err = SetAgentValue("search", service.NoneSearchEngine)
	default:
		service.Warnf("Error: '%s' is not a valid search engine. Options: google, tavily, bing, none", name)
		return false
	}
	if err != nil {
		service.Errorf("Error saving configuration: %s\n", err)
		return false
	}
	return true
}

func GetAllSearchEngines() map[string]string {
	searchEngines := viper.GetStringMap("search_engines")

	keys := make([]string, 0, len(searchEngines))
	for k := range searchEngines {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	engines := make(map[string]string)
	for _, k := range keys {
		v := searchEngines[k]
		if configMap, ok := v.(map[string]interface{}); ok {
			// Convert the inner map to a string representation
			var pairs []string
			for k, v := range configMap {
				pairs = append(pairs, fmt.Sprintf("\t%s: %v", k, v))
			}
			engines[k] = strings.Join(pairs, "\n")
		}
	}

	return engines
}

func GetSearchEngineInfo(name string) map[string]any {
	enginesMap := viper.GetStringMap("search_engines")

	if enginesMap == nil {
		return nil
	}

	engineConfig, exists := enginesMap[name]
	if !exists {
		return nil
	}

	// Convert the map[string]interface{} to map[string]any
	if configMap, ok := engineConfig.(map[string]interface{}); ok {
		// Create a new map with string keys and any values
		resultMap := make(map[string]any)
		for k, v := range configMap {
			resultMap[k] = v
		}
		resultMap["name"] = name
		return resultMap
	}

	return nil
}

func GetEffectiveSearchEngine() (name string, info map[string]any) {
	defaultName := GetAgentString("search")
	enginesMap := viper.GetStringMap("search_engines")
	if defaultName != "" {
		if engineConfig, ok := enginesMap[defaultName]; ok {
			// Convert the map[string]interface{} to map[string]string
			if configMap, ok := engineConfig.(map[string]interface{}); ok {
				configMap["name"] = defaultName
				return defaultName, configMap
			}
			service.Warnf("Warning: Default Search Engine '%s' has invalid configuration format", defaultName)
		} else {
			service.Warnf("Warning: Default Search Engine '%s' not found in configuration. Falling back...", defaultName)
		}
	}

	// 3. No search engine available
	logger.Debugln("No search engine to use!")
	return "", nil
}

func init() {
	// Add search command to the root command
	rootCmd.AddCommand(searchCmd)

	// Add subcommands to search command
	searchCmd.AddCommand(searchListCmd)
	searchCmd.AddCommand(searchSaveCmd)
	searchCmd.AddCommand(searchSwitchCmd)
	searchCmd.AddCommand(searchSetCmd)
}

func ListSearchTools() {
	enabled := IsSearchEnabled()
	fmt.Println("Available[✔] search tools:")
	for _, tool := range service.GetAllSearchTools() {
		if enabled {
			fmt.Printf("[✔] %s\n", tool)
		} else {
			fmt.Printf("[ ] %s\n", tool)
		}
	}
}
