package cmd

import (
	"encoding/json"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/service"
	"github.com/activebook/gllm/util"
	"github.com/spf13/cobra"
)

var (
	statusJSONFlag bool
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show system status and latest token usage",
	Long: `Display the active agent configuration, model limits, runtime modes,
context window utilization, and detailed token consumption from the latest turn.`,
	Run: func(cmd *cobra.Command, args []string) {
		store := data.NewConfigStore()
		agent := store.GetActiveAgent()
		snapshot, _ := service.GetLatestTokenUsage()

		currentSession := sessionName
		if currentSession == "" {
			currentSession = "default"
		}

		if statusJSONFlag {
			type StatusResponse struct {
				Agent      *data.AgentConfig           `json:"agent,omitempty"`
				Session    string                      `json:"session"`
				PlanMode   bool                        `json:"plan_mode"`
				YoloMode   bool                        `json:"yolo_mode"`
				TokenUsage *service.TokenUsageSnapshot `json:"token_usage,omitempty"`
			}

			resp := StatusResponse{
				Agent:      agent,
				Session:    currentSession,
				PlanMode:   data.GetPlanModeInSession(),
				YoloMode:   data.GetYoloModeInSession(),
				TokenUsage: snapshot,
			}

			jsonData, err := json.MarshalIndent(resp, "", "  ")
			if err != nil {
				util.Errorf(cmd, "Failed to marshal status JSON: %v\n", err)
				return
			}
			util.Println(cmd, string(jsonData))
			return
		}

		card := service.RenderStatusCard(snapshot, agent, currentSession)
		util.Println(cmd, card)
	},
}

func init() {
	statusCmd.Flags().BoolVar(&statusJSONFlag, "json", false, "Output status and token metrics in JSON format")
	rootCmd.AddCommand(statusCmd)
}
