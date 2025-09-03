package cmd

import (
	"fmt"
	"strings"

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

var mcpAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new MCP server",
	Long:  `Add a new MCP server to the configuration. Requires name and type. For sse/http types, url is required. For std/local types, command is required.`,
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		serverType, _ := cmd.Flags().GetString("type")
		url, _ := cmd.Flags().GetString("url")
		command, _ := cmd.Flags().GetString("command")
		headers, _ := cmd.Flags().GetStringSlice("header")
		envVars, _ := cmd.Flags().GetStringSlice("env")

		// Validate required fields
		if name == "" {
			fmt.Println("Error: name is required")
			return
		}
		if serverType == "" {
			fmt.Println("Error: type is required")
			return
		}

		// Validate type
		validTypes := map[string]bool{"std": true, "local": true, "sse": true, "http": true}
		if !validTypes[serverType] {
			fmt.Println("Error: type must be one of: std, local, sse, http")
			return
		}

		// Validate type-specific required fields
		if serverType == "sse" || serverType == "http" {
			if url == "" {
				fmt.Printf("Error: url is required for type %s\n", serverType)
				return
			}
		} else if serverType == "std" || serverType == "local" {
			if command == "" {
				fmt.Printf("Error: command is required for type %s\n", serverType)
				return
			}
		}

		// Load existing config
		config, err := service.LoadMCPServers()
		if err != nil {
			fmt.Printf("Error loading MCP config: %v\n", err)
			return
		}

		// Initialize config if nil
		if config == nil {
			config = &service.MCPConfig{
				MCPServers:      make(map[string]service.MCPServerConfig),
				AllowMCPServers: []string{},
			}
		}

		// Check if server already exists
		if _, exists := config.MCPServers[name]; exists {
			fmt.Printf("Error: MCP server '%s' already exists. Use 'set' to update it.\n", name)
			return
		}

		// Create new server config
		serverConfig := service.MCPServerConfig{
			Name: name,
			Type: serverType,
		}

		// Set type-specific fields
		if serverType == "sse" {
			serverConfig.Url = url
		} else if serverType == "http" {
			serverConfig.HttpUrl = url
		} else if serverType == "std" || serverType == "local" {
			// Parse command into Command and Args
			parts := strings.Fields(command)
			if len(parts) > 0 {
				serverConfig.Command = parts[0]
				if len(parts) > 1 {
					serverConfig.Args = parts[1:]
				}
			}
		}

		// Parse headers
		if len(headers) > 0 {
			serverConfig.Headers = make(map[string]string)
			for _, header := range headers {
				parts := strings.SplitN(header, "=", 2)
				if len(parts) == 2 {
					serverConfig.Headers[parts[0]] = parts[1]
				}
			}
		}

		// Parse env vars
		if len(envVars) > 0 {
			serverConfig.Env = make(map[string]string)
			for _, env := range envVars {
				parts := strings.SplitN(env, "=", 2)
				if len(parts) == 2 {
					serverConfig.Env[parts[0]] = parts[1]
				}
			}
		}

		// Add to config
		serverConfig.Allowed = true // allow by default
		config.MCPServers[name] = serverConfig

		// Add to allowed servers by default
		config.AllowMCPServers = append(config.AllowMCPServers, name)

		// Save config
		err = service.SaveMCPServers(config)
		if err != nil {
			fmt.Printf("Error saving MCP config: %v\n", err)
			return
		}

		fmt.Printf("Successfully added MCP server '%s':\n", name)
		fmt.Printf("  Type: %s\n", serverConfig.Type)
		if serverConfig.Url != "" {
			fmt.Printf("  URL: %s\n", serverConfig.Url)
		}
		if serverConfig.HttpUrl != "" {
			fmt.Printf("  HTTP URL: %s\n", serverConfig.HttpUrl)
		}
		if serverConfig.Command != "" {
			fmt.Printf("  Command: %s", serverConfig.Command)
			if len(serverConfig.Args) > 0 {
				fmt.Printf(" %s", strings.Join(serverConfig.Args, " "))
			}
			fmt.Println()
		}
		if len(serverConfig.Headers) > 0 {
			fmt.Println("  Headers:")
			for k, v := range serverConfig.Headers {
				fmt.Printf("    %s: %s\n", k, v)
			}
		}
		if len(serverConfig.Env) > 0 {
			fmt.Println("  Environment:")
			for k, v := range serverConfig.Env {
				fmt.Printf("    %s: %s\n", k, v)
			}
		}
	},
}

var mcpSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set/update an MCP server",
	Long:  `Set or update an existing MCP server in the configuration. Requires name. Type is determined from existing server. For sse/http types, url is required. For std/local types, command is required.`,
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		url, _ := cmd.Flags().GetString("url")
		command, _ := cmd.Flags().GetString("command")
		headers, _ := cmd.Flags().GetStringSlice("header")
		envVars, _ := cmd.Flags().GetStringSlice("env")
		allow, _ := cmd.Flags().GetBool("allow")

		// Validate required fields
		if name == "" {
			fmt.Println("Error: name is required")
			return
		}

		// Load existing config
		config, err := service.LoadMCPServers()
		if err != nil {
			fmt.Printf("Error loading MCP config: %v\n", err)
			return
		}

		// Check if server exists
		existingServer, exists := config.MCPServers[name]
		if !exists {
			fmt.Printf("Error: MCP server '%s' does not exist. Use 'add' to create a new server.\n", name)
			return
		}

		// Get type from existing server
		serverType := existingServer.Type

		// Validate type-specific required fields
		if serverType == "sse" || serverType == "http" {
			if url == "" {
				fmt.Printf("Error: url is required for type %s\n", serverType)
				return
			}
		} else if serverType == "std" || serverType == "local" {
			if command == "" {
				fmt.Printf("Error: command is required for type %s\n", serverType)
				return
			}
		}

		// Create updated server config based on existing
		serverConfig := existingServer
		serverConfig.Name = name

		// Set type-specific fields
		if serverType == "sse" {
			serverConfig.Url = url
		} else if serverType == "http" {
			serverConfig.HttpUrl = url
		} else if serverType == "std" || serverType == "local" {
			// Parse command into Command and Args
			parts := strings.Fields(command)
			if len(parts) > 0 {
				serverConfig.Command = parts[0]
				if len(parts) > 1 {
					serverConfig.Args = parts[1:]
				}
			}
		}

		// Parse headers (merge with existing)
		if len(headers) > 0 {
			if serverConfig.Headers == nil {
				serverConfig.Headers = make(map[string]string)
			}
			for _, header := range headers {
				parts := strings.SplitN(header, "=", 2)
				if len(parts) == 2 {
					serverConfig.Headers[parts[0]] = parts[1]
				}
			}
		}

		// Parse env vars (merge with existing)
		if len(envVars) > 0 {
			if serverConfig.Env == nil {
				serverConfig.Env = make(map[string]string)
			}
			for _, env := range envVars {
				parts := strings.SplitN(env, "=", 2)
				if len(parts) == 2 {
					serverConfig.Env[parts[0]] = parts[1]
				}
			}
		}

		// Handle allow flag - add or remove from AllowMCPServers
		if allow {
			serverConfig.Allowed = true
			// Check if already in AllowMCPServers
			found := false
			for _, allowed := range config.AllowMCPServers {
				if allowed == name {
					found = true
					break
				}
			}
			if !found {
				config.AllowMCPServers = append(config.AllowMCPServers, name)
			}
		} else {
			serverConfig.Allowed = false
			// Remove from AllowMCPServers if present
			for i, allowed := range config.AllowMCPServers {
				if allowed == name {
					config.AllowMCPServers = append(config.AllowMCPServers[:i], config.AllowMCPServers[i+1:]...)
					break
				}
			}
		}

		// Update in config
		config.MCPServers[name] = serverConfig

		// Save config
		err = service.SaveMCPServers(config)
		if err != nil {
			fmt.Printf("Error saving MCP config: %v\n", err)
			return
		}

		fmt.Printf("Successfully updated MCP server '%s':\n", name)
		fmt.Printf("  Type: %s\n", serverConfig.Type)
		if serverConfig.Url != "" {
			fmt.Printf("  URL: %s\n", serverConfig.Url)
		}
		if serverConfig.HttpUrl != "" {
			fmt.Printf("  HTTP URL: %s\n", serverConfig.HttpUrl)
		}
		if serverConfig.Command != "" {
			fmt.Printf("  Command: %s", serverConfig.Command)
			if len(serverConfig.Args) > 0 {
				fmt.Printf(" %s", strings.Join(serverConfig.Args, " "))
			}
			fmt.Println()
		}
		if len(serverConfig.Headers) > 0 {
			fmt.Println("  Headers:")
			for k, v := range serverConfig.Headers {
				fmt.Printf("    %s: %s\n", k, v)
			}
		}
		if len(serverConfig.Env) > 0 {
			fmt.Println("  Environment:")
			for k, v := range serverConfig.Env {
				fmt.Printf("    %s: %s\n", k, v)
			}
		}
	},
}

var mcpRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove an MCP server",
	Long:  `Remove an MCP server from the configuration. Requires name of the server to remove.`,
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")

		// Validate required fields
		if name == "" {
			fmt.Println("Error: name is required")
			return
		}

		// Load existing config
		config, err := service.LoadMCPServers()
		if err != nil {
			fmt.Printf("Error loading MCP config: %v\n", err)
			return
		}

		// Check if server exists
		if _, exists := config.MCPServers[name]; !exists {
			fmt.Printf("Error: MCP server '%s' does not exist\n", name)
			return
		}

		// Remove the server from config
		delete(config.MCPServers, name)

		// Also remove from AllowMCPServers if present
		for i, allowed := range config.AllowMCPServers {
			if allowed == name {
				config.AllowMCPServers = append(config.AllowMCPServers[:i], config.AllowMCPServers[i+1:]...)
				break
			}
		}

		// Save config
		err = service.SaveMCPServers(config)
		if err != nil {
			fmt.Printf("Error saving MCP config: %v\n", err)
			return
		}

		fmt.Printf("Successfully removed MCP server '%s'\n", name)
	},
}

func init() {
	mcpListCmd.Flags().BoolP("all", "a", false, "List all mcp servers, including blocked ones")
	mcpAddCmd.Flags().StringP("name", "n", "", "Name of the MCP server (required)")
	mcpAddCmd.Flags().StringP("type", "t", "", "Type of the MCP server: std, local, sse, http (required)")
	mcpAddCmd.Flags().StringP("url", "u", "", "URL for sse/http type servers")
	mcpAddCmd.Flags().StringP("command", "c", "", "Command for std/local type servers")
	mcpAddCmd.Flags().StringSliceP("header", "H", []string{}, "HTTP headers in key=value format (can be used multiple times)")
	mcpAddCmd.Flags().StringSliceP("env", "e", []string{}, "Environment variables in key=value format (can be used multiple times)")
	mcpSetCmd.Flags().StringP("name", "n", "", "Name of the MCP server (required)")
	mcpSetCmd.Flags().StringP("url", "u", "", "URL for sse/http type servers")
	mcpSetCmd.Flags().StringP("command", "c", "", "Command for std/local type servers")
	mcpSetCmd.Flags().StringSliceP("header", "H", []string{}, "HTTP headers in key=value format (can be used multiple times)")
	mcpSetCmd.Flags().StringSliceP("env", "e", []string{}, "Environment variables in key=value format (can be used multiple times)")
	mcpSetCmd.Flags().BoolP("allow", "a", false, "Allow this MCP server to be used")
	mcpRemoveCmd.Flags().StringP("name", "n", "", "Name of the MCP server to remove (required)")
	mcpCmd.AddCommand(mcpListCmd)
	mcpCmd.AddCommand(mcpOnCmd)
	mcpCmd.AddCommand(mcpOffCmd)
	mcpCmd.AddCommand(mcpAddCmd)
	mcpCmd.AddCommand(mcpSetCmd)
	mcpCmd.AddCommand(mcpRemoveCmd)
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
