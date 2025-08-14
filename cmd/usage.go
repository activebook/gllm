package cmd

import (
	"fmt"

	"github.com/activebook/gllm/service"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(usageCmd)
	usageCmd.AddCommand(usageOnCmd)
	usageCmd.AddCommand(usageOffCmd)
}

var usageCmd = &cobra.Command{
	Use:   "usage",
	Short: "Whether to include token usage metainfo in the output",
	Long: `When Usage is switched on, the output will include token usage metainfo.
When Usage is switched off, the output will not include any token usage metainfo.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(cmd.Long)
		fmt.Println("-------------------------------------------")
		fmt.Print("Usage output is currently switched: ")
		usage := viper.GetString("default.usage")
		switch usage {
		case "on":
			fmt.Println(switchOnColor + "on" + resetColor)
		case "off":
			fmt.Println(switchOffColor + "off" + resetColor)
		default:
			fmt.Println(switchOffColor + "off" + resetColor)
		} // switch
	},
}

var usageOnCmd = &cobra.Command{
	Use:   "on",
	Short: "Switch usage output on",
	Run: func(cmd *cobra.Command, args []string) {
		viper.Set("default.usage", "on")

		// Write the config file
		if err := writeConfig(); err != nil {
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
		viper.Set("default.usage", "off")

		// Write the config file
		if err := writeConfig(); err != nil {
			service.Errorf("failed to save usage format output: %w", err)
			return
		}

		fmt.Println("Usage output switched " + switchOffColor + "off" + resetColor)
	},
}

func SwitchUsageMetainfo(s string) error {
	switch s {
	case "on":
		viper.Set("default.usage", "on")
	case "off":
		viper.Set("default.usage", "off")
	default:
	}

	// Write the config file
	if err := writeConfig(); err != nil {
		service.Errorf("failed to save usage format output: %w", err)
		return err
	}
	return nil
}
