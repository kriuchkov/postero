package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderFooter builds the bottom status bar and keeps its help text scoped to the active layer.
func renderFooter(m Model, width int) string {
	style := m.styles.Footer.Width(width)
	if m.commandActive {
		commandLine := lipgloss.NewStyle().Width(max(width-2, 1)).Render(m.searchInput.View())
		return style.Render(commandLine)
	}

	status := "Ready"
	statusStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.SubText)
	if m.statusMessage != "" {
		status = m.statusMessage
		if m.statusError {
			statusStyle = statusStyle.Foreground(lipgloss.Color("203"))
		} else {
			statusStyle = statusStyle.Foreground(m.styles.Palette.Primary)
		}
	}

	statusText := statusStyle.Render(status)
	if tag := strings.TrimSpace(m.activeTagID); tag != "" {
		statusText = lipgloss.JoinHorizontal(
			lipgloss.Left,
			statusText,
			lipgloss.NewStyle().Foreground(m.styles.Palette.SubText).Render("  •  "),
			footerTagBadgeStyle(m).Render("tag: "+strings.ReplaceAll(tag, "_", " ")),
		)
	}
	if m.state == stateSidebar {
		if legend := sidebarFooterTagLegend(m, max(width/3, 24)); legend != "" {
			statusText = lipgloss.JoinHorizontal(
				lipgloss.Left,
				statusText,
				lipgloss.NewStyle().Foreground(m.styles.Palette.SubText).Render("  •  "),
				legend,
			)
		}
	}
	if badge := footerSearchModeBadge(m); badge != "" {
		statusText = lipgloss.JoinHorizontal(
			lipgloss.Left,
			statusText,
			lipgloss.NewStyle().Foreground(m.styles.Palette.SubText).Render("  •  "),
			badge,
		)
	}
	if loading := footerLoadingBadge(m); loading != "" {
		statusText = lipgloss.JoinHorizontal(
			lipgloss.Left,
			statusText,
			lipgloss.NewStyle().Foreground(m.styles.Palette.SubText).Render("  •  "),
			loading,
		)
	}

	helpText := footerHelpText(m, width)
	helpStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.SubText)
	return style.Render(joinHeaderColumns(width, statusText, helpStyle.Render(helpText)))
}

func footerSearchModeBadge(m Model) string {
	if !m.searchActive && strings.TrimSpace(m.searchQuery) == "" {
		return ""
	}
	label := "backend search"
	if m.searchDebouncing {
		label = "backend search pending"
	}
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(m.styles.Palette.Text).
		Background(m.styles.Palette.Faint).
		Padding(0, 1).
		Render(label)
}

func footerLoadingBadge(m Model) string {
	if !m.messagesLoading || m.commandActive || m.state == stateCompose {
		return ""
	}
	frame := loadingFrames[m.loadingFrame%len(loadingFrames)]
	label := frame + " loading"
	if m.fetchOffset > 0 {
		label = frame + " loading more"
	}
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(m.styles.Palette.Highlight).
		Background(m.styles.Palette.Primary).
		Padding(0, 1).
		Render(label)
}

func footerTagBadgeStyle(m Model) lipgloss.Style {
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(m.styles.Palette.Highlight).
		Background(m.styles.Palette.Secondary).
		Padding(0, 1)
	if strings.TrimSpace(m.searchQuery) != "" || m.searchActive {
		return style.
			Foreground(m.styles.Palette.Text).
			Background(m.styles.Palette.Faint)
	}
	if strings.TrimSpace(m.activeAccountID) != "" {
		return style.
			Foreground(m.styles.Palette.Highlight).
			Background(m.styles.Palette.Primary)
	}
	return style
}

func footerHelpText(m Model, width int) string {
	candidates := footerHelpCandidates(m)
	maxWidth := max(width-24, 24)
	for _, candidate := range candidates {
		if lipgloss.Width(candidate) <= maxWidth {
			return candidate
		}
	}
	return m.help.ShortHelpView(m.keys.ShortHelp())
}

