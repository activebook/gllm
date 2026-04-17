package cmd

import (
	"fmt"

	"github.com/activebook/gllm/util"

	"github.com/activebook/gllm/data"
	"github.com/spf13/cobra"
)

func init() {
	configCmd.AddCommand(verboseCmd)
	verboseCmd.AddCommand(verboseSwitchCmd)
}

var verboseCmd = &cobra.Command{
	Use:   "verbose",
	Short: "Manage verbose output mode settings",
	Long: `Display or manage verbose output for agent interactions.
When enabled, displays detailed tool calls, reasoning steps, and subagent progress.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Print Long description
		util.Println(cmd, cmd.Long)
		util.Println(cmd)
		settings := data.GetSettingsStore()
		current := settings.GetVerboseEnabled()
		util.Print(cmd, renderVerboseStatus(current))
	},
}

var verboseSwitchCmd = &cobra.Command{
	Use:     "switch [true|false]",
	Aliases: []string{"sw"},
	Short:   "Toggle or set verbose mode",
	Args:    cobra.MaximumNArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return []string{"true", "false", "on", "off", "enable", "disable"}, cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	Run: func(cmd *cobra.Command, args []string) {
		settings := data.GetSettingsStore()
		current := settings.GetVerboseEnabled()

		var enable bool
		if len(args) == 0 {
			// Toggle behavior
			enable = !current
		} else {
			arg := args[0]
			switch arg {
			case "true", "on", "enable":
				enable = true
			case "false", "off", "disable":
				enable = false
			default:
				util.Printf(cmd, "%sInvalid argument: %s. Use 'true' or 'false'.%s\n", data.StatusErrorColor, arg, data.ResetSeq)
				return
			}
		}

		if err := settings.SetVerboseEnabled(enable); err != nil {
			util.Printf(cmd, "%sFailed to update settings: %v%s\n", data.StatusErrorColor, err, data.ResetSeq)
			return
		}

		util.Print(cmd, renderVerboseStatus(enable))
	},
}

func renderVerboseStatus(enabled bool) string {
	status := data.SwitchOffColor + "false" + data.ResetSeq
	if enabled {
		status = data.SwitchOnColor + "true" + data.ResetSeq
	}
	return fmt.Sprintf("Verbose mode: %s\n", status)
}
