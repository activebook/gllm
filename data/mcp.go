package data

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// MCPServer represents an MCP server configuration with strong typing.
type MCPServer struct {
	Name        string            // Name is the key, derived from JSON map key
	Command     string            // Command to execute (for local/stdio servers)
	Args        []string          // Arguments to pass to the command
	Type        string            // Server type: "sse", "http", "stdio", etc.
	URL         string            // URL for SSE/HTTP servers
	HTTPUrl     string            // HTTP URL for streamable servers
	BaseURL     string            // Base URL for SSE
	Headers     map[string]string // HTTP headers
	Env         map[string]string // Environment variables
	WorkDir     string            // Working directory
	Cwd         string            // Alternative working directory field
	Description string            // Human-readable description
	Allowed     bool              // Whether this server is allowed (derived from allowMCPServers)
}

// mcpConfigFile represents the raw JSON structure of mcp.json
type mcpConfigFile struct {
	MCPServers map[string]mcpServerJSON `json:"mcpServers"`
}

// mcpServerJSON is the raw JSON representation of an MCP server
type mcpServerJSON struct {
	Command     string            `json:"command,omitempty"`
	Args        []string          `json:"args,omitempty"`
	Type        string            `json:"type,omitempty"`
	URL         string            `json:"url,omitempty"`
	HTTPUrl     string            `json:"httpUrl,omitempty"`
	BaseURL     string            `json:"baseUrl,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	WorkDir     string            `json:"working_directory,omitempty"`
	Cwd         string            `json:"cwd,omitempty"`
	Name        string            `json:"name,omitempty"`
	Description string            `json:"description,omitempty"`
}

// MCPStore provides typed access to mcp.json configuration.
type MCPStore struct {
	path string
}

// NewMCPStore creates a new MCPStore with the default path.
func NewMCPStore() *MCPStore {
	return &MCPStore{
		path: GetMcpFilePath(),
	}
}

// GetPath returns the path to the MCP configuration file.
func (m *MCPStore) GetPath() string {
	return m.path
}

// Load reads all MCP server configurations.
// Returns servers with Allowed status from settings.json.
func (m *MCPStore) Load() (map[string]*MCPServer, error) {
	if _, err := os.Stat(m.path); os.IsNotExist(err) {
		return make(map[string]*MCPServer), nil
	}

	data, err := os.ReadFile(m.path)
	if err != nil {
		return nil, fmt.Errorf("failed to read MCP config: %w", err)
	}

	var config mcpConfigFile
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse MCP config: %w", err)
	}

	// Get allowed list from settings
	settingsStore := GetSettingsStore()
	allowedList := settingsStore.GetAllowedMCPServers()
	allowedSet := make(map[string]bool)
	for _, name := range allowedList {
		allowedSet[name] = true
	}

	// Convert to strongly-typed MCPServer structs
	servers := make(map[string]*MCPServer)
	for name, raw := range config.MCPServers {
		if raw.Type == "" {
			raw.Type = "stdio"
		}
		servers[name] = &MCPServer{
			Name:        name,
			Command:     raw.Command,
			Args:        raw.Args,
			Type:        raw.Type,
			URL:         raw.URL,
			HTTPUrl:     raw.HTTPUrl,
			BaseURL:     raw.BaseURL,
			Headers:     raw.Headers,
			Env:         raw.Env,
			WorkDir:     raw.WorkDir,
			Cwd:         raw.Cwd,
			Description: raw.Description,
			Allowed:     allowedSet[name],
		}
	}

	return servers, nil
}

// GetServer returns a specific MCP server by name.
func (m *MCPStore) GetServer(name string) (*MCPServer, error) {
	servers, err := m.Load()
	if err != nil {
		return nil, err
	}

	server, exists := servers[name]
	if !exists {
		return nil, fmt.Errorf("MCP server '%s' not found", name)
	}

	return server, nil
}

// CreateEmptyConfigFile creates an empty MCP config file.
func (m *MCPStore) CreateTemplate() error {
	if err := os.MkdirAll(filepath.Dir(m.path), 0755); err != nil {
		return fmt.Errorf("failed to create MCP config directory: %w", err)
	}
	// Write a template
	templateConfig := &mcpConfigFile{
		MCPServers: make(map[string]mcpServerJSON),
	}
	data, err := json.MarshalIndent(templateConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal MCP config: %w", err)
	}
	return os.WriteFile(m.path, data, 0644)
}

// Save writes all MCP server configurations to disk (without allowed list).
func (m *MCPStore) Save(servers map[string]*MCPServer) error {
	if err := os.MkdirAll(filepath.Dir(m.path), 0755); err != nil {
		return fmt.Errorf("failed to create MCP config directory: %w", err)
	}

	// Convert to JSON structure
	config := mcpConfigFile{
		MCPServers: make(map[string]mcpServerJSON),
	}

	for name, server := range servers {
		config.MCPServers[name] = mcpServerJSON{
			Command:     server.Command,
			Args:        server.Args,
			Type:        server.Type,
			URL:         server.URL,
			HTTPUrl:     server.HTTPUrl,
			BaseURL:     server.BaseURL,
			Headers:     server.Headers,
			Env:         server.Env,
			WorkDir:     server.WorkDir,
			Cwd:         server.Cwd,
			Name:        server.Name,
			Description: server.Description,
		}
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal MCP config: %w", err)
	}

	return os.WriteFile(m.path, data, 0644)
}

// Export exports all MCP server configurations to a JSON file.
func (m *MCPStore) Export(path string) error {
	servers, err := m.Load()
	if err != nil {
		return err
	}
	return m.SaveToPath(servers, path)
}

// Import imports MCP server configurations from a JSON file.
func (m *MCPStore) Import(path string) error {
	servers, err := m.LoadFromPath(path)
	if err != nil {
		return err
	}
	return m.Save(servers)
}

// AddServer adds a new MCP server. Returns error if it already exists.
func (m *MCPStore) AddServer(server *MCPServer) error {
	servers, err := m.Load()
	if err != nil {
		return err
	}

	if _, exists := servers[server.Name]; exists {
		return fmt.Errorf("MCP server '%s' already exists", server.Name)
	}

	servers[server.Name] = server

	// Update settings if Allowed is true
	if server.Allowed {
		settingsStore := GetSettingsStore()
		if err := settingsStore.AllowMCPServer(server.Name); err != nil {
			return err
		}
	}

	return m.Save(servers)
}

// UpdateServer updates an existing MCP server.
func (m *MCPStore) UpdateServer(server *MCPServer) error {
	servers, err := m.Load()
	if err != nil {
		return err
	}

	if _, exists := servers[server.Name]; !exists {
		return fmt.Errorf("MCP server '%s' not found", server.Name)
	}

	servers[server.Name] = server

	// Update allowed list in settings based on Allowed flag
	settingsStore := GetSettingsStore()
	if server.Allowed {
		if err := settingsStore.AllowMCPServer(server.Name); err != nil {
			return err
		}
	} else {
		if err := settingsStore.BlockMCPServer(server.Name); err != nil {
			return err
		}
	}

	return m.Save(servers)
}

// RemoveServer removes an MCP server.
func (m *MCPStore) RemoveServer(name string) error {
	servers, err := m.Load()
	if err != nil {
		return err
	}

	if _, exists := servers[name]; !exists {
		return fmt.Errorf("MCP server '%s' not found", name)
	}

	delete(servers, name)

	// Remove from settings allowed list
	settingsStore := GetSettingsStore()
	if err := settingsStore.BlockMCPServer(name); err != nil {
		return err
	}

	return m.Save(servers)
}

// SaveToPath writes MCP configuration to a specific path (for export).
func (m *MCPStore) SaveToPath(servers map[string]*MCPServer, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	config := mcpConfigFile{
		MCPServers: make(map[string]mcpServerJSON),
	}

	for name, server := range servers {
		config.MCPServers[name] = mcpServerJSON{
			Command:     server.Command,
			Args:        server.Args,
			Type:        server.Type,
			URL:         server.URL,
			HTTPUrl:     server.HTTPUrl,
			BaseURL:     server.BaseURL,
			Headers:     server.Headers,
			Env:         server.Env,
			WorkDir:     server.WorkDir,
			Cwd:         server.Cwd,
			Name:        server.Name,
			Description: server.Description,
		}
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// LoadFromPath reads MCP configuration from a specific path (for import).
func (m *MCPStore) LoadFromPath(path string) (map[string]*MCPServer, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("file not found: %s", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config mcpConfigFile
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// Get allowed list from settings (ignore what's in the file)
	settingsStore := GetSettingsStore()
	allowedList := settingsStore.GetAllowedMCPServers()
	allowedSet := make(map[string]bool)
	for _, name := range allowedList {
		allowedSet[name] = true
	}

	servers := make(map[string]*MCPServer)
	for name, raw := range config.MCPServers {
		if raw.Type == "" {
			raw.Type = "stdio"
		}
		servers[name] = &MCPServer{
			Name:        name,
			Command:     raw.Command,
			Args:        raw.Args,
			Type:        raw.Type,
			URL:         raw.URL,
			HTTPUrl:     raw.HTTPUrl,
			BaseURL:     raw.BaseURL,
			Headers:     raw.Headers,
			Env:         raw.Env,
			WorkDir:     raw.WorkDir,
			Cwd:         raw.Cwd,
			Description: raw.Description,
			Allowed:     allowedSet[name],
		}
	}

	return servers, nil
}
