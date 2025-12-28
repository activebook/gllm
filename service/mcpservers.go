package service

import (
	"github.com/activebook/gllm/data"
)

// MCPServerConfig uses data.MCPServer for configuration.
// Note: The runtime MCPServer struct is defined in service/mcp.go and is different.

// MCPServerConfig is DEPRECATED. Use MCPServer instead.
// Kept for backward compatibility.
type MCPServerConfig struct {
	Command     string            `json:"command,omitempty"`
	Args        []string          `json:"args,omitempty"`
	Type        string            `json:"type,omitempty"`
	Url         string            `json:"url,omitempty"`
	HttpUrl     string            `json:"httpUrl,omitempty"`
	BaseUrl     string            `json:"baseUrl,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	WorkDir     string            `json:"working_directory,omitempty"`
	Cwd         string            `json:"cwd,omitempty"`
	Name        string            `json:"name,omitempty"`
	Description string            `json:"description,omitempty"`
	Allowed     bool
}

// MCPConfig represents the overall MCP configuration.
// DEPRECATED: Use data.MCPStore methods directly.
type MCPConfig struct {
	MCPServers      map[string]MCPServerConfig `json:"mcpServers"`
	AllowMCPServers []string                   `json:"allowMCPServers"`
}

// GetMCPServersPath returns the path to the MCP servers configuration file.
func GetMCPServersPath() string {
	store := data.NewMCPStore()
	return store.GetPath()
}

// LoadMCPServers reads the MCP configuration.
// Returns legacy MCPConfig for backward compatibility.
func LoadMCPServers() (*MCPConfig, error) {
	store := data.NewMCPStore()
	servers, allowed, err := store.Load()
	if err != nil {
		return nil, err
	}

	if len(servers) == 0 {
		return nil, nil
	}

	return toMCPConfig(servers, allowed), nil
}

// SaveMCPServers writes the MCP configuration.
func SaveMCPServers(config *MCPConfig) error {
	store := data.NewMCPStore()
	servers, allowed := fromMCPConfig(config)
	return store.Save(servers, allowed)
}

// SaveMCPServersToPath writes the MCP configuration to a specific path.
func SaveMCPServersToPath(config *MCPConfig, path string) error {
	store := data.NewMCPStore()
	servers, allowed := fromMCPConfig(config)
	return store.SaveToPath(servers, allowed, path)
}

// LoadMCPServersFromPath reads the MCP configuration from a specific path.
func LoadMCPServersFromPath(path string) (*MCPConfig, error) {
	store := data.NewMCPStore()
	servers, allowed, err := store.LoadFromPath(path)
	if err != nil {
		return nil, err
	}

	if len(servers) == 0 {
		return nil, nil
	}

	return toMCPConfig(servers, allowed), nil
}

// toMCPConfig converts data.MCPServer map to legacy MCPConfig.
func toMCPConfig(servers map[string]*data.MCPServer, allowed []string) *MCPConfig {
	config := &MCPConfig{
		MCPServers:      make(map[string]MCPServerConfig),
		AllowMCPServers: allowed,
	}

	for name, server := range servers {
		config.MCPServers[name] = MCPServerConfig{
			Command:     server.Command,
			Args:        server.Args,
			Type:        server.Type,
			Url:         server.URL,
			HttpUrl:     server.HTTPUrl,
			BaseUrl:     server.BaseURL,
			Headers:     server.Headers,
			Env:         server.Env,
			WorkDir:     server.WorkDir,
			Cwd:         server.Cwd,
			Name:        server.Name,
			Description: server.Description,
			Allowed:     server.Allowed,
		}
	}

	return config
}

// fromMCPConfig converts legacy MCPConfig to data.MCPServer map.
func fromMCPConfig(config *MCPConfig) (map[string]*data.MCPServer, []string) {
	if config == nil {
		return nil, nil
	}

	servers := make(map[string]*data.MCPServer)
	for name, sc := range config.MCPServers {
		servers[name] = &data.MCPServer{
			Name:        name,
			Command:     sc.Command,
			Args:        sc.Args,
			Type:        sc.Type,
			URL:         sc.Url,
			HTTPUrl:     sc.HttpUrl,
			BaseURL:     sc.BaseUrl,
			Headers:     sc.Headers,
			Env:         sc.Env,
			WorkDir:     sc.WorkDir,
			Cwd:         sc.Cwd,
			Description: sc.Description,
			Allowed:     sc.Allowed,
		}
	}

	return servers, config.AllowMCPServers
}
