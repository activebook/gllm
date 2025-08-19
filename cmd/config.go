// File: cmd/config.go
package cmd

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/activebook/gllm/service"
	"github.com/fatih/color"
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
	// 	fmt.Println("Use 'gllm config [subcommand] --help' for more information.")
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

// configExportCmd represents the config export command
var configExportCmd = &cobra.Command{
	Use:   "export [file]",
	Short: "Export configuration to a file",
	Long: `Export current configuration to a file.

If no file is specified, the configuration will be exported to 'gllm-config.yaml' 
in the current directory.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var exportFile string
		
		if len(args) == 0 {
			exportFile = "gllm-config.yaml"
		} else {
			exportFile = args[0]
		}
		
		// Get all configuration settings
		configMap := viper.AllSettings()
		
		// Create a new viper instance for export
		exportViper := viper.New()
		for key, value := range configMap {
			exportViper.Set(key, value)
		}
		
		// Set the export file
		exportViper.SetConfigFile(exportFile)
		
		// Write the configuration to the file
		if err := exportViper.WriteConfigAs(exportFile); err != nil {
			service.Errorf("Error exporting configuration: %s\n", err)
			return
		}
		
		fmt.Printf("Configuration exported successfully to: %s\n", exportFile)
	},
}

// configImportCmd represents the config import command
var configImportCmd = &cobra.Command{
	Use:   "import [file]",
	Short: "Import configuration from a file",
	Long: `Import configuration from a file.

This will merge the imported configuration with the current configuration,
with the imported values taking precedence.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		importFile := args[0]
		
		// Check if file exists
		if _, err := os.Stat(importFile); os.IsNotExist(err) {
			service.Errorf("Configuration file does not exist: %s\n", importFile)
			return
		}
		
		// Create a new viper instance for import
		importViper := viper.New()
		importViper.SetConfigFile(importFile)
		
		// Read the configuration file
		if err := importViper.ReadInConfig(); err != nil {
			service.Errorf("Error reading configuration file: %s\n", err)
			return
		}
		
		// Get all settings from the import file
		importedSettings := importViper.AllSettings()
		
		// Merge imported settings with current configuration
		for key, value := range importedSettings {
			viper.Set(key, value)
		}
		
		// Save the merged configuration
		if err := writeConfig(); err != nil {
			service.Errorf("Error saving configuration: %s\n", err)
			return
		}
		
		fmt.Printf("Configuration imported successfully from: %s\n", importFile)
	},
}

