// File: cmd/skill.go
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/internal/ui"
	"github.com/activebook/gllm/util"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var (
	// skillsInstallPaths holds the paths flag values
	skillsInstallPaths []string

	// skillsUpdateAll holds the --all flag value
	skillsUpdateAll bool
)

func init() {
	rootCmd.AddCommand(skillsCmd)
	skillsCmd.AddCommand(skillsListCmd)
	skillsCmd.AddCommand(skillsInstallCmd)
	skillsInstallCmd.Flags().StringSliceVar(&skillsInstallPaths, "path", []string{}, "Paths to the skill directories within the git repository (comma separated or multiple flags)")
	skillsCmd.AddCommand(skillsUninstallCmd)
	skillsCmd.AddCommand(skillsSwCmd)
	skillsCmd.AddCommand(skillsUpdateCmd)
	skillsUpdateCmd.Flags().BoolVarP(&skillsUpdateAll, "all", "a", false, "Update all installed skills that have source tracking")
}

// skillsCmd represents the skills subcommand
var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Manage agent skills",
	Long: `Manage skills that extend agent capabilities.
Agent Skills are a lightweight, open format for extending AI agent capabilities with specialized knowledge and workflows.

Use 'gllm skills switch' to switch skills on/off.
Use 'gllm skills list' to list all installed skills.
Use 'gllm skills install <path>' to install a skill.
Use 'gllm skills uninstall <name>' to uninstall a skill.`,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return []string{"list", "install", "uninstall", "switch"}, cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
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
			util.Errorf("Failed to scan skills: %v\n", err)
			return
		}

		if len(skills) == 0 {
			fmt.Println("No skills installed.")
			fmt.Println()
			fmt.Println("Use 'gllm skills install <path>' to install a skill.")
			return
		}

		fmt.Println("Installed skills:")
		fmt.Println()

		// Sort skills by name
		sort.Slice(skills, func(i, j int) bool {
			return strings.ToLower(skills[i].Name) < strings.ToLower(skills[j].Name)
		})

		for _, skill := range skills {
			printSkillMeta(skill)
		}
		fmt.Printf("%s = Enabled skill\n", ui.FormatEnabledIndicator(true))
		fmt.Printf("Skills directory: %s\n", data.GetSkillsDirPath())
	},
}

