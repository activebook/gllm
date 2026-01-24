package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/service"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Enable/Disable MCP (Model Context Protocol) servers and tools",
	Long: `MCP gives gllm the ability to access external data, tools, and services.
Use 'gllm mcp switch' to toggle this capability on or off.`,
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
		enabled := service.IsMCPServersEnabled(agent.Capabilities)
		if enabled {
			fmt.Println(switchOnColor + "enabled" + resetColor)
		} else {
			fmt.Println(switchOffColor + "disabled" + resetColor)
		}
		fmt.Println("\nUse 'gllm mcp switch' to change.")
	},
}

var mcpSwitchCmd = &cobra.Command{
	Use:     "switch",
	Aliases: []string{"sw"},
	Short:   "Switch MCP capability on/off",
	Long:    "Interactive switch to enable or disable MCP capability for the current agent.",
	Run: func(cmd *cobra.Command, args []string) {
		store := data.NewConfigStore()
		agent := store.GetActiveAgent()
		if agent == nil {
			fmt.Println("No active agent to configure.")
			return
		}

		current := service.IsMCPServersEnabled(agent.Capabilities)
		var enable bool

		// Helper for options
		onOpt := huh.NewOption("On  - Enable MCP servers", true).Selected(current)
		offOpt := huh.NewOption("Off - Disable MCP servers", false).Selected(!current)

		err := huh.NewSelect[bool]().
			Title("MCP Capability").
			Description("Allow agent to use Model Context Protocol servers?").
			Options(onOpt, offOpt).
			Value(&enable).
			Run()

		if err != nil {
			fmt.Println("Operation cancelled.")
			return
		}

		if enable {
			agent.Capabilities = service.EnableMCPServers(agent.Capabilities)
			fmt.Println("MCP capability " + switchOnColor + "Enabled" + resetColor)
		} else {
			agent.Capabilities = service.DisableMCPServers(agent.Capabilities)
			fmt.Println("MCP capability " + switchOffColor + "Disabled" + resetColor)
		}

		if err := store.SetAgent(agent.Name, agent); err != nil {
			fmt.Printf("Error writing config: %v\n", err)
			return
		}
	},
}

var mcpLoadCmd = &cobra.Command{
	Use:   "load",
	Short: "List all available MCP tools",
	Long:  `Lists all tools available from configured MCP servers.`,
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

var mcpListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls", "show", "pr"},
	Short:   "List MCP servers",
	Long:    `List all MCP servers in the configuration.`,
	Run: func(cmd *cobra.Command, args []string) {
		store := data.NewMCPStore()
		servers, _, err := store.Load()
		if err != nil {
			fmt.Printf("Error loading MCP config: %v\n", err)
			return
		}

		if len(servers) == 0 {
			fmt.Println("No MCP servers defined.")
			return
		}

		fmt.Println("MCP servers:")

		// Sort keys for consistent output
		names := make([]string, 0, len(servers))
		for name := range servers {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			server := servers[name]
			indicator := "  "
			pname := fmt.Sprintf("%-18s", name)
			status := grayColor("(blocked)")

			if server.Allowed {
				indicator = highlightColor("* ")
				pname = highlightColor(pname)
				status = greenColor("(allowed)")
			}

			fmt.Printf("%s%s %-7s %s\n", indicator, pname, server.Type, status)
		}

		fmt.Printf("\n(*) Indicates the allowed MCP server.\n")
	},
}

