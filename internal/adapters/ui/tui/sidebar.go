package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func renderSidebar(m Model, width, height int) string {
	style := m.styles.Sidebar.Width(width).Height(height)
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(m.styles.Palette.Text)
	sectionStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.SubText).Bold(true).MarginTop(1)
	itemStyle := sidebarItemStyle(m, false, false)
	selectedStyle := sidebarItemStyle(m, true, false)
	activeStyle := sidebarItemStyle(m, false, false).Foreground(m.styles.Palette.Primary).Bold(true)
	mutedStyle := sidebarItemStyle(m, false, true)

	lines := []string{titleStyle.Render("Mailboxes")}
	favoritesHeaderShown := false
	accountsHeaderShown := false
	for i, item := range m.sidebarItems {
		if item == "" {
			lines = append(lines, "")
			continue
		}
		trimmed := strings.TrimSpace(item)
		if !favoritesHeaderShown && trimmed != "Accounts:" && !strings.HasPrefix(item, "  ") {
			lines = append(lines, sectionStyle.Render("Favorites"))
			favoritesHeaderShown = true
		}
		if trimmed == "Accounts:" {
			lines = append(lines, sectionStyle.Render("Accounts"))
			accountsHeaderShown = true
			continue
		}

		icon := sidebarIcon(trimmed)
		label := fmt.Sprintf("%s %s", icon, trimmed)
		if strings.HasPrefix(item, "  ") {
			label = "  " + label
		}

		rendered := itemStyle.Render(label)
		if m.sidebarCursor == i {
			rendered = selectedStyle.Render(label)
		} else if strings.HasPrefix(item, "  ") && strings.EqualFold(strings.TrimSpace(item), strings.TrimSpace(m.activeAccountID)) {
			rendered = activeStyle.Render(label)
		} else if accountsHeaderShown && strings.HasPrefix(item, "  ") {
			rendered = mutedStyle.Render(label)
		} else {
			rendered = itemStyle.Render(label)
		}
		lines = append(lines, rendered)
	}

	return style.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func sidebarItemStyle(m Model, selected, muted bool) lipgloss.Style {
	style := lipgloss.NewStyle().Padding(0, 1).Foreground(m.styles.Palette.Text)
	if muted {
		style = style.Foreground(m.styles.Palette.SubText)
	}
	if selected {
		style = style.
			Foreground(m.styles.Palette.Highlight).
			Background(m.styles.Palette.Primary).
			Bold(true)
	}
	return style
}

func sidebarIcon(item string) string {
	switch item {
	case "Inbox":
		return "◎"
	case "Sent":
		return "↗"
	case "Drafts":
		return "✎"
	case "Archive":
		return "▣"
	case "Trash":
		return "⌫"
	case "Spam":
		return "!"
	default:
		return "•"
	}
}
