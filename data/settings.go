// Package data provides settings management via settings.json.
// This file handles user-level settings separate from gllm.yaml configuration.
package data

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// SkillsSettings holds skill-related settings.
type SkillsSettings struct {
	Disabled []string `json:"disabled"`
}

// MCPSettings holds MCP-related settings.
type MCPSettings struct {
	Allowed []string `json:"allowed"`
}

// Settings represents the structure of settings.json.
type Settings struct {
	MCP    MCPSettings    `json:"mcp"`
	Skills SkillsSettings `json:"skills"`
}

// SettingsStore provides access to settings.json.
type SettingsStore struct {
	path     string
	settings Settings
	mu       sync.RWMutex
}

var (
	settingsStoreInstance *SettingsStore
	settingsStoreOnce     sync.Once
)

// GetSettingsStore returns the singleton instance of SettingsStore.
func GetSettingsStore() *SettingsStore {
	settingsStoreOnce.Do(func() {
		settingsStoreInstance = NewSettingsStore()
		_ = settingsStoreInstance.Load() // Best effort load
	})
	return settingsStoreInstance
}

// NewSettingsStore creates a new SettingsStore.
func NewSettingsStore() *SettingsStore {
	return &SettingsStore{
		path: GetSettingsFilePath(),
		settings: Settings{
			Skills: SkillsSettings{
				Disabled: []string{},
			},
			MCP: MCPSettings{
				Allowed: []string{},
			},
		},
	}
}

// Load reads settings from disk.
func (s *SettingsStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, use defaults
			return nil
		}
		return fmt.Errorf("failed to read settings file: %w", err)
	}

	if err := json.Unmarshal(data, &s.settings); err != nil {
		return fmt.Errorf("failed to parse settings file: %w", err)
	}

	return nil
}

// Save writes settings to disk.
func (s *SettingsStore) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(s.path), 0750); err != nil {
		return fmt.Errorf("failed to create settings directory: %w", err)
	}

	data, err := json.MarshalIndent(s.settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	if err := os.WriteFile(s.path, data, 0644); err != nil {
		return fmt.Errorf("failed to write settings file: %w", err)
	}

	return nil
}

// GetDisabledSkills returns the list of disabled skill names.
func (s *SettingsStore) GetDisabledSkills() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings.Skills.Disabled
}

// IsSkillDisabled checks if a skill is in the disabled list.
func (s *SettingsStore) IsSkillDisabled(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, d := range s.settings.Skills.Disabled {
		if d == name {
			return true
		}
	}
	return false
}

// DisableSkill adds a skill to the disabled list.
func (s *SettingsStore) DisableSkill(name string) error {
	s.mu.Lock()
	// Check if already disabled
	for _, d := range s.settings.Skills.Disabled {
		if d == name {
			s.mu.Unlock()
			return nil // Already disabled
		}
	}
	s.settings.Skills.Disabled = append(s.settings.Skills.Disabled, name)
	s.mu.Unlock()
	return s.Save()
}

// EnableSkill removes a skill from the disabled list.
func (s *SettingsStore) EnableSkill(name string) error {
	s.mu.Lock()
	newDisabled := make([]string, 0, len(s.settings.Skills.Disabled))
	for _, d := range s.settings.Skills.Disabled {
		if d != name {
			newDisabled = append(newDisabled, d)
		}
	}
	s.settings.Skills.Disabled = newDisabled
	s.mu.Unlock()
	return s.Save()
}

// GetAllowedMCPServers returns the list of allowed MCP server names.
func (s *SettingsStore) GetAllowedMCPServers() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings.MCP.Allowed
}

// IsMCPServerAllowed checks if an MCP server is in the allowed list.
func (s *SettingsStore) IsMCPServerAllowed(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, a := range s.settings.MCP.Allowed {
		if a == name {
			return true
		}
	}
	return false
}

// AllowMCPServer adds an MCP server to the allowed list.
func (s *SettingsStore) AllowMCPServer(name string) error {
	s.mu.Lock()
	// Check if already allowed
	for _, a := range s.settings.MCP.Allowed {
		if a == name {
			s.mu.Unlock()
			return nil // Already allowed
		}
	}
	s.settings.MCP.Allowed = append(s.settings.MCP.Allowed, name)
	s.mu.Unlock()
	return s.Save()
}

// BlockMCPServer removes an MCP server from the allowed list.
func (s *SettingsStore) BlockMCPServer(name string) error {
	s.mu.Lock()
	newAllowed := make([]string, 0, len(s.settings.MCP.Allowed))
	for _, a := range s.settings.MCP.Allowed {
		if a != name {
			newAllowed = append(newAllowed, a)
		}
	}
	s.settings.MCP.Allowed = newAllowed
	s.mu.Unlock()
	return s.Save()
}

// SetAllowedMCPServers sets the entire list of allowed MCP servers.
func (s *SettingsStore) SetAllowedMCPServers(allowed []string) error {
	s.mu.Lock()
	s.settings.MCP.Allowed = allowed
	s.mu.Unlock()
	return s.Save()
}
