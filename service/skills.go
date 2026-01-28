package service

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/activebook/gllm/data"
)

// SkillManager handles skill operations
type SkillManager struct {
	skills    []data.SkillMetadata
	skillsDir string
	mu        sync.RWMutex
}

var (
	skillManagerInstance *SkillManager
	skillManagerOnce     sync.Once
)

// GetSkillManager returns the singleton instance of SkillManager
func GetSkillManager() *SkillManager {
	skillManagerOnce.Do(func() {
		skillManagerInstance = NewSkillManager()
	})
	return skillManagerInstance
}

// NewSkillManager creates a new SkillManager
func NewSkillManager() *SkillManager {
	return &SkillManager{
		skillsDir: data.GetSkillsDirPath(),
	}
}

// LoadMetadata scans and loads skill metadata
func (sm *SkillManager) LoadMetadata() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	skills, err := data.ScanSkills()
	if err != nil {
		return err
	}
	sm.skills = skills
	return nil
}

// GetAvailableSkills returns the XML string for system prompt injection
// Skills that are disabled in settings.json are excluded from the output.
func (sm *SkillManager) GetAvailableSkills() string {
	if err := sm.LoadMetadata(); err != nil {
		// Log warning but don't fail - skills are optional
		Warnf("Failed to load skills: %v", err)
		return ""
	}

	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if len(sm.skills) == 0 {
		return ""
	}

	// Get disabled skills from settings
	settingsStore := data.GetSettingsStore()

	var sb strings.Builder
	sb.WriteString("<available_skills>\n")
	for _, skill := range sm.skills {
		// Skip disabled skills
		if settingsStore.IsSkillDisabled(skill.Name) {
			continue
		}
		fmt.Fprintf(&sb, "  <skill>\n")
		sb.WriteString(fmt.Sprintf("    <name>%s</name>\n", skill.Name))
		sb.WriteString(fmt.Sprintf("    <description>%s</description>\n", skill.Description))
		sb.WriteString(fmt.Sprintf("    <location>%s</location>\n", skill.Location))
		fmt.Fprintf(&sb, "  </skill>\n")
	}
	sb.WriteString("</available_skills>")
	return sb.String()
}

// ActivateSkill activates a skill by name and returns its instructions along with available resources.
//
// This function performs the following operations:
// 1. Searches for the skill by name (case-insensitive) in the loaded skills list
// 2. Reads the skill's file content from disk
// 3. Extracts the instruction content by removing YAML frontmatter
// 4. Generates a file tree representation of the skill's directory
// 5. Returns a structured XML string containing the skill instructions and available resources
//
// The returned XML format is:
//
//	<activated_skill name="skill-name">
//	  <instructions>
//	    ...instruction content...
//	  </instructions>
//	  <available_resources root="/path/to/skill/dir">
//	    ...file tree...
//	  </available_resources>
//	</activated_skill>
//
// Parameters:
//   - name: The name of the skill to activate (case-insensitive)
//
// Returns:
//   - string: A structured XML string containing the skill instructions and available resources
//   - string: The skill description
//   - string: A structured XML string representing the skill's directory tree
//   - error: An error if the skill is not found, the file cannot be read, or the file tree cannot be generated
//
// Note:
//   - The function assumes skill files follow the YAML frontmatter format with three "---" delimiters
//   - The function uses read locks for concurrent access to the skills list
//   - If the skill file does not contain frontmatter, the entire file content is treated as instructions

// ActivateSkill activates a skill by name and returns its instructions, description, and available resources.
func (sm *SkillManager) ActivateSkill(name string) (string, string, string, error) {
	sm.mu.RLock()
	var selectedSkill *data.SkillMetadata
	lowerName := strings.ToLower(name)
	for _, s := range sm.skills {
		if strings.ToLower(s.Name) == lowerName {
			selectedSkill = &s
			break
		}
	}
	sm.mu.RUnlock()

	if selectedSkill == nil {
		return "", "", "", fmt.Errorf("skill '%s' not found", name)
	}

	// Read content (already validated during scan/parse)
	content, err := os.ReadFile(selectedSkill.Location)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to read skill file: %w", err)
	}

	// Remove frontmatter
	fullContent := string(content)
	parts := strings.SplitN(fullContent, "---", 3)
	instructionContent := fullContent
	if len(parts) >= 3 {
		instructionContent = strings.TrimSpace(parts[2])
	}

	// Generate file tree
	skillDir := filepath.Dir(selectedSkill.Location) + string(filepath.Separator)
	tree, err := sm.GenerateFileTree(skillDir)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to generate file tree: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<activated_skill name=\"%s\">\n", name))
	sb.WriteString("  <instructions>\n")
	sb.WriteString(instructionContent)
	sb.WriteString("\n  </instructions>\n")
	sb.WriteString(fmt.Sprintf("  <available_resources root=\"%s\">\n", skillDir))
	sb.WriteString(tree)
	sb.WriteString("  </available_resources>\n")
	sb.WriteString("</activated_skill>")

	return sb.String(), selectedSkill.Description, tree, nil
}

// GenerateFileTree generates a professional tree representation of the skill directory
// utilizing Unicode box-drawing characters for enhanced structural clarity.
func (sm *SkillManager) GenerateFileTree(dir string) (string, error) {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s\n", dir))
	err := sm.generateTreeRecursive(dir, "", &sb)
	if err != nil {
		return "", err
	}
	return sb.String(), nil
}

// generateTreeRecursive is a helper that traverses the directory tree and builds the string representation.
func (sm *SkillManager) generateTreeRecursive(dir string, prefix string, sb *strings.Builder) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	// Filter and separate directories and files for organized sorting
	var filtered []os.DirEntry
	for _, entry := range entries {
		name := entry.Name()
		if name == ".git" || name == "node_modules" || name == ".DS_Store" || strings.HasPrefix(name, ".") {
			continue
		}
		filtered = append(filtered, entry)
	}

	// Sort logic: Directories first, then alphabetical
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].IsDir() && !filtered[j].IsDir() {
			return true
		}
		if !filtered[i].IsDir() && filtered[j].IsDir() {
			return false
		}
		return filtered[i].Name() < filtered[j].Name()
	})

	for i, entry := range filtered {
		isLast := i == len(filtered)-1
		connector := "├── "
		newPrefix := prefix + "│   "
		if isLast {
			connector = "└── "
			newPrefix = prefix + "    "
		}

		name := entry.Name()
		if entry.IsDir() {
			sb.WriteString(fmt.Sprintf("%s%s%s/\n", prefix, connector, name))
			if err := sm.generateTreeRecursive(filepath.Join(dir, name), newPrefix, sb); err != nil {
				return err
			}
		} else {
			sb.WriteString(fmt.Sprintf("%s%s%s\n", prefix, connector, name))
		}
	}

	return nil
}

// CreateTestSkill creates a temporary test skill for verification
func (sm *SkillManager) CreateTestSkill(rootPath string) (string, error) {
	// Create skill directory
	skillDir := filepath.Join(rootPath, "test-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return "", err
	}

	// Create SKILL.md
	skillContent := `---
name: test-skill
description: A test skill for verification purposes.
---

# Test Skill Instructions

This is a test skill. When activated, please confirm that you can read this instruction.
Also check the available resources below.
`
	skillFile := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte(skillContent), 0644); err != nil {
		return "", err
	}

	// Create a resource file
	resourceFile := filepath.Join(skillDir, "scripts", "helper.py")
	if err := os.MkdirAll(filepath.Dir(resourceFile), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(resourceFile, []byte("# Helper script"), 0644); err != nil {
		return "", err
	}

	return skillDir, nil
}
