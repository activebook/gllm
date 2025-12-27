// File: service/agents.go
package service

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/viper"
)

// WriteConfig saves the current viper configuration to the determined config file path.
// It handles creation of the directory if needed.
func WriteConfig() error {
	// Get the path where viper is currently configured to write
	// If --config flag was used, it respects that. Otherwise, uses the default path.
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		// If no config file was used (e.g., it didn't exist), use the default path
		configFile = getDefaultConfigFilePath()
		// We need to explicitly tell Viper to write to this file
		viper.SetConfigFile(configFile)
	}

	// Ensure the directory exists
	configDir := filepath.Dir(configFile)
	if err := os.MkdirAll(configDir, 0750); err != nil { // Use 0750 for permissions
		return fmt.Errorf("failed to create config directory '%s': %w", configDir, err)
	}

	// Write the config file
	// Use WriteConfigAs to ensure it writes even if the file doesn't exist yet
	if err := viper.WriteConfigAs(configFile); err != nil {
		return fmt.Errorf("failed to write configuration file '%s': %w", configFile, err)
	}

	return nil
}

// getDefaultConfigFilePath returns the default config file path
func getDefaultConfigFilePath() string {
	// This is a simplified version - in the real implementation this would
	// be more complex to match the cmd package logic
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		homeDir, _ := os.UserHomeDir()
		userConfigDir = filepath.Join(homeDir, ".config")
	}
	return filepath.Join(userConfigDir, "gllm", "gllm.yaml")
}

// AgentConfig represents the configuration for an agent
type AgentConfig map[string]interface{}

// GetAllAgents returns all configured agents
func GetAllAgents() (map[string]AgentConfig, error) {
	agentsMap := viper.GetStringMap("agents")
	if agentsMap == nil {
		return nil, fmt.Errorf("no agents found")
	}

	agents := make(map[string]AgentConfig)
	for name, config := range agentsMap {
		if configMap, ok := config.(map[string]interface{}); ok {
			agents[name] = configMap
		} else if configMap, ok := config.(AgentConfig); ok {
			agents[name] = configMap
		} else if configMap, ok := config.(map[interface{}]interface{}); ok {
			// Handle map[interface{}]interface{} if viper returns it
			converted := make(map[string]interface{})
			for k, v := range configMap {
				converted[fmt.Sprint(k)] = v
			}
			agents[name] = converted
		}
	}
	return agents, nil
}

// GetAgent returns a specific agent configuration
func GetAgent(name string) (AgentConfig, error) {
	agentsMap := viper.GetStringMap("agents")
	if agentsMap == nil {
		return nil, fmt.Errorf("no agents found")
	}

	if config, exists := agentsMap[name]; exists {
		if configMap, ok := config.(map[string]interface{}); ok {
			return configMap, nil
		}
		if configMap, ok := config.(AgentConfig); ok {
			return configMap, nil
		}
		if configMap, ok := config.(map[interface{}]interface{}); ok {
			converted := make(map[string]interface{})
			for k, v := range configMap {
				converted[fmt.Sprint(k)] = v
			}
			return converted, nil
		}

		return nil, fmt.Errorf("invalid agent configuration for '%s' (type %T)", name, config)
	}
	return nil, fmt.Errorf("agent named '%s' not found", name)
}

// AddAgent adds a new agent with the current configuration
func AddAgent(name string) error {
	// Get current agent configuration
	currentConfig := getCurrentAgentConfig()
	return AddAgentWithConfig(name, currentConfig)
}

// AddAgentWithConfig adds a new agent with the specified configuration
func AddAgentWithConfig(name string, config AgentConfig) error {
	// Get existing agents map
	agentsMap := viper.GetStringMap("agents")
	if agentsMap == nil {
		agentsMap = make(map[string]interface{})
	}

	// Check if agent already exists
	if _, exists := agentsMap[name]; exists {
		return fmt.Errorf("agent named '%s' already exists", name)
	}

	// Add the new agent
	agentsMap[name] = config
	viper.Set("agents", agentsMap)

	// Write the config file
	if err := WriteConfig(); err != nil {
		return fmt.Errorf("failed to save agent: %w", err)
	}

	return nil
}

