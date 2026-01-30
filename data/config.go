package data

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/viper"
)

// AgentConfig represents a fully-typed agent configuration.
// All fields are strongly typed - no interface{} leaks to other layers.
// Named AgentConfig to avoid conflict with the runtime Agent struct in service/agent.go.
type AgentConfig struct {
	Name          string   // Name is the key in the agents map, not stored in YAML
	Model         Model    // Model name reference
	Tools         []string // List of enabled tools
	Capabilities  []string // List of enabled capabilities (mcp, skills, usage, markdown, subagents)
	Think         string   // Thinking level: off, low, medium, high
	Template      string   // Template reference
	SystemPrompt  string   // System prompt reference
	MaxRecursions int      // Maximum tool call recursions
}

// Model represents a model definition.
type Model struct {
	Name     string  // Name is the key, not stored in YAML
	Provider string  // Provider name (e.g., "openai", "gemini")
	Endpoint string  // Model endpoint
	Key      string  // Model key
	Model    string  // Model name
	Temp     float32 // Model temperature
	TopP     float32 // Model top_p
	Seed     *int32  // Model seed
}

// SearchEngine represents search engine configuration.
type SearchEngine struct {
	Name      string            // Name is the key
	DeepDive  int               // Number of deep dive search results
	Reference int               // Number of reference search results
	Config    map[string]string // Additional configuration
}

// ConfigStore provides typed access to gllm.yaml configuration.
// It wraps viper internally and exposes only typed interfaces.
type ConfigStore struct {
	v *viper.Viper
}

// NewConfigStore creates a new ConfigStore using the existing viper configuration.
// This reuses whatever config file viper has already loaded.
func NewConfigStore() *ConfigStore {
	/*
	 * Viper merges configuration from various sources,
	 * many of which are either case insensitive or use different casing than other sources.
	 * In order to provide the best experience when using multiple sources,
	 * all keys are made case insensitive GitHub -
	 * which means they're internally lowercased.
	 */
	/*
	 * Solution:
	 * For keys that are case sensitive, we use ToLower to normalize them.
	 * Set and Get Keys.
	 * Or: the ideal pattern is to store the "canonical ID" (lowercased) as the key,
	 * and store a separate DisplayName field inside the configuration value.
	 *
	 * The ToLower call acts as a normalization gate.
	 * Without it, the Go map's inherent case-sensitivity would "leak" into the configuration persistence layer,
	 * breaking the abstraction of case-insensitivity that the rest of the system relies on.
	 * It is not just about the final YAML file; it is about ensuring that the Go runtime’s
	 * map lookup remains synchronized with Viper's configuration logic.
	 */
	return &ConfigStore{v: viper.GetViper()}
}

// GetActiveAgentName returns the name of the currently active agent.
func (c *ConfigStore) GetActiveAgentName() string {
	agentVal := c.v.Get("agent")
	if name, ok := agentVal.(string); ok {
		return name
	}
	return ""
}

// Export saves the current configuration to the specified path.
func (c *ConfigStore) Export(path string) error {
	// Create a new viper instance for export to avoid changing the current config file path
	exportViper := viper.New()

	// Copy all settings
	settings := c.v.AllSettings()
	for k, v := range settings {
		exportViper.Set(k, v)
	}

	exportViper.SetConfigFile(path)
	return exportViper.WriteConfig()
}

// Import loads configuration from the specified path and merges it into the current configuration.
func (c *ConfigStore) Import(path string) error {
	importViper := viper.New()
	importViper.SetConfigFile(path)

	if err := importViper.ReadInConfig(); err != nil {
		return err
	}

	// Merge settings
	settings := importViper.AllSettings()
	for k, v := range settings {
		c.v.Set(k, v)
	}

	return c.Save()
}

// GetEditor returns the configured editor.
func (c *ConfigStore) GetEditor() string {
	return c.v.GetString("chat.editor")
}

// SetEditor sets the configured editor.
func (c *ConfigStore) SetEditor(editor string) error {
	c.v.Set("chat.editor", editor)
	return c.Save()
}

// SetConfigFile sets the configuration file path.
func (c *ConfigStore) SetConfigFile(path string) error {
	c.v.SetConfigFile(path) // Set the config file path
	c.v.AutomaticEnv()      // Read in environment variables that match

	// Set default log settings in Viper *before* reading the config
	// This ensures these keys exist even if not in the file
	c.v.SetDefault("log.level", "info")

	// If a config file is found, read it in.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	if err := c.v.ReadInConfig(); err != nil {
		return err
	}
	return nil
}

// ConfigFileUsed returns the path to the config file being used.
func (c *ConfigStore) ConfigFileUsed() string {
	// Return the path to the config file being used
	return c.v.ConfigFileUsed()
}

