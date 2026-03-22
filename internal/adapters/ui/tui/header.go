package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func renderHeader(m Model, width, height int) string {
	style := m.styles.Header.Width(width)
	appTitleStyle := lipgloss.NewStyle().Bold(true).Foreground(m.styles.Palette.Highlight)
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(m.styles.Palette.Text)
	pillStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.Text).Background(m.styles.Palette.Faint).Padding(0, 1)
	primaryPillStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.Highlight).Background(m.styles.Palette.Primary).Padding(0, 1)
	mutedPillStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.SubText).Background(m.styles.Palette.Secondary).Padding(0, 1)
	searchStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.SubText)

	if m.state == stateCompose {
		composeTitle := "New Message"
		if m.activeDraft != nil && m.activeDraft.ID != "" {
			composeTitle = "Edit Draft"
		}
		left := lipgloss.JoinVertical(
			lipgloss.Left,
			appTitleStyle.Render("Postero"),
			titleStyle.Render(composeTitle+"  •  "+m.composeAccountLabel()),
		)
		escLabel := "Esc Cancel"
		modeLabel := "i Insert"
		if m.composeEditing {
			escLabel = "Esc Normal"
			if m.focusIndex == 3 {
				modeLabel = "Enter New Line"
			} else {
				modeLabel = "Enter Next Field"
			}
		}
		right := lipgloss.JoinHorizontal(
			lipgloss.Left,
			mutedPillStyle.Render(escLabel),
			lipgloss.NewStyle().MarginLeft(1).Render(mutedPillStyle.Render("Ctrl+O Save")),
			lipgloss.NewStyle().MarginLeft(1).Render(primaryPillStyle.Render("Ctrl+X Send")),
			lipgloss.NewStyle().MarginLeft(1).Render(mutedPillStyle.Render(modeLabel)),
			lipgloss.NewStyle().MarginLeft(1).Render(mutedPillStyle.Render("j/k Fields")),
			lipgloss.NewStyle().MarginLeft(1).Render(mutedPillStyle.Render("h/l Account")),
		)
		return style.Render(joinHeaderColumns(width, left, right))
	}

	hasSelection := len(m.messages) > 0 && m.listCursor >= 0 && m.listCursor < len(m.messages)
	hasDraftSelection := hasSelection && m.messages[m.listCursor] != nil && m.messages[m.listCursor].IsDraft
	mailboxTitle := currentMailboxTitle(m)
	mailboxSubtitle := mailboxSubtitle(m)
	left := lipgloss.JoinVertical(
		lipgloss.Left,
		appTitleStyle.Render("Postero"),
		titleStyle.Render(mailboxTitle+"  •  "+mailboxSubtitle),
	)
	if scope := activeAccountScopeLabel(m); scope != "" {
		left = lipgloss.JoinVertical(
			lipgloss.Left,
			left,
			searchStyle.Render("Account: "+scope+"  •  Esc clears scope"),
		)
	}
	if strings.TrimSpace(m.searchQuery) != "" || m.searchActive {
		searchLabel := "Filter: " + searchDisplayValue(m)
		if m.searchActive {
			searchLabel += "  •  typing"
		}
		left = lipgloss.JoinVertical(lipgloss.Left, left, searchStyle.Render(searchLabel))
	}

	actions := []string{primaryPillStyle.Render("c Compose")}
	searchPill := mutedPillStyle.Render("/ Search")
	if m.searchActive || strings.TrimSpace(m.searchQuery) != "" {
		searchPill = pillStyle.Render("/ Search")
	}
	actions = append(actions, searchPill)
	if m.pendingUndo != nil {
		actions = append(actions, pillStyle.Render("u Undo"))
	}
	if hasSelection {
		actions = append(actions,
			pillStyle.Render("r Reply"),
			pillStyle.Render("R All"),
			pillStyle.Render("f Forward"),
			pillStyle.Render("a Archive"),
			pillStyle.Render("! Spam"),
			pillStyle.Render("d Trash"),
		)
	}
	if hasDraftSelection {
		actions = append(actions, pillStyle.Render("Enter Edit Draft"))
	}
	right := lipgloss.JoinHorizontal(lipgloss.Left, actions...)
	placed := joinHeaderColumns(width, left, right)

	if height > 0 {
		style = style.Height(height)
	}
	return style.Render(placed)
}

func currentMailboxTitle(m Model) string {
	if m.sidebarCursor >= 0 && m.sidebarCursor < len(m.sidebarItems) {
		rawItem := m.sidebarItems[m.sidebarCursor]
		item := strings.TrimSpace(rawItem)
		if item != "" && item != "Accounts:" {
			if scope := activeAccountScopeLabel(m); scope != "" && !strings.HasPrefix(rawItem, "  ") {
				return item + " • " + scope
			}
			return item
		}
	}
	return "All Mail"
}

func activeAccountScopeLabel(m Model) string {
	return strings.TrimSpace(m.activeAccountID)
}

func mailboxSubtitle(m Model) string {
	messageCount := len(m.messages)
	totalCount := len(m.allMessages)
	unreadCount := 0
	for _, msg := range m.messages {
		if msg != nil && !msg.IsRead {
			unreadCount++
		}
	}
	if strings.TrimSpace(m.searchQuery) != "" {
		if unreadCount == 0 {
			return fmt.Sprintf("%d of %d messages", messageCount, totalCount)
		}
		return fmt.Sprintf("%d of %d messages, %d unread", messageCount, totalCount, unreadCount)
	}
	if unreadCount == 0 {
		return fmt.Sprintf("%d messages", messageCount)
	}
	return fmt.Sprintf("%d messages, %d unread", messageCount, unreadCount)
}

func searchDisplayValue(m Model) string {
	if m.searchActive {
		value := strings.TrimSpace(m.searchInput.Value())
		if value != "" {
			return value
		}
	}
	if strings.TrimSpace(m.searchQuery) != "" {
		return m.searchQuery
	}
	return "subject, sender, body"
}

func joinHeaderColumns(width int, left, right string) string {
	availableWidth := width - 2
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	gap := availableWidth - leftWidth - rightWidth
	if gap < 2 {
		gap = 2
	}
	spacer := lipgloss.NewStyle().Width(gap).Render("")
	return lipgloss.JoinHorizontal(lipgloss.Top, left, spacer, right)
}