// configModelCmd represents the config model command (stub for now)
var configPrintCmd = &cobra.Command{
	Use:     "print",
	Aliases: []string{"pr", "all", "list", "ls"},
	Short:   "Print all configurations",
	Long:    `Print all configuration including all LLM models, system prompts, and templates. and all default settings (e.g., default model, default system prompt, default template).`,
	Run: func(cmd *cobra.Command, args []string) {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

		sectionColor := color.New(color.FgCyan, color.Bold).SprintFunc()
		headerColor := color.New(color.FgYellow, color.Bold).SprintFunc()
		highlightColor := color.New(color.FgGreen, color.Bold).SprintFunc()
		keyColor := color.New(color.FgMagenta, color.Bold).SprintFunc()

		printSection := func(title string) {
			fmt.Println()
			fullTitle := fmt.Sprintf("=== %s ===", strings.ToUpper(title))
			lineWidth := 50
			padding := (lineWidth - len(fullTitle)) / 2
			if padding < 0 {
				padding = 0
			}
			fmt.Printf("%s%s\n", strings.Repeat(" ", padding), sectionColor(fullTitle))
			fmt.Println(color.New(color.FgCyan).Sprint(strings.Repeat("-", lineWidth)))
		}

		printSection("CONFIGURATION SUMMARY")

		// Models section
		printSection("Models")
		models, err := GetAllModels()
		if err != nil {
			service.Errorf("Error retrieving models: %s\n", err)
		} else {
			fmt.Fprintln(w, headerColor(" MODEL ")+"\t"+headerColor(" SETTINGS "))
			fmt.Fprintln(w, headerColor("-------")+"\t"+headerColor("----------"))
			defaultName := GetEffectModelName()
			for name, settings := range models {
				if name == defaultName {
					fmt.Fprintf(w, "%s\t%v\n", highlightColor("*"+name+"*"), settings)
				} else {
					fmt.Fprintf(w, "%s\t%v\n", keyColor(name), settings)
				}
			}
			w.Flush()
		}

		// System Prompts section
		printSection("System Prompts")
		sysPrompts := GetAllSystemPrompts()
		fmt.Fprintln(w, headerColor(" NAME ")+"\t"+headerColor(" CONTENT "))
		fmt.Fprintln(w, headerColor("------")+"\t"+headerColor("---------"))
		fmt.Fprintf(w, "%v\n", sysPrompts)
		w.Flush()

		// Templates section
		printSection("Templates")
		templates := GetAllTemplates()
		fmt.Fprintln(w, headerColor(" NAME ")+"\t"+headerColor(" CONTENT "))
		fmt.Fprintln(w, headerColor("------")+"\t"+headerColor("---------"))
		fmt.Fprintf(w, "%v\n", templates)
		w.Flush()

		// Search Engines section
		printSection("Search Engines")
		searchEngines := GetAllSearchEngines()
		fmt.Fprintln(w, headerColor(" Search ")+"\t"+headerColor(" SETTINGS "))
		fmt.Fprintln(w, headerColor("-------")+"\t"+headerColor("----------"))
		defaultSearch := GetEffectSearchEnginelName()
		for name, settings := range searchEngines {
			coloredName := name
			if name == defaultSearch {
				coloredName = highlightColor("*" + name + "*")
			} else {
				coloredName = keyColor(name)
			}
			fmt.Fprintf(w, "%s\t%s\n", coloredName, (fmt.Sprintf("%v", settings)))
		}
		w.Flush()

		// Plugins section
		printSection("Tools")
		fmt.Fprintln(w, headerColor(" Tool ")+"\t"+headerColor(" Enabled "))
		fmt.Fprintln(w, headerColor("------")+"\t"+headerColor("----------"))

		toolsEnabled := AreToolsEnabled()
		toolsStatus := highlightColor("Yes")
		if !toolsEnabled {
			toolsStatus = color.New(color.FgRed, color.Bold).Sprint("No")
		}
		for _, tool := range service.GetAllEmbeddingTools() {
			fmt.Fprintf(w, "%s\t%s\n", keyColor(tool), toolsStatus)
		}
		w.Flush()

		// Default Configuration section
		printSection("Default Configuration")

		mark := GetMarkdownSwitch()
		fmt.Printf("\n%s: %v\n", keyColor("Markdown Format"), mark)

		// Display max recursions value
		maxRecursions := viper.GetInt("max_recursions")
		if maxRecursions <= 0 {
			maxRecursions = 5 // Default value
		}
		fmt.Printf("%s: %d\n", keyColor("Max Recursions"), maxRecursions)

		modelName, modelInfo := GetEffectiveModel()
		fmt.Printf("\n%s: %v\n", keyColor("Default Model"), highlightColor(modelName))
		fmt.Fprintln(w, headerColor(" PROPERTY ")+"\t"+headerColor(" VALUE "))
		fmt.Fprintln(w, headerColor("----------")+"\t"+headerColor("-------"))
		for property, value := range modelInfo {
			fmt.Fprintf(w, "%s\t%s\n", keyColor(property), (fmt.Sprintf("%v", value)))
		}
		w.Flush()

		searchName, searchEngine := GetEffectiveSearchEngine()
		fmt.Printf("\n%s: %v\n", keyColor("Default Search Engine"), highlightColor(searchName))
		fmt.Fprintln(w, headerColor(" PROPERTY ")+"\t"+headerColor(" VALUE "))
		fmt.Fprintln(w, headerColor("----------")+"\t"+headerColor("-------"))
		pairs := []struct{ k, v string }{}
		for property, value := range searchEngine {
			pairs = append(pairs, struct{ k, v string }{
				keyColor(property),
				(fmt.Sprintf("%v", value)),
			})
		}
		sort.Slice(pairs, func(i, j int) bool { return pairs[i].k > pairs[j].k })
		for _, pair := range pairs {
			fmt.Fprintf(w, "%s\t%s\n", pair.k, pair.v)
		}
		w.Flush()

		fmt.Println(color.New(color.FgCyan, color.Bold).Sprint(strings.Repeat("=", 50)))
	},
}

