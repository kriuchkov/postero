package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func renderContent(m Model, width, height int) string {
	style := m.styles.Content.Width(width).Height(height)
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(m.styles.Palette.Highlight)
	subtitleStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.SubText)

	if m.state == stateCompose {
		if m.activeDraft == nil {
			return style.Render("Loading draft...")
		}

		labelStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.SubText).Width(8).Align(lipgloss.Right).MarginRight(1)
		accountStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.Text)
		accountHintStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.SubText)
		contextTitleStyle := lipgloss.NewStyle().Bold(true).Foreground(m.styles.Palette.Text)
		contextHintStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.SubText)
		modeStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.SubText)
		contextCardStyle := lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(m.styles.Palette.Secondary).
			PaddingLeft(1).
			MarginBottom(1)

		accountValue := m.composeAccountLabel()
		accountView := accountStyle.Render(accountValue)
		if m.focusIndex == 0 {
			accountView = lipgloss.NewStyle().Foreground(m.styles.Palette.Primary).Bold(true).Render(accountValue)
			accountView = lipgloss.JoinHorizontal(lipgloss.Left, accountView, accountHintStyle.Render("  h/l or ↑/↓ switch"))
		}

		toView := m.toInput.View()
		subjectView := m.subjectInput.View()

		if m.focusIndex == 1 {
			toView = lipgloss.NewStyle().Foreground(m.styles.Palette.Primary).Render(toView)
		}
		if m.focusIndex == 2 {
			subjectView = lipgloss.NewStyle().Foreground(m.styles.Palette.Primary).Render(subjectView)
		}

		title := strings.TrimSpace(m.composeTitle)
		if title == "" {
			title = "Composer"
		}
		hint := strings.TrimSpace(m.composeHint)
		if hint == "" {
			hint = "Write now, save when ready."
		}
		modeHint := "Navigation mode. Press Enter or i to edit the selected field."
		if m.composeEditing {
			modeHint = "Writing mode. Press Esc to leave editing without closing the draft."
		}

		header := lipgloss.JoinVertical(
			lipgloss.Left,
			titleStyle.Render(title),
			subtitleStyle.Render(hint),
			modeStyle.Render(modeHint),
		)
		account := lipgloss.JoinHorizontal(lipgloss.Top, labelStyle.Render("Account:"), accountView)
		to := lipgloss.JoinHorizontal(lipgloss.Top, labelStyle.Render("To:"), toView)
		subject := lipgloss.JoinHorizontal(lipgloss.Top, labelStyle.Render("Subject:"), subjectView)

		separator := lipgloss.NewStyle().
			Width(width-4).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(m.styles.Palette.Faint).
			MarginTop(1).
			MarginBottom(1).
			Render("")

		// Body editor
		m.bodyInput.SetWidth(width - 4)
		m.bodyInput.SetHeight(height - 12) // Reserve space for headers

		bodyView := m.bodyInput.View()
		if m.focusIndex == 3 {
			bodyView = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, false, false, true).BorderForeground(m.styles.Palette.Primary).Render(bodyView)
		}

		composeContext := ""
		if title != "Composer" || hint != "Write now, save when ready." {
			composeContext = contextCardStyle.Render(lipgloss.JoinVertical(
				lipgloss.Left,
				contextTitleStyle.Render(title),
				contextHintStyle.Render(hint),
			))
		}

		content := lipgloss.JoinVertical(lipgloss.Left,
			header,
			"",
			account,
			to,
			subject,
			separator,
			composeContext,
			bodyView,
		)

		return style.Render(content)
	}

	if len(m.messages) == 0 || m.listCursor < 0 || m.listCursor >= len(m.messages) {
		empty := lipgloss.JoinVertical(
			lipgloss.Left,
			titleStyle.Render("Welcome to Postero"),
			subtitleStyle.Render("Choose a mailbox and select a message to start reading."),
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

func contentViewportLayout(m Model, width, height int) (string, int, int) {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(m.styles.Palette.Highlight).MarginBottom(1).Width(max(1, width-4))
	metaLabelStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.SubText).Width(8)
	metaValueStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.Text)
	hintStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.SubText)
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
	bodyHeight := height - lipgloss.Height(lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render(msg.Subject),
		renderMessageChips(msg, false),
		headerMeta,
		separator,
	))
	if bodyHeight < 1 {
		bodyHeight = 1
	}
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

	bodyHeight = height - lipgloss.Height(headerBlock)
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	return headerBlock, bodyWidth, bodyHeight
}

func contentViewportHint(width, statusWidth int) string {
	availableWidth := width - statusWidth - 2
	candidates := []string{
		"j/k line | ctrl+d/u half | pgup/dn | g/G",
		"j/k | ctrl+d/u | pgup/dn | g/G",
		"j/k | pgup/dn | g/G",
		"j/k | g/G",
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
