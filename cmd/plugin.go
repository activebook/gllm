// File: cmd/plugin.go
package cmd

import (
	"fmt"

	"github.com/activebook/gllm/util"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/internal/ui"
	"github.com/activebook/gllm/service"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

// KnownPlugins enumerates all available global plugins.
var KnownPlugins = []struct {
	ID    string
	Label string
	Desc  string
}{
	{
		ID:    service.PluginVSCodeCompanion,
		Label: service.PluginVSCodeCompanionTitle,
		Desc:  service.PluginVSCodeCompanionDesc,
	},
}

func init() {
	rootCmd.AddCommand(pluginCmd)
	pluginCmd.AddCommand(pluginSwitchCmd)
}

var pluginCmd = &cobra.Command{
	Use:     "plugin",
	Aliases: []string{"plugins", "pg"},
	Short:   "Manage gllm plugins",
	Long: `View and manage plugins for gllm.
Use 'gllm plugin switch' to toggle plugins on or off.`,
	Run: func(cmd *cobra.Command, args []string) {
		store := data.GetSettingsStore()
		enabled := store.GetEnabledPlugins()
		printPluginSummary(enabled)
		util.Println(cmd, "Use 'gllm plugin switch' to change.")
	},
}

var pluginSwitchCmd = &cobra.Command{
	Use:     "switch",
	Aliases: []string{"sw", "sel", "select"},
	Short:   "Toggle plugins on/off",
	Long:    "Interactive switch to enable or disable gllm plugins.",
	Run: func(cmd *cobra.Command, args []string) {
		store := data.GetSettingsStore()
		currentlyEnabled := store.GetEnabledPlugins()

		// Build a fast-lookup set of currently-enabled IDs
		enabledSet := make(map[string]bool, len(currentlyEnabled))
		for _, id := range currentlyEnabled {
			enabledSet[id] = true
		}

		var options []huh.Option[string]
		var selected []string

		for _, p := range KnownPlugins {
			if enabledSet[p.ID] {
				options = append(options, huh.NewOption(p.Label, p.ID).Selected(true))
				selected = append(selected, p.ID)
			} else {
				options = append(options, huh.NewOption(p.Label, p.ID))
			}
		}

		// Sort: selected items appear at the top of the list
		ui.SortMultiOptions(options, selected)

		msPlugins := huh.NewMultiSelect[string]().
			Title("Available Plugins").
			Description("Use space to toggle, enter to confirm.").
			Options(options...).
			Value(&selected)

		// Dynamic note panel that shows the description of the highlighted item
		pluginNote := ui.GetDynamicHuhNote("Plugin Details", msPlugins, func(highlighted string) string {
			for _, p := range KnownPlugins {
				if p.Label == highlighted || p.ID == highlighted {
					return p.Desc
				}
			}
			return ""
		})

		err := huh.NewForm(
			huh.NewGroup(msPlugins, pluginNote),
		).Run()
		if err != nil {
			util.Println(cmd, "Operation cancelled.")
			return
		}

		if err := store.SetEnabledPlugins(selected); err != nil {
			util.Printf(cmd, "Error saving plugin settings: %v\n", err)
			return
		}

		util.Printf(cmd, "Plugin settings updated. %d enabled.\n", len(selected))
		util.Println(cmd)
		printPluginSummary(selected)
	},
}

func printPluginSummary(enabled []string) {
	fmt.Println("Available Plugins:")
	fmt.Println()

	enabledSet := make(map[string]bool, len(enabled))
	for _, id := range enabled {
		enabledSet[id] = true
	}

	for _, p := range KnownPlugins {
		indicator := ui.FormatEnabledIndicator(enabledSet[p.ID])
		fmt.Printf("%s %s\n", indicator, p.Label)
		if p.Desc != "" {
			fmt.Printf("%s%s%s\n", data.DetailColor, p.Desc, data.ResetSeq)
		}
		fmt.Println()
	}

	fmt.Printf("%s = Enabled\n", ui.FormatEnabledIndicator(true))
}
