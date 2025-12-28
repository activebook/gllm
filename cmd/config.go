// File: cmd/config.go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/service"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
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
	Use:   "export [directory]",
	Short: "Export configuration to a directory",
	Long: `Export current configuration to a directory.

If no directory is specified, the configuration will be exported to the current directory.
Files will be saved as 'gllm.yaml' and 'mcp.json' (if MCP config exists).`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var exportDir string

		if len(args) == 0 {
			exportDir = "."
		} else {
			exportDir = args[0]
		}

		// Ensure the export directory exists
		if err := os.MkdirAll(exportDir, 0755); err != nil {
			service.Errorf("Error creating export directory: %s\n", err)
			return
		}

		// Set export file paths
		configExportFile := filepath.Join(exportDir, "gllm.yaml")
		mcpExportFile := filepath.Join(exportDir, "mcp.json")

		configStore := data.NewConfigStore()

		// Write the configuration to the file using ConfigStore
		if err := configStore.Export(configExportFile); err != nil {
			service.Errorf("Error exporting configuration: %s\n", err)
			return
		}

		fmt.Printf("Configuration exported successfully to: %s\n", configExportFile)

		mcpStore := data.NewMCPStore()

		// Check if MCP config exists and export it
		servers, allowed, err := mcpStore.Load()
		if err != nil {
			service.Errorf("Error loading MCP configuration: %s\n", err)
			return
		}

		if servers != nil {
			// MCP config exists, save it to export location
			if err := mcpStore.SaveToPath(servers, allowed, mcpExportFile); err != nil {
				service.Errorf("Error exporting MCP configuration: %s\n", err)
				return
			}
			fmt.Printf("MCP configuration exported successfully to: %s\n", mcpExportFile)
		} else {
			fmt.Printf("No MCP configuration found to export\n")
		}
	},
}

// configImportCmd represents the config import command
var configImportCmd = &cobra.Command{
	Use:   "import [directory]",
	Short: "Import configuration from a directory",
	Long: `Import configuration from a directory.

This will look for 'gllm.yaml' and 'mcp.json' files in the specified directory
and merge them with the current configuration. If no directory is specified,
it will look in the current directory.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var importDir string

		if len(args) == 0 {
			importDir = "."
		} else {
			importDir = args[0]
		}

		// Set import file paths
		configImportFile := filepath.Join(importDir, "gllm.yaml")
		mcpImportFile := filepath.Join(importDir, "mcp.json")

		// Check if main config file exists
		if _, err := os.Stat(configImportFile); os.IsNotExist(err) {
			service.Errorf("Configuration file does not exist: %s\n", configImportFile)
			return
		}

		storeConfig := data.NewConfigStore()

		// Import the configuration using ConfigStore
		if err := storeConfig.Import(configImportFile); err != nil {
			service.Errorf("Error importing configuration: %s\n", err)
			return
		}

		fmt.Printf("Configuration imported successfully from: %s\n", configImportFile)

		// Check if MCP config exists and import it
		if _, err := os.Stat(mcpImportFile); os.IsNotExist(err) {
			fmt.Printf("No MCP configuration file found to import\n")
		}

		// MCP config exists, load and save it
		mcpStore := data.NewMCPStore()
		mcpConfig, allowed, err := mcpStore.LoadFromPath(mcpImportFile)
		if err != nil {
			service.Errorf("Error loading MCP configuration: %s\n", err)
			return
		}
		if mcpConfig != nil {
			// Save the MCP config to the default location
			if err := mcpStore.Save(mcpConfig, allowed); err != nil {
				service.Errorf("Error saving MCP configuration: %s\n", err)
				return
			}
			fmt.Printf("MCP configuration imported successfully from: %s\n", mcpImportFile)
		}
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

		printSection := func(title string) {
			fmt.Println()
			fullTitle := fmt.Sprintf("=== %s ===", strings.ToUpper(title))
			fmt.Printf("%s\n", sectionColor(fullTitle))
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

	// Add flags for other prompt commands if needed in the future
}
