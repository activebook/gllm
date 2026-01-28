package cmd

import (
	"fmt"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/internal/ui"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(themeCmd)
	themeCmd.AddCommand(themeSwitchCmd)
}

var themeCmd = &cobra.Command{
	Use:   "theme",
	Short: "Manage and switch themes",
	Long:  "Manage and switch themes, you can use switch command to switch to a different theme.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("\nCurrent Theme: %s%s%s\n\n", data.HighlightColor, data.CurrentThemeName, data.ResetSeq)

		var samples []string

		// 1. Enable/Disable
		samples = append(samples, fmt.Sprintf("%-20s: %sEnabled%s / %sDisabled%s", "Toggle State", data.SwitchOnColor, data.ResetSeq, data.SwitchOffColor, data.ResetSeq))

		// 2. Status Levels
		samples = append(samples, fmt.Sprintf("%-20s: %sInfo%s | %sWarn%s | %sSuccess%s | %sDebug%s | %sError%s",
			"Status Levels",
			data.StatusInfoColor, data.ResetSeq,
			data.StatusWarnColor, data.ResetSeq,
			data.StatusSuccessColor, data.ResetSeq,
			data.StatusDebugColor, data.ResetSeq,
			data.StatusErrorColor, data.ResetSeq))

		samples = append(samples, fmt.Sprintf("%-20s: %sToken Count%s", "Labels", data.LabelColor, data.ResetSeq))

		// 3. Normal / Thinking
		samples = append(samples, fmt.Sprintf("%-20s: %sThinking ↓%s", "Thinking State", data.ReasoningActiveColor, data.ResetSeq))
		samples = append(samples, fmt.Sprintf("%-20s: %sInner Thinking...%s", "Thinking Message", data.ReasoningDoneColor, data.ResetSeq))

		// Thinking Effort
		samples = append(samples, fmt.Sprintf("%-20s: %sHigh%s | %sMedium%s | %sLow%s | %sOff%s", "Thinking Effort", data.ReasoningHighColor, data.ResetSeq, data.ReasoningMedColor, data.ResetSeq, data.ReasoningLowColor, data.ResetSeq, data.ReasoningOffColor, data.ResetSeq))

		// Normal Message
		samples = append(samples, fmt.Sprintf("%-20s: Assistant Response", "Normal Message"))

		// 4. Task Complete
		samples = append(samples, fmt.Sprintf("%-20s: %s[✓] Task Completed%s", "Completion", data.TaskCompleteColor, data.ResetSeq))

		// 5. Tool Call
		samples = append(samples, fmt.Sprintf("%-20s: %s[TOOL] execute_command()%s", "Tool Call", data.ToolCallColor, data.ResetSeq))

		block := lipgloss.JoinVertical(lipgloss.Left, samples...)

		style := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(data.BorderHex)).
			Padding(1, 2).
			MarginLeft(1)

		fmt.Println(style.Render(block))
	},
}

var themeSwitchCmd = &cobra.Command{
	Use:     "switch [name]",
	Aliases: []string{"sw"},
	Short:   "Switch to a different theme",
	Long:    "Switch to a different theme, you can use fuzzy search to find the theme you want to switch to.",
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var name string
		if len(args) > 0 {
			name = args[0]
		} else {
			// Interactive mode with filtering
			themes := data.ListThemes()
			options := make([]huh.Option[string], len(themes))
			for i, t := range themes {
				options[i] = huh.NewOption(t, t)
			}
			ui.SortOptions(options, data.CurrentThemeName)

			err := huh.NewSelect[string]().
				Title("Select Theme").
				Description("Search through 300+ themes using / to filter").
				Options(options...).
				Filtering(true).
				Value(&name).
				Run()

			if err != nil {
				return // User cancelled
			}
		}

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
		// Run theme command to show new samples
		themeCmd.Run(themeCmd, []string{})
	},
}
