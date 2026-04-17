package cmd

import (
	"fmt"

	"github.com/activebook/gllm/util"

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
		util.Printf(cmd, "Current Theme: %s\n", data.CurrentThemeName)
		util.Printf(cmd, "Terminal Color Support: ")
		p := termenv.ColorProfile()
		if p == termenv.TrueColor {
			util.Println(cmd, data.SwitchOnColor+"TRUE COLOR (24-bit)"+data.ResetSeq)
		} else {
			util.Println(cmd, data.StatusWarnColor+fmt.Sprintf("%v", p)+data.ResetSeq)
		}
		util.Println(cmd)

		// Helper
		printColor := func(name, colorSeq string) {
			util.Printf(cmd, "%s%s%s (Text)\n", colorSeq, name, data.ResetSeq)
		}

		util.Println(cmd, "=== Semantic Colors ===")

		util.Println(cmd, "\n-- Roles --")
		printColor("RoleSystemColor", data.RoleSystemColor)
		printColor("RoleUserColor", data.RoleUserColor)
		printColor("RoleAssistantColor", data.RoleAssistantColor)

		util.Println(cmd, "\n-- Tools --")
		printColor("ToolCallColor", data.ToolCallColor)
		printColor("ToolResponseColor", data.ToolResponseColor)

		util.Println(cmd, "\n-- Status --")
		printColor("StatusErrorColor", data.StatusErrorColor)
		printColor("StatusSuccessColor", data.StatusSuccessColor)
		printColor("StatusWarnColor", data.StatusWarnColor)
		printColor("StatusInfoColor", data.StatusInfoColor)
		printColor("StatusDebugColor", data.StatusDebugColor)

		util.Println(cmd, "\n-- Reasoning --")
		printColor("ReasoningTagColor", data.ReasoningTagColor)
		printColor("ReasoningTextColor", data.ReasoningTextColor)

		printColor("ReasoningOffColor", data.ReasoningOffColor)
		printColor("ReasoningLowColor", data.ReasoningLowColor)
		printColor("ReasoningMedColor", data.ReasoningMedColor)
		printColor("ReasoningHighColor", data.ReasoningHighColor)

		util.Println(cmd, "\n-- UI & Workflow --")
		printColor("SwitchOnColor", data.SwitchOnColor)
		printColor("SwitchOffColor", data.SwitchOffColor)
		printColor("TaskCompleteColor", data.TaskCompleteColor)
		printColor("WorkflowColor", data.WorkflowColor)
		printColor("AgentRoleColor", data.AgentRoleColor)
		printColor("ModelColor", data.ModelColor)
		printColor("DirectoryColor", data.DirectoryColor)
		printColor("PromptColor", data.PromptColor)
		printColor("MediaColor", data.MediaColor)

		util.Println(cmd, "\n-- Diff --")
		printColor("DiffAddedColor", data.DiffAddedColor)
		printColor("DiffRemovedColor", data.DiffRemovedColor)
		printColor("DiffHeaderColor", data.DiffHeaderColor)
		printColor("DiffSeparatorColor", data.DiffSeparatorColor)
		printColor("DiffAddedBgColor", data.DiffAddedBgColor)
		printColor("DiffRemovedBgColor", data.DiffRemovedBgColor)

		util.Println(cmd, "\n-- UI & Colors --")
		printColor("BorderColor", data.BorderColor)
		printColor("SectionColor", data.SectionColor)
		printColor("KeyColor", data.KeyColor)
		printColor("HighlightColor", data.HighlightColor)
		printColor("DetailColor", data.DetailColor)
		printColor("ShellOutputColor", data.ShellOutputColor)

		util.Println(cmd)
		util.Println(cmd, data.ResetSeq)
	},
}