// footerHelpCandidates derives footer help from
// the current interaction mode instead of using one global legend.
func footerHelpCandidates(m Model) []string {
	if m.state == stateCompose {
		return composeFooterHelpCandidates(m)
	}

	if m.commandActive {
		return []string{
			"enter run • esc cancel • try compose inbox drafts refresh quit",
			"enter run • esc cancel • compose inbox quit",
		}
	}

	if m.searchActive {
		return []string{
			"type to backend-search • enter apply now • esc clear • n/N move through results",
			"enter apply • esc clear • n/N move",
		}
	}

	hasSearch := strings.TrimSpace(m.searchQuery) != ""
	hasSelection := footerHasSelection(m)
	canOpen := m.state == stateSidebar || (m.state == stateList && hasSelection)
	canUsePaneKeys := m.state == stateList
	canUseListViewportKeys := m.state == stateList && hasSelection
	canScroll := m.state == stateContent
	canMessageActions := hasSelection && (m.state == stateList || m.state == stateContent)
	canUndo := m.pendingUndo != nil
	clearScope := footerClearScopeHint(m)

	switch m.state {
	case stateSidebar:
		long := footerJoinSegments(
			"j/k move",
			footerIf(canOpen, "enter/l open"),
			footerIf(hasSearch, "n/N results"),
			"gg/G",
			clearScope,
			footerIf(canUndo, "u undo"),
			"/ search",
			": commands",
		)
		short := footerJoinSegments(
			"j/k",
			footerIf(canOpen, "enter/l"),
			footerIf(hasSearch, "n/N"),
			"gg/G",
			footerIf(canUndo, "u"),
			"/",
			": cmd",
		)
		return []string{long, short}
	case stateList:
		long := footerJoinSegments(
			"j/k move",
			footerIf(hasSearch, "n/N next hit"),
			footerIf(canOpen, "enter/l read"),
			footerIf(canUsePaneKeys, "h/l panes"),
			"gg/G",
			footerIf(canUseListViewportKeys, "H/M/L"),
			footerIf(hasSelection, "ctrl+d/u"),
			footerIf(canMessageActions, "r/R/f"),
			footerIf(canMessageActions, "a/!/d"),
			footerIf(canUndo, "u undo"),
			"/ search",
			": commands",
		)
		short := footerJoinSegments(
			"j/k",
			footerIf(hasSearch, "n/N"),
			footerIf(canOpen, "enter/l"),
			footerIf(canUsePaneKeys, "h/l"),
			"gg/G",
			footerIf(canUseListViewportKeys, "H/M/L"),
			footerIf(hasSelection, "ctrl+d/u"),
			footerIf(canMessageActions, "r/f"),
			footerIf(canMessageActions, "a/!/d"),
			footerIf(canUndo, "u"),
			"/",
			": cmd",
		)
		compact := footerJoinSegments(
			"j/k",
			footerIf(hasSearch, "n/N"),
			footerIf(canOpen, "enter/l"),
			footerIf(canUsePaneKeys, "h/l"),
			"gg/G",
			footerIf(hasSelection, "ctrl+d/u"),
			footerIf(canMessageActions, "r/f"),
			footerIf(canMessageActions, "d"),
			"/",
		)
		return []string{long, short, compact}
	case stateContent:
		long := footerJoinSegments(
			footerIf(canScroll, "j/k scroll"),
			footerIf(hasSearch, "n/N next hit"),
			"gg/G",
			footerIf(hasSelection, "ctrl+d/u"),
			"h back",
			footerIf(canMessageActions, "r/R/f"),
			footerIf(canMessageActions, "a/!/d"),
			footerIf(canUndo, "u undo"),
			"/ search",
			": commands",
		)
		short := footerJoinSegments(
			footerIf(canScroll, "j/k"),
			footerIf(hasSearch, "n/N"),
			"gg/G",
			footerIf(hasSelection, "ctrl+d/u"),
			"h",
			footerIf(canMessageActions, "r/f"),
			footerIf(canMessageActions, "a/!/d"),
			footerIf(canUndo, "u"),
			"/",
			": cmd",
		)
		compact := footerJoinSegments(
			footerIf(canScroll, "j/k"),
			footerIf(hasSearch, "n/N"),
			"gg/G",
			footerIf(hasSelection, "ctrl+d/u"),
			"h",
			footerIf(canMessageActions, "r/f"),
		)
		return []string{long, short, compact}
	case stateCompose:
		return []string{m.help.ShortHelpView(m.keys.ShortHelp())}
	default:
		return []string{m.help.ShortHelpView(m.keys.ShortHelp())}
	}
}