// skillsInstallCmd installs a skill from a path
var skillsInstallCmd = &cobra.Command{
	Use:     "install <path|url>",
	Aliases: []string{"add"},
	Short:   "Install a skill from a path or git URL",
	Long: `Install a skill by copying its directory to the skills storage.
The source can be a local directory path or a git repository URL.
If a git URL is provided, the repository will be cloned temporarily.
You can use the --path flag to specify a subdirectory within the git repository.
The source (local or resolved git path) must contain a valid SKILL.md file with frontmatter.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Stop any global indicator to prevent UI interference
		ui.GetIndicator().Stop()

		source := args[0]
		var tempDir string
		var cleanup func()
		var isRemote bool // Whether this skill is installed from remote

		// Check if source is a URL (starts with http/https or ends with .git)
		if strings.HasPrefix(source, "http") || strings.HasSuffix(source, ".git") {
			isRemote = true

			// Create temp dir
			var err error
			tempDir, err = os.MkdirTemp("", "gllm-skill-clone-*")
			if err != nil {
				util.Errorf("Failed to create temp directory: %v\n", err)
				return
			}
			cleanup = func() { os.RemoveAll(tempDir) }
			defer cleanup()

			if err := downloadRepo(source, tempDir); err != nil {
				util.Errorf("%v\n", err)
				return
			}
		} else {
			// Local path
			var err error
			tempDir, err = filepath.Abs(source)
			if err != nil {
				util.Errorf("Invalid path: %v\n", err)
				return
			}
		}

		// Discover skills in the specified paths
		targetSubPaths, err := discoverSkills(tempDir, skillsInstallPaths)
		if err != nil {
			util.Errorf("Discovery failed: %v\n", err)
			return
		}

		// If multiple skills are found, prompt for confirmation
		if len(targetSubPaths) > 1 {
			var names []string
			for _, p := range targetSubPaths {
				// Just show the subpath/name for clarity
				if p == "" {
					names = append(names, "(root)")
				} else {
					names = append(names, p)
				}
			}
			sort.Strings(names)

			var confirm bool
			err := huh.NewConfirm().
				Title(fmt.Sprintf("Discovered %d skills. Install all?", len(targetSubPaths))).
				Description(strings.Join(names, ", ")).
				Affirmative("Yes").
				Negative("No").
				Value(&confirm).
				Run()
			if err != nil || !confirm {
				fmt.Println("Aborted.")
				return
			}
		}

		successCount := 0
		failCount := 0

		for _, subPath := range targetSubPaths {
			absSkillDir := tempDir
			if subPath != "" {
				absSkillDir = filepath.Join(tempDir, subPath)
			}

			if len(targetSubPaths) > 1 {
				fmt.Printf("Installing: %s\n", subPath)
			}

			if err := installSingleSkill(absSkillDir, isRemote, source, subPath); err != nil {
				util.Errorf("Failed to install skill from path '%s': %v\n", subPath, err)
				failCount++
			} else {
				successCount++
			}
		}

		if len(targetSubPaths) > 1 {
			if failCount > 0 {
				fmt.Printf("Installation complete. Successfully installed %d skills. %d failures.\n", successCount, failCount)
			} else {
				fmt.Printf("Installation complete. Successfully installed %d skills.\n", successCount)
			}
		}
	},
}

// installSingleSkill handles the validation, copying, and metadata saving of a single skill directory
func installSingleSkill(absSkillDirPath string, isRemote bool, sourceURL string, subPath string) error {
	// Check if source exists and is a directory
	info, err := os.Stat(absSkillDirPath)
	if err != nil {
		return fmt.Errorf("cannot access path: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path must be a directory containing %s", data.SkillFile)
	}

	// Validate SKILL.md exists
	skillFilePath := filepath.Join(absSkillDirPath, data.SkillFile)
	if _, err := os.Stat(skillFilePath); err != nil {
		return fmt.Errorf("%s not found in %s", data.SkillFile, absSkillDirPath)
	}

	// Parse and validate frontmatter
	meta, err := data.ParseSkillFrontmatter(skillFilePath)
	if err != nil {
		return fmt.Errorf("invalid %s: %w", data.SkillFile, err)
	}

	if meta.Name == "" {
		return fmt.Errorf("%s must have a 'name' field in frontmatter", data.SkillFile)
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
			util.Warnf("user aborted overwrite for skill '%s'", meta.Name)
			return nil
		}
		// Remove existing
		if err := os.RemoveAll(destDir); err != nil {
			return fmt.Errorf("failed to remove existing skill: %w", err)
		}
	}

	// Copy directory
	if err := copyDir(absSkillDirPath, destDir); err != nil {
		return fmt.Errorf("failed to copy skill: %w", err)
	}

	// Save metadata if installed from a remote source
	if isRemote {
		sourceMeta := &data.SkillSourceMeta{
			SourceURL:   sourceURL,
			SubPath:     subPath,
			InstallDate: time.Now().UTC().Format(time.RFC3339),
		}
		if err := data.SaveSkillSourceMeta(destDir, sourceMeta); err != nil {
			// Don't fail the whole installation, but warn the user
			util.Warnf("Failed to save source tracking metadata: %v\n", err)
		}
	}

	fmt.Printf("Skill '%s' installed successfully.\n", meta.Name)
	return nil
}

// discoverSkills scans the provided paths (or root) for SKILL.md files.
// If a path is not a skill itself, it scans its immediate subdirectories.
func discoverSkills(tempDir string, initialPaths []string) ([]string, error) {
	if len(initialPaths) == 0 {
		initialPaths = []string{""}
	}

	var targetSubPaths []string
	for _, sp := range initialPaths {
		fullPathToSub := filepath.Join(tempDir, sp)

		// 1. Check if the path itself is a skill
		if _, err := os.Stat(filepath.Join(fullPathToSub, data.SkillFile)); err == nil {
			targetSubPaths = append(targetSubPaths, sp)
			continue
		}

		// 2. If not, try to discover skills in immediate subdirectories
		entries, err := os.ReadDir(fullPathToSub)
		if err != nil {
			// If we can't read it, we'll just add it and let the installation loop handle the error
			targetSubPaths = append(targetSubPaths, sp)
			continue
		}

		discoveredCount := 0
		for _, entry := range entries {
			if entry.IsDir() {
				if _, err := os.Stat(filepath.Join(fullPathToSub, entry.Name(), data.SkillFile)); err == nil {
					targetSubPaths = append(targetSubPaths, filepath.Join(sp, entry.Name()))
					discoveredCount++
				}
			}
		}

		// If no skills were found in subdirectories, still include the original path
		// so that the main installation logic can report the "SKILL.md not found" error properly.
		if discoveredCount == 0 {
			targetSubPaths = append(targetSubPaths, sp)
		}
	}

	return targetSubPaths, nil
}

// skillsUninstallCmd removes an installed skill
var skillsUninstallCmd = &cobra.Command{
	Use:     "uninstall <name>",
	Aliases: []string{"rm", "remove"},
	Short:   "Uninstall an installed skill",
	Long:    `Uninstall a skill by deleting its directory from the skills storage.`,
	Run: func(cmd *cobra.Command, args []string) {
		var skillName string

		if len(args) > 0 {
			skillName = args[0]
		} else {
			// Interactive select
			skills, err := data.ScanSkills()
			if err != nil {
				util.Errorf("Failed to scan skills: %v\n", err)
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
			ui.SortOptions(options, "")
			height := ui.GetTermFitHeight(len(options))

			err = huh.NewSelect[string]().
				Title("Select skill to uninstall").
				Description("Choose the skill you wish to remove from your system").
				Height(height).
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
			util.Errorf("Failed to scan skills: %v\n", err)
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
			util.Errorf("Skill '%s' not found\n", skillName)
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
			util.Errorf("Failed to remove skill: %v\n", err)
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
			util.Errorf("Failed to scan skills: %v\n", err)
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

		ui.SortMultiOptions(options, enabledSkills)
		height := ui.GetTermFitHeight(len(options))

		var selectedSkills []string
		multiSelect := huh.NewMultiSelect[string]().
			Title("Select Skills").
			Description("Choose which skills to enable. Press space to toggle, enter to confirm.").
			Height(height).
			Options(options...).
			Value(&selectedSkills)
		note := ui.GetDynamicHuhNote("Skill Description", multiSelect, func(name string) string {
			for _, s := range skills {
				if s.Name == name {
					return s.Description
				}
			}
			return ""
		})
		err = huh.NewForm(
			huh.NewGroup(
				multiSelect,
				note,
			),
		).Run()

		if err != nil {
			util.Errorf("%v\n", err)
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
		fmt.Printf("%d skill(s) enabled.\n", len(selectedSkills))
		fmt.Println()
		skillsListCmd.Run(skillsListCmd, args)
	},
}

// skillsUpdateCmd updates an installed skill
var skillsUpdateCmd = &cobra.Command{
	Use:     "update [name]",
	Aliases: []string{"up"},
	Short:   "Update an installed skill from its original source",
	Long: `Update a skill by re-downloading it from its original source (e.g., a GitHub repository).
This command requires that the skill was originally installed from a remote URL.

Use 'gllm skills update <name>' to update a specific skill.
Use 'gllm skills update --all' to update all skills that support updating.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Stop any global indicator to prevent UI interference
		ui.GetIndicator().Stop()

		if !skillsUpdateAll && len(args) == 0 {
			util.Errorf("You must specify a skill name or use the --all flag.\n")
			return
		}
		if skillsUpdateAll && len(args) > 0 {
			util.Errorf("Cannot specify both a skill name and the --all flag.\n")
			return
		}

		skills, err := data.ScanSkills()
		if err != nil {
			util.Errorf("Failed to scan skills: %v\n", err)
			return
		}

		// Filter skills that actually have update metadata
		var updatableSkills []data.SkillMetadata
		for _, s := range skills {
			if s.SourceMeta != nil {
				updatableSkills = append(updatableSkills, s)
			}
		}

		if len(updatableSkills) == 0 {
			fmt.Println("No updatable skills found. (Only skills installed from a URL can be updated)")
			return
		}

		if skillsUpdateAll {
			// Update all skills with batched processing
			// First prompt: Summary
			var names []string
			for _, s := range updatableSkills {
				names = append(names, s.Name)
			}
			sort.Strings(names)

			var confirm1 bool
			err := huh.NewConfirm().
				Title(fmt.Sprintf("Found %d skills with source tracking. Update all?", len(updatableSkills))).
				Description(fmt.Sprintf("%s", strings.Join(names, ", "))).
				Affirmative("Yes").
				Negative("No").
				Value(&confirm1).
				Run()
			if err != nil || !confirm1 {
				fmt.Println("Aborted.")
				return
			}

			// Second prompt: Destructive warning
			var confirm2 bool
			err = huh.NewConfirm().
				Title("WARNING: This will overwrite any local modifications in ALL selected skills. Are you absolutely sure?").
				Affirmative("Yes, overwrite all").
				Negative("No, abort").
				Value(&confirm2).
				Run()
			if err != nil || !confirm2 {
				fmt.Println("Aborted.")
				return
			}

			// Execute batch update - all skills processed together efficiently
			if err := executeSkillUpdate(updatableSkills...); err != nil {
				util.Errorf("Update failed: %v\n", err)
			}

		} else {
			// Single skill update
			skillName := args[0]
			var targetSkill *data.SkillMetadata

			for _, s := range updatableSkills {
				if strings.EqualFold(s.Name, skillName) {
					targetSkill = &s
					break
				}
			}

			if targetSkill == nil {
				// Check if it exists but just doesn't have metadata to give a better error message
				for _, s := range skills {
					if strings.EqualFold(s.Name, skillName) {
						util.Errorf("Skill holds no origin metadata (likely installed from a local path) and cannot be updated.\n")
						return
					}
				}
				util.Errorf("Skill '%s' not found.\n", skillName)
				return
			}

			var confirm bool
			err := huh.NewConfirm().
				Title(fmt.Sprintf("Updating will overwrite any local modifications to '%s'. Proceed?", targetSkill.Name)).
				Affirmative("Yes, overwrite").
				Negative("No, abort").
				Value(&confirm).
				Run()
			if err != nil || !confirm {
				fmt.Println("Aborted.")
				return
			}

			// Single skill update
			if err := executeSkillUpdate(*targetSkill); err != nil {
				util.Errorf("Failed to update %s: %v\n", targetSkill.Name, err)
			} else {
				// Success message already printed by executeSkillUpdate
			}
		}
	},
}

