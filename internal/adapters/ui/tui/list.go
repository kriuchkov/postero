package tui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/kriuchkov/postero/internal/core/models"
)

const listItemHeight = 5

type listCursorMode int

const (
	listCursorNone listCursorMode = iota
	listCursorPassive
	listCursorActive
)

func renderList(m Model, width, height int) string {
	style := m.styles.List.Width(width).Height(height)
	headerStyle := paneTitleStyle(m, stateList)
	subtitleStyle := paneSubtitleStyle(m, stateList).MarginBottom(1)
	emptyStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.SubText)
	trackStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.Secondary)
	thumbStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.Faint)

	if height < 1 {
		return style.Render("")
	}

	innerWidth := max(width-4, 12)

	listHeader := lipgloss.JoinVertical(
		lipgloss.Left,
		headerStyle.Render(currentMailboxTitle(m)),
		subtitleStyle.Render(mailboxSubtitle(m)),
	)
	if m.searchActive {
		listHeader = lipgloss.JoinVertical(
			lipgloss.Left,
			listHeader,
			lipgloss.NewStyle().Foreground(m.styles.Palette.Text).Render(m.searchInput.View()),
		)
	} else if strings.TrimSpace(m.searchQuery) != "" {
		listHeader = lipgloss.JoinVertical(
			lipgloss.Left,
			listHeader,
			lipgloss.NewStyle().Foreground(m.styles.Palette.SubText).Render("Filter: "+m.searchQuery),
		)
	}
	headerHeight := lipgloss.Height(listHeader)
	availableItemsHeight := height - headerHeight
	if availableItemsHeight < 1 {
		return style.Render(listHeader)
	}

	if len(m.messages) == 0 {
		emptySubtitle := "Try another mailbox or sync an account."
		emptyTitle := "No messages"
		if strings.TrimSpace(m.searchQuery) != "" {
			emptyTitle = "No matches"
			emptySubtitle = "Refine the filter or press Esc to clear search."
		}
		empty := lipgloss.Place(
			innerWidth,
			availableItemsHeight,
			lipgloss.Left,
			lipgloss.Top,
			lipgloss.JoinVertical(
				lipgloss.Left,
				headerStyle.Render(emptyTitle),
				emptyStyle.Render(emptySubtitle),
			),
		)
		return style.Render(lipgloss.JoinVertical(lipgloss.Left, listHeader, empty))
	}

	scrollIndicatorWidth := 2
	contentWidth := innerWidth - scrollIndicatorWidth - 1
	if contentWidth < 10 {
		contentWidth = innerWidth
		scrollIndicatorWidth = 0
	}

	itemsPerPage := max(availableItemsHeight/listItemHeight, 1)

	// Determining the visible window based on cursor
	start := 0
	if m.listCursor >= itemsPerPage {
		// Keep cursor at bottom or scroll?
		// Simple scrolling:
		start = m.listCursor - itemsPerPage + 1
	}
	end := min(start+itemsPerPage, len(m.messages))
	if start > end {
		start = end
	}

	var renderedItems []string
	for i := start; i < end; i++ {
		msg := m.messages[i]
		cursorMode := currentListCursorMode(m, i)
		isSelected := cursorMode == listCursorActive
		isCursor := cursorMode != listCursorNone

		cardStyle := lipgloss.NewStyle().
			Width(contentWidth).
			Padding(0, 1).
			MarginBottom(1).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(m.styles.Palette.Faint)
		if isSelected {
			cardStyle = cardStyle.Background(m.styles.Palette.Faint).BorderForeground(m.styles.Palette.Primary)
		} else if cursorMode == listCursorPassive {
			cardStyle = cardStyle.BorderForeground(m.styles.Palette.Secondary)
		}
		cardInnerWidth := max(contentWidth-cardStyle.GetHorizontalFrameSize(), 1)

		sender := msg.From
		if idx := strings.Index(sender, "<"); idx > 0 {
			sender = strings.TrimSpace(sender[:idx])
		}
		if sender == "" {
			sender = "Unknown sender"
		}

		dateStr := msg.Date.Format("02/01/06")
		if msg.Date.Year() == time.Now().Year() && msg.Date.YearDay() == time.Now().YearDay() {
			dateStr = msg.Date.Format("15:04")
		}

		senderStyle := lipgloss.NewStyle().Bold(true).Foreground(m.styles.Palette.Highlight)
		dateStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.SubText)
		if isSelected {
			dateStyle = dateStyle.Foreground(m.styles.Palette.Highlight).Background(m.styles.Palette.Faint)
			senderStyle = senderStyle.Foreground(m.styles.Palette.Highlight).Background(m.styles.Palette.Faint)
		} else if isCursor {
			dateStyle = dateStyle.Foreground(m.styles.Palette.Text)
			senderStyle = senderStyle.Foreground(m.styles.Palette.Text)
		}

		senderMaxWidth := cardInnerWidth - lipgloss.Width(dateStr) - 1
		if senderMaxWidth < 0 {
			senderMaxWidth = 5
		}

		if len(sender) > senderMaxWidth {
			sender = sender[:senderMaxWidth-1] + "…"
		}

		// Fill space between sender and date
		gap := max(cardInnerWidth-lipgloss.Width(sender)-lipgloss.Width(dateStr), 1)

		spacerStyle := lipgloss.NewStyle()
		if isSelected {
			spacerStyle = spacerStyle.Background(m.styles.Palette.Faint)
		}
		spacer := spacerStyle.Render(strings.Repeat(" ", gap))

		row1 := fmt.Sprintf("%s%s%s", senderStyle.Render(sender), spacer, dateStyle.Render(dateStr))

		subject := msg.Subject
		if subject == "" {
			subject = "(No Subject)"
		}
		if lipgloss.Width(subject) > cardInnerWidth {
			subject = truncateText(subject, cardInnerWidth)
		}
		subjectStyle := lipgloss.NewStyle().Bold(true).Foreground(m.styles.Palette.Highlight)
		if isSelected {
			subjectStyle = subjectStyle.Background(m.styles.Palette.Faint).Foreground(m.styles.Palette.Highlight)
		} else if isCursor {
			subjectStyle = subjectStyle.Foreground(m.styles.Palette.Text)
		}
		row2 := subjectStyle.Render(subject)

		preview := strings.ReplaceAll(msg.Body, "\n", " ")
		if lipgloss.Width(preview) > cardInnerWidth {
			preview = truncateText(preview, cardInnerWidth)
		}
		previewStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.SubText)
		if isSelected {
			previewStyle = previewStyle.Foreground(m.styles.Palette.Highlight).Background(m.styles.Palette.Faint)
		} else if isCursor {
			previewStyle = previewStyle.Foreground(m.styles.Palette.Text)
		}
		row3 := previewStyle.Render(preview)

		chips := renderMessageChips(msg, isSelected)
		rowStyle := lipgloss.NewStyle().Width(cardInnerWidth).MaxWidth(cardInnerWidth)
		if isSelected {
			rowStyle = rowStyle.Background(m.styles.Palette.Faint)
		}
		row1 = rowStyle.Render(row1)
		row2 = rowStyle.Render(row2)
		row3 = rowStyle.Render(row3)
		rows := []string{row1, row2}
		if chips != "" {
			rows = append(rows, rowStyle.Render(chips))
		}
		rows = append(rows, row3)
		block := lipgloss.JoinVertical(lipgloss.Left, rows...)
		renderedItems = append(renderedItems, cardStyle.Render(block))
	}

	itemsBody := lipgloss.JoinVertical(lipgloss.Left, renderedItems...)
	itemsBody = lipgloss.NewStyle().Width(contentWidth).Height(availableItemsHeight).Render(itemsBody)
	if scrollIndicatorWidth > 0 {
		indicator := renderListScrollIndicator(availableItemsHeight, len(m.messages), itemsPerPage, start, trackStyle, thumbStyle)
		itemsBody = lipgloss.JoinHorizontal(lipgloss.Top, itemsBody, " ", indicator)
	}
	body := lipgloss.JoinVertical(lipgloss.Left, listHeader, itemsBody)
	return style.Render(body)
}