var mcpExportCmd = &cobra.Command{
	Use:   "export [file]",
	Short: "Export MCP configuration to a file",
	Long: `Export MCP configuration to a file or directory.

If a directory is specified, the configuration will be exported as 'mcp.json' 
to that directory. If a file path is specified, it will be exported directly 
to that file. If no target is specified, it defaults to 'mcp.json' 
in the current directory.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var exportPath string

		if len(args) == 0 {
			exportPath = "mcp.json"
		} else {
			exportPath = args[0]
		}

		// Check if it's a directory
		if info, err := os.Stat(exportPath); err == nil && info.IsDir() {
			exportPath = filepath.Join(exportPath, "mcp.json")
		}

		store := data.NewMCPStore()
		err := store.Export(exportPath)
		if err != nil {
			fmt.Printf("Error exporting MCP configuration: %v\n", err)
			return
		}

		fmt.Printf("MCP configuration exported successfully to: %s\n", exportPath)
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

// mcpSelectCmd (formerly mcpSwitchCmd)
var mcpSelectCmd = &cobra.Command{
	Use:     "select",
	Aliases: []string{"sel", "switch"}, // Keep switch alias for muscle memory but it might conflict if not careful?
	// Cobra handles subcommands. 'switch' is now a sibling command.
	// If both exist, we should probably remove 'switch' alias from here to avoid ambiguity or overwrite.
	// Removing 'switch' alias from 'select' command.
	Short: "Toggle which MCP servers are allowed",
	Long:  `Interactively select which MCP servers should be allowed. Use space to toggle, enter to confirm.`,
	Run: func(cmd *cobra.Command, args []string) {
		store := data.NewMCPStore()
		servers, _, err := store.Load()
		if err != nil {
			fmt.Printf("Error loading MCP config: %v\n", err)
			return
		}

		if len(servers) == 0 {
			fmt.Println("No MCP servers defined.")
			return
		}

		// Sort keys for consistent output
		names := make([]string, 0, len(servers))
		for name := range servers {
			names = append(names, name)
		}

		// Build options and pre-select allowed ones
		var options []huh.Option[string]
		var selected []string
		for _, name := range names {
			server := servers[name]
			label := fmt.Sprintf("%-18s [%s]", name, server.Type)
			options = append(options, huh.NewOption(label, name))
			if server.Allowed {
				selected = append(selected, name)
			}
		}

		// Sort options by name alphabetically and keep selected ones at top
		SortMultiOptions(options, selected)

		err = huh.NewMultiSelect[string]().
			Title("Select MCP servers to allow").
			Description("Use space to toggle, enter to confirm.").
			Options(options...).
			Value(&selected).
			Run()
		if err != nil {
			return // User cancelled
		}

		// Create a set for fast lookup
		allowedSet := make(map[string]bool)
		for _, s := range selected {
			allowedSet[s] = true
		}

		// Update allowed state for each server
		for name, server := range servers {
			server.Allowed = allowedSet[name]
		}

		// Save updated config
		if err := store.Save(servers, selected); err != nil {
			fmt.Printf("Error saving MCP config: %v\n", err)
			return
		}

		fmt.Printf("Updated allowed MCP servers: %v\n", selected)
	},
}

var mcpSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Interactively edit the MCP configuration",
	Long:  `Opens an interactive editor to modify the MCP configuration JSON directly.`,
	Run: func(cmd *cobra.Command, args []string) {
		store := data.NewMCPStore()
		configPath := store.GetPath()

		// Ensure file exists
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			if err := store.CreateTemplate(); err != nil {
				service.Errorf("Error creating MCP configuration file: %v\n", err)
				return
			}
		}

		// Read content
		contentBytes, err := os.ReadFile(configPath)
		if err != nil {
			service.Errorf("Error reading config file: %v", err)
			return
		}
		content := string(contentBytes)

		// Count how many lines are in the content
		lineCount := len(strings.Split(content, "\n"))
		height := 10
		if lineCount > 10 {
			height = lineCount + 5
		}
		if height > 40 {
			height = 40
		}

		// Bugfix:
		// huh.NewText() only support 90 newlines, above that, it cannot add newlines
		// so we must switch to editor to edit that file
		if lineCount >= 80 {
			editor := getPreferredEditor()
			// Open in detected editor
			cmdE := exec.Command(editor, configPath)
			cmdE.Stdin = os.Stdin
			cmdE.Stdout = os.Stdout
			cmdE.Stderr = os.Stderr

			fmt.Printf("Opening in %s...\n", editor)
			if err := cmdE.Run(); err != nil {
				service.Errorf("Editor failed: %v", err)
				return
			}
			// Reload content
			contentBytes, _ := os.ReadFile(configPath)
			content = string(contentBytes)
		} else {
			// Interactive edit
			form := huh.NewForm(
				huh.NewGroup(
					huh.NewText().
						Title("Edit MCP Configuration (JSON)").
						Description("The MCP (Model Context Protocol) enables communication with locally running MCP servers that provide additional tools and resources to extend capabilities.").
						Validate(func(v string) error {
							if v == "" {
								return errors.New("content cannot be empty")
							}
							// Validate JSON
							var js map[string]interface{}
							if err := json.Unmarshal([]byte(v), &js); err != nil {
								return fmt.Errorf("invalid JSON content - %v", err)
							}
							return nil
						}).
						Placeholder("{\"mcpServers\": {}}").
						Value(&content).
						WithHeight(height),
				),
			).WithKeyMap(GetHuhKeyMap())
			err = form.Run()
			if err != nil {
				fmt.Println("Edit cancelled.")
				return
			}
		}

		// Validate JSON
		var js map[string]interface{}
		if err := json.Unmarshal([]byte(content), &js); err != nil {
			service.Errorf("Invalid JSON content - %v", err)
			return
		}

		// Make it pretty
		var prettyJSON bytes.Buffer
		content = strings.TrimSpace(content)
		json.Indent(&prettyJSON, []byte(content), "", "  ")

		// Save content
		if err := os.WriteFile(configPath, prettyJSON.Bytes(), 0644); err != nil {
			service.Errorf("Error saving config file: %v", err)
			return
		}

		fmt.Println("MCP configuration updated successfully.")

		// Reload MCPs
		mcpListCmd.Run(cmd, args)
	},
}

func init() {
	mcpLoadCmd.Flags().BoolP("all", "a", false, "List all mcp servers, including blocked ones")
	mcpLoadCmd.Flags().BoolP("prompts", "p", false, "Include MCP prompts, if available")
	mcpLoadCmd.Flags().BoolP("resources", "r", false, "Include MCP resources, if available")

	mcpCmd.AddCommand(mcpLoadCmd)
	mcpCmd.AddCommand(mcpListCmd)
	mcpCmd.AddCommand(mcpSwitchCmd) // Capability toggle
	mcpCmd.AddCommand(mcpSelectCmd) // Server selection (was switch)
	mcpCmd.AddCommand(mcpExportCmd)
	mcpCmd.AddCommand(mcpImportCmd)
	mcpCmd.AddCommand(mcpPathCmd)
	mcpCmd.AddCommand(mcpSetCmd)

	rootCmd.AddCommand(mcpCmd)
}
