package cmd

import (
	"fmt"

	"github.com/activebook/gllm/data"
	"github.com/muesli/termenv"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(colorCmd)
}

var colorCmd = &cobra.Command{
	Use:    "color",
	Hidden: true, // hidden from help
	Short:  "Test different colors of gllm output",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Current Theme: %s\n", data.CurrentThemeName)
		fmt.Printf("Terminal Color Support: ")
		p := termenv.ColorProfile()
		if p == termenv.TrueColor {
			fmt.Println(data.SwitchOnColor + "TRUE COLOR (24-bit)" + data.ResetSeq)
		} else {
			fmt.Println(data.StatusWarnColor + fmt.Sprintf("%v", p) + data.ResetSeq)
		}
		fmt.Println()

		// Helper
		printColor := func(name, colorSeq string) {
			fmt.Printf("%s%s%s (Text)\n", colorSeq, name, data.ResetSeq)
		}

		fmt.Println("=== Semantic Colors ===")

		fmt.Println("\n-- Roles --")
		printColor("RoleSystemColor", data.RoleSystemColor)
		printColor("RoleUserColor", data.RoleUserColor)
		printColor("RoleAssistantColor", data.RoleAssistantColor)

		fmt.Println("\n-- Tools --")
		printColor("ToolCallColor", data.ToolCallColor)
		printColor("ToolResponseColor", data.ToolResponseColor)

		fmt.Println("\n-- Status --")
		printColor("StatusErrorColor", data.StatusErrorColor)
		printColor("StatusSuccessColor", data.StatusSuccessColor)
		printColor("StatusWarnColor", data.StatusWarnColor)
		printColor("StatusInfoColor", data.StatusInfoColor)
		printColor("StatusDebugColor", data.StatusDebugColor)

		fmt.Println("\n-- Reasoning --")
		printColor("ReasoningActiveColor", data.ReasoningActiveColor)
		printColor("ReasoningDoneColor", data.ReasoningDoneColor)

		printColor("ReasoningOffColor", data.ReasoningOffColor)
		printColor("ReasoningLowColor", data.ReasoningLowColor)
		printColor("ReasoningMedColor", data.ReasoningMedColor)
		printColor("ReasoningHighColor", data.ReasoningHighColor)

		fmt.Println("\n-- UI & Workflow --")
		printColor("SwitchOnColor", data.SwitchOnColor)
		printColor("SwitchOffColor", data.SwitchOffColor)
		printColor("TaskCompleteColor", data.TaskCompleteColor)
		printColor("WorkflowColor", data.WorkflowColor)
		printColor("AgentRoleColor", data.AgentRoleColor)
		printColor("ModelColor", data.ModelColor)
		printColor("DirectoryColor", data.DirectoryColor)
		printColor("PromptColor", data.PromptColor)
		printColor("MediaColor", data.MediaColor)

		fmt.Println("\n-- Diff --")
		printColor("DiffAddedColor", data.DiffAddedColor)
		printColor("DiffRemovedColor", data.DiffRemovedColor)
		printColor("DiffHeaderColor", data.DiffHeaderColor)
		printColor("DiffSeparatorColor", data.DiffSeparatorColor)
		printColor("DiffAddedBgColor", data.DiffAddedBgColor)
		printColor("DiffRemovedBgColor", data.DiffRemovedBgColor)

		fmt.Println("\n-- UI & Colors --")
		printColor("BorderColor", data.BorderColor)
		printColor("SectionColor", data.SectionColor)
		printColor("KeyColor", data.KeyColor)
		printColor("HighlightColor", data.HighlightColor)
		printColor("DetailColor", data.DetailColor)
		printColor("ShellOutputColor", data.ShellOutputColor)

		fmt.Println()
		fmt.Println(data.ResetSeq)
	},
}
