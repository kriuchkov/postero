package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/kriuchkov/postero/internal/core/models"
)

type listCursorMode int

const (
	listCursorNone listCursorMode = iota
	listCursorPassive
	listCursorActive
)

// renderList builds the list pane from a fixed header and a height-aware window of measured message cards.
func renderList(m Model, width, height int) string {
	style := m.styles.List.Width(width).Height(height)
	emptyStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.SubText)
	trackStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.Secondary)
	thumbStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.Faint)

	if height < 1 {
		return style.Render("")
	}

	innerWidth := max(width-2, 14)
	listHeader := renderListHeader(m)
	headerHeight := lipgloss.Height(listHeader)
	availableItemsHeight := height - headerHeight
	if availableItemsHeight < 1 {
		return style.Render(listHeader)
	}

	if len(m.messages) == 0 {
		if loadingRow, loadingHeight := renderListLoadingRow(m, innerWidth); loadingRow != "" {
			body := lipgloss.Place(
				innerWidth,
				max(availableItemsHeight, loadingHeight),
				lipgloss.Left,
				lipgloss.Top,
				loadingRow,
			)
			return style.Render(lipgloss.JoinVertical(lipgloss.Left, listHeader, body))
		}
		emptySubtitle := "Try another mailbox or sync an account."
		emptyTitle := "No messages"
		emptyTitleStyle := paneTitleStyle(m, stateList)
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
				emptyTitleStyle.Render(emptyTitle),
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

	start, end := listWindowRangeForAvailableHeight(m, availableItemsHeight)

	var renderedItems []string
	renderedHeight := 0
	for i := start; i < end; i++ {
		cursorMode := currentListCursorMode(m, i)
		card, cardHeight := renderListCard(m, m.messages[i], contentWidth, cursorMode)
		if cardHeight == 0 {
			continue
		}
		if renderedHeight > 0 && renderedHeight+cardHeight > availableItemsHeight {
			break
		}
		renderedItems = append(renderedItems, card)
		renderedHeight += cardHeight
	}
	if loadingRow, loadingHeight := renderListLoadingRow(m, contentWidth); loadingRow != "" &&
		renderedHeight+loadingHeight <= availableItemsHeight {
		renderedItems = append(renderedItems, loadingRow)
	}

	itemsBody := lipgloss.JoinVertical(lipgloss.Left, renderedItems...)
	itemsBody = lipgloss.NewStyle().Width(contentWidth).Height(availableItemsHeight).Render(itemsBody)

	if scrollIndicatorWidth > 0 {
		indicator := renderListScrollIndicator(availableItemsHeight, len(m.messages), len(renderedItems), start, trackStyle, thumbStyle)
		itemsBody = lipgloss.JoinHorizontal(lipgloss.Top, itemsBody, " ", indicator)
	}

	body := lipgloss.JoinVertical(lipgloss.Left, listHeader, itemsBody)
	return style.Render(body)
}

func renderListLoadingRow(m Model, width int) (string, int) {
	if !m.messagesLoading || width < 10 {
		return "", 0
	}
	frame := loadingFrames[m.loadingFrame%len(loadingFrames)]
	loadingMore := m.fetchOffset > 0
	label := frame + " Loading mailbox..."
	if strings.TrimSpace(m.searchQuery) != "" {
		label = frame + " Searching mailbox..."
	}
	if loadingMore {
		label = frame + " Loading more messages..."
		if strings.TrimSpace(m.searchQuery) != "" {
			label = frame + " Searching more messages..."
		}
	}
	row := listLoadingRowStyle(m, loadingMore).
		Width(width).
		Render(label)
	return row, 1
}

func listLoadingRowStyle(m Model, loadingMore bool) lipgloss.Style {
	if loadingMore {
		return lipgloss.NewStyle().
			Foreground(m.styles.Palette.SubText)
	}
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(m.styles.Palette.Highlight).
		Background(m.styles.Palette.Secondary).
		Padding(0, 1)
}

func renderListHeader(m Model) string {
	headerStyle := paneTitleStyle(m, stateList)
	subtitleStyle := paneSubtitleStyle(m, stateList).MarginBottom(1)
	listHeader := lipgloss.JoinVertical(
		lipgloss.Left,
		headerStyle.Render(currentMailboxTitle(m)),
		subtitleStyle.Render(mailboxSubtitle(m)),
	)
	if m.searchActive {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			listHeader,
			lipgloss.NewStyle().Foreground(m.styles.Palette.Text).Render(m.searchInput.View()),
		)
	}
	if strings.TrimSpace(m.searchQuery) != "" {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			listHeader,
			lipgloss.NewStyle().Foreground(m.styles.Palette.SubText).Render("Filter: "+m.searchQuery),
		)
	}
	return listHeader
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

func listWindowRange(m Model, height int) (int, int) {
	if len(m.messages) == 0 {
		return 0, 0
	}
	if height < 1 {
		return 0, len(m.messages)
	}
	availableItemsHeight := height - lipgloss.Height(renderListHeader(m))
	if availableItemsHeight < 1 {
		return 0, min(1, len(m.messages))
	}
	return listWindowRangeForAvailableHeight(m, availableItemsHeight)
}

// listWindowRangeForAvailableHeight expands around the cursor using estimated card heights so keyboard movement stays centered in the viewport.
func listWindowRangeForAvailableHeight(m Model, availableItemsHeight int) (int, int) {
	if len(m.messages) == 0 {
		return 0, 0
	}
	if availableItemsHeight < 1 {
		return 0, min(1, len(m.messages))
	}

	cursor := max(m.listCursor, 0)
	if cursor >= len(m.messages) {
		cursor = len(m.messages) - 1
	}

	start := cursor
	usedHeight := listCardHeight(m.messages[cursor])
	for start > 0 {
		nextHeight := listCardHeight(m.messages[start-1])
		if usedHeight+nextHeight > availableItemsHeight {
			break
		}
		start--
		usedHeight += nextHeight
	}

	end := cursor + 1
	for end < len(m.messages) {
		nextHeight := listCardHeight(m.messages[end])
		if usedHeight+nextHeight > availableItemsHeight {
			break
		}
		usedHeight += nextHeight
		end++
	}
	return start, end
}

func listCardHeight(msg *models.Message) int {
	if msg == nil {
		return 0
	}

	height := 4
	if renderMessageChips(msg, false) != "" {
		height++
	}
	return height
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
