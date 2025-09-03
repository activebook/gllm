package service

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
)

// MCPServerConfig represents the configuration for an MCP server
type MCPServerConfig struct {
	Command     string            `json:"command,omitempty"` // Command to execute
	Args        []string          `json:"args,omitempty"`    // Arguments to pass to the command
	Type        string            `json:"type,omitempty"`    // sse or http
	Url         string            `json:"url,omitempty"`     // URL for SSE
	HttpUrl     string            `json:"httpUrl,omitempty"` // URL for HTTP(Streamable)
	BaseUrl     string            `json:"baseUrl,omitempty"` // URL for SSE
	Headers     map[string]string `json:"headers,omitempty"` // Headers for SSE/HTTP
	Env         map[string]string `json:"env,omitempty"`     // Environment variables for the command
	WorkDir     string            `json:"working_directory,omitempty"`
	Cwd         string            `json:"cwd,omitempty"`
	Name        string            `json:"name,omitempty"`
	Description string            `json:"description,omitempty"`
	Alllowed    bool              // whether to allow MCP servers
}

// MCPConfig represents the overall MCP configuration
type MCPConfig struct {
	MCPServers      map[string]MCPServerConfig `json:"mcpServers"`      // name:server
	AllowMCPServers []string                   `json:"allowMCPServers"` // whether to allow MCP servers
}

// LoadMCPServers reads the MCP configuration from the specified JSON file
// and initializes the MCP client with the configured servers
func LoadMCPServers() (*MCPConfig, error) {
	// Get the path to the MCP servers configuration file
	configPath := getMCPServersPath()

	// Check if the configuration file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, nil
	}

	// Read the configuration file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	// Unmarshal the JSON data into the MCPConfig struct
	var config MCPConfig
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	// Create a set of allowed servers for quick lookup
	for _, s := range config.AllowMCPServers {
		if s != "" {
			if server, exists := config.MCPServers[s]; exists {
				server.Alllowed = true
				config.MCPServers[s] = server
			}
		}
	}

	return &config, nil
}

func getMCPServersPath() string {
	var err error
	// Prefer os.UserConfigDir()
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		// Fallback to home directory if UserConfigDir fails
		userConfigDir, _ = homedir.Dir()
	}

	// App specific directory: e.g., ~/.config/gllm or ~/Library/Application Support/gllm
	appConfigDir := filepath.Join(userConfigDir, "gllm")

	// Default config file path: e.g., ~/.config/gllm/.mcp.json
	return filepath.Join(appConfigDir, "mcp.json")
}
