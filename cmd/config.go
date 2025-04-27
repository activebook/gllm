// File: cmd/config.go
package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/activebook/gllm/service"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// configCmd represents the base command when called without any subcommands
var configCmd = &cobra.Command{
	Use:     "config",
	Aliases: []string{"cfg"}, // Optional alias
	Short:   "Manage gllm configuration",
	Long: `View and manage settings for gllm.

Use subcommands to target specific configuration areas like models or prompts,
or use 'config path' to see where the configuration file is located.`,
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

// configPathCmd represents the config path command
var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show the location of the configuration file",
	Long:  `Displays the full path to the configuration file gllm attempts to load.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Check if a config file was explicitly loaded by Viper
		usedCfgFile := viper.ConfigFileUsed()
		if usedCfgFile != "" {
			fmt.Printf("Configuration file in use: %s\n", usedCfgFile)
			// You could add a check here to see if it differs from the default path
			defaultPath := getDefaultConfigFilePath()
			if usedCfgFile != defaultPath {
				fmt.Printf("Note: This differs from the default path: %s\n", defaultPath)
			}
		} else {
			// If no config file was loaded, show the default path where gllm looks
			fmt.Printf("No configuration file loaded.\nDefault location is: %s\n", getDefaultConfigFilePath())
		}
	},
}

// configModelCmd represents the config model command (stub for now)
var configPrintCmd = &cobra.Command{
	Use:     "print",
	Aliases: []string{"pr", "all", "list", "ls"}, // Optional alias
	Short:   "Print all configurations",
	Long: `Print all configuration including all LLM models, system prompts, and templates.
and all default settings (e.g., default model, default system prompt, default template).`,
	Run: func(cmd *cobra.Command, args []string) {
		// Create a tabwriter for formatted tabular output
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

		// Helper function to print section headers
		printSection := func(title string) {
			fmt.Println()

			// Calculate the full title with decorations
			fullTitle := fmt.Sprintf("=== %s ===", strings.ToUpper(title))

			// Calculate padding needed to center
			lineWidth := 50
			padding := (lineWidth - len(fullTitle)) / 2

			// If padding is negative (title too long), ensure it's at least 0
			if padding < 0 {
				padding = 0
			}

			// Print the centered title
			fmt.Printf("%s%s\n", strings.Repeat(" ", padding), fullTitle)
			fmt.Println(strings.Repeat("-", lineWidth))
		}

		printSection("CONFIGURATION SUMMARY")

		// Models section
		printSection("Models")
		models, err := GetAllModels()
		if err != nil {
			service.Errorf("Error retrieving models: %s\n", err)
		} else {
			// Print headers for the table
			fmt.Fprintln(w, " MODEL \t SETTINGS ")
			fmt.Fprintln(w, "-------\t----------")

			// Print model data
			defaultName := GetEffectModelName()
			for name, settings := range models {
				if name == defaultName {
					fmt.Fprintf(w, "*%s*%v\n", name, settings)
				} else {
					fmt.Fprintf(w, "%s%v\n", name, settings)
				}
			}
			w.Flush()
		}

		// System Prompts section
		printSection("System Prompts")
		sysPrompts := GetAllSystemPrompts()

		// Print headers for the table
		fmt.Fprintln(w, " NAME \t CONTENT ")
		fmt.Fprintln(w, "------\t---------")

		// Handle system prompts based on their structure
		fmt.Fprintf(w, "%v\n", sysPrompts)
		w.Flush()

		// Templates section
		printSection("Templates")
		templates := GetAllTemplates()

		// Print headers for the table
		fmt.Fprintln(w, " NAME \t CONTENT ")
		fmt.Fprintln(w, "------\t---------")

		// Handle templates based on their structure
		fmt.Fprintf(w, "%v\n", templates)
		w.Flush()

		// Search Engines section
		printSection("Search Engines")
		searchEngines := GetAllSearchEngines()
		fmt.Fprintln(w, " Search \t SETTINGS ")
		fmt.Fprintln(w, "-------\t----------")
		// Print model data
		defaultName := GetEffectSearchEnginelName()
		for name, settings := range searchEngines {
			if name == defaultName {
				fmt.Fprintf(w, "*%s*%v\n", name, settings)
			} else {
				fmt.Fprintf(w, "%s%v\n", name, settings)
			}
		}
		w.Flush()

		// Plugins section
		printSection("Plugins")
		plugins := GetLoadedPlugins()
		fmt.Fprintln(w, " Plugin \t Loaded ")
		fmt.Fprintln(w, "-------\t----------")
		for name, loaded := range plugins {
			fmt.Fprintf(w, "%s\t%v\n", name, loaded)
		}
		w.Flush()

		// Default Configuration section
		printSection("Default Configuration")

		// Default System Prompt
		mark := GetMarkdownSwitch()
		fmt.Printf("\nMarkdown Format: %v\n", mark)

		// Default Model
		modelName, modelInfo := GetEffectiveModel()
		fmt.Printf("\nDefault Model: %v\n", modelName)

		// Format the model data as a table
		fmt.Fprintln(w, " PROPERTY \t VALUE ")
		fmt.Fprintln(w, "----------\t-------")
		for property, value := range modelInfo {
			fmt.Fprintf(w, "%s\t%v\n", property, value)
		}
		w.Flush()

		// Default Search Engine
		fmt.Println("\nDefault Search Engine:")
		searchEngine := GetEffectiveSearchEngine()
		fmt.Fprintln(w, " PROPERTY \t VALUE ")
		fmt.Fprintln(w, "----------\t-------")
		pairs := []string{}
		for property, value := range searchEngine {
			pairs = append(pairs, fmt.Sprintf("%s\t%v", property, value))
		}
		sort.Sort(sort.Reverse(sort.StringSlice(pairs)))
		for _, pair := range pairs {
			fmt.Fprintln(w, pair)
		}
		w.Flush()

		fmt.Println(strings.Repeat("=", 50))
	},
}

func init() {
	// Add configCmd to the root command
	rootCmd.AddCommand(configCmd)

	// Add subcommands to configCmd
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configPrintCmd)

	// Add flags for other prompt commands if needed in the future
}
