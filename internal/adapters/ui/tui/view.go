package tui

import (
	"github.com/charmbracelet/lipgloss"
)

const (
	minSidebarWidth = 16
	maxSidebarWidth = 22
	minListWidth    = 34
	maxListWidth    = 46
	minContentWidth = 40
)

func (m Model) View() string {
	// Layout is composed top-down: header, three-pane body, then footer.
	if m.width == 0 || m.height == 0 {
		return "Initialising..."
	}

	header := renderHeader(m, m.width)
	headerHeight := max(lipgloss.Height(header), 1)

	footer := renderFooter(m, m.width)
	footerHeight := max(lipgloss.Height(footer), 1)

	mainHeight := max(m.height-headerHeight-footerHeight, 0)
	paneFrameHeight := m.styles.Sidebar.GetVerticalFrameSize()
	if listFrameHeight := m.styles.List.GetVerticalFrameSize(); listFrameHeight > paneFrameHeight {
		paneFrameHeight = listFrameHeight
	}
	if contentFrameHeight := m.styles.Content.GetVerticalFrameSize(); contentFrameHeight > paneFrameHeight {
		paneFrameHeight = contentFrameHeight
	}
	paneHeight := max(mainHeight-paneFrameHeight, 0)

	sidebarWidth, listWidth, contentWidth := paneWidths(m.width)

	sidebar := renderSidebar(m, sidebarWidth, paneHeight)
	list := renderList(m, listWidth, paneHeight)
	content := renderContent(m, contentWidth, paneHeight)

	main := lipgloss.JoinHorizontal(
		lipgloss.Top,
		sidebar,
		list,
		content,
	)

	root := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		main,
		footer,
	)

	return root
}

func paneWidths(totalWidth int) (int, int, int) {
	availableWidth := max(totalWidth-4, minSidebarWidth+minListWidth+minContentWidth)

	sidebarWidth := clampInt(int(float64(totalWidth)*0.18), minSidebarWidth, maxSidebarWidth)
	remainingWidth := availableWidth - sidebarWidth
	listWidth := clampInt(int(float64(remainingWidth)*0.38), minListWidth, maxListWidth)
	contentWidth := availableWidth - sidebarWidth - listWidth

	if contentWidth < minContentWidth {
		listWidth -= minContentWidth - contentWidth
		if listWidth < minListWidth {
			sidebarWidth -= minListWidth - listWidth
			listWidth = minListWidth
			if sidebarWidth < minSidebarWidth {
				sidebarWidth = minSidebarWidth
			}
		}
		contentWidth = availableWidth - sidebarWidth - listWidth
	}

	return sidebarWidth, listWidth, contentWidth
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
