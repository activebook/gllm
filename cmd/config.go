package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/service"
	"github.com/spf13/cobra"
)

// configCmd represents the base command when called without any subcommands
var configCmd = &cobra.Command{
	Use:     "config",
	Aliases: []string{"cfg", "settings", "set"}, // Optional alias
	Short:   "Manage gllm configuration/settings",
	Long: `View and manage settings for gllm.
use 'config path' to see where the settings file is located.`,
	// Add completion support
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return []string{"list", "set", "get", "edit", "path", "export", "import", "--help"}, cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
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
		store := data.NewConfigStore()

		// Check if a config file was explicitly loaded by Viper
		usedCfgFile := store.ConfigFileUsed()
		if usedCfgFile != "" {
			fmt.Printf("Configuration file in use: %s\n", usedCfgFile)
		} else {
			fmt.Printf("No configuration file loaded.\nDefault location is: %s\n", data.GetConfigFilePath())
		}
	},
}

// configExportCmd represents the config export command
var configExportCmd = &cobra.Command{
	Use:     "export [file]",
	Aliases: []string{"exp", "e"},
	Short:   "Export current configuration to a file or directory",
	Long: `Export current configuration to a file or directory.

If a directory is specified, the configuration will be exported as 'gllm.yaml' 
to that directory. If a file path is specified, it will be exported directly 
to that file. If no target is specified, it defaults to 'gllm.yaml' 
in the current directory.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var exportPath string

		if len(args) == 0 {
			exportPath = "gllm.yaml"
		} else {
			exportPath = args[0]
		}

		// Check if it's a directory
		if info, err := os.Stat(exportPath); err == nil && info.IsDir() {
			exportPath = filepath.Join(exportPath, "gllm.yaml")
		}

		configStore := data.NewConfigStore()

		// Write the configuration to the file using ConfigStore
		if err := configStore.Export(exportPath); err != nil {
			service.Errorf("Error exporting configuration: %s\n", err)
			return
		}

		fmt.Printf("Configuration exported successfully to: %s\n", exportPath)
	},
}

// configImportCmd represents the config import command
var configImportCmd = &cobra.Command{
	Use:   "import [file]",
	Short: "Import configuration from a file",
	Long: `Import configuration from a file.

This will replace the current configuration with the settings from the specified file.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		importFile := args[0]

		// Check if main config file exists
		if _, err := os.Stat(importFile); os.IsNotExist(err) {
			service.Errorf("Configuration file does not exist: %s\n", importFile)
			return
		}

		storeConfig := data.NewConfigStore()

		// Import the configuration using ConfigStore
		if err := storeConfig.Import(importFile); err != nil {
			service.Errorf("Error importing configuration: %s\n", err)
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

		printSection := func(title string) {
			fmt.Println()
			fullTitle := fmt.Sprintf("=== %s ===", strings.ToUpper(title))
			fmt.Printf("%s%s%s\n", data.SectionColor, fullTitle, data.ResetSeq)
		}

		// Models section
		printSection("Models")
		modelListCmd.Run(modelListCmd, []string{})
		w.Flush()

		// System Prompts section
		printSection("System Prompts")
		systemListCmd.Run(systemListCmd, []string{})
		w.Flush()

		// Templates section
		printSection("Templates")
		templateListCmd.Run(templateListCmd, []string{})
		w.Flush()

		// Memory section
		printSection("Memory")
		memoryListCmd.Run(memoryListCmd, []string{})
		w.Flush()

		// Search Engines section
		printSection("Search Engines")
		searchListCmd.Run(searchListCmd, []string{})
		w.Flush()

		// Plugins section
		printSection("Tools")
		ListAllTools()
		w.Flush()

		// Skills section
		printSection("Skills")
		skillsListCmd.Run(skillsListCmd, []string{})
		w.Flush()

		// Current Agent section
		printSection("Agents")
		agentCmd.Run(agentCmd, []string{})
		w.Flush()
	},
}

func init() {
	// Add configCmd to the root command
	rootCmd.AddCommand(configCmd)

	// Add subcommands to configCmd
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configPrintCmd)
	configCmd.AddCommand(configExportCmd) // Register theconfig export command
	configCmd.AddCommand(configImportCmd) // Register the config import command
}
