package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func renderHeader(m Model, width int) string {
	style := m.styles.Header.Width(width)

	appTitleStyle := lipgloss.NewStyle().Bold(true).Foreground(m.styles.Palette.Highlight)
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(m.styles.Palette.Text)
	pillStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.Text).Background(m.styles.Palette.Faint).Padding(0, 1)
	primaryPillStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.Highlight).Background(m.styles.Palette.Primary).Padding(0, 1)
	mutedPillStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.SubText).Background(m.styles.Palette.Secondary).Padding(0, 1)
	searchStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.SubText)
	searchBadgeStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.Text).Background(m.styles.Palette.Faint).Padding(0, 1)

	if m.state == stateCompose {
		return style.Render(renderComposeHeader(m, width, appTitleStyle, titleStyle))
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
		searchLabel := backendSearchModeLabel(m)
		left = lipgloss.JoinVertical(
			lipgloss.Left,
			left,
			lipgloss.JoinHorizontal(
				lipgloss.Left,
				searchBadgeStyle.Render("/ "+searchDisplayValue(m)),
				" ",
				searchStyle.Render(searchLabel),
			),
		)
	}

	coreActions := []string{primaryPillStyle.Render("c Compose")}
	searchPill := mutedPillStyle.Render("/ Search")
	if m.searchActive || strings.TrimSpace(m.searchQuery) != "" {
		searchPill = pillStyle.Render("/ Search")
	}
	coreActions = append(coreActions, searchPill)
	commandPill := mutedPillStyle.Render(": Cmd")
	if m.commandActive {
		commandPill = pillStyle.Render(": Cmd")
	}
	coreActions = append(coreActions, commandPill)
	if m.pendingUndo != nil {
		coreActions = append(coreActions, pillStyle.Render("u Undo"))
	}

	messageActions := []string{}
	if hasSelection {
		messageActions = append(messageActions,
			pillStyle.Render("r Reply"),
			pillStyle.Render("R All"),
			pillStyle.Render("f Forward"),
			pillStyle.Render("a Archive"),
			pillStyle.Render("! Spam"),
			pillStyle.Render("d Trash"),
		)
	}
	if hasDraftSelection {
		messageActions = append(messageActions, pillStyle.Render("Enter Edit Draft"))
	}
	actions := append(coreActions, messageActions...)
	actionWidth := max(width-lipgloss.Width(left)-4, 28)
	right := renderBrowseHeaderActions(actionWidth, actions)
	placed := joinHeaderColumns(width, left, right)
	return style.Render(placed)
}

func renderBrowseHeaderActions(maxWidth int, groups ...[]string) string {
	lines := make([]string, 0, len(groups))
	for _, group := range groups {
		wrapped := wrapHeaderPills(group, maxWidth)
		if len(wrapped) == 0 {
			continue
		}
		lines = append(lines, wrapped...)
	}
	if len(lines) == 0 {
		return ""
	}
	return lipgloss.JoinVertical(lipgloss.Right, lines...)
}

func wrapHeaderPills(pills []string, maxWidth int) []string {
	if len(pills) == 0 {
		return nil
	}
	if maxWidth < 1 {
		maxWidth = 1
	}
	rows := make([]string, 0, 2)
	current := make([]string, 0, len(pills))
	currentWidth := 0
	for _, pill := range pills {
		pillWidth := lipgloss.Width(pill)
		candidateWidth := pillWidth
		if len(current) > 0 {
			candidateWidth = currentWidth + 1 + pillWidth
		}
		if len(current) > 0 && candidateWidth > maxWidth {
			rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Left, interleaveHeaderPills(current)...))
			current = []string{pill}
			currentWidth = pillWidth
			continue
		}
		current = append(current, pill)
		currentWidth = candidateWidth
	}
	if len(current) > 0 {
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Left, interleaveHeaderPills(current)...))
	}
	return rows
}

func interleaveHeaderPills(pills []string) []string {
	if len(pills) == 0 {
		return nil
	}
	parts := make([]string, 0, len(pills)*2-1)
	for index, pill := range pills {
		if index > 0 {
			parts = append(parts, " ")
		}
		parts = append(parts, pill)
	}
	return parts
}

