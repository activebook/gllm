package cmd

import (
	"encoding/json"
	"strings"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/service"
	"github.com/activebook/gllm/util"
	"github.com/spf13/cobra"
)

var (
	goalStatusFlag     bool
	goalCompleteFlag   bool
	goalClearFlag      bool
	goalJSONFlag       bool
	goalCriteriaFlag   string
	goalMilestonesFlag []string
	goalNotesFlag      string
)

// goalCmd represents the goal command
var goalCmd = &cobra.Command{
	Use:   "goal [objective]",
	Short: "Set, inspect, or manage invariant session goals and milestones",
	Long: `Establish, track, and verify persistent session goals.
Unlike standard conversational turns, goals act as immutable executive invariants
anchored in the agent's system prompt on every generation cycle.`,
	Run: func(cmd *cobra.Command, args []string) {
		targetSession := GetContextSession(cmd)
		if targetSession == "" {
			targetSession = sessionName
		}
		if targetSession != "" {
			data.SetActiveSessionName(targetSession)
		}

		// 1. Clear goal
		if goalClearFlag {
			data.ClearActiveGoal()
			util.Println(cmd, "Active session goal cleared.")
			return
		}

		// 2. Complete goal
		if goalCompleteFlag {
			notes := goalNotesFlag
			if len(args) > 0 && notes == "" {
				notes = strings.Join(args, " ")
			}
			if ok := data.CompleteActiveGoal(notes); ok {
				util.Println(cmd, "Active session goal marked as COMPLETED ✅")
			} else {
				util.Println(cmd, "No active session goal to complete.")
			}
			return
		}

		// 3. Set new goal if argument provided
		if len(args) > 0 {
			objective := strings.Join(args, " ")
			goal := data.SetActiveGoal(objective, goalCriteriaFlag, goalMilestonesFlag)

			if goalJSONFlag {
				printGoalJSON(cmd, goal)
				return
			}

			card := service.RenderGoalCard(goal)
			util.Println(cmd, card)
			return
		}

		// 4. Default: Show active goal
		goal, _ := data.GetActiveGoal()

		if goalJSONFlag {
			printGoalJSON(cmd, goal)
			return
		}

		card := service.RenderGoalCard(goal)
		util.Println(cmd, card)
	},
}

func printGoalJSON(cmd *cobra.Command, goal *data.SessionGoal) {
	if goal == nil {
		util.Println(cmd, "null")
		return
	}

	jsonData, err := json.MarshalIndent(goal, "", "  ")
	if err != nil {
		util.Errorf(cmd, "Failed to marshal goal JSON: %v\n", err)
		return
	}
	util.Println(cmd, string(jsonData))
}

func init() {
	goalCmd.Flags().BoolVar(&goalStatusFlag, "status", false, "Show active goal card")
	goalCmd.Flags().BoolVar(&goalCompleteFlag, "complete", false, "Mark active goal as completed")
	goalCmd.Flags().BoolVar(&goalClearFlag, "clear", false, "Clear the active goal")
	goalCmd.Flags().BoolVar(&goalJSONFlag, "json", false, "Output goal in JSON format")
	goalCmd.Flags().StringVar(&goalCriteriaFlag, "criteria", "", "Acceptance criteria required for completion")
	goalCmd.Flags().StringSliceVar(&goalMilestonesFlag, "milestone", []string{}, "Milestone description (repeatable)")
	goalCmd.Flags().StringVar(&goalNotesFlag, "notes", "", "Verification notes upon completion")
	goalCmd.Flags().StringVarP(&sessionName, "session", "s", "", "Specify session name for the goal")

	rootCmd.AddCommand(goalCmd)
}
