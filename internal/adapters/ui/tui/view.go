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
	if m.state == stateSetup {
		return renderSetupView(m)
	}
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

	sidebarWidth, listWidth, contentWidth := paneWidths(m, m.width)

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

	// Safety net: never emit more than the terminal can show. The layout math
	// above targets exactly m.width×m.height, but a pathologically small terminal
	// (or an unexpectedly tall pane) must degrade by clipping, not by wrapping into
	// a garbled screen.
	screen := lipgloss.NewStyle().MaxWidth(m.width).MaxHeight(m.height).Render(root)
	if m.aiPromptActive {
		screen = overlayCenter(screen, m.renderAIReplyPopup(), m.width, m.height)
	}
	return screen
}

// paneFrameWidth is the total horizontal space the three pane styles spend on
// borders and padding — it must be reserved before dividing the remaining width,
// or the rendered panes (content + frame) overflow the terminal.
func paneFrameWidth(m Model) int {
	return m.styles.Sidebar.GetHorizontalFrameSize() +
		m.styles.List.GetHorizontalFrameSize() +
		m.styles.Content.GetHorizontalFrameSize()
}

func paneWidths(m Model, totalWidth int) (int, int, int) {
	availableWidth := max(totalWidth-paneFrameWidth(m), minSidebarWidth+minListWidth+minContentWidth)

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
