package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/service"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Enable/Disable MCP (Model Context Protocol) servers and tools",
	Long: `MCP gives gllm the ability to access external data, tools, and services.
Switch on/off to enable/disable all mcp servers`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(cmd.Long)
		fmt.Println()
		fmt.Print("MCP is currently: ")
		store := data.NewConfigStore()
		agent := store.GetActiveAgent()
		if agent == nil {
			fmt.Println(switchOffColor + "disabled" + resetColor)
			return
		}
		enabled := agent.MCP
		if enabled {
			fmt.Println(switchOnColor + "enabled" + resetColor)
		} else {
			fmt.Println(switchOffColor + "disabled" + resetColor)
		}
	},
}

var mcpLoadCmd = &cobra.Command{
	Use:     "load",
	Aliases: []string{"ls", "show", "pr"},
	Short:   "List all available MCP tools",
	Long:    `Lists all tools available from configured MCP servers.`,
	Run: func(cmd *cobra.Command, args []string) {
		all, _ := cmd.Flags().GetBool("all")
		prompts, _ := cmd.Flags().GetBool("prompts")
		resources, _ := cmd.Flags().GetBool("resources")

		// Load config from data store
		store := data.NewMCPStore()
		mcpConfig, _, err := store.Load()
		if err != nil {
			fmt.Printf("Error loading MCP config: %v\n", err)
			return
		}

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
		indicator := service.NewIndicator("")
		indicator.Start("MCP Loading...")
		err = client.Init(mcpConfig, service.MCPLoadOption{
			LoadAll:       all,
			LoadTools:     true,
			LoadResources: resources,
			LoadPrompts:   prompts,
		}) // Load all servers if detail is true, else false
		indicator.Stop()
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
		fmt.Println()

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
			if server.Resources != nil && len(*server.Resources) > 0 {
				fmt.Println("  Resources:")
				for _, resource := range *server.Resources {
					fmt.Printf("    • %s%s%s\n", cmdOutputColor, resource.Name, resetColor)
					if resource.Description != "" {
						fmt.Printf("      Description: %s\n", resource.Description)
					}
					if resource.URI != "" {
						fmt.Printf("      URI: %s\n", resource.URI)
					}
					if resource.MIMEType != "" {
						fmt.Printf("      MIME Type: %s\n", resource.MIMEType)
					}
					fmt.Println()
				}
			}
			if server.Prompts != nil && len(*server.Prompts) > 0 {
				fmt.Println("  Prompts:")
				for _, prompt := range *server.Prompts {
					fmt.Printf("    • %s%s%s\n", cmdOutputColor, prompt.Name, resetColor)
					if prompt.Description != "" {
						fmt.Printf("      Description: %s\n", prompt.Description)
					}
					if len(prompt.Parameters) > 0 {
						fmt.Printf("      Parameters:\n")
						for param, desc := range prompt.Parameters {
							fmt.Printf("        - %s: %s\n", param, desc)
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
		store := data.NewConfigStore()
		agent := store.GetActiveAgent()
		if agent == nil {
			fmt.Printf("Error: No active agent found\n")
			return
		}
		agent.MCP = true
		if err := store.SetAgent(agent.Name, agent); err != nil {
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
		store := data.NewConfigStore()
		agent := store.GetActiveAgent()
		if agent == nil {
			fmt.Printf("Error: No active agent found\n")
			return
		}
		agent.MCP = false
		if err := store.SetAgent(agent.Name, agent); err != nil {
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
		switch serverType {
		case "sse", "http":
			if url == "" {
				fmt.Printf("Error: url is required for type %s\n", serverType)
				return
			}
		case "std", "local":
			if command == "" {
				fmt.Printf("Error: command is required for type %s\n", serverType)
				return
			}
		}

		// Create new server config
		serverConfig := &data.MCPServer{
			Name:    name,
			Type:    serverType,
			Allowed: true, // allow by default
		}

		// Set type-specific fields
		switch serverType {
		case "sse":
			serverConfig.URL = url
		case "http":
			serverConfig.HTTPUrl = url
		case "std", "local":
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

		// Add to store
		store := data.NewMCPStore()
		err := store.AddServer(serverConfig)
		if err != nil {
			fmt.Printf("Error saving MCP config: %v\n", err)
			return
		}

		fmt.Printf("Successfully added MCP server '%s':\n", name)
		fmt.Printf("  Type: %s\n", serverConfig.Type)
		fmt.Printf("  Allowed: %t\n", serverConfig.Allowed)
		if serverConfig.URL != "" {
			fmt.Printf("  URL: %s\n", serverConfig.URL)
		}
		if serverConfig.HTTPUrl != "" {
			fmt.Printf("  HTTP URL: %s\n", serverConfig.HTTPUrl)
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
	Long:  `Set or update an existing MCP server in the configuration. Requires name. Type is determined from existing server. Only validate required fields when they are being set.`,
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

		// Load existing server
		store := data.NewMCPStore()
		existingServer, err := store.GetServer(name)
		if err != nil {
			fmt.Printf("Error: %v. Use 'add' to create a new server.\n", err)
			return
		}

		// Get type from existing server
		serverType := existingServer.Type

		// Validate type-specific required fields only when flags are provided
		urlChanged := cmd.Flags().Changed("url")
		commandChanged := cmd.Flags().Changed("command")

		switch serverType {
		case "sse", "http":
			if urlChanged && url == "" {
				fmt.Printf("Error: url is required for type %s\n", serverType)
				return
			}
		case "std", "local":
			if commandChanged && command == "" {
				fmt.Printf("Error: command is required for type %s\n", serverType)
				return
			}
		}

		// Update server config
		serverConfig := existingServer

		// Set type-specific fields only when flags are provided
		if serverType == "sse" && urlChanged {
			serverConfig.URL = url
		} else if serverType == "http" && urlChanged {
			serverConfig.HTTPUrl = url
		} else if (serverType == "std" || serverType == "local") && commandChanged {
			// Parse command into Command and Args
			parts := strings.Fields(command)
			if len(parts) > 0 {
				serverConfig.Command = parts[0]
				if len(parts) > 1 {
					serverConfig.Args = parts[1:]
				}
			}
		}

		// Parse headers (merge with existing) only when provided
		if cmd.Flags().Changed("header") {
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

		// Parse env vars (merge with existing) only when provided
		if cmd.Flags().Changed("env") {
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

		// Handle allow flag only when explicitly provided
		if cmd.Flags().Changed("allow") {
			serverConfig.Allowed = allow
		}

		// Save config
		err = store.UpdateServer(serverConfig)
		if err != nil {
			fmt.Printf("Error saving MCP config: %v\n", err)
			return
		}

		fmt.Printf("Successfully updated MCP server '%s':\n", name)
		fmt.Printf("  Type: %s\n", serverConfig.Type)
		fmt.Printf("  Allowed: %t\n", serverConfig.Allowed)
		if serverConfig.URL != "" {
			fmt.Printf("  URL: %s\n", serverConfig.URL)
		}
		if serverConfig.HTTPUrl != "" {
			fmt.Printf("  HTTP URL: %s\n", serverConfig.HTTPUrl)
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

		store := data.NewMCPStore()
		err := store.RemoveServer(name)
		if err != nil {
			fmt.Printf("Error removing MCP server '%s': %v\n", name, err)
			return
		}

		fmt.Printf("Successfully removed MCP server '%s'\n", name)
	},
}

var mcpListCmd = &cobra.Command{
	Use:   "list",
	Short: "List MCP servers",
	Long:  `List all MCP servers in the configuration.`,
	Run: func(cmd *cobra.Command, args []string) {
		store := data.NewMCPStore()
		servers, _, err := store.Load()
		if err != nil {
			fmt.Printf("Error loading MCP config: %v\n", err)
			return
		}

		fmt.Println("MCP servers:")
		for name, server := range servers {
			fmt.Printf("  %s: %s %t\n", name, server.Type, server.Allowed)
		}
	},
}

var mcpExportCmd = &cobra.Command{
	Use:   "export [file]",
	Short: "Export MCP configuration to a file",
	Long: `Export MCP configuration to a file.

If no file is specified, the configuration will be exported to 'mcp.json' 
in the current directory.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var exportFile string

		if len(args) == 0 {
			exportFile = "mcp.json"
		} else {
			exportFile = args[0]
		}

		store := data.NewMCPStore()
		err := store.Export(exportFile)
		if err != nil {
			fmt.Printf("Error exporting MCP configuration: %v\n", err)
			return
		}

		fmt.Printf("MCP configuration exported successfully to: %s\n", exportFile)
	},
}

var mcpImportCmd = &cobra.Command{
	Use:   "import [file]",
	Short: "Import MCP configuration from a file",
	Long: `Import MCP configuration from a file.

This will replace the current MCP configuration with the imported one.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		importFile := args[0]

		store := data.NewMCPStore()
		err := store.Import(importFile)
		if err != nil {
			fmt.Printf("Error importing MCP configuration: %v\n", err)
			return
		}

		fmt.Printf("MCP configuration imported successfully from: %s\n", importFile)
	},
}

var mcpPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show the location of the MCP configuration file",
	Long:  `Displays the full path to the MCP configuration file. You can manually edit it and reload the available MCPs.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Get the MCP config path
		store := data.NewMCPStore()
		configPath := store.GetPath()

		// Check if the file exists
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			// File doesn't exist, initialize with template
			err := store.CreateTemplate()
			if err != nil {
				fmt.Printf("Error initializing MCP configuration file: %v\n", err)
				return
			}
			fmt.Printf("MCP configuration file initialized at: %s\n", configPath)
		} else {
			fmt.Printf("MCP configuration file location: %s\n", configPath)
		}
	},
}

func init() {
	mcpLoadCmd.Flags().BoolP("all", "a", false, "List all mcp servers, including blocked ones")
	mcpLoadCmd.Flags().BoolP("prompts", "p", false, "Include MCP prompts, if available")
	mcpLoadCmd.Flags().BoolP("resources", "r", false, "Include MCP resources, if available")
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
	mcpCmd.AddCommand(mcpLoadCmd)
	mcpCmd.AddCommand(mcpListCmd)
	mcpCmd.AddCommand(mcpOnCmd)
	mcpCmd.AddCommand(mcpOffCmd)
	mcpCmd.AddCommand(mcpAddCmd)
	mcpCmd.AddCommand(mcpSetCmd)
	mcpCmd.AddCommand(mcpRemoveCmd)
	mcpCmd.AddCommand(mcpExportCmd)
	mcpCmd.AddCommand(mcpImportCmd)
	mcpCmd.AddCommand(mcpPathCmd)
	rootCmd.AddCommand(mcpCmd)
}
