// File: cmd/update.go
// Implements the `gllm update` subcommand and a background update-check goroutine.
package cmd

import (
	"fmt"
	"runtime"
	"time"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/internal/ui"
	"github.com/activebook/gllm/service"
	"github.com/activebook/gllm/util"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// updateCmd is the `gllm update` subcommand.
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Check for and apply the latest gllm update",
	Long:  `Queries GitHub Releases for a newer version of gllm and, with your confirmation, replaces the running binary in place.`,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return runUpdate(cmd, true)
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

// StartBackgroundUpdateCheck launches a single goroutine that checks for
// updates if 24 hours have elapsed since the last check.
// The result is stored in pendingUpdateVersion for non-intrusive display.
func StartBackgroundUpdateCheck() {
	go func() {
		ss := data.GetSettingsStore()
		// Check if 24 hours have elapsed since the last check.
		if time.Since(ss.GetLastUpdateCheck()) < 24*time.Hour {
			return
		}
		// Perform the check quietly; do not log on error.
		util.LogDebugf("Current version is:%s\n", version)
		release, err := service.CheckLatest(version)
		if err != nil {
			ss.SetLastUpdateCheck(time.Now())
			UpdateWarn(err.Error())
			return
		}
		// Always record the time so we don't hammer GitHub on every start.
		_ = ss.SetLastUpdateCheck(time.Now())
		if release.Newer {
			UpdateVersion(release.Version)
		}
	}()
}

// runUpdate performs the explicit update flow.
// interactive => prompt for confirmation via huh; otherwise auto-apply.
func runUpdate(cmd *cobra.Command, interactive bool) error {
	util.Printf(cmd, "Current version: %s\n", version)

	ui.GetIndicator().Start(ui.IndicatorCheckingUpdate)
	release, err := service.CheckLatest(version)
	ui.GetIndicator().Stop()
	if err != nil {
		return fmt.Errorf("update check failed: %w", err)
	}

	if !release.Newer {
		util.LogSuccessf("You are already on the latest version (%s).\n", release.Version)
		util.Print(cmd, renderAlternativeUpdateInstructions())
		return nil
	}

	util.Printf(cmd, "New version available: %s\n", release.Version)
	util.Print(cmd, renderAlternativeUpdateInstructions())
	util.Println(cmd)

	if interactive {
		var confirmed bool
		err = huh.NewConfirm().
			Title(fmt.Sprintf("Update gllm to %s?", release.Version)).
			Description("The binary will be replaced in place. Restart gllm after updating.").
			Value(&confirmed).
			Run()
		if err != nil || !confirmed {
			util.Println(cmd, "Update cancelled.")
			return nil
		}
	}

	ui.GetIndicator().Start(ui.IndicatorInstallingUpdate)
	err = service.ApplyUpdate(release)
	ui.GetIndicator().Stop()
	if err != nil {
		return err
	}

	notifyStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(data.UpdateSuccessHex))
	util.Println(cmd, notifyStyle.Render(fmt.Sprintf("✓ Successfully updated to %s!", release.Version)))
	util.Println(cmd, "Please restart gllm to use the new version.")
	return nil
}

// updateVersion returns a non-intrusive update notification.
func UpdateVersion(latestVersion string) {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color(data.UpdateAvailableHex)).
		Bold(true)
	text := style.Render(fmt.Sprintf("⬆ Update available: %s → %s  (type /update to install)", version, latestVersion))
	ui.SendEvent(ui.BannerMsg{Text: text})
}

// UpdateWarnBanner sends a warning banner to the UI
// The chatinput must be there to receive the banner message
// Otherwise the banner will be lost
// Use for background goroutine warning
func UpdateWarn(warn string) {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color(data.WarnStatusHex)).
		Bold(true)
	text := style.Render(fmt.Sprintf("▲ Warning: %s", warn))
	ui.SendEvent(ui.BannerMsg{Text: text})
}

// renderAlternativeUpdateInstructions builds the platform-specific update hint as a string.
func renderAlternativeUpdateInstructions() string {
	var cmd string
	switch runtime.GOOS {
	case "darwin":
		cmd = "brew update && brew upgrade gllm --cask"
	case "windows":
		cmd = "scoop update gllm"
	default:
		cmd = "curl -fsSL https://raw.githubusercontent.com/activebook/gllm/main/build/install.sh | sh"
	}
	labelStyle := lipgloss.NewStyle().Faint(true)
	codeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(data.KeyHex))
	return fmt.Sprintf("%s %s\n", labelStyle.Render("Alternative update method:"), codeStyle.Render(cmd))
}
