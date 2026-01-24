// File: cmd/skill.go
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/service"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

// skillsCmd represents the skills subcommand
var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Manage agent skills",
	Long: `Manage skills that extend agent capabilities.
Agent Skills are a lightweight, open format for extending AI agent capabilities with specialized knowledge and workflows.

Use 'gllm skills switch' to switch skills on/off.
Use 'gllm skills list' to list all installed skills.
Use 'gllm skills install <path>' to install a skill.
Use 'gllm skills uninstall <name>' to uninstall a skill.
`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(cmd.Long)
		fmt.Println()
		// Default action: list skills
		skillsListCmd.Run(skillsListCmd, args)
	},
}

// skillsListCmd lists all installed skills
var skillsListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls", "pr", "print"},
	Short:   "List all installed skills",
	Long:    `List all skills installed in the skills directory with their enabled/disabled status.`,
	Run: func(cmd *cobra.Command, args []string) {
		skills, err := data.ScanSkills()
		if err != nil {
			service.Errorf("Failed to scan skills: %v", err)
			return
		}

		if len(skills) == 0 {
			fmt.Println("No skills installed.")
			fmt.Println()
			fmt.Println("Use 'gllm skills install <path>' to install a skill.")
			return
		}

		settingsStore := data.GetSettingsStore()

		fmt.Println("Installed skills:")
		fmt.Println()

		// Sort skills by name
		sort.Slice(skills, func(i, j int) bool {
			return strings.ToLower(skills[i].Name) < strings.ToLower(skills[j].Name)
		})

		for _, skill := range skills {
			status := greenColor("Enabled")
			if settingsStore.IsSkillDisabled(skill.Name) {
				status = grayColor("Disabled")
			}
			fmt.Printf("  %s [%s]\n", skill.Name, status)
			if skill.Description != "" {
				fmt.Printf("    %s\n", grayColor(skill.Description))
			}
		}
		fmt.Println()
		fmt.Printf("Skills directory: %s\n", data.GetSkillsDirPath())
	},
}

// skillsInstallPath holds the path flag value
var skillsInstallPath string

