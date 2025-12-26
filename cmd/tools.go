package cmd

import (
	"fmt"

	"github.com/activebook/gllm/service"
	"github.com/spf13/cobra"
)

var toolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "Enable/Disable embedding tools globally",
	Long: `Tools give gllm the ability to interact with the file system, execute commands, and perform other operations.
Switch on/off to enable/disable all embedding tools`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(cmd.Long)
		fmt.Println("-------------------------------------------")
		ListAllTools()
	},
}

var toolsOnCmd = &cobra.Command{
	Use:   "on",
	Short: "Enable all embedding tools",
	Run: func(cmd *cobra.Command, args []string) {
		err := SetAgentValue("tools", true)
		if err != nil {
			fmt.Printf("Error writing config: %v\n", err)
			return
		}
		fmt.Printf("All embedding tools %s\n\n", switchOnColor+"enabled"+resetColor)
		ListAllTools()
	},
}

var toolsOffCmd = &cobra.Command{
	Use:   "off",
	Short: "Disable all embedding tools",
	Run: func(cmd *cobra.Command, args []string) {
		err := SetAgentValue("tools", false)
		if err != nil {
			fmt.Printf("Error writing config: %v\n", err)
			return
		}
		fmt.Printf("All embedding tools %s\n\n", switchOffColor+"disabled"+resetColor)
		ListAllTools()
	},
}

func init() {
	toolsCmd.AddCommand(toolsOnCmd)
	toolsCmd.AddCommand(toolsOffCmd)
	rootCmd.AddCommand(toolsCmd)
}

func AreToolsEnabled() bool {
	enabled := GetAgentBool("tools")
	return enabled
}

func SwitchUseTools(s string) {
	switch s {
	case "on":
		toolsOnCmd.Run(toolsOnCmd, nil)
	case "off":
		toolsOffCmd.Run(toolsOffCmd, nil)
	default:
		toolsCmd.Run(toolsCmd, nil)
	}
}

func ListEmbeddingTools() {
	enabled := AreToolsEnabled()
	fmt.Println("Available[✔] embedding tools:")
	for _, tool := range service.GetAllEmbeddingTools() {
		if enabled {
			fmt.Printf("[✔] %s\n", tool)
		} else {
			fmt.Printf("[ ] %s\n", tool)
		}
	}
}

func ListAllTools() {
	ListEmbeddingTools()
	fmt.Println()
	ListSearchTools()
}
