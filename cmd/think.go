package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var thinkCmd = &cobra.Command{
	Use:   "think",
	Short: "Enable/Disable deep think mode",
	Long:  `Deep think mode enhances the model's reasoning capabilities.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(cmd.Long)
		fmt.Println("-------------------------------------------")

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
		viper.Set("agent.think", true)

		// Save the configuration
		if err := writeConfig(); err != nil {
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
		viper.Set("agent.think", false)

		// Save the configuration
		if err := writeConfig(); err != nil {
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
	enabled := false
	if viper.IsSet("agent.think") {
		enabled = viper.GetBool("agent.think")
	}
	return enabled
}