// SetAgent updates an existing agent configuration
func SetAgent(name string, config AgentConfig) error {
	// Get existing agents map
	agentsMap := viper.GetStringMap("agents")
	if agentsMap == nil {
		return fmt.Errorf("no agents found")
	}

	// Check if agent exists
	if _, exists := agentsMap[name]; !exists {
		return fmt.Errorf("agent named '%s' not found", name)
	}

	// Update the agent
	agentsMap[name] = config
	viper.Set("agents", agentsMap)

	// Write the config file
	if err := WriteConfig(); err != nil {
		return fmt.Errorf("failed to save agent: %w", err)
	}

	return nil
}

// RemoveAgent removes an agent
func RemoveAgent(name string) error {
	agentsMap := viper.GetStringMap("agents")
	if agentsMap == nil {
		return fmt.Errorf("no agents found")
	}

	if _, exists := agentsMap[name]; !exists {
		return fmt.Errorf("agent named '%s' not found", name)
	}

	// Delete the agent
	delete(agentsMap, name)
	viper.Set("agents", agentsMap)

	// Write the config file
	if err := WriteConfig(); err != nil {
		return fmt.Errorf("failed to save configuration after removing agent: %w", err)
	}

	return nil
}

// RenameAgent renames an existing agent
func RenameAgent(oldName, newName string) error {
	agentsMap := viper.GetStringMap("agents")
	if agentsMap == nil {
		return fmt.Errorf("no agents found")
	}

	// Check if old agent exists
	if _, exists := agentsMap[oldName]; !exists {
		return fmt.Errorf("agent named '%s' not found", oldName)
	}

	// Check if new name already exists
	if _, exists := agentsMap[newName]; exists {
		return fmt.Errorf("agent named '%s' already exists", newName)
	}

	// Check for reserved names
	if newName == "current" || newName == "active" {
		return fmt.Errorf("'%s' is a reserved name", newName)
	}

	// Get the agent configuration
	agentConfig, ok := agentsMap[oldName].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid agent configuration for '%s'", oldName)
	}

	// Remove old agent and add with new name
	delete(agentsMap, oldName)
	agentsMap[newName] = agentConfig
	viper.Set("agents", agentsMap)

	// Write the config file
	if err := WriteConfig(); err != nil {
		return fmt.Errorf("failed to save configuration after renaming agent: %w", err)
	}

	return nil
}

// SwitchToAgent switches to the specified agent by setting it as the active agent
func SwitchToAgent(name string) error {
	// Verify agent exists
	_, err := GetAgent(name)
	if err != nil {
		return err
	}

	// Set the active agent name
	viper.Set("agent", name)

	// Write the config file
	if err := WriteConfig(); err != nil {
		return fmt.Errorf("failed to switch to agent: %w", err)
	}

	return nil
}

// getCurrentAgentConfig returns the current agent configuration
// It resolves the configuration from the active agent name
func getCurrentAgentConfig() AgentConfig {
	// Check if "agent" is a string (new format)
	agentVal := viper.Get("agent")

	if agentName, ok := agentVal.(string); ok && agentName != "" {
		// New format: reference to an agent
		if config, err := GetAgent(agentName); err == nil {
			// Add the name to the config for display purposes
			config["name"] = agentName
			// Ensure we have resolved reference values locally for this session if needed
			// But for a pure config object, we return as is. The consuming code might need resolution.
			// We already resolve system_prompt and template in other places (GetEffectiveSystemPrompt and GetEffectiveTemplate)
			// return resolveAgentConfig(config)
			return AgentConfig(config)
		}
		// If agent not found, fall back to empty or default
	} else if agentMap, ok := agentVal.(map[string]interface{}); ok {
		// Legacy format: map in "agent"
		// We should probably convert this to map[string]interface{} (AgentConfig)
		return AgentConfig(agentMap)
	}

	// If using viper.GetStringMap("agent") directly for legacy support
	legacyMap := viper.GetStringMap("agent")
	if len(legacyMap) > 0 {
		return AgentConfig(legacyMap)
	}

	return make(AgentConfig)
}