// downloadRepo downloads or clones a repository to a temporary directory
// This is a shared helper for both install and update commands
func downloadRepo(sourceURL, destDir string) error {
	if util.HasGit() {
		fmt.Printf("Cloning %s...\n", sourceURL)
		gitCmd := exec.Command("git", "clone", sourceURL, destDir)
		gitCmd.Stdout = os.Stdout
		gitCmd.Stderr = os.Stderr
		if err := gitCmd.Run(); err != nil {
			return fmt.Errorf("failed to clone repository: %w", err)
		}
	} else if util.IsGitHubURL(sourceURL) {
		fmt.Printf("Downloading archive from %s...\n", sourceURL)
		zipURL := util.GetGitHubZipURL(sourceURL)
		if err := util.DownloadAndExtractZip(zipURL, destDir); err != nil {
			return fmt.Errorf("failed to download and extract skill: %w", err)
		}
	} else {
		return fmt.Errorf("Git is not installed, and the source is not a standard GitHub repository")
	}
	return nil
}

// executeSkillUpdate handles the download/replace logic for one or more skills
// Uses batch processing for efficiency when multiple skills share the same source URL
func executeSkillUpdate(skills ...data.SkillMetadata) error {
	if len(skills) == 0 {
		return fmt.Errorf("no skills to update")
	}

	// Group skills by SourceURL
	skillsByURL := make(map[string][]data.SkillMetadata)
	for _, skill := range skills {
		if skill.SourceMeta == nil {
			continue
		}
		sourceURL := skill.SourceMeta.SourceURL
		skillsByURL[sourceURL] = append(skillsByURL[sourceURL], skill)
	}

	if len(skillsByURL) == 0 {
		return fmt.Errorf("no valid source metadata found")
	}

	// Process each source URL once
	for sourceURL, groupSkills := range skillsByURL {
		if len(groupSkills) > 1 {
			fmt.Printf("\nProcessing %d skills from %s...\n", len(groupSkills), sourceURL)
		}

		// Create temp directory for this source
		tempDir, err := os.MkdirTemp("", "gllm-skill-update-*")
		if err != nil {
			util.Errorf("Failed to create temp directory for %s: %v\n", sourceURL, err)
			continue
		}

		// Download/clone once per source URL
		if err := downloadRepo(sourceURL, tempDir); err != nil {
			util.Errorf("Failed to download source from %s: %v\n", sourceURL, err)
			os.RemoveAll(tempDir)
			continue
		}

		// Update all skills from this source
		for i, skill := range groupSkills {
			if len(groupSkills) > 1 {
				fmt.Printf("[%d/%d] Updating %s...\n", i+1, len(groupSkills), skill.Name)
			}

			meta := skill.SourceMeta
			if meta == nil {
				util.Errorf("Missing source metadata for %s\n", skill.Name)
				continue
			}

			destDir := filepath.Dir(skill.Location)

			// Resolve sub-path
			absPath := tempDir
			if meta.SubPath != "" {
				absPath = filepath.Join(tempDir, meta.SubPath)
			}

			// Validate new skill
			skillFilePath := filepath.Join(absPath, data.SkillFile)
			if _, err := os.Stat(skillFilePath); err != nil {
				util.Errorf("New version of %s does not contain %s\n", skill.Name, data.SkillFile)
				continue
			}

			// Remove old and copy new
			if err := os.RemoveAll(destDir); err != nil {
				util.Errorf("Failed to clean existing directory for %s: %v\n", skill.Name, err)
				continue
			}

			if err := copyDir(absPath, destDir); err != nil {
				util.Errorf("Failed to copy new files for %s: %v\n", skill.Name, err)
				continue
			}

			// Update metadata timestamp
			meta.InstallDate = time.Now().UTC().Format(time.RFC3339)
			if err := data.SaveSkillSourceMeta(destDir, meta); err != nil {
				util.Warnf("Failed to update metadata for %s: %v\n", skill.Name, err)
			}

			fmt.Printf("Skill '%s' updated successfully.\n", skill.Name)
		}

		// Cleanup temp directory after processing all skills from this source
		os.RemoveAll(tempDir)
	}

	return nil
}

// printSkillMeta prints a skill in a formatted way
func printSkillMeta(skill data.SkillMetadata) {
	settingsStore := data.GetSettingsStore()
	enabled := !settingsStore.IsSkillDisabled(skill.Name)
	indicator := ui.FormatEnabledIndicator(enabled)

	fmt.Printf("  %s %s\n", indicator, skill.Name)
	if skill.Description != "" {
		lines := strings.Split(skill.Description, "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				fmt.Printf("  %s%s%s\n", data.DetailColor, line, data.ResetSeq)
			}
		}
	}
	fmt.Println()
}