func renderComposeHeader(m Model, width int, appTitleStyle, titleStyle lipgloss.Style) string {
	composeTitle := "New Message"
	if m.activeDraft != nil && m.activeDraft.ID != "" {
		composeTitle = "Edit Draft"
	}
	left := lipgloss.JoinVertical(
		lipgloss.Left,
		appTitleStyle.Render("Postero"),
		titleStyle.Render(composeTitle+"  •  "+m.composeAccountLabel()),
	)
	right := lipgloss.JoinHorizontal(lipgloss.Left, composeHeaderActions(m, width)...)
	return joinHeaderColumns(width, left, right)
}

type composeHeaderContext struct {
	emphasizeMode    bool
	showAccount      bool
	emphasizeAccount bool
	showBody         bool
	emphasizeBody    bool
}

type composeHeaderActionTone int

const (
	composeActionNeutral composeHeaderActionTone = iota
	composeActionCalm
	composeActionAccent
	composeActionSecondary
)

type composeHeaderActionSpec struct {
	key       string
	action    string
	tone      composeHeaderActionTone
	emphasize bool
}

func currentComposeHeaderContext(m Model) composeHeaderContext {
	if m.composeEditing {
		return composeHeaderContext{emphasizeMode: true}
	}

	context := composeHeaderContext{
		showBody: true,
	}

	switch m.focusIndex {
	case 0:
		context.showAccount = true
		context.emphasizeAccount = true
	case 3:
		context.emphasizeBody = true
	default:
		context.emphasizeMode = true
	}

	return context
}

func composeHeaderActions(m Model, width int) []string {
	neutralKeyStyle := lipgloss.NewStyle().Bold(true).Foreground(m.styles.Palette.Text).Background(m.styles.Palette.Faint).Padding(0, 1)
	neutralLabelStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.SubText)
	calmKeyStyle := lipgloss.NewStyle().Bold(true).Foreground(m.styles.Palette.Highlight).Background(m.styles.Palette.Secondary).Padding(0, 1)
	calmLabelStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.Text)
	accentKeyStyle := lipgloss.NewStyle().Bold(true).Foreground(m.styles.Palette.Highlight).Background(m.styles.Palette.Primary).Padding(0, 1)
	accentLabelStyle := lipgloss.NewStyle().Bold(true).Foreground(m.styles.Palette.Highlight)
	secondaryKeyStyle := lipgloss.NewStyle().Bold(true).Foreground(m.styles.Palette.Highlight).Background(m.styles.Palette.Secondary).Padding(0, 1)
	secondaryLabelStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.SubText)
	focusedSecondaryKeyStyle := lipgloss.NewStyle().Bold(true).Foreground(m.styles.Palette.Highlight).Background(m.styles.Palette.Primary).Padding(0, 1)
	focusedSecondaryLabelStyle := lipgloss.NewStyle().Bold(true).Foreground(m.styles.Palette.Text)

	actions := make([]string, 0, 6)
	for _, spec := range composeHeaderActionSpecs(m, width) {
		keyStyle := secondaryKeyStyle
		labelStyle := secondaryLabelStyle
		switch spec.tone {
		case composeActionNeutral:
			keyStyle = neutralKeyStyle
			labelStyle = neutralLabelStyle
		case composeActionCalm:
			keyStyle = calmKeyStyle
			labelStyle = calmLabelStyle
		case composeActionAccent:
			keyStyle = accentKeyStyle
			labelStyle = accentLabelStyle
		case composeActionSecondary:
			if spec.emphasize {
				keyStyle = focusedSecondaryKeyStyle
				labelStyle = focusedSecondaryLabelStyle
			}
		}
		actions = append(actions, lipgloss.NewStyle().MarginLeft(1).Render(composeHeaderHint(spec.key, spec.action, keyStyle, labelStyle)))
	}

	if len(actions) > 0 {
		actions[0] = strings.TrimLeft(actions[0], " ")
	}
	return actions
}

