package cmd

import (
	"fmt"

	"github.com/activebook/gllm/service"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var toolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "Enable/Disable embedding tools globally",
	Long: `Tools give gllm the ability to interact with the file system, execute commands, and perform other operations.
By default, all tools are enabled. You can disable these tools.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(cmd.Long)
		fmt.Println("-------------------------------------------")
		ListEmbeddingTools()
	},
}

var toolsOnCmd = &cobra.Command{
	Use:   "on",
	Short: "Enable all tools",
	Run: func(cmd *cobra.Command, args []string) {
		viper.Set("agent.tools", "on")
		err := viper.WriteConfig()
		if err != nil {
			fmt.Printf("Error writing config: %v\n", err)
			return
		}
		fmt.Printf("All tools %s\n\n", switchOnColor+"enabled"+resetColor)
		ListEmbeddingTools()
	},
}

var toolsOffCmd = &cobra.Command{
	Use:   "off",
	Short: "Disable all tools",
	Run: func(cmd *cobra.Command, args []string) {
		viper.Set("agent.tools", "off")
		err := viper.WriteConfig()
		if err != nil {
			fmt.Printf("Error writing config: %v\n", err)
			return
		}
		fmt.Printf("All tools %s\n\n", switchOffColor+"disabled"+resetColor)
		ListEmbeddingTools()
	},
}

func init() {
	toolsCmd.AddCommand(toolsOnCmd)
	toolsCmd.AddCommand(toolsOffCmd)
	rootCmd.AddCommand(toolsCmd)
}

func AreToolsEnabled() bool {
	// By default, tools are enabled
	enabled := true
	if viper.IsSet("agent.tools") {
		enabled = viper.GetString("agent.tools") == "on"
	}
	return enabled
}

func SetToolsEnabled(enabled bool) error {
	value := "off"
	if enabled {
		value = "on"
	}
	viper.Set("agent.tools", value)
	if err := viper.WriteConfig(); err != nil {
		return fmt.Errorf("failed to save tools configuration: %w", err)
	}
	return nil
}

func SwitchUseTools(s string) error {
	switch s {
	case "on":
		viper.Set("agent.tools", "on")
	case "off":
		viper.Set("agent.tools", "off")
	default:
		return nil
	}

	// Write the config file
	if err := viper.WriteConfig(); err != nil {
		return fmt.Errorf("failed to save tools configuration: %w", err)
	}
	return nil
}

func ListEmbeddingTools() {
	enabled := AreToolsEnabled()
	fmt.Println("Available tools:")
	for _, tool := range service.GetAllEmbeddingTools() {
		if enabled {
			fmt.Printf("[✔] %s\n", tool)
		} else {
			fmt.Printf("[ ] %s\n", tool)
		}
	}
	fmt.Println("\n[✔] Indicates that a tool is enabled.")
}
