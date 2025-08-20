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
		usage := viper.GetString("agent.usage")
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
		viper.Set("agent.usage", "on")

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
		viper.Set("agent.usage", "off")

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
		viper.Set("agent.usage", "on")
		fmt.Println("Usage output switched " + switchOnColor + "on" + resetColor)
	case "off":
		viper.Set("agent.usage", "off")
		fmt.Println("Usage output switched " + switchOffColor + "off" + resetColor)
	default:
	}

	// Write the config file
	if err := writeConfig(); err != nil {
		service.Errorf("failed to save usage format output: %w", err)
		return err
	}
	return nil
}

func GetUsageMetainfoStatus() string {
	usage := viper.GetString("agent.usage")
	switch usage {
	case "on":
		return "on"
	case "off":
		return "off"
	default:
		return "off"
	}
}

func PrintUsageMetainfoStatus() {
	usage := viper.GetString("agent.usage")
	switch usage {
	case "on":
		fmt.Println("Usage output is currently switched " + switchOnColor + "on" + resetColor)
	case "off":
		fmt.Println("Usage output is currently switched " + switchOffColor + "off" + resetColor)
	default:
	}
}

func IncludeUsageMetainfo() bool {
	usage := viper.GetString("agent.usage")
	switch usage {
	case "on":
		return true
	case "off":
		return false
	default:
		return false
	}
}
