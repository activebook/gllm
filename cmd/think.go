package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var thinkCmd = &cobra.Command{
	Use:   "think",
	Short: "Enable/Disable deep think mode",
	Long:  `Deep think mode enhances the model's reasoning capabilities.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(cmd.Long)
		fmt.Println()

		// Show current status
		enabled := IsThinkEnabled()
		if enabled {
			fmt.Printf("Deep think mode: %s\n", switchOnColor+"on"+resetColor)
		} else {
			fmt.Printf("Deep think mode: %s\n", switchOffColor+"off"+resetColor)
		}
	},
}

var thinkOnCmd = &cobra.Command{
	Use:   "on",
	Short: "Enable deep think mode",
	Long:  `Enable deep think mode which enhances the model's reasoning capabilities.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Set the agent.think flag to true in the configuration
		if err := SetAgentValue("think", true); err != nil {
			fmt.Printf("Error saving config: %v\n", err)
			return
		}

		fmt.Printf("Deep think mode %s\n", switchOnColor+"on"+resetColor)
	},
}

var thinkOffCmd = &cobra.Command{
	Use:   "off",
	Short: "Disable deep think mode",
	Long:  `Disable deep think mode to return to normal operation.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Set the agent.think flag to false in the configuration
		if err := SetAgentValue("think", false); err != nil {
			fmt.Printf("Error saving config: %v\n", err)
			return
		}

		fmt.Printf("Deep think mode %s\n", switchOffColor+"off"+resetColor)
	},
}

func init() {
	// Add subcommands to the main think command
	thinkCmd.AddCommand(thinkOnCmd)
	thinkCmd.AddCommand(thinkOffCmd)

	// Add the main think command to the root command
	rootCmd.AddCommand(thinkCmd)
}

func SwitchThinkMode(mode string) {
	switch mode {
	case "on":
		thinkOnCmd.Run(thinkCmd, []string{})
	case "off":
		thinkOffCmd.Run(thinkCmd, []string{})
	default:
		fmt.Printf("invalid think mode: %s", mode)
	}
}

// IsThinkEnabled returns whether deep think mode is enabled
func IsThinkEnabled() bool {
	// By default, deep think mode is disabled
	enabled := GetAgentBool("think")
	return enabled
}
