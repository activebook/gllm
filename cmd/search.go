package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/activebook/gllm/service"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// searchCmd represents the search command
var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Configure and manage search engines",
	Long:  `Configure API keys and settings for various search engines used with gllm.`,
}

// searchGoogleCmd represents the google search command
var searchGoogleCmd = &cobra.Command{
	Use:   "google",
	Short: "Configure Google search engine",
	Long: `Configure Google Custom Search JSON API.
Custom Search JSON API provides 100 search queries per day for free.
The cx parameter is the key for the custom search engine.`,
	Run: func(cmd *cobra.Command, args []string) {
		key, _ := cmd.Flags().GetString("key")
		cx, _ := cmd.Flags().GetString("cx")

		if key == "" || cx == "" {
			googleKey := viper.GetString("search_engines.google.key")
			googleCx := viper.GetString("search_engines.google.cx")
			if googleKey == "" || googleCx == "" {
				service.Warnf("Warning: Google Search is not yet configured. Please set API key first.")
			}
			fmt.Println("Google Custom Search:")
			fmt.Printf("  API Key: %s\n", maskAPIKey(googleKey))
			fmt.Printf("  CX: %s\n", maskAPIKey(googleCx))
			fmt.Println("  Quota: 100 searches per day (free tier)")
			fmt.Println("You can use --key and --cx to update the API key.")
			fmt.Println("Both API key and cx values are required.")
			return
		}

		// Save configuration
		viper.Set("search_engines.google.key", key)
		viper.Set("search_engines.google.cx", cx)
		if err := viper.WriteConfig(); err != nil {
			service.Errorf("Error saving configuration: %s\n", err)
			return
		}

		fmt.Println("Google search configuration saved successfully")
	},
}

// searchTavilyCmd represents the tavily search command
var searchTavilyCmd = &cobra.Command{
	Use:   "tavily",
	Short: "Configure Tavily search engine",
	Long:  `Configure Tavily API. Tavily provides 1000 search queries per month for free.`,
	Run: func(cmd *cobra.Command, args []string) {
		key, _ := cmd.Flags().GetString("key")

		if key == "" {
			tavilyKey := viper.GetString("search_engines.tavily.key")
			if tavilyKey == "" {
				service.Warnf("Warning: Tavily Search is not yet configured. Please set API key first.")
			}
			fmt.Println("Tavily Search:")
			fmt.Printf("  API Key: %s\n", maskAPIKey(tavilyKey))
			fmt.Println("  Quota: 1000 searches per month (free tier)")
			fmt.Println("You can use --key to update the API key.")
			return
		}

		// Save configuration
		viper.Set("search_engines.tavily.key", key)
		if err := viper.WriteConfig(); err != nil {
			service.Errorf("Error saving configuration: %s\n", err)
			return
		}

		fmt.Println("Tavily search configuration saved successfully")
	},
}

// searchBingCmd represents the bing search command
var searchBingCmd = &cobra.Command{
	Use:   "bing",
	Short: "Configure Bing search engine",
	Long:  `Configure Bing API. Bing isn't supported by gllm at the moment.`,
	Run: func(cmd *cobra.Command, args []string) {
		key, _ := cmd.Flags().GetString("key")

		if key == "" {
			bingKey := viper.GetString("search_engines.bing.key")
			if bingKey == "" {
				service.Warnf("Warning: Bing Search is not yet configured. Please set API key first.")
			}
			fmt.Println("Bing Search:")
			fmt.Printf("  API Key: %s\n", maskAPIKey(bingKey))
			fmt.Println("  Quota: 100 searches per month (free tier) - SerpAPI")
			fmt.Println("You can use --key to update the API key.")
			return
		}

		// Save configuration
		viper.Set("search_engines.bing.key", key)
		if err := viper.WriteConfig(); err != nil {
			service.Errorf("Error saving configuration: %s\n", err)
			return
		}

		fmt.Println("Bing search configuration saved successfully")
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

		// Update the list command to show default status
		// In the listCmd.Run function, add:
		defaultEngine := viper.GetString("default.search")
		fmt.Println()
		if defaultEngine != "" {
			fmt.Printf("Default search engine:%s\n", defaultEngine)
		} else {
			fmt.Println("No default search engine set.")
		}
		fmt.Println("-------------------------")
	},
}

// searchDefaultCmd represents the command to set default search engine
var searchDefaultCmd = &cobra.Command{
	Use:     "default",
	Aliases: []string{"def"},
	Short:   "Set the default search engine",
	Long:    `Set which search engine to use by default when performing searches(RAG).`,
	Run: func(cmd *cobra.Command, args []string) {
		// Display current default if no arguments provided
		if len(args) == 0 {
			current := viper.GetString("default.search")
			if current == "" {
				fmt.Println("No default search engine set. Available options: google, tavily, bing, none")
			} else {
				fmt.Printf("Current default search engine: %s\n", current)
			}
			return
		}

		// Set new default
		engine := strings.ToLower(args[0])
		if engine != "google" && engine != "tavily" && engine != "bing" && engine != "none" {
			service.Errorf("Error: '%s' is not a valid search engine. Options: google, tavily, bing, none\n", engine)
			return
		}

		// Check if the selected engine is configured
		key := viper.GetString(fmt.Sprintf("search_engines.%s.key", engine))
		if key == "" {
			service.Warnf("Warning: %s is not yet configured. Please set API key first.", engine)
			return
		}

		viper.Set("default.search", engine)
		if err := viper.WriteConfig(); err != nil {
			service.Errorf("Error saving configuration: %s\n", err)
			return
		}

		fmt.Printf("Default search engine set to: %s\n", engine)
	},
}

var searchSaveCmd = &cobra.Command{
	Use:   "save [on|off]",
	Short: "Enable or disable saving search results",
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

func GetEffectSearchEnginelName() string {
	defaultName := viper.GetString("default.search")
	return defaultName
}

func SetEffectSearchEnginelName(name string) bool {
	switch name {
	case service.GoogleSearchEngine:
		viper.Set("default.search", "google")
	case service.TavilySearchEngine:
		viper.Set("default.search", "tavily")
	case service.BingSearchEngine:
		viper.Set("default.search", "bing")
	case service.NoneSearchEngine:
		viper.Set("default.search", "none")
	default:
		service.Warnf("Error: '%s' is not a valid search engine. Options: google, tavily, bing, none", name)
		return false
	}
	if err := viper.WriteConfig(); err != nil {
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

func GetEffectiveSearchEngine() (name string, info map[string]any) {
	defaultName := viper.GetString("default.search")
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
	searchCmd.AddCommand(searchGoogleCmd)
	searchCmd.AddCommand(searchTavilyCmd)
	searchCmd.AddCommand(searchBingCmd)
	searchCmd.AddCommand(searchListCmd)
	searchCmd.AddCommand(searchDefaultCmd)
	searchCmd.AddCommand(searchSaveCmd)

	// Google flags
	searchGoogleCmd.Flags().StringP("key", "k", "", "Google Custom Search API key")
	searchGoogleCmd.Flags().StringP("cx", "c", "", "Google Custom Search Engine ID")

	// Tavily flags
	searchTavilyCmd.Flags().StringP("key", "k", "", "Tavily API key")

	// Bing flags
	searchBingCmd.Flags().StringP("key", "k", "", "Bing API key")
}
