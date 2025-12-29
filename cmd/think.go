package cmd

import (
	"fmt"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/service"
	"github.com/spf13/cobra"
)

var thinkCmd = &cobra.Command{
	Use:   "think",
	Short: "Enable/Disable deep think mode",
	Long:  `Deep think mode enhances the model's reasoning capabilities.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(cmd.Long)
		fmt.Println()

		store := data.NewConfigStore()
		agent := store.GetActiveAgent()
		if agent == nil {
			fmt.Println(switchOffColor + "off" + resetColor)
			return
		}
		if agent.Think {
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
		store := data.NewConfigStore()
		agent := store.GetActiveAgent()
		if agent == nil {
			fmt.Println(switchOffColor + "off" + resetColor)
			return
		}
		agent.Think = true
		if err := store.SetAgent(agent.Name, agent); err != nil {
			service.Errorf("failed to save think mode: %v", err)
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
		store := data.NewConfigStore()
		agent := store.GetActiveAgent()
		if agent == nil {
			fmt.Println(switchOffColor + "off" + resetColor)
			return
		}
		agent.Think = false
		if err := store.SetAgent(agent.Name, agent); err != nil {
			service.Errorf("failed to save think mode: %v", err)
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
