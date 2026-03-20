package data

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type AgentFrontmatter struct {
	Name          string   `yaml:"name"`
	Description   string   `yaml:"description,omitempty"`
	Model         string   `yaml:"model"`
	Tools         []string `yaml:"tools,omitempty"`
	Capabilities  []string `yaml:"capabilities,omitempty"`
	Think         string   `yaml:"think,omitempty"`
	MaxRecursions int      `yaml:"max_recursions,omitempty"`
}

// EnsureAgentsDir creates the agents directory if it doesn't exist.
func EnsureAgentsDir() error {
	return os.MkdirAll(GetAgentsDirPath(), 0750)
}

func (c *ConfigStore) ParseAgentFile(path string) (*AgentConfig, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read agent file: %w", err)
	}

	s := string(content)
	if !strings.HasPrefix(s, "---") {
		return nil, fmt.Errorf("agent file missing frontmatter in %s", path)
	}

	parts := strings.SplitN(s, "---", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid frontmatter format in %s", path)
	}

	frontmatterStr := parts[1]
	systemPromptStr := strings.TrimSpace(parts[2])

	var meta AgentFrontmatter
	if err := yaml.Unmarshal([]byte(frontmatterStr), &meta); err != nil {
		return nil, fmt.Errorf("failed to parse frontmatter in %s: %w", path, err)
	}

	if meta.MaxRecursions == 0 {
		meta.MaxRecursions = 50 // default
	}

	agentName := strings.TrimSuffix(filepath.Base(path), ".md")

	modelMap := map[string]interface{}{"model": meta.Model}

	agent := &AgentConfig{
		Name:          agentName,
		Description:   meta.Description,
		Model:         c.getModelFromAgentMap(modelMap, "model"),
		Think:         meta.Think,
		SystemPrompt:  systemPromptStr,
		MaxRecursions: meta.MaxRecursions,
		Tools:         meta.Tools,
		Capabilities:  meta.Capabilities,
	}

	if meta.Name != "" {
		agent.Name = meta.Name
	}

	return agent, nil
}

func (c *ConfigStore) writeAgentFile(agent *AgentConfig) error {
	if err := EnsureAgentsDir(); err != nil {
		return err
	}

	filename := filepath.Join(GetAgentsDirPath(), agent.Name+".md")

	meta := AgentFrontmatter{
		Name:          agent.Name,
		Description:   agent.Description,
		Model:         agent.Model.Name,
		Tools:         agent.Tools,
		Capabilities:  agent.Capabilities,
		Think:         agent.Think,
		MaxRecursions: agent.MaxRecursions,
	}

	yamlData, err := yaml.Marshal(&meta)
	if err != nil {
		return fmt.Errorf("failed to marshal agent frontmatter: %w", err)
	}

	content := fmt.Sprintf("---\n%s---\n\n%s\n", string(yamlData), agent.SystemPrompt)

	return os.WriteFile(filename, []byte(content), 0644)
}

var validAgentNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// ValidateAgentName checks if the agent name is filesystem-safe.
func ValidateAgentName(name string) error {
	if !validAgentNameRegex.MatchString(name) {
		return fmt.Errorf("agent name '%s' is invalid: only alphanumeric characters, dashes, and underscores are allowed", name)
	}
	return nil
}

// ExportAgent exports an agent's .md file to the specified destination path.
// It validates the agent exists and is well-formed before exporting.
func (c *ConfigStore) ExportAgent(name, destPath string) error {
	name = strings.ToLower(name)

	// Check if agent exists
	srcPath := filepath.Join(GetAgentsDirPath(), name+".md")
	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		return fmt.Errorf("agent '%s' not found", name)
	}

	// Validate before exporting
	if _, err := c.ParseAgentFile(srcPath); err != nil {
		return fmt.Errorf("agent file is malformed: %w", err)
	}

	// Read source file
	content, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("failed to read agent file: %w", err)
	}

	// Write to destination
	if err := os.WriteFile(destPath, content, fs.FileMode(0644)); err != nil {
		return fmt.Errorf("failed to write export file: %w", err)
	}

	return nil
}

// ImportAgent imports an agent from a .md file into the agents directory.
// It validates the file format and checks for name conflicts.
func (c *ConfigStore) ImportAgent(srcPath string) error {
	// Check if source file exists
	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", srcPath)
	}

	// Parse and validate frontmatter
	agent, err := c.ParseAgentFile(srcPath)
	if err != nil {
		return fmt.Errorf("invalid agent file: %w", err)
	}

	if agent.Name == "" {
		return fmt.Errorf("agent file is missing a 'name' field in its frontmatter")
	}

	if err := ValidateAgentName(agent.Name); err != nil {
		return fmt.Errorf("agent name is invalid: %w", err)
	}

	// Ensure agents directory exists
	if err := EnsureAgentsDir(); err != nil {
		return fmt.Errorf("failed to create agents directory: %w", err)
	}

	// Read source file
	content, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	// Write to agents directory
	destPath := filepath.Join(GetAgentsDirPath(), agent.Name+".md")
	if err := os.WriteFile(destPath, content, fs.FileMode(0644)); err != nil {
		return fmt.Errorf("failed to write agent file: %w", err)
	}

	return nil
}
