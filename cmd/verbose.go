package cmd

import (
	"fmt"

	"github.com/activebook/gllm/data"
	"github.com/spf13/cobra"
)

func init() {
	configCmd.AddCommand(verboseCmd)
}

var verboseCmd = &cobra.Command{
	Use:   "verbose [on|off]",
	Short: "Manage verbose output mode settings",
	Long: `Enable or disable verbose output for agent interactions. When enabled, displays detailed tool calls, reasoning steps, and subagent progress.

Note: This command is also available as 'gllm config verbose'.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		settings := data.GetSettingsStore()
		current := settings.GetVerboseEnabled()

		if len(args) == 0 {
			// Toggle behavior
			enable := !current
			if err := settings.SetVerboseEnabled(enable); err != nil {
				fmt.Printf("Failed to update settings: %v\n", err)
				return
			}
			fmt.Printf("Verbose mode toggled.\n")
			PrintVerboseStatus(enable)
			return
		}

		arg := args[0]
		var enable bool
		switch arg {
		case "on", "enable", "true":
			enable = true
		case "off", "disable", "false":
			enable = false
		default:
			fmt.Printf("Invalid argument: %s. Use 'on' or 'off'.\n", arg)
			return
		}

		if err := settings.SetVerboseEnabled(enable); err != nil {
			fmt.Printf("Failed to update settings: %v\n", err)
			return
		}

		fmt.Printf("Verbose mode set to: %s\n", arg)
		PrintVerboseStatus(enable)
	},
}

func PrintVerboseStatus(enabled bool) {
	indicator := data.StatusSuccessColor + "[✔]" + data.ResetSeq
	if !enabled {
		indicator = data.StatusErrorColor + "[ ]" + data.ResetSeq
	}
	fmt.Printf("Verbose mode: %s\n", indicator)
}