func currentListCursorMode(m Model, index int) listCursorMode {
	if index != m.listCursor {
		return listCursorNone
	}
	if m.state == stateList {
		return listCursorActive
	}
	return listCursorPassive
}

func truncateText(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}
	runes := []rune(value)
	if width == 1 {
		return "…"
	}
	for len(runes) > 0 {
		candidate := string(runes) + "…"
		if lipgloss.Width(candidate) <= width {
			return candidate
		}
		runes = runes[:len(runes)-1]
	}
	return "…"
}

func listWindowRange(m Model, height int) (int, int) {
	if len(m.messages) == 0 {
		return 0, 0
	}
	if height < 1 {
		return 0, len(m.messages)
	}

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(m.styles.Palette.Text)
	subtitleStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.SubText).MarginBottom(1)
	listHeader := lipgloss.JoinVertical(
		lipgloss.Left,
		headerStyle.Render(currentMailboxTitle(m)),
		subtitleStyle.Render(mailboxSubtitle(m)),
	)
	if m.searchActive {
		listHeader = lipgloss.JoinVertical(
			lipgloss.Left,
			listHeader,
			lipgloss.NewStyle().Foreground(m.styles.Palette.Text).Render(m.searchInput.View()),
		)
	} else if strings.TrimSpace(m.searchQuery) != "" {
		listHeader = lipgloss.JoinVertical(
			lipgloss.Left,
			listHeader,
			lipgloss.NewStyle().Foreground(m.styles.Palette.SubText).Render("Filter: "+m.searchQuery),
		)
	}
	availableItemsHeight := height - lipgloss.Height(listHeader)
	if availableItemsHeight < 1 {
		return 0, min(1, len(m.messages))
	}

	itemsPerPage := max(availableItemsHeight/listItemHeight, 1)
	start := 0
	if m.listCursor >= itemsPerPage {
		start = m.listCursor - itemsPerPage + 1
	}
	end := min(start+itemsPerPage, len(m.messages))
	if start > end {
		start = end
	}
	return start, end
}

