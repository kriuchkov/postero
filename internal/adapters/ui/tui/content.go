package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func renderContent(m Model, width, height int) string {
	style := m.styles.Content.Width(width).Height(height)
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(m.styles.Palette.Highlight)

	if m.state == stateCompose {
		return style.Render(renderComposeContent(m, width, height, titleStyle))
	}

	if len(m.messages) == 0 || m.listCursor < 0 || m.listCursor >= len(m.messages) {
		empty := lipgloss.JoinVertical(
			lipgloss.Left,
			titleStyle.Render("Welcome to Postero"),
			lipgloss.NewStyle().Foreground(m.styles.Palette.SubText).Render("Choose a mailbox and select a message to start reading."),
		)
		return style.Render(lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, empty))
	}

	headerBlock, bodyWidth, bodyHeight := contentViewportLayout(m, width, height)
	bodyViewport := m.contentViewport
	bodyViewport.Width = bodyWidth
	bodyViewport.Height = bodyHeight
	bodyViewport.SetContent(m.currentMessageBody())
	bodyViewport.Style = lipgloss.NewStyle().Foreground(m.styles.Palette.Text)
	if bodyViewport.Height < 1 {
		bodyViewport.Height = 1
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		headerBlock,
		bodyViewport.View(),
	)

	return style.Render(content)
}

func renderComposeContent(m Model, width, height int, titleStyle lipgloss.Style) string {
	if m.activeDraft == nil {
		return "Loading draft..."
	}

	labelStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.SubText).Width(8).Align(lipgloss.Right).MarginRight(1)
	modeStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.SubText)

	title, _ := composeContextText(m)
	headerLines := []string{titleStyle.Render(title)}
	headerLines = append(headerLines, modeStyle.Render(composeModeHint(m.composeEditing, width)))
	header := lipgloss.JoinVertical(lipgloss.Left, headerLines...)
	separator := lipgloss.NewStyle().
		Width(width-4).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(m.styles.Palette.Faint).
		MarginTop(1).
		MarginBottom(1).
		Render("")

	m.bodyInput.SetWidth(width - 4)
	m.bodyInput.SetHeight(height - 12)
	bodyView := composeBodyView(m)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"",
		lipgloss.JoinHorizontal(lipgloss.Top, labelStyle.Render("Account:"), composeAccountView(m)),
		lipgloss.JoinHorizontal(lipgloss.Top, labelStyle.Render("To:"), composeFieldView(m, 1, m.toInput.View())),
		lipgloss.JoinHorizontal(lipgloss.Top, labelStyle.Render("Subject:"), composeFieldView(m, 2, m.subjectInput.View())),
		separator,
		bodyView,
	)
}

func composeAccountView(m Model) string {
	accountStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.Text)
	accountHintStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.SubText)
	accountValue := m.composeAccountLabel()
	accountView := accountStyle.Render(accountValue)
	if m.focusIndex != 0 {
		return accountView
	}
	accountView = lipgloss.NewStyle().Foreground(m.styles.Palette.Primary).Bold(true).Render(accountValue)
	return lipgloss.JoinHorizontal(lipgloss.Left, accountView, accountHintStyle.Render("  h/l or ↑/↓ switch"))
}

func composeFieldView(m Model, focusIndex int, view string) string {
	if m.focusIndex != focusIndex {
		return view
	}
	return lipgloss.NewStyle().Foreground(m.styles.Palette.Primary).Render(view)
}

func composeBodyView(m Model) string {
	bodyView := m.bodyInput.View()
	if m.focusIndex != 3 {
		return bodyView
	}
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(m.styles.Palette.Primary).
		Render(bodyView)
}

func composeContextText(m Model) (string, string) {
	title := strings.TrimSpace(m.composeTitle)
	if title == "" {
		title = "Composer"
	}
	hint := strings.TrimSpace(m.composeHint)
	if hint == "" {
		hint = "Write now, save when ready."
	}
	return title, hint
}

func composeModeHint(composeEditing bool, width int) string {
	if composeEditing {
		candidates := []string{
			"Insert. Esc normal.",
			"Insert. Esc normal.",
		}
		for _, candidate := range candidates {
			if lipgloss.Width(candidate) <= max(width-4, 1) {
				return candidate
			}
		}
		return candidates[len(candidates)-1]
	}
	candidates := []string{
		"Normal. Enter/i/o/O edit.",
		"Normal. i/o/O edit.",
	}
	for _, candidate := range candidates {
		if lipgloss.Width(candidate) <= max(width-4, 1) {
			return candidate
		}
	}
	return candidates[len(candidates)-1]
}