// skillsInstallCmd installs a skill from a path
var skillsInstallCmd = &cobra.Command{
	Use:   "install <path|url>",
	Short: "Install a skill from a path or git URL",
	Long: `Install a skill by copying its directory to the skills storage.
The source can be a local directory path or a git repository URL.
If a git URL is provided, the repository will be cloned temporarily.
You can use the --path flag to specify a subdirectory within the git repository.
The source (local or resolved git path) must contain a valid SKILL.md file with frontmatter.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		source := args[0]
		var absPath string
		var cleanup func()

		// Check if source is a URL (starts with http/https or ends with .git)
		if strings.HasPrefix(source, "http") || strings.HasSuffix(source, ".git") {
			// Create temp dir
			tempDir, err := os.MkdirTemp("", "gllm-skill-clone-*")
			if err != nil {
				service.Errorf("Failed to create temp directory: %v", err)
				return
			}
			cleanup = func() { os.RemoveAll(tempDir) }
			defer cleanup()

			fmt.Printf("Cloning %s...\n", source)
			// Clone git repo
			gitCmd := exec.Command("git", "clone", source, tempDir)
			gitCmd.Stdout = os.Stdout
			gitCmd.Stderr = os.Stderr
			if err := gitCmd.Run(); err != nil {
				service.Errorf("Failed to clone repository: %v", err)
				return
			}

			// Determine path within repo
			if skillsInstallPath != "" {
				absPath = filepath.Join(tempDir, skillsInstallPath)
			} else {
				absPath = tempDir
			}
		} else {
			// Local path
			var err error
			absPath, err = filepath.Abs(source)
			if err != nil {
				service.Errorf("Invalid path: %v", err)
				return
			}
		}

		// Check if source exists and is a directory
		info, err := os.Stat(absPath)
		if err != nil {
			service.Errorf("Cannot access path: %v", err)
			return
		}
		if !info.IsDir() {
			service.Errorf("Path must be a directory containing SKILL.md")
			return
		}

		// Validate SKILL.md exists
		skillFilePath := filepath.Join(absPath, data.SkillFile)
		if _, err := os.Stat(skillFilePath); err != nil {
			service.Errorf("SKILL.md not found in %s", absPath)
			return
		}

		// Parse and validate frontmatter
		meta, err := data.ParseSkillFrontmatter(skillFilePath)
		if err != nil {
			service.Errorf("Invalid SKILL.md: %v", err)
			return
		}

		if meta.Name == "" {
			service.Errorf("SKILL.md must have a 'name' field in frontmatter")
			return
		}

		// Use the skill name from frontmatter as destination folder name
		destDir := filepath.Join(data.GetSkillsDirPath(), meta.Name)

		// Check if skill already exists
		if _, err := os.Stat(destDir); err == nil {
			var confirm bool
			err := huh.NewConfirm().
				Title(fmt.Sprintf("Skill '%s' already exists. Overwrite?", meta.Name)).
				Affirmative("Yes").
				Negative("No").
				Value(&confirm).
				Run()
			if err != nil || !confirm {
				fmt.Println("Aborted.")
				return
			}
			// Remove existing
			if err := os.RemoveAll(destDir); err != nil {
				service.Errorf("Failed to remove existing skill: %v", err)
				return
			}
		}

		// Copy directory
		if err := copyDir(absPath, destDir); err != nil {
			service.Errorf("Failed to copy skill: %v", err)
			return
		}

		fmt.Printf("Skill '%s' installed successfully.\n", meta.Name)
	},
}

// skillsUninstallCmd removes an installed skill
var skillsUninstallCmd = &cobra.Command{
	Use:   "uninstall <name>",
	Short: "Uninstall an installed skill",
	Long:  `Uninstall a skill by deleting its directory from the skills storage.`,
	Run: func(cmd *cobra.Command, args []string) {
		var skillName string

		if len(args) > 0 {
			skillName = args[0]
		} else {
			// Interactive select
			skills, err := data.ScanSkills()
			if err != nil {
				service.Errorf("Failed to scan skills: %v", err)
				return
			}
			if len(skills) == 0 {
				fmt.Println("No skills installed.")
				return
			}

			var options []huh.Option[string]
			for _, s := range skills {
				options = append(options, huh.NewOption(s.Name, s.Name))
			}
			SortOptions(options, "")

			err = huh.NewSelect[string]().
				Title("Select skill to uninstall").
				Options(options...).
				Value(&skillName).
				Run()
			if err != nil {
				fmt.Println("Aborted.")
				return
			}
		}

		// Find skill directory
		skills, err := data.ScanSkills()
		if err != nil {
			service.Errorf("Failed to scan skills: %v", err)
			return
		}

		var skillPath string
		for _, s := range skills {
			if strings.EqualFold(s.Name, skillName) {
				skillPath = filepath.Dir(s.Location)
				skillName = s.Name // Use canonical name
				break
			}
		}

		if skillPath == "" {
			service.Errorf("Skill '%s' not found", skillName)
			return
		}

		// Confirm removal
		var confirm bool
		err = huh.NewConfirm().
			Title(fmt.Sprintf("Are you sure you want to remove skill '%s'?", skillName)).
			Affirmative("Yes").
			Negative("No").
			Value(&confirm).
			Run()
		if err != nil || !confirm {
			fmt.Println("Aborted.")
			return
		}

		// Remove directory
		if err := os.RemoveAll(skillPath); err != nil {
			service.Errorf("Failed to remove skill: %v", err)
			return
		}

		// Also remove from disabled list if present
		settingsStore := data.GetSettingsStore()
		_ = settingsStore.EnableSkill(skillName) // Ignore error, just cleanup

		fmt.Printf("Skill '%s' removed successfully.\n", skillName)
	},
}

// skillsSwCmd toggles skills interactively
var skillsSwCmd = &cobra.Command{
	Use:     "switch",
	Aliases: []string{"sw", "sel", "select"},
	Short:   "Switch skills on/off",
	Long:    "Choose which skills to enable or disable interactively.",
	Run: func(cmd *cobra.Command, args []string) {
		skills, err := data.ScanSkills()
		if err != nil {
			service.Errorf("Failed to scan skills: %v", err)
			return
		}

		if len(skills) == 0 {
			fmt.Println("No skills installed.")
			return
		}

		settingsStore := data.GetSettingsStore()

		var options []huh.Option[string]
		var enabledSkills []string
		for _, s := range skills {
			opt := huh.NewOption(s.Name, s.Name)
			if !settingsStore.IsSkillDisabled(s.Name) {
				opt = opt.Selected(true)
				enabledSkills = append(enabledSkills, s.Name)
			}
			options = append(options, opt)
		}

		SortMultiOptions(options, enabledSkills)

		var selectedSkills []string
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title("Select Skills").
					Description("Choose which skills to enable. Press space to toggle, enter to confirm.").
					Options(options...).
					Value(&selectedSkills),
			),
		).Run()

		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		// Update settings
		selectedSet := make(map[string]bool)
		for _, name := range selectedSkills {
			selectedSet[name] = true
		}

		for _, s := range skills {
			if selectedSet[s.Name] {
				_ = settingsStore.EnableSkill(s.Name)
			} else {
				_ = settingsStore.DisableSkill(s.Name)
			}
		}

		// Run skills list to show updated list
		fmt.Printf("\n%d skill(s) enabled.\n", len(selectedSkills))
		fmt.Println()
		skillsListCmd.Run(skillsListCmd, args)
	},
}

func init() {
	rootCmd.AddCommand(skillsCmd)
	skillsCmd.AddCommand(skillsListCmd)
	skillsCmd.AddCommand(skillsInstallCmd)
	skillsInstallCmd.Flags().StringVar(&skillsInstallPath, "path", "", "Path to the skill directory within the git repository")
	skillsCmd.AddCommand(skillsUninstallCmd)
	skillsCmd.AddCommand(skillsSwCmd)
}
