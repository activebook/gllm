// File: cmd/config.go
package cmd

import (
	"fmt"
	"os"
	"sort"
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
		printSection("Plugins")
		plugins := GetLoadedPlugins()
		fmt.Fprintln(w, headerColor(" Plugin ")+"\t"+headerColor(" Loaded "))
		fmt.Fprintln(w, headerColor("-------")+"\t"+headerColor("----------"))
		for name, loaded := range plugins {
			loadedStr := highlightColor("Yes")
			if !loaded {
				loadedStr = color.New(color.FgRed, color.Bold).Sprint("No")
			}
			fmt.Fprintf(w, "%s\t%s\n", keyColor(name), loadedStr)
		}
		w.Flush()

		// Default Configuration section
		printSection("Default Configuration")

		mark := GetMarkdownSwitch()
		fmt.Printf("\n%s: %v\n", keyColor("Markdown Format"), mark)

		modelName, modelInfo := GetEffectiveModel()
		fmt.Printf("\n%s: %v\n", keyColor("Default Model"), highlightColor(modelName))
		fmt.Fprintln(w, headerColor(" PROPERTY ")+"\t"+headerColor(" VALUE "))
		fmt.Fprintln(w, headerColor("----------")+"\t"+headerColor("-------"))
		for property, value := range modelInfo {
			fmt.Fprintf(w, "%s\t%s\n", keyColor(property), (fmt.Sprintf("%v", value)))
		}
		w.Flush()

		searchName, searchEngine := GetEffectiveSearchEngine()
		fmt.Printf("\n%s: %v\n", keyColor("Default Search Engine"), (searchName))
		fmt.Fprintln(w, headerColor(" PROPERTY ")+"\t"+headerColor(" VALUE "))
		fmt.Fprintln(w, headerColor("----------")+"\t"+headerColor("-------"))
		pairs := []struct{ k, v string }{}
		for property, value := range searchEngine {
			pairs = append(pairs, struct{ k, v string }{
				keyColor(property),
				highlightColor(fmt.Sprintf("%v", value)),
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

	// Add flags for other prompt commands if needed in the future
}