func footerHasSelection(m Model) bool {
	return len(m.messages) > 0 && m.listCursor >= 0 && m.listCursor < len(m.messages) && m.messages[m.listCursor] != nil
}

func footerClearScopeHint(m Model) string {
	if m.state != stateSidebar {
		return ""
	}
	if strings.TrimSpace(m.activeTagID) != "" {
		return "esc clear tag"
	}
	if strings.TrimSpace(m.activeAccountID) != "" {
		return "esc clear scope"
	}
	if strings.TrimSpace(m.searchQuery) != "" {
		return "esc clear"
	}
	return ""
}

func footerIf(condition bool, segment string) string {
	if !condition {
		return ""
	}
	return segment
}

func footerJoinSegments(segments ...string) string {
	filtered := make([]string, 0, len(segments))
	for _, segment := range segments {
		if strings.TrimSpace(segment) == "" {
			continue
		}
		filtered = append(filtered, segment)
	}
	return strings.Join(filtered, " • ")
}

func sidebarFooterTagHelp(m Model) (string, string) {
	rows := sidebarTags(m)
	if len(rows) == 0 {
		return "tags none", "tags none"
	}
	longParts := make([]string, 0, min(len(rows), 4))
	shortKeys := make([]string, 0, min(len(rows), 6))
	for index, row := range rows {
		key := strings.ToLower(string(row.hotkey))
		if index < 4 {
			longParts = append(longParts, key+" "+row.label)
		}
		if index < 6 {
			shortKeys = append(shortKeys, key)
		}
	}
	long := "tags " + strings.Join(longParts, " • ")
	if len(rows) > 4 {
		long += " • …"
	}
	short := "tags " + strings.Join(shortKeys, "/")
	return long, short
}

func sidebarFooterTagLegend(m Model, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	long, short := sidebarFooterTagHelp(m)
	style := footerTagLegendStyle(m)
	for _, candidate := range []string{long, short} {
		rendered := style.Render(candidate)
		if lipgloss.Width(rendered) <= maxWidth {
			return rendered
		}
	}
	if short == "tags none" {
		return style.Render(short)
	}
	return style.Render("tags")
}

func footerTagLegendStyle(m Model) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(m.styles.Palette.Text).
		Background(m.styles.Palette.Faint).
		Padding(0, 1)
}

func composeFooterHelpCandidates(m Model) []string {
	if m.composeEditing {
		return composeEditingFooterHelpCandidates(m)
	}

	switch m.focusIndex {
	case 0:
		return []string{
			"h/l acct • enter next • j/k fields • tab next • gg/G • 0/$ • ctrl+o save • ctrl+x send",
			"h/l acct • enter next • j/k • gg/G • 0/$ • ctrl+o save • ctrl+x send",
			"h/l acct • enter • j/k • ctrl+o save • ctrl+x send",
		}
	case 3:
		return []string{
			"o/O body • enter edit • j/k fields • tab next • gg/G • 0/$ • ctrl+o save • ctrl+x send",
			"o/O body • enter edit • j/k • gg/G • 0/$ • ctrl+o save • ctrl+x send",
			"o/O body • enter • j/k • ctrl+o save • ctrl+x send",
		}
	default:
		return []string{
			"enter/i edit • j/k fields • o/O body • tab next • gg/G • 0/$ • ctrl+o save • ctrl+x send",
			"enter/i edit • j/k • o/O body • gg/G • 0/$ • ctrl+o save • ctrl+x send",
			"enter/i edit • j/k • ctrl+o save • ctrl+x send",
		}
	}
}

func composeEditingFooterHelpCandidates(m Model) []string {
	enterAction := "next"
	if m.focusIndex == 3 {
		enterAction = "newline"
	}

	return []string{
		"esc normal • enter " + enterAction + " • ctrl+o save • ctrl+x send",
		"esc normal • ctrl+o save • ctrl+x send",
	}
}
