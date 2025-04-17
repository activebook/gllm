package cmd

import (
	"fmt"

	"github.com/activebook/gllm/service"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	// Terminal colors
	switchOffColor  = "\033[90m" // Bright Black
	switchOnColor   = "\033[92m" // Bright Green
	switchOnlyColor = "\033[41m" // BG Red
	resetColor      = "\033[0m"
)

func init() {
	rootCmd.AddCommand(markdownCmd)
	markdownCmd.AddCommand(markdownOnCmd)
	markdownCmd.AddCommand(markdownOffCmd)
	markdownCmd.AddCommand(markdownOnlyCmd)
}

var markdownCmd = &cobra.Command{
	Use:   "markdown",
	Short: "Switch markdown output on/off/only",
	Long: `When Markdown is switched on, the output will include a Markdown-formatted version.
When Markdown is switched off, the output will not include any Markdown formatting.

*Ps. If set Markdown 'only', then only keep the Markdown-formatted content.*`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(cmd.Long)
		fmt.Println("-------------------------------------------")
		fmt.Print("Markdown output is currently switched: ")
		mark := viper.GetString("default.markdown")
		switch mark {
		case "on":
			fmt.Println(switchOnColor + "on" + resetColor)
		case "only":
			fmt.Println(switchOnlyColor + "only" + resetColor)
		case "off":
			fmt.Println(switchOffColor + "off" + resetColor)
		default:
			fmt.Println(switchOffColor + "off" + resetColor)
		} // switch
	},
}

var markdownOnCmd = &cobra.Command{
	Use:   "on",
	Short: "Switch markdown output on",
	Run: func(cmd *cobra.Command, args []string) {
		viper.Set("default.markdown", "on")

		// Write the config file
		if err := writeConfig(); err != nil {
			service.Errorf("failed to save markdown format output: %w", err)
			return
		}

		fmt.Println("Makedown output switched " + switchOnColor + "on" + resetColor)
	},
}

var markdownOffCmd = &cobra.Command{
	Use:   "off",
	Short: "Switch markdown output off",
	Run: func(cmd *cobra.Command, args []string) {
		viper.Set("default.markdown", "off")

		// Write the config file
		if err := writeConfig(); err != nil {
			service.Errorf("failed to save markdown format output: %w", err)
			return
		}

		fmt.Println("Makedown output switched " + switchOffColor + "off" + resetColor)
	},
}

var markdownOnlyCmd = &cobra.Command{
	Use:   "only",
	Short: "Switch markdown output to only",
	Long:  `If set Markdown 'only', then only keep the Markdown-formatted content.`,
	Run: func(cmd *cobra.Command, args []string) {
		viper.Set("default.markdown", "only")

		// Write the config file
		if err := writeConfig(); err != nil {
			service.Errorf("failed to save markdown format output: %w", err)
			return
		}

		fmt.Println("Makedown output switched " + switchOnlyColor + "only" + resetColor)
	},
}

func SwitchMarkdown(s string) error {
	if s == "on" {
		viper.Set("default.markdown", "on")
	} else if s == "only" {
		viper.Set("default.markdown", "only")
	} else {
		viper.Set("default.markdown", "off")
	}

	// Write the config file
	if err := writeConfig(); err != nil {
		service.Errorf("failed to save markdown format output: %w", err)
		return err
	}
	return nil
}

func GetMarkdownSwitch() string {
	mark := viper.GetString("default.markdown")
	return mark
}
