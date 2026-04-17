package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/activebook/gllm/data"
	"github.com/activebook/gllm/internal/ui"
	"github.com/activebook/gllm/io"
	"github.com/charmbracelet/lipgloss"
)

// printReplWelcome renders the dynamic welcome banner with logo, specs, and tips.
func printReplWelcome() {
	termWidth := io.GetTerminalWidth()
	safeWidth := max(40, termWidth-4)

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(data.BorderHex)).
		Width(safeWidth).
		Margin(0, 1).
		Padding(0, 1)

	innerWidth := safeWidth - borderStyle.GetHorizontalFrameSize()

	// --- 1. Logo Panel (Fixed Width) ---
	logo := ui.GetLogo(data.KeyHex, data.LabelHex, 0.5)
	welcomeText := logo + "\nWelcome back!\n" + data.DetailColor + " (v" + version + ")" + data.ResetSeq

	logoWidth := 30 // Approximate width of the 8bitfortress GLLM logo + padding
	logoContent := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(data.KeyHex)).
		Width(logoWidth).
		Align(lipgloss.Center).
		Padding(1, 1, 0, 1).
		Render(welcomeText)

	// --- 2. Specs Panel (Fixed Width) ---
	specs := []string{}
	for cmd, desc := range replSpecMap {
		specs = append(specs, fmt.Sprintf("• %s: %s", cmd, desc))
	}
	sort.Strings(specs)
	specsText := strings.Join(specs, "\n")

	// --- Layout Engine (Breakpoints) ---
	var inner string

	if termWidth >= 100 {
		// Breakpoint 1: Ultrawide (Show all 3 columns)
		specsWidth := 40
		specsContent := lipgloss.NewStyle().
			Foreground(lipgloss.Color(data.LabelHex)).
			Width(specsWidth).
			Align(lipgloss.Left).
			Padding(0, 0, 0, 2).
			Render(specsText)

		// Calculate remaining width for tips
		tipsWidth := innerWidth - logoWidth - specsWidth
		if tipsWidth < 20 {
			tipsWidth = 20 // minimum viable width
		}

		maxHeight := lipgloss.Height(specsContent)
		if lipgloss.Height(logoContent) > maxHeight {
			maxHeight = lipgloss.Height(logoContent)
		}

		tipsStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(data.DetailHex)).
			Width(tipsWidth).
			Align(lipgloss.Left).
			Padding(0, 0, 0, 2)

		tips := getRandomTips(6)
		var validTipsLines []string
		validTipsLines = append(validTipsLines, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(data.KeyHex)).Render("💡 Pro Tips:"))

		for _, tip := range tips {
			testLines := append(validTipsLines, fmt.Sprintf("• %s", tip))
			testContent := tipsStyle.Render(strings.Join(testLines, "\n"))
			if lipgloss.Height(testContent) > maxHeight {
				break
			}
			validTipsLines = testLines
		}
		tipsContent := tipsStyle.Render(strings.Join(validTipsLines, "\n"))

		inner = lipgloss.JoinHorizontal(
			lipgloss.Top,
			logoContent,
			specsContent,
			tipsContent,
		)
	} else if termWidth >= 80 {
		// Breakpoint 2: Wide (Show Logo + Specs, hide Tips)
		flexibleSpecs := lipgloss.NewStyle().
			Foreground(lipgloss.Color(data.LabelHex)).
			Width(innerWidth-logoWidth).
			Align(lipgloss.Left).
			Padding(0, 0, 0, 2).
			Render(specsText)

		inner = lipgloss.JoinHorizontal(
			lipgloss.Top,
			logoContent,
			flexibleSpecs,
		)
	} else if termWidth >= 60 {
		// Breakpoint 3: Medium (Stack Logo over Specs, show Tips on right)
		leftColumnWidth := 38
		rightColumnWidth := innerWidth - leftColumnWidth

		stackedLogo := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(data.KeyHex)).
			Width(leftColumnWidth).
			Align(lipgloss.Left).
			Padding(0, 0, 0, 0).
			Render(welcomeText)

		stackedSpecs := lipgloss.NewStyle().
			Foreground(lipgloss.Color(data.LabelHex)).
			Width(leftColumnWidth).
			Align(lipgloss.Left).
			Padding(1, 0, 0, 0).
			Render(specsText)

		leftColumn := lipgloss.JoinVertical(
			lipgloss.Left,
			stackedLogo,
			stackedSpecs,
		)

		maxHeight := lipgloss.Height(leftColumn)

		tipsStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(data.DetailHex)).
			Width(rightColumnWidth).
			Align(lipgloss.Left).
			Padding(0, 0, 0, 1)

		tips := getRandomTips(5)
		var validTipsLines []string
		validTipsLines = append(validTipsLines, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(data.KeyHex)).Render("💡 Pro Tips:"))

		for _, tip := range tips {
			testLines := append(validTipsLines, fmt.Sprintf("• %s", tip))
			testContent := tipsStyle.Render(strings.Join(testLines, "\n"))
			if lipgloss.Height(testContent) > maxHeight+2 {
				break
			}
			validTipsLines = testLines
		}
		stackedTips := tipsStyle.Render(strings.Join(validTipsLines, "\n"))

		inner = lipgloss.JoinHorizontal(
			lipgloss.Top,
			leftColumn,
			stackedTips,
		)
	} else {
		// Breakpoint 4: Narrow (Stack Logo over Specs, hide Tips)
		stackedLogo := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(data.KeyHex)).
			Width(innerWidth).
			Align(lipgloss.Left).
			Padding(0, 0, 0, 0).
			Render(welcomeText)

		stackedSpecs := lipgloss.NewStyle().
			Foreground(lipgloss.Color(data.LabelHex)).
			Width(innerWidth).
			Align(lipgloss.Left).
			Padding(1, 0, 0, 0).
			Render(specsText)

		inner = lipgloss.JoinVertical(
			lipgloss.Left,
			stackedLogo,
			stackedSpecs,
		)
	}

	banner := borderStyle.Render(inner)
	fmt.Println(banner)

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(data.DetailHex)).
		Width(safeWidth).
		Align(lipgloss.Center).
		Italic(true)
	fmt.Println(hintStyle.Padding(0, 2).Render("Type your message below and press Enter to send."))
	fmt.Println()
}