// resolveAgentConfig resolves lazy references in the configuration
// legacy
func resolveAgentConfig(config AgentConfig) AgentConfig {
	resolved := make(AgentConfig)
	for k, v := range config {
		resolved[k] = v
	}

	// Resolve template
	if t, ok := resolved["template"].(string); ok {
		resolved["template"] = ResolveTemplateReference(t)
	}

	// Resolve system prompt
	if s, ok := resolved["system_prompt"].(string); ok {
		resolved["system_prompt"] = ResolveSystemPromptReference(s)
	}

	return resolved
}

// MigrateCurrentConfigToDefaultAgent migrates the current agent config to a "default" agent
func MigrateCurrentConfigToDefaultAgent() error {
	agentsMap := viper.GetStringMap("agents")
	if agentsMap == nil {
		agentsMap = make(map[string]interface{})
	}

	// Check if "agent" is already a string (already migrated or new format)
	if _, ok := viper.Get("agent").(string); ok {
		return nil
	}

	// Check if default agent already exists
	if _, exists := agentsMap["default"]; exists {
		// If default exists, just switch to it if we are currently using legacy map
		if _, ok := viper.Get("agent").(map[string]interface{}); ok {
			viper.Set("agent", "default")
			WriteConfig()
		}
		return nil
	}

	// Get current agent configuration
	// Note: getCurrentAgentConfig handles legacy map reading
	currentConfig := getCurrentAgentConfig()
	if len(currentConfig) == 0 {
		return nil // No config to migrate
	}

	// Add as default agent
	agentsMap["default"] = currentConfig
	viper.Set("agents", agentsMap)

	// Set active agent to default
	viper.Set("agent", "default")

	// Write the config file
	if err := WriteConfig(); err != nil {
		return fmt.Errorf("failed to migrate current config to default agent: %w", err)
	}

	return nil
}

// GetAgentNames returns a sorted list of agent names
func GetAgentNames() ([]string, error) {
	agentsMap := viper.GetStringMap("agents")
	if agentsMap == nil {
		return nil, fmt.Errorf("no agents found")
	}

	names := make([]string, 0, len(agentsMap))
	for name := range agentsMap {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// GetCurrentAgentName returns the name of the currently active agent
func GetCurrentAgentName() string {
	agentVal := viper.Get("agent")
	if name, ok := agentVal.(string); ok {
		return name
	}
	return "unknown"
}

// GetCurrentAgentConfig returns the current agent configuration
// This is the public version that calls the internal one
func GetCurrentAgentConfig() AgentConfig {
	return getCurrentAgentConfig()
}

// ResolveTemplateReference resolves a template reference to actual content lazily
// If the template contains spaces/tabs/newlines, treat as plain text
// Otherwise, try to resolve as a reference to a named template
func ResolveTemplateReference(template string) string {
	if template == "" {
		return ""
	}

	// Check if it's a reference (no spaces/tabs/newlines)
	if !strings.ContainsAny(template, " \t\n") {
		templates := viper.GetStringMapString("templates")
		if templateContent, exists := templates[template]; exists {
			return templateContent // Use resolved content
		}
	}
	return template // Use as plain text
}

// ResolveSystemPromptReference resolves a system prompt reference to actual content lazily
// If the system prompt contains spaces/tabs/newlines, treat as plain text
// Otherwise, try to resolve as a reference to a named system prompt
func ResolveSystemPromptReference(sysPrompt string) string {
	if sysPrompt == "" {
		return ""
	}

	// Check if it's a reference (no spaces/tabs/newlines)
	if !strings.ContainsAny(sysPrompt, " \t\n") {
		sysPrompts := viper.GetStringMapString("system_prompts")
		if sysPromptContent, exists := sysPrompts[sysPrompt]; exists {
			return sysPromptContent // Use resolved content
		}
	}
	return sysPrompt // Use as plain text
}