func composeHeaderActionSpecs(m Model, width int) []composeHeaderActionSpec {
	context := currentComposeHeaderContext(m)

	saveKey := "CTRL+O"
	sendKey := "CTRL+X"
	if width < 84 {
		saveKey = "^O"
		sendKey = "^X"
	}

	actions := []composeHeaderActionSpec{
		{key: "ESC", action: composeEscAction(m), tone: composeActionNeutral},
		{key: saveKey, action: "save", tone: composeActionCalm},
		{key: sendKey, action: "send", tone: composeActionAccent},
	}

	modeKey := composeHeaderModeKey(m)
	modeAction := composeHeaderModeAction(m)
	if modeKey != "" && modeAction != "" {
		actions = append(actions, composeHeaderActionSpec{key: modeKey, action: modeAction, tone: composeActionSecondary, emphasize: context.emphasizeMode})
	}

	if width >= 110 {
		actions = append(actions, composeWideHeaderActionSpecs(m, context)...)
		return actions
	}

	if width >= 96 {
		if spec, ok := composeCompactHeaderActionSpec(m, context); ok {
			actions = append(actions, spec)
		}
		return actions
	}

	return actions
}

func composeWideHeaderActionSpecs(m Model, context composeHeaderContext) []composeHeaderActionSpec {
	actions := make([]composeHeaderActionSpec, 0, 3)
	if !m.composeEditing {
		actions = append(actions, composeHeaderActionSpec{key: "J/K", action: "move", tone: composeActionSecondary})
	}

	if context.showAccount {
		actions = append(actions, composeHeaderActionSpec{key: "H/L", action: "acct", tone: composeActionSecondary, emphasize: context.emphasizeAccount})
	}

	if context.showBody {
		actions = append(actions, composeHeaderActionSpec{key: "O/O", action: "body", tone: composeActionSecondary, emphasize: context.emphasizeBody})
	}

	return actions
}

func composeCompactHeaderActionSpec(m Model, context composeHeaderContext) (composeHeaderActionSpec, bool) {
	if m.composeEditing {
		return composeHeaderActionSpec{}, false
	}
	if context.showAccount {
		return composeHeaderActionSpec{key: "H/L", action: "acct", tone: composeActionSecondary, emphasize: true}, true
	}
	if context.emphasizeBody {
		return composeHeaderActionSpec{key: "O/O", action: "body", tone: composeActionSecondary, emphasize: true}, true
	}
	return composeHeaderActionSpec{key: "J/K", action: "move", tone: composeActionSecondary}, true
}

func composeHeaderHint(keyLabel, actionLabel string, keyStyle, labelStyle lipgloss.Style) string {
	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		keyStyle.Render(keyLabel),
		" ",
		labelStyle.Render(actionLabel),
	)
}

func composeEscAction(m Model) string {
	if m.composeEditing {
		return "normal"
	}
	return "back"
}

func composeHeaderModeKey(m Model) string {
	if !m.composeEditing {
		return "I"
	}
	if m.focusIndex < 1 {
		return ""
	}
	return "ENTER"
}

func composeHeaderModeAction(m Model) string {
	if !m.composeEditing {
		return "edit"
	}
	if m.focusIndex == 3 {
		return "nl"
	}
	return "next"
}

func currentMailboxTitle(m Model) string {
	if tag := strings.TrimSpace(m.activeTagID); tag != "" {
		title := strings.ReplaceAll(tag, "_", " ")
		if scope := activeAccountScopeLabel(m); scope != "" {
			return title + " • " + scope
		}
		return title
	}
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

func backendSearchModeLabel(m Model) string {
	label := "backend search"
	if m.searchDebouncing {
		return label + "  •  waiting"
	}
	if m.messagesLoading {
		return label + "  •  loading"
	}
	if m.searchActive {
		return label + "  •  editing"
	}
	return label + "  •  active"
}

func joinHeaderColumns(width int, left, right string) string {
	availableWidth := width - 2
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	gap := max(availableWidth-leftWidth-rightWidth, 2)
	spacer := lipgloss.NewStyle().Width(gap).Render("")
	return lipgloss.JoinHorizontal(lipgloss.Top, left, spacer, right)
}
