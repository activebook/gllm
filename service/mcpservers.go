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
	Allowed     bool              // whether to allow MCP servers
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
	configPath := GetMCPServersPath()

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
				// If the server exists in the AllowMCPServers list,
				// Mark the server as allowed
				server.Allowed = true
				config.MCPServers[s] = server
			} else if server.Allowed {
				// If the server does not exist in the AllowMCPServers list,
				// Mark the server as not allowed
				server.Allowed = false
				config.MCPServers[s] = server
			}
		}
	}

	return &config, nil
}

// SaveMCPServers writes the MCP configuration to the specified JSON file
func SaveMCPServers(config *MCPConfig) error {
	// Get the path to the MCP servers configuration file
	configPath := GetMCPServersPath()

	// Ensure the directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Marshal the config to JSON
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	// Write to file
	return os.WriteFile(configPath, data, 0644)
}

// SaveMCPServersToPath writes the MCP configuration to a specific path
func SaveMCPServersToPath(config *MCPConfig, path string) error {
	// Ensure the directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Marshal the config to JSON
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	// Write to file
	return os.WriteFile(path, data, 0644)
}

// LoadMCPServersFromPath reads the MCP configuration from a specific path
func LoadMCPServersFromPath(path string) (*MCPConfig, error) {
	// Check if the configuration file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}

	// Read the configuration file
	data, err := os.ReadFile(path)
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
				// If the server exists in the AllowMCPServers list,
				// Mark the server as allowed
				server.Allowed = true
				config.MCPServers[s] = server
			} else if server.Allowed {
				// If the server does not exist in the AllowMCPServers list,
				// Mark the server as not allowed
				server.Allowed = false
				config.MCPServers[s] = server
			}
		}
	}

	return &config, nil
}

// GetMCPServersPath returns the path to the MCP servers configuration file
func GetMCPServersPath() string {
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
