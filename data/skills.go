package data

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	SkillFile = "SKILL.md"
)

// SkillMetadata represents the metadata for a single skill.
type SkillMetadata struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Location    string `yaml:"-"` // Full path to SKILL.md, not in YAML
}

// EnsureSkillsDir creates the skills directory if it doesn't exist.
func EnsureSkillsDir() error {
	return os.MkdirAll(GetSkillsDirPath(), 0750)
}

// ParseSkillFrontmatter reads a SKILL.md file and extracts its metadata.
func ParseSkillFrontmatter(path string) (*SkillMetadata, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read skill file: %w", err)
	}

	// Extract frontmatter (between --- separators)
	s := string(content)
	if !strings.HasPrefix(s, "---") {
		return nil, fmt.Errorf("skill file missing frontmatter")
	}

	parts := strings.SplitN(s, "---", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid frontmatter format")
	}

	frontmatter := parts[1]
	var meta SkillMetadata
	if err := yaml.Unmarshal([]byte(frontmatter), &meta); err != nil {
		return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	meta.Location = path
	return &meta, nil
}

// ScanSkills scans the skills directory for valid skills.
// A valid skill is a directory containing a SKILL.md file with valid frontmatter.
func ScanSkills() ([]SkillMetadata, error) {
	if err := EnsureSkillsDir(); err != nil {
		return nil, fmt.Errorf("failed to ensure skills directory: %w", err)
	}

	skillsDir := GetSkillsDirPath()
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []SkillMetadata{}, nil
		}
		return nil, fmt.Errorf("failed to read skills directory: %w", err)
	}

	var skills []SkillMetadata
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// resolve the path to full path
		skillPath := filepath.Join(skillsDir, entry.Name(), SkillFile)
		if _, err := os.Stat(skillPath); err != nil {
			continue // Skip directories without SKILL.md
		}

		meta, err := ParseSkillFrontmatter(skillPath)
		if err != nil {
			// Log error but continue scanning other skills
			fmt.Printf("Warning: Skipping invalid skill at %s: %v\n", skillPath, err)
		}

		skills = append(skills, *meta)
	}

	return skills, nil
}
