// File: cmd/search.go
package cmd

import (
	"fmt"
	"strings"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/service"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

// searchCmd represents the search command
var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Configure and manage search engines globally",
	Long: `Configure API keys and settings for various search engines used with gllm.
You can switch to use which search engine.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(cmd.Long)
		store := data.NewConfigStore()
		agent := store.GetActiveAgent()
		if agent == nil {
			fmt.Println("No active agent found")
			return
		}
		defaultEngine := agent.Search.Name
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
		store := data.NewConfigStore()
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
			activeAgent := store.GetActiveAgent()
			if activeAgent == nil {
				fmt.Println("No active agent found")
				return nil
			}
			current := activeAgent.Search.Name
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

		activeAgent := store.GetActiveAgent()
		if activeAgent == nil {
			fmt.Println("No active agent found")
			return nil
		}
		activeAgent.Search.Name = engine
		if err := store.SetAgent(activeAgent.Name, activeAgent); err != nil {
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

		// Get all search engines
		store := data.NewConfigStore()
		engines := store.GetSearchEngines()
		engineConfig := engines[engine]
		if engineConfig == nil {
			engineConfig = &data.SearchEngine{}
		}

		// Configure based on engine
		switch engine {
		case service.GoogleSearchEngine:
			key := engineConfig.Config["key"]
			cx := engineConfig.Config["cx"]
			dd := engineConfig.DeepDive
			mr := engineConfig.Reference

			if dd == 0 {
				dd = 3 // default
			}
			if mr == 0 {
				mr = 5 // default
			}
			ddStr := fmt.Sprintf("%d", dd)
			mrStr := fmt.Sprintf("%d", mr)

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
					huh.NewInput().
						Title("Deep Dive limit").
						Description("Number of links to fetch content from (default: 3)").
						Value(&ddStr).
						Validate(validateInt),
					huh.NewInput().
						Title("Max References").
						Description("Number of references to display (default: 5)").
						Value(&mrStr).
						Validate(validateInt),
				),
			).Run()
			if err != nil {
				return nil
			}

			engineConfig.Config["key"] = key
			engineConfig.Config["cx"] = cx
			engineConfig.DeepDive = toInt(ddStr)
			engineConfig.Reference = toInt(mrStr)

		case service.BingSearchEngine:
			key := engineConfig.Config["key"]
			dd := engineConfig.DeepDive
			mr := engineConfig.Reference

			if dd == 0 {
				dd = 3
			}
			if mr == 0 {
				mr = 5
			}
			ddStr := fmt.Sprintf("%d", dd)
			mrStr := fmt.Sprintf("%d", mr)

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
					huh.NewInput().
						Title("Deep Dive limit").
						Description("Number of links to fetch content from (default: 3)").
						Value(&ddStr).
						Validate(validateInt),
					huh.NewInput().
						Title("Max References").
						Description("Number of references to display (default: 5)").
						Value(&mrStr).
						Validate(validateInt),
				),
			).Run()
			if err != nil {
				return nil
			}

			engineConfig.Config["key"] = key
			engineConfig.DeepDive = toInt(ddStr)
			engineConfig.Reference = toInt(mrStr)

		case service.TavilySearchEngine:
			key := engineConfig.Config["key"]
			dd := engineConfig.DeepDive
			mr := engineConfig.Reference

			if dd == 0 {
				dd = 3
			}
			if mr == 0 {
				mr = 5
			}
			ddStr := fmt.Sprintf("%d", dd)
			mrStr := fmt.Sprintf("%d", mr)

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
					huh.NewInput().
						Title("Deep Dive limit").
						Description("Number of links to fetch content from (default: 3)").
						Value(&ddStr).
						Validate(validateInt),
					huh.NewInput().
						Title("Max References").
						Description("Number of references to display (default: 5)").
						Value(&mrStr).
						Validate(validateInt),
				),
			).Run()
			if err != nil {
				return nil
			}

			engineConfig.Config["key"] = key
			engineConfig.DeepDive = toInt(ddStr)
			engineConfig.Reference = toInt(mrStr)

		default:
			return fmt.Errorf("unknown search engine: %s", engine)
		}

		engines[engine] = engineConfig
		if err := store.SetSearchEngine(engine, engineConfig); err != nil {
			return fmt.Errorf("failed to save %s config: %w", engine, err)
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
		fmt.Println()

		store := data.NewConfigStore()
		engines := store.GetSearchEngines()

		// Google
		googleConfig := engines[service.GoogleSearchEngine]
		if googleConfig != nil {
			fmt.Println("Google Search:")
			fmt.Printf("  API Key: %s\n", maskAPIKey(googleConfig.Config["key"]))
			fmt.Printf("  CX: %s\n", maskAPIKey(googleConfig.Config["cx"]))
			fmt.Println("  DeepDive limit: ", googleConfig.DeepDive)
			fmt.Println("  Max References: ", googleConfig.Reference)
			fmt.Println("  Quota: 100 searches per day (free tier)")
		}

		// Tavily
		tavilyConfig := engines[service.TavilySearchEngine]
		if tavilyConfig != nil {
			fmt.Println("Tavily Search:")
			fmt.Printf("  API Key: %s\n", maskAPIKey(tavilyConfig.Config["key"]))
			fmt.Println("  DeepDive limit: ", tavilyConfig.DeepDive)
			fmt.Println("  Max References: ", tavilyConfig.Reference)
			fmt.Println("  Quota: 1000 searches per month (free tier)")
		}

		// Bing
		bingConfig := engines[service.BingSearchEngine]
		if bingConfig != nil {
			fmt.Println("Bing Search:")
			fmt.Printf("  API Key: %s\n", maskAPIKey(bingConfig.Config["key"]))
			fmt.Println("  DeepDive limit: ", bingConfig.DeepDive)
			fmt.Println("  Max References: ", bingConfig.Reference)
			fmt.Println("  Quota: 100 searches per month (free tier) - SerpAPI")
		}

		if (googleConfig == nil || googleConfig.Config["key"] == "") &&
			(tavilyConfig == nil || tavilyConfig.Config["key"] == "") &&
			(bingConfig == nil || bingConfig.Config["key"] == "") {
			fmt.Println("No search engines are currently configured.")
			fmt.Println("Use 'gllm search [engine] --key YOUR_KEY' to configure.")
		}

		fmt.Println()

		// Update the list command to show default status
		defaultEngine := GetEffectSearchEngineName()
		if defaultEngine != "" {
			fmt.Printf("Current search engine set to %s\n", switchOnColor+defaultEngine+resetColor)
		} else {
			fmt.Println("No search engine set.")
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
	store := data.NewConfigStore()
	activeAgent := store.GetActiveAgent()
	if activeAgent == nil {
		return ""
	}
	defaultName := activeAgent.Search.Name
	return defaultName
}

func init() {
	// Add search command to the root command
	rootCmd.AddCommand(searchCmd)

	// Add subcommands to search command
	searchCmd.AddCommand(searchListCmd)
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