func contentViewportLayout(m Model, width, height int) (string, int, int) {
	titleStyle := paneTitleStyle(m, stateContent).MarginBottom(1).Width(max(1, width-4))
	metaLabelStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.SubText).Width(8)
	metaValueStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.Text)
	hintStyle := paneSubtitleStyle(m, stateContent)
	statusStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.Highlight).Background(m.styles.Palette.Faint).Padding(0, 1)
	renderHeader := func(label, value string) string {
		return lipgloss.JoinHorizontal(lipgloss.Top,
			metaLabelStyle.Render(label+":"),
			metaValueStyle.Render(value),
		)
	}

	if len(m.messages) == 0 || m.listCursor < 0 || m.listCursor >= len(m.messages) || m.messages[m.listCursor] == nil {
		return "", max(1, width-4), max(1, height)
	}

	msg := m.messages[m.listCursor]
	from := renderHeader("From", msg.From)
	to := renderHeader("To", strings.Join(msg.To, ", "))
	date := renderHeader("Date", msg.Date.Format("Mon, 02 Jan 2006 15:04"))
	mailbox := renderHeader("Mailbox", currentMailboxTitle(m))

	headerMeta := lipgloss.JoinVertical(lipgloss.Left, from, to, date, mailbox)
	separator := lipgloss.NewStyle().
		Width(max(1, width-4)).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(m.styles.Palette.Faint).
		MarginTop(1).
		MarginBottom(1).
		Render("")
	bodyWidth := max(1, width-4)
	bodyHeight := max(height-lipgloss.Height(lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render(msg.Subject),
		renderMessageChips(msg, false),
		headerMeta,
		separator,
	)), 1)
	statusText := contentViewportStatus(m.contentViewport.YOffset, bodyHeight, contentLineCount(m.currentMessageBody()))
	statusView := statusStyle.Render(statusText)
	readerControls := joinHeaderColumns(
		bodyWidth,
		hintStyle.Render(contentViewportHint(bodyWidth, lipgloss.Width(statusView))),
		statusView,
	)

	headerBlock := lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render(msg.Subject),
		renderMessageChips(msg, false),
		headerMeta,
		readerControls,
		separator,
	)

	bodyHeight = max(height-lipgloss.Height(headerBlock), 1)

	return headerBlock, bodyWidth, bodyHeight
}

func contentViewportHint(width, statusWidth int) string {
	availableWidth := width - statusWidth - 2
	candidates := []string{
		"h/l pane | j/k line | ctrl+d/u | gg/G | 0/$",
		"j/k | ctrl+d/u | gg/G | 0/$",
		"j/k | gg/G | 0/$",
		"j/k | gg/G",
	}
	for _, candidate := range candidates {
		if lipgloss.Width(candidate) <= availableWidth {
			return candidate
		}
	}
	return "j/k"
}

func contentViewportStatus(offset, bodyHeight, totalLines int) string {
	if totalLines <= 0 {
		return "Empty"
	}
	firstLine := min(totalLines, offset+1)
	lastLine := min(totalLines, offset+max(1, bodyHeight))
	position := "Top"
	if offset > 0 {
		if lastLine >= totalLines {
			position = "Bottom"
		} else {
			position = fmt.Sprintf("%d%%", (lastLine*100)/totalLines)
		}
	}
	return fmt.Sprintf("%s • %d-%d/%d", position, firstLine, lastLine, totalLines)
}

func contentLineCount(content string) int {
	if content == "" {
		return 0
	}
	return strings.Count(content, "\n") + 1
}

func (m Model) composeAccountLabel() string {
	if m.activeDraft == nil {
		return ""
	}
	accountID := strings.TrimSpace(m.activeDraft.AccountID)
	if accountID == "" {
		accountID = m.defaultAcctID
	}
	from := strings.TrimSpace(m.senderForAccount(accountID))
	if from == "" {
		return accountID
	}
	if accountID == "" {
		return from
	}
	return accountID + " <" + from + ">"
}
