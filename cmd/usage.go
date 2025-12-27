package cmd

import (
	"fmt"

	"github.com/activebook/gllm/service"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(usageCmd)
	usageCmd.AddCommand(usageOnCmd)
	usageCmd.AddCommand(usageOffCmd)
}

var usageCmd = &cobra.Command{
	Use:     "usage",
	Aliases: []string{"ua"}, // Optional alias
	Short:   "Whether to include token usage metainfo in the output",
	Long: `When Usage is switched on, the output will include token usage metainfo.
When Usage is switched off, the output will not include any token usage metainfo.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(cmd.Long)
		fmt.Println()
		fmt.Print("Usage output is currently switched: ")
		usage := GetAgentBool("usage")
		if usage {
			fmt.Println(switchOnColor + "on" + resetColor)
		} else {
			fmt.Println(switchOffColor + "off" + resetColor)
		}
	},
}

var usageOnCmd = &cobra.Command{
	Use:   "on",
	Short: "Switch usage output on",
	Run: func(cmd *cobra.Command, args []string) {
		if err := SetAgentValue("usage", true); err != nil {
			service.Errorf("failed to save usage format output: %w", err)
			return
		}

		fmt.Println("Usage output switched " + switchOnColor + "on" + resetColor)
	},
}

var usageOffCmd = &cobra.Command{
	Use:   "off",
	Short: "Switch usage output off",
	Run: func(cmd *cobra.Command, args []string) {
		if err := SetAgentValue("usage", false); err != nil {
			service.Errorf("failed to save usage format output: %w", err)
			return
		}

		fmt.Println("Usage output switched " + switchOffColor + "off" + resetColor)
	},
}

func SwitchUsageMetainfo(s string) {
	switch s {
	case "on":
		usageOnCmd.Run(usageCmd, []string{})
	case "off":
		usageOffCmd.Run(usageCmd, []string{})
	default:
		usageCmd.Run(usageCmd, []string{})
	}
}

func IncludeUsageMetainfo() bool {
	usage := GetAgentBool("usage")
	return usage
}
