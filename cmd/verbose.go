package cmd

import (
	"fmt"

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
		fmt.Println(cmd.Long)
		fmt.Println()
		settings := data.GetSettingsStore()
		current := settings.GetVerboseEnabled()
		PrintVerboseStatus(current)
	},
}

var verboseSwitchCmd = &cobra.Command{
	Use:     "switch [true|false]",
	Aliases: []string{"sw"},
	Short:   "Toggle or set verbose mode",
	Args:    cobra.MaximumNArgs(1),
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
				fmt.Printf("%sInvalid argument: %s. Use 'true' or 'false'.%s\n", data.StatusErrorColor, arg, data.ResetSeq)
				return
			}
		}

		if err := settings.SetVerboseEnabled(enable); err != nil {
			fmt.Printf("%sFailed to update settings: %v%s\n", data.StatusErrorColor, err, data.ResetSeq)
			return
		}

		PrintVerboseStatus(enable)
	},
}

func PrintVerboseStatus(enabled bool) {
	var status string
	if enabled {
		status = data.SwitchOnColor + "true" + data.ResetSeq
	} else {
		status = data.SwitchOffColor + "false" + data.ResetSeq
	}
	fmt.Printf("Verbose mode: %s\n", status)
}
