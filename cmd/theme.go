package cmd

import (
	"fmt"
	"strings"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/internal/ui"
	"github.com/activebook/gllm/io"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

const markdownSample = `## This is a Header.
* This is a **paragraph**.
* Using ` + "`%s`" + ` style.
* [Google](https://www.google.com)
### Code Block
` + "```python" + `
def hello_world():
    print("Hello, world!")

hello_world()
` + "```" + `
### Blockquote
---
> This is a blockquote.
---`

func init() {
	configCmd.AddCommand(themeCmd)
	themeCmd.AddCommand(themeSwitchCmd)
}

var themeCmd = &cobra.Command{
	Use:   "theme",
	Short: "Manage and switch themes",
	Run: func(cmd *cobra.Command, args []string) {
		termWidth := io.GetTerminalWidth()
		safeWidth := max(40, termWidth-4)

		borderStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(data.BorderHex)).
			Width(safeWidth).
			Margin(0, 1).
			Padding(1, 1, 0)

		innerWidth := safeWidth - borderStyle.GetHorizontalFrameSize()

		// Split into left (~55%) and right (~45%) columns
		separatorWidth := 0
		leftWidth := (innerWidth - separatorWidth) * 55 / 100
		rightWidth := innerWidth - leftWidth - separatorWidth

		// --- Left panel: logo + theme info ---
		logo := ui.GetLogo(data.KeyHex, data.LabelHex, 0.5)

		leftParts := []string{}
		leftParts = append(leftParts, lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(data.KeyHex)).
			Width(leftWidth).
			Padding(0, 1).
			Align(lipgloss.Center).
			Render(logo+"\n"+data.CurrentThemeName))

		leftParts = append(leftParts, lipgloss.NewStyle().
			Foreground(lipgloss.Color(data.KeyHex)).
			Width(leftWidth).
			Render(strings.Repeat("─", leftWidth-4)))

		leftParts = append(leftParts, fmt.Sprintf("%-14s: %sEnabled%s / %sDisabled%s", "Toggle State", data.SwitchOnColor, data.ResetSeq, data.SwitchOffColor, data.ResetSeq))
		leftParts = append(leftParts, fmt.Sprintf("%-14s: %sInfo%s | %sWarn%s | %sSuccess%s | %sDebug%s | %sError%s",
			"Status Levels",
			data.StatusInfoColor, data.ResetSeq,
			data.StatusWarnColor, data.ResetSeq,
			data.StatusSuccessColor, data.ResetSeq,
			data.StatusDebugColor, data.ResetSeq,
			data.StatusErrorColor, data.ResetSeq))
		leftParts = append(leftParts, fmt.Sprintf("%-14s: %sToken Count%s\n", "Labels", data.LabelColor, data.ResetSeq))
		leftParts = append(leftParts, fmt.Sprintf("%-14s: %sThinking ↓%s", "Thinking State", data.ReasoningActiveColor, data.ResetSeq))
		leftParts = append(leftParts, fmt.Sprintf("%-14s: %sInner Thinking...%s", "Thinking Msg", data.ReasoningDoneColor, data.ResetSeq))
		leftParts = append(leftParts, fmt.Sprintf("%-14s: %sHigh%s | %sMed%s | %sLow%s | %sMin%s | %sOff%s\n",
			"Thinking Effort",
			data.ReasoningHighColor, data.ResetSeq,
			data.ReasoningMedColor, data.ResetSeq,
			data.ReasoningLowColor, data.ResetSeq,
			data.ReasoningMinColor, data.ResetSeq,
			data.ReasoningOffColor, data.ResetSeq))
		leftParts = append(leftParts, fmt.Sprintf("%-14s: Assistant Response", "Normal Msg"))
		leftParts = append(leftParts, fmt.Sprintf("%-14s: %s[✓] Task Completed%s\n", "Completion", data.TaskCompleteColor, data.ResetSeq))
		leftParts = append(leftParts, fmt.Sprintf("%-14s: %s[TOOL] exec_cmd()%s", "Tool Call", data.ToolCallColor, data.ResetSeq))

		leftContent := lipgloss.NewStyle().
			Foreground(lipgloss.Color(data.LabelHex)).
			Width(leftWidth).
			Align(lipgloss.Left).
			Padding(1, 1).
			Render(strings.Join(leftParts, "\n"))

		// --- Right panel: markdown preview ---
		glamourStyle := data.MostSimilarGlamourStyle()
		tr, _ := glamour.NewTermRenderer(
			glamour.WithStandardStyle(glamourStyle),
			glamour.WithWordWrap(rightWidth),
		)
		md := fmt.Sprintf(markdownSample, glamourStyle)
		out, _ := tr.Render(md)
		out = strings.TrimSuffix(out, "\n")

		rightParts := []string{}
		rightParts = append(rightParts, lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(data.KeyHex)).
			Width(rightWidth).
			Align(lipgloss.Center).
			Padding(0, 0, 0, 1).
			Render("Markdown"))
		rightParts = append(rightParts, out)

		rightContent := lipgloss.NewStyle().
			Foreground(lipgloss.Color(data.LabelHex)).
			Width(rightWidth).
			Align(lipgloss.Left).
			Render(strings.Join(rightParts, "\n"))

		// --- Combine panels horizontally ---
		inner := lipgloss.JoinHorizontal(
			lipgloss.Top,
			leftContent,
			rightContent,
		)

		banner := borderStyle.Render(inner)
		fmt.Println(banner)
	},
}

var themeSwitchCmd = &cobra.Command{
	Use:     "switch [name]",
	Aliases: []string{"sw"},
	Short:   "Switch to a different theme",
	Long:    "Switch to a different theme. You can use fuzzy search to find the theme you want to switch to.",
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
			height := io.GetTermFitHeight(len(options))

			err := huh.NewSelect[string]().
				Title("Select Theme").
				Description("Search through 300+ themes using / to filter").
				Height(height).
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