func init() {
	// Add configCmd to the root command
	rootCmd.AddCommand(configCmd)

	// Add subcommands to configCmd
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configPrintCmd)
	configCmd.AddCommand(configSetCmd) // Register the config set command
	configCmd.AddCommand(configMaxRecursionsCmd)
	configCmd.AddCommand(configExportCmd) // Register the config export command
	configCmd.AddCommand(configImportCmd) // Register the config import command

	// Add flags for other prompt commands if needed in the future
}

// configMaxRecursionsCmd represents the config max-recursions command
var configMaxRecursionsCmd = &cobra.Command{
	Use:     "max-recursions [value]",
	Aliases: []string{"mr"},
	Short:   "Get or set the maximum number of Model calling recursions allowed",
	Long: `Get or set the maximum number of Model calling recursions allowed in the application.

If no value is provided, the current setting is displayed.
If a value is provided, it sets the new maximum recursions value.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			// No argument provided - show current value
			maxRecursions := viper.GetInt("max_recursions")
			if maxRecursions <= 0 {
				maxRecursions = 5 // Default value
			}
			// Set the new value in viper
			viper.Set("max_recursions", maxRecursions)

			// Save the configuration to file
			err := viper.WriteConfig()
			if err != nil {
				service.Errorf("Error saving config: %s\n", err)
				return
			}
			fmt.Printf("Current maximum recursions: %d\n", maxRecursions)
		} else {
			// Argument provided - parse and set new value
			var err error
			newValue := args[0]
			maxRecursions, err := strconv.Atoi(newValue)
			if err != nil {
				service.Errorf("Invalid value: %s. Please provide a valid integer.\n", newValue)
				return
			}

			if maxRecursions < 1 {
				service.Errorf("Value must be a positive integer (at least 1).\n")
				return
			}

			// Set the new value in viper
			viper.Set("max_recursions", maxRecursions)

			// Save the configuration to file
			err = viper.WriteConfig()
			if err != nil {
				service.Errorf("Error saving config: %s\n", err)
				return
			}

			fmt.Printf("Maximum recursions set to: %d\n", maxRecursions)
		}
	},
}

// configSetCmd represents the command to set configuration values
var configSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set a configuration value",
	Long:  `Set a configuration value that will persist across sessions.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 2 {
			service.Errorf("Usage: gllm config set <key> <value>\n")
			return
		}

		key := args[0]
		value := args[1]

		switch key {
		case "max_recursions":
			// Parse the value as an integer
			num, err := strconv.Atoi(value)
			if err != nil {
				service.Errorf("Invalid value for max_recursions: %s (must be an integer)\n", value)
				return
			}
			if num <= 0 {
				service.Errorf("Invalid value for max_recursions: %d (must be positive)\n", num)
				return
			}
			viper.Set("max_recursions", num)
		default:
			service.Errorf("Unknown configuration key: %s\n", key)
			return
		}

		// Write the config file
		if err := writeConfig(); err != nil {
			service.Errorf("Error saving configuration: %s\n", err)
			return
		}

		fmt.Printf("Configuration '%s' set to '%s' successfully.\n", key, value)
	},
}

func GetMaxRecursions() int {
	maxRecursions := viper.GetInt("max_recursions")
	if maxRecursions <= 0 {
		maxRecursions = 5 // Default value
	}
	return maxRecursions
}
