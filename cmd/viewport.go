package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wrap"
)

var (
	titleStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Right = "├"
		return lipgloss.NewStyle().BorderStyle(b).Padding(0, 1)
	}()

	infoStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Left = "┤"
		return lipgloss.NewStyle().BorderStyle(b).Padding(0, 1)
	}()
)

type ViewportModel struct {
	viewport   viewport.Model
	provider   string
	content    string
	ready      bool
	headerFunc func() string
}

func NewViewportModel(provider string, content string, headerFunc func() string) ViewportModel {
	return ViewportModel{
		provider:   provider,
		content:    content,
		headerFunc: headerFunc,
	}
}

func (m ViewportModel) Init() tea.Cmd {
	return nil
}

func (m ViewportModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if k := msg.String(); k == "ctrl+c" || k == "q" || k == "esc" {
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		headerHeight := lipgloss.Height(m.headerView())
		footerHeight := lipgloss.Height(m.footerView())
		verticalMarginHeight := headerHeight + footerHeight

		// Fix Details: I switched from wordwrap to github.com/muesli/reflow/wrap.
		// The wrap package uses a hard-wrapping strategy that correctly calculates character widths (handling East Asian Widths),
		// so lines will now break correctly for Chinese, Japanese, and other multi-byte characters.
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			m.viewport.YPosition = headerHeight
			wrappedContent := wrapWithIndentation(m.content, msg.Width)
			m.viewport.SetContent(wrappedContent)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMarginHeight
			wrappedContent := wrapWithIndentation(m.content, msg.Width)
			m.viewport.SetContent(wrappedContent)
		}
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func wrapWithIndentation(s string, width int) string {
	lines := strings.Split(s, "\n")
	var wrappedLines []string
	for _, line := range lines {
		if lipgloss.Width(line) <= width {
			wrappedLines = append(wrappedLines, line)
			continue
		}

		// Find leading whitespace
		var indent string
		for _, r := range line {
			if r == ' ' || r == '\t' {
				indent += string(r)
			} else {
				break
			}
		}

		indentWidth := lipgloss.Width(indent)
		// If indent is too wide, or we have no content after indent, just wrap normally
		if indentWidth >= width-10 || indentWidth >= width {
			wrappedLines = append(wrappedLines, wrap.String(line, width))
			continue
		}

		content := line[len(indent):]
		wrappedContent := wrap.String(content, width-indentWidth)
		contentLines := strings.Split(wrappedContent, "\n")

		for _, cl := range contentLines {
			wrappedLines = append(wrappedLines, indent+cl)
		}
	}
	return strings.Join(wrappedLines, "\n")
}

func (m ViewportModel) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}
	return fmt.Sprintf("%s\n%s\n%s", m.headerView(), m.viewport.View(), m.footerView())
}

func (m ViewportModel) headerView() string {
	title := titleStyle.Render(m.headerFunc())
	pdinfo := fmt.Sprintf("── [%s] ──", m.provider)
	line := strings.Repeat("─", max(0, m.viewport.Width-lipgloss.Width(title)-lipgloss.Width(pdinfo)))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, line, pdinfo)
}

func (m ViewportModel) footerView() string {
	info := infoStyle.Render(fmt.Sprintf("%3.f%%", m.viewport.ScrollPercent()*100))
	tips := "─ space/f/d: Next • b/u: Prev • j/k: Scroll • q: Quit ─"
	line := strings.Repeat("─", max(0, m.viewport.Width-lipgloss.Width(info)-lipgloss.Width(tips)))
	return lipgloss.JoinHorizontal(lipgloss.Center, line, tips, info)
}