// GetAgent returns a specific agent configuration by name.
// Returns nil if agent doesn't exist.
func (c *ConfigStore) GetAgent(name string) *AgentConfig {
	name = strings.ToLower(name)
	agentsMap := c.v.GetStringMap("agents")
	if agentsMap == nil {
		return nil
	}

	config, exists := agentsMap[name]
	if !exists {
		return nil
	}

	return c.parseAgentConfig(name, config)
}

// GetAllAgents returns all configured agents as a map.
func (c *ConfigStore) GetAllAgents() map[string]*AgentConfig {
	agentsMap := c.v.GetStringMap("agents")
	if agentsMap == nil {
		return nil
	}

	result := make(map[string]*AgentConfig)
	for name, config := range agentsMap {
		agent := c.parseAgentConfig(name, config)
		if agent == nil {
			continue // Skip invalid agents
		}
		result[name] = agent
	}
	return result
}

// GetAgentNames returns a sorted list of agent names.
func (c *ConfigStore) GetAgentNames() []string {
	agentsMap := c.v.GetStringMap("agents")
	if agentsMap == nil {
		return nil
	}

	names := make([]string, 0, len(agentsMap))
	for name := range agentsMap {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// SetAgent saves or updates an agent configuration.
func (c *ConfigStore) SetAgent(name string, agent *AgentConfig) error {
	name = strings.ToLower(name)
	agentsMap := c.v.GetStringMap("agents")
	if agentsMap == nil {
		agentsMap = make(map[string]interface{})
	}

	// Convert Agent struct to map for viper
	agentsMap[name] = c.agentToMap(agent)
	c.v.Set("agents", agentsMap)

	return c.Save()
}

// DeleteAgent removes an agent configuration.
func (c *ConfigStore) DeleteAgent(name string) error {
	name = strings.ToLower(name)
	agentsMap := c.v.GetStringMap("agents")
	if agentsMap == nil {
		return fmt.Errorf("no agents configured")
	}

	if _, exists := agentsMap[name]; !exists {
		return fmt.Errorf("agent '%s' not found", name)
	}

	delete(agentsMap, name)
	c.v.Set("agents", agentsMap)

	return c.Save()
}

// SetActiveAgent sets the active agent name.
func (c *ConfigStore) SetActiveAgent(name string) error {
	c.v.Set("agent", name)
	return c.Save()
}

// GetActiveAgent returns the currently active agent configuration.
func (c *ConfigStore) GetActiveAgent() *AgentConfig {
	name := c.GetActiveAgentName()
	if name == "" {
		// Don't use default first one, would cause user confuse
		return nil
	}
	return c.GetAgent(name)
}

// GetModels returns all configured models.
func (c *ConfigStore) GetModels() map[string]*Model {
	modelsMap := c.v.GetStringMap("models")
	result := make(map[string]*Model)

	for name, config := range modelsMap {
		if configMap := toStringMap(config); configMap != nil {
			model := c.mapToModel(name, configMap)
			result[name] = &model
		}
	}
	return result
}

// SetModel adds or updates a model.
func (c *ConfigStore) SetModel(name string, model *Model) error {
	name = strings.ToLower(name)
	modelsMap := c.v.GetStringMap("models")
	if modelsMap == nil {
		modelsMap = make(map[string]interface{})
	}
	modelConfigMap := c.modelToMap(model)
	modelsMap[name] = modelConfigMap
	c.v.Set("models", modelsMap)
	return c.Save()
}

func (c *ConfigStore) GetModel(name string) *Model {
	name = strings.ToLower(name)
	modelsMap := c.v.GetStringMap("models")
	if modelConfig, ok := modelsMap[name]; ok {
		if configMap := toStringMap(modelConfig); configMap != nil {
			model := c.mapToModel(name, configMap)
			return &model
		}
	}
	return nil
}

// getModelFromAgentMap returns a single model's config.
// for private use only
func (c *ConfigStore) getModelFromAgentMap(m map[string]interface{}, key string) Model {
	val, exists := m[key]
	if !exists {
		return Model{}
	}

	// Model is a string reference (alias)
	if name, ok := val.(string); ok {
		name = strings.ToLower(name)
		modelsMap := c.v.GetStringMap("models")
		if modelConfig, ok := modelsMap[name]; ok {
			if configMap := toStringMap(modelConfig); configMap != nil {
				model := c.mapToModel(name, configMap)
				return model
			}
		}
		// Return partial model if not found found (or just name)
		return Model{Name: name}
	}

	return Model{}
}

// DeleteModel removes a model.
func (c *ConfigStore) DeleteModel(name string) error {
	name = strings.ToLower(name)
	modelsMap := c.v.GetStringMap("models")
	if modelsMap == nil {
		return fmt.Errorf("no models configured")
	}
	delete(modelsMap, name)
	c.v.Set("models", modelsMap)
	return c.Save()
}

// GetSearchEngines returns all configured search engines.
func (c *ConfigStore) GetSearchEngines() map[string]*SearchEngine {
	searchMap := c.v.GetStringMap("search_engines")
	result := make(map[string]*SearchEngine)

	for name, config := range searchMap {
		if configMap := toStringMap(config); configMap != nil {
			se := c.mapToSearchEngine(name, configMap)
			result[name] = &se
		}
	}
	return result
}

// GetSearchEngine returns a specific search engine by name.
func (c *ConfigStore) GetSearchEngine(name string) *SearchEngine {
	name = strings.ToLower(name)
	searchMap := c.v.GetStringMap("search_engines")
	if searchConfig, ok := searchMap[name]; ok {
		if configMap := toStringMap(searchConfig); configMap != nil {
			se := c.mapToSearchEngine(name, configMap)
			return &se
		}
	}
	return nil
}

// SetSearchEngine adds or updates a search engine.
func (c *ConfigStore) SetSearchEngine(name string, se *SearchEngine) error {
	name = strings.ToLower(name)
	searchMap := c.v.GetStringMap("search_engines")
	if searchMap == nil {
		searchMap = make(map[string]interface{})
	}
	searchMap[name] = c.searchEngineToMap(se)
	c.v.Set("search_engines", searchMap)
	return c.Save()
}

// DeleteSearchEngine removes a search engine.
func (c *ConfigStore) DeleteSearchEngine(name string) error {
	name = strings.ToLower(name)
	searchMap := c.v.GetStringMap("search_engines")
	if searchMap == nil {
		return fmt.Errorf("no search engines configured")
	}
	delete(searchMap, name)
	c.v.Set("search_engines", searchMap)
	return c.Save()
}

// GetTemplates returns all configured templates.
func (c *ConfigStore) GetTemplates() map[string]string {
	return c.v.GetStringMapString("templates")
}

// GetTemplate returns a specific template by name.
func (c *ConfigStore) GetTemplate(name string) string {
	name = strings.ToLower(name)
	return c.v.GetStringMapString("templates")[name]
}

// SetTemplate adds or updates a template.
func (c *ConfigStore) SetTemplate(name, content string) error {
	name = strings.ToLower(name)
	templates := c.v.GetStringMapString("templates")
	if templates == nil {
		templates = make(map[string]string)
	}
	templates[name] = content
	c.v.Set("templates", templates)
	return c.Save()
}

// DeleteTemplate removes a template.
func (c *ConfigStore) DeleteTemplate(name string) error {
	name = strings.ToLower(name)
	templates := c.v.GetStringMapString("templates")
	if templates == nil {
		return fmt.Errorf("no templates configured")
	}
	delete(templates, name)
	c.v.Set("templates", templates)
	return c.Save()
}

// GetSystemPrompts returns all configured system prompts.
func (c *ConfigStore) GetSystemPrompts() map[string]string {
	return c.v.GetStringMapString("system_prompts")
}

// GetSystemPrompt returns a specific system prompt by name.
func (c *ConfigStore) GetSystemPrompt(name string) string {
	name = strings.ToLower(name)
	return c.v.GetStringMapString("system_prompts")[name]
}

// SetSystemPrompt adds or updates a system prompt.
func (c *ConfigStore) SetSystemPrompt(name, content string) error {
	name = strings.ToLower(name)
	prompts := c.v.GetStringMapString("system_prompts")
	if prompts == nil {
		prompts = make(map[string]string)
	}
	prompts[name] = content
	c.v.Set("system_prompts", prompts)
	return c.Save()
}

// DeleteSystemPrompt removes a system prompt.
func (c *ConfigStore) DeleteSystemPrompt(name string) error {
	name = strings.ToLower(name)
	prompts := c.v.GetStringMapString("system_prompts")
	if prompts == nil {
		return fmt.Errorf("no system prompts configured")
	}
	delete(prompts, name)
	c.v.Set("system_prompts", prompts)
	return c.Save()
}

// GetString returns a string value from config.
func (c *ConfigStore) GetString(key string) string {
	return c.v.GetString(key)
}

// GetInt returns an int value from config.
func (c *ConfigStore) GetInt(key string) int {
	return c.v.GetInt(key)
}

// GetStringMap returns a string map from config.
func (c *ConfigStore) GetStringMap(key string) map[string]interface{} {
	return c.v.GetStringMap(key)
}

// GetStringMapString returns a string-to-string map from config.
func (c *ConfigStore) GetStringMapString(key string) map[string]string {
	return c.v.GetStringMapString(key)
}

// Save persists the configuration to disk.
func (c *ConfigStore) Save() error {
	configFile := c.v.ConfigFileUsed()
	if configFile == "" {
		configFile = filepath.Join(GetConfigDir(), "gllm.yaml")
		c.v.SetConfigFile(configFile)
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(configFile), 0750); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	return c.v.WriteConfigAs(configFile)
}

// parseAgentConfig converts a raw config value to an Agent struct.
// This is where ALL type assertions happen - keeping them out of cmd/service.
func (c *ConfigStore) parseAgentConfig(name string, config interface{}) *AgentConfig {
	configMap := toStringMap(config)
	if configMap == nil {
		return nil
	}

	agent := &AgentConfig{
		Name:          name,
		Model:         c.getModelFromAgentMap(configMap, "model"),
		Think:         getString(configMap, "think"),
		Template:      getString(configMap, "template"),
		SystemPrompt:  getString(configMap, "system_prompt"),
		MaxRecursions: getInt(configMap, "max_recursions", 10),
		Tools:         getStringSlice(configMap, "tools"),
		Capabilities:  getStringSlice(configMap, "capabilities"),
	}

	return agent
}

// agentToMap converts an Agent struct to a map for viper storage.
func (c *ConfigStore) agentToMap(agent *AgentConfig) map[string]interface{} {
	return map[string]interface{}{
		"model":          agent.Model.Name,
		"tools":          agent.Tools,
		"capabilities":   agent.Capabilities,
		"think":          agent.Think,
		"template":       agent.Template,
		"system_prompt":  agent.SystemPrompt,
		"max_recursions": agent.MaxRecursions,
	}
}

// modelToMap converts a Model struct to a map for viper storage.
func (c *ConfigStore) modelToMap(model *Model) map[string]interface{} {
	m := map[string]interface{}{
		"endpoint":    model.Endpoint,
		"key":         model.Key,
		"model":       model.Model,
		"temperature": model.Temp,
		"top_p":       model.TopP,
		"provider":    model.Provider,
	}
	if model.Seed != nil {
		m["seed"] = *model.Seed
	}
	return m
}

// mapToModel converts a map to Model struct helper
func (c *ConfigStore) mapToModel(name string, m map[string]interface{}) Model {
	return Model{
		Name:     name,
		Provider: getString(m, "provider"),
		Endpoint: getString(m, "endpoint"),
		Key:      getString(m, "key"),
		Model:    getString(m, "model"),
		Temp:     getFloat(m, "temperature", 1.0),
		TopP:     getFloat(m, "top_p", 1.0),
		Seed:     getPtrInt(m, "seed"),
	}
}

func (c *ConfigStore) searchEngineToMap(se *SearchEngine) map[string]interface{} {
	m := map[string]interface{}{
		"deep_dive":  se.DeepDive,
		"references": se.Reference,
	}
	for k, v := range se.Config {
		m[k] = v
	}
	return m
}

func (c *ConfigStore) mapToSearchEngine(name string, m map[string]interface{}) SearchEngine {
	se := SearchEngine{
		Name:      name,
		DeepDive:  getInt(m, "deep_dive", 3),
		Reference: getInt(m, "references", 5),
		Config:    make(map[string]string),
	}
	// Copy all string values to Config
	for k, v := range m {
		if s, ok := v.(string); ok {
			se.Config[k] = s
		}
	}
	return se
}

// Helper functions for type-safe extraction from interface{} maps.
// These are ONLY used within the data package.

func toStringMap(v interface{}) map[string]interface{} {
	switch m := v.(type) {
	case map[string]interface{}:
		return m
	case map[interface{}]interface{}:
		result := make(map[string]interface{})
		for k, val := range m {
			result[fmt.Sprint(k)] = val
		}
		return result
	}
	return nil
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

func getInt(m map[string]interface{}, key string, defaultVal int) int {
	switch v := m[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return defaultVal
}

func getFloat(m map[string]interface{}, key string, defaultVal float32) float32 {
	switch v := m[key].(type) {
	case float32:
		return v
	case float64:
		return float32(v)
	// Buxfix: handle int values as well
	// When saving 0 to the YAML configuration, it was sometimes parsed as an integer 0.
	// This caused getFloat to return the default value(1.0) instead of 0.0.
	case int:
		return float32(v)
	case int64:
		return float32(v)
	}
	return defaultVal
}

func getPtrInt(m map[string]interface{}, key string) *int32 {
	switch v := m[key].(type) {
	case int:
		vv := int32(v)
		return &vv
	case int32:
		return &v
	case int64:
		vv := int32(v)
		return &vv
	case float64:
		vv := int32(v)
		return &vv
	}
	return nil
}

func getStringSlice(m map[string]interface{}, key string) []string {
	switch v := m[key].(type) {
	case []string:
		return v
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}
