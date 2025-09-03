package cmd

import (
	"fmt"

	"github.com/activebook/gllm/service"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Enable/Disable MCP (Model Context Protocol) servers and tools",
	Long: `MCP gives gllm the ability to access external data, tools, and services.
Switch on/off to enable/disable all mcp servers`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(cmd.Long)
		fmt.Println("-------------------------------------------")
		fmt.Print("MCP is currently: ")
		enabled := viper.GetBool("agent.mcp")
		if enabled {
			fmt.Println(switchOnColor + "enabled" + resetColor)
		} else {
			fmt.Println(switchOffColor + "disabled" + resetColor)
		}
	},
}

var mcpListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available MCP tools",
	Long:  `Lists all tools available from configured MCP servers.`,
	Run: func(cmd *cobra.Command, args []string) {
		all, _ := cmd.Flags().GetBool("all")
		// here we don't need to use the shared instance
		// because we just need to check the available servers and tools
		// not making any calls to the servers
		var client *service.MCPClient
		if !all {
			client = service.GetMCPClient() // use the shared instance
		} else {
			// create a new instance to check all tools
			client = &service.MCPClient{}
			defer client.Close() // ensure resources are cleaned up
		}
		err := client.Init(all) // Load all servers if detail is true, else false
		if err != nil {
			fmt.Printf("Error initializing MCP client: %v\n", err)
			return
		}

		servers := client.GetAllServers()
		if len(servers) == 0 {
			fmt.Println("No MCP servers available.")
			return
		}

		fmt.Println("Available MCP Servers:")
		fmt.Println("====================")

		for _, server := range servers {
			status := switchOffColor + "Blocked" + resetColor
			if server.Allowed {
				status = switchOnColor + "Allowed" + resetColor
			}
			fmt.Printf("\n%sServer: %s%s (%s)\n", switchOnColor, server.Name, resetColor, status)
			if server.Tools != nil {
				for _, tool := range *server.Tools {
					fmt.Printf("  • %s%s%s\n", cmdOutputColor, tool.Name, resetColor)
					if tool.Description != "" {
						fmt.Printf("    Description: %s\n", tool.Description)
					}
					if len(tool.Parameters) > 0 {
						fmt.Printf("    Parameters:\n")
						for param, desc := range tool.Parameters {
							fmt.Printf("      - %s: %s\n", param, desc)
						}
					}
					fmt.Println()
				}
			}
		}
	},
}

var mcpOnCmd = &cobra.Command{
	Use:   "on",
	Short: "Enable MCP Servers",
	Run: func(cmd *cobra.Command, args []string) {
		viper.Set("agent.mcp", true)
		err := viper.WriteConfig()
		if err != nil {
			fmt.Printf("Error writing config: %v\n", err)
			return
		}
		fmt.Printf("MCP %s\n", switchOnColor+"enabled"+resetColor)
	},
}

var mcpOffCmd = &cobra.Command{
	Use:   "off",
	Short: "Disable MCP Servers",
	Run: func(cmd *cobra.Command, args []string) {
		viper.Set("agent.mcp", false)
		err := viper.WriteConfig()
		if err != nil {
			fmt.Printf("Error writing config: %v\n", err)
			return
		}
		fmt.Printf("MCP %s\n", switchOffColor+"disabled"+resetColor)
	},
}

func init() {
	mcpListCmd.Flags().BoolP("all", "a", false, "List all mcp servers, including blocked ones")
	mcpCmd.AddCommand(mcpListCmd)
	mcpCmd.AddCommand(mcpOnCmd)
	mcpCmd.AddCommand(mcpOffCmd)
	rootCmd.AddCommand(mcpCmd)
}

func AreMCPServersEnabled() bool {
	enabled := viper.GetBool("agent.mcp")
	return enabled
}

func SwitchMCP(s string) {
	switch s {
	case "on":
		mcpOnCmd.Run(mcpOnCmd, nil)
	case "off":
		mcpOffCmd.Run(mcpOffCmd, nil)
	default:
		mcpCmd.Run(mcpCmd, nil)
	}
}
