package cmd

import (
	"fmt"

	"github.com/activebook/gllm/service"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Manage MCP (Model Context Protocol) servers and tools",
	Long:  `Commands for interacting with MCP servers and listing available tools.`,
}

var mcpListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available MCP tools",
	Long:  `Lists all tools available from configured MCP servers.`,
	Run: func(cmd *cobra.Command, args []string) {
		all, _ := cmd.Flags().GetBool("all")
		client := &service.MCPClient{}
		err := client.Init(all) // Load all servers if detail is true, else false
		if err != nil {
			fmt.Printf("Error initializing MCP client: %v\n", err)
			return
		}
		defer client.Close()

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

func init() {
	mcpListCmd.Flags().BoolP("all", "a", false, "List all mcp servers, including blocked ones")
	mcpCmd.AddCommand(mcpListCmd)
	rootCmd.AddCommand(mcpCmd)
}
