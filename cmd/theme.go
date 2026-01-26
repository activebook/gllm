package cmd

import (
	"fmt"

	"github.com/activebook/gllm/data"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(themeCmd)
	themeCmd.AddCommand(themeListCmd)
	themeCmd.AddCommand(themeSwitchCmd)
}

var themeCmd = &cobra.Command{
	Use:   "theme",
	Short: "Manage and switch themes",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Current Theme: %s%s%s\n", data.HighlightColor, data.CurrentThemeName, data.ResetSeq)
		fmt.Println()
		fmt.Println("--- Theme Color Sample ---")

		// 1. Enable/Disable
		fmt.Printf("%-20s: %sEnabled%s / %sDisabled%s\n", "Toggle State", data.SwitchOnColor, data.ResetSeq, data.SwitchOffColor, data.ResetSeq)

		// 2. Success/Warning/Debug/Info/Error
		fmt.Printf("%-20s: %sInfo%s | %sWarn%s | %sSuccess%s | %sDebug%s | %sError%s\n",
			"Status Levels",
			data.StatusInfoColor, data.ResetSeq,
			data.StatusWarnColor, data.ResetSeq,
			data.StatusSuccessColor, data.ResetSeq,
			data.StatusDebugColor, data.ResetSeq,
			data.StatusErrorColor, data.ResetSeq)

		// 3. Normal / Thinking
		fmt.Printf("%-20s: %sThinking ✓%s\n", "Thinking State", data.ReasoningActiveColor, data.ResetSeq)
		fmt.Printf("%-20s: %sInner Thinking...%s\n", "Thinking Message", data.ReasoningDoneColor, data.ResetSeq)
		fmt.Printf("%-20s: %sAssistant Response%s\n", "Normal Message", data.ForegroundColor, data.ResetSeq)

		// 4. Task Complete
		fmt.Printf("%-20s: %s[✓] Task Completed%s\n", "Completion", data.TaskCompleteColor, data.ResetSeq)

		// 5. Tool Call
		fmt.Printf("%-20s: %s[TOOL] execute_command()%s\n", "Tool Call", data.ToolCallColor, data.ResetSeq)

		// 6. Border Line
		fmt.Printf("%-20s: %s----------%s\n", "Border Line", data.BorderColor, data.ResetSeq)
		fmt.Println("--------------------------")
	},
}

var themeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available themes",
	Run: func(cmd *cobra.Command, args []string) {
		themes := data.ListThemes()
		for _, name := range themes {
			if name == data.CurrentThemeName {
				fmt.Printf("%s* %s%s\n", data.SwitchOnColor, name, data.ResetSeq)
			} else {
				fmt.Println(name)
			}
		}
	},
}

var themeSwitchCmd = &cobra.Command{
	Use:     "switch <name>",
	Aliases: []string{"sw"},
	Short:   "Switch to a different theme",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		err := data.LoadTheme(name)
		if err != nil {
			fmt.Printf("%sError: %v%s\n", data.StatusErrorColor, err, data.ResetSeq)
			return
		}

		err = data.SaveThemeConfig(name)
		if err != nil {
			fmt.Printf("%sWarning: Failed to save theme config: %v%s\n", data.StatusWarnColor, err, data.ResetSeq)
		}

		fmt.Printf("%sSuccessfully switched to theme: %s%s\n", data.StatusSuccessColor, name, data.ResetSeq)
	},
}