func renderListScrollIndicator(height, total, visible, start int, trackStyle, thumbStyle lipgloss.Style) string {
	if height <= 0 {
		return ""
	}
	if total <= visible || visible <= 0 {
		return lipgloss.NewStyle().Width(1).Height(height).Render("")
	}

	thumbHeight := min(max(int(float64(visible)/float64(total)*float64(height)), 1), height)
	maxThumbTop := height - thumbHeight
	thumbTop := 0
	if total > visible && maxThumbTop > 0 {
		thumbTop = int(float64(start) / float64(total-visible) * float64(maxThumbTop))
	}

	lines := make([]string, 0, height)
	for i := range height {
		if i >= thumbTop && i < thumbTop+thumbHeight {
			lines = append(lines, thumbStyle.Render("▎"))
			continue
		}
		lines = append(lines, trackStyle.Render("╎"))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func renderMessageChips(msg *models.MessageDTO, selected bool) string {
	if msg == nil {
		return ""
	}
	chips := []string{}
	if !msg.IsRead {
		chips = append(chips, unreadChipStyle(selected).Render("Unread"))
	}
	if msg.IsDraft {
		chips = append(chips, draftChipStyle(selected).Render("Draft"))
	}
	if msg.IsSpam {
		chips = append(chips, spamChipStyle(selected).Render("Spam"))
	}
	if slices.Contains(msg.Labels, "archive") {
		chips = append(chips, archiveChipStyle(selected).Render("Archive"))
	}
	return lipgloss.JoinHorizontal(lipgloss.Left, chips...)
}

func baseChipStyle(selected bool) lipgloss.Style {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Background(lipgloss.Color("238")).Padding(0, 1).MarginRight(1)
	if selected {
		style = style.Foreground(lipgloss.Color("255")).Background(lipgloss.Color("239"))
	}
	return style
}

func unreadChipStyle(selected bool) lipgloss.Style {
	style := baseChipStyle(selected).Foreground(lipgloss.Color("39"))
	if selected {
		style = style.Foreground(lipgloss.Color("255")).Background(lipgloss.Color("33"))
	}
	return style
}

func draftChipStyle(selected bool) lipgloss.Style {
	style := baseChipStyle(selected)
	if selected {
		style = style.Foreground(lipgloss.Color("255")).Background(lipgloss.Color("241"))
	}
	return style
}

func spamChipStyle(selected bool) lipgloss.Style {
	style := baseChipStyle(selected).Foreground(lipgloss.Color("203"))
	if selected {
		style = style.Foreground(lipgloss.Color("255")).Background(lipgloss.Color("160"))
	}
	return style
}

func archiveChipStyle(selected bool) lipgloss.Style {
	style := baseChipStyle(selected)
	if selected {
		style = style.Foreground(lipgloss.Color("255")).Background(lipgloss.Color("24"))
	}
	return style
}
