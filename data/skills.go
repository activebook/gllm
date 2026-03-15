package data

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	SkillFile     = "SKILL.md"
	SkillMetaFile = "skill.meta.json"
)

// SkillSourceMeta tracks the origin of an installed skill for update purposes.
type SkillSourceMeta struct {
	SourceURL   string `json:"source_url"`         // Essential for remote skills
	SubPath     string `json:"sub_path,omitempty"` // Essential for nested skill installs
	InstallDate string `json:"install_date"`       // Essential for update tracking
}

// SkillMetadata represents the metadata for a single skill.
type SkillMetadata struct {
	Name        string           `yaml:"name"`
	Description string           `yaml:"description"`
	Location    string           `yaml:"-"` // Full path to SKILL.md, not in YAML
	SourceMeta  *SkillSourceMeta `yaml:"-"` // Loaded from skill.meta.json, not in YAML
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

		singleSkillDir := filepath.Join(skillsDir, entry.Name())

		// resolve the path to full path
		skillPath := filepath.Join(singleSkillDir, SkillFile)
		if _, err := os.Stat(skillPath); err != nil {
			continue // Skip directories without SKILL.md
		}

		meta, err := ParseSkillFrontmatter(skillPath)
		if err != nil {
			// Log error but continue scanning other skills
			fmt.Printf("Warning: Skipping invalid skill at %s: %v\n", skillPath, err)
			continue
		}

		// Attempt to load source metadata
		sourceMeta, _ := LoadSkillSourceMeta(singleSkillDir)
		meta.SourceMeta = sourceMeta

		skills = append(skills, *meta)
	}

	return skills, nil
}

// LoadSkillSourceMeta reads the source metadata for a skill from its directory.
// It returns nil, nil if the file does not exist.
func LoadSkillSourceMeta(skillDir string) (*SkillSourceMeta, error) {
	metaPath := filepath.Join(skillDir, SkillMetaFile)
	content, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Not an error, just an untracked/local skill
		}
		return nil, fmt.Errorf("failed to read skill metadata file: %w", err)
	}

	var meta SkillSourceMeta
	if err := json.Unmarshal(content, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse skill metadata: %w", err)
	}

	return &meta, nil
}

// SaveSkillSourceMeta writes the source metadata to the skill's directory.
func SaveSkillSourceMeta(skillDir string, meta *SkillSourceMeta) error {
	metaPath := filepath.Join(skillDir, SkillMetaFile)
	content, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal skill metadata: %w", err)
	}

	if err := os.WriteFile(metaPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write skill metadata file: %w", err)
	}

	return nil
}
