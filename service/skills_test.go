package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/activebook/gllm/data"
)

func TestSkillManager(t *testing.T) {
	// Setup temporary skill directory
	tmpDir, err := os.MkdirTemp("", "gllm-skills-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Mock valid skill content
	skillName := "test_skill"
	skillContent := `---
name: test_skill
description: A test skill
---
# Instructions
Do testing stuff.`

	// Create skill structure
	skillPath := filepath.Join(tmpDir, skillName)
	err = os.MkdirAll(skillPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create skill dir: %v", err)
	}

	err = os.WriteFile(filepath.Join(skillPath, "SKILL.md"), []byte(skillContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	// Create a resource file
	err = os.WriteFile(filepath.Join(skillPath, "README.md"), []byte("# Readme"), 0644)
	if err != nil {
		t.Fatalf("Failed to write README.md: %v", err)
	}

	// Override default config dir used by data package if possible,
	// but since we can't easily change `data` package internals without dependency injection/globals,
	// we will instantiate SkillManager manually and inject path if we refactor it,
	// or we just trust our ScanSkills works if we point to specific dir.

	// However, `data.ScanSkills` uses `GetSkillsDirPath()` which is hardcoded to config dir.
	// To test this properly without writing to real config dir, we need to modify `data` package to allow overriding or `SkillManager` to accept path.

	// In `service/skills.go`:
	// func NewSkillManager() *SkillManager { return &SkillManager{ skillsDir: data.GetSkillsDirPath() } }

	// We can manually create SkillManager with custom path if we make it public or use internal test.

	sm := &SkillManager{
		skillsDir: tmpDir,
	}

	// We need to Mock `ScanSkills` or make `ScanSkills` accept a path.
	// data.ScanSkills() currently uses data.GetSkillsDirPath().

	// Let's modify `data/skills.go` to accept a path?
	// Or simpler: Test `ActivateSkill` which logic resides in `SkillManager` but `LoadMetadata` relies on `data`.

	// Let's verify `ActivateSkill` logic first, assuming metadata is loaded.

	sm.skills = []data.SkillMetadata{
		{
			Name:        skillName,
			Description: "A test skill",
			Location:    filepath.Join(skillPath, "SKILL.md"),
		},
	}

	// Test ActivateSkill
	instructions, err := sm.ActivateSkill(skillName)
	if err != nil {
		t.Fatalf("ActivateSkill failed: %v", err)
	}

	if !strings.Contains(instructions, "Do testing stuff") {
		t.Errorf("Instructions missing content. Got: %s", instructions)
	}

	if !strings.Contains(instructions, "<available_resources") {
		t.Errorf("Instructions missing resources. Got: %s", instructions)
	}

	if !strings.Contains(instructions, "README.md") {
		t.Errorf("Instructions missing README.md in file tree. Got: %s", instructions)
	}

	t.Logf("ActivateSkill output:\n%s", instructions)
}
