package tui

import (
	"github.com/charmbracelet/lipgloss"
)

const (
	minSidebarWidth = 16
	maxSidebarWidth = 22
	minListWidth    = 34
	maxListWidth    = 48
	minContentWidth = 40
)

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Initialising..."
	}

	header := renderHeader(m, m.width, 0)
	headerHeight := lipgloss.Height(header)
	if headerHeight < 1 {
		headerHeight = 1
	}

	footer := renderFooter(m, m.width)
	footerHeight := lipgloss.Height(footer)
	if footerHeight < 1 {
		footerHeight = 1
	}

	mainHeight := m.height - headerHeight - footerHeight
	if mainHeight < 0 {
		mainHeight = 0
	}
	paneFrameHeight := m.styles.Sidebar.GetVerticalFrameSize()
	if listFrameHeight := m.styles.List.GetVerticalFrameSize(); listFrameHeight > paneFrameHeight {
		paneFrameHeight = listFrameHeight
	}
	if contentFrameHeight := m.styles.Content.GetVerticalFrameSize(); contentFrameHeight > paneFrameHeight {
		paneFrameHeight = contentFrameHeight
	}
	paneHeight := mainHeight - paneFrameHeight
	if paneHeight < 0 {
		paneHeight = 0
	}

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

func renderFooter(m Model, width int) string {
	style := m.styles.Footer.Width(width)
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
	helpText := m.help.ShortHelpView(m.keys.ShortHelp())
	helpStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.SubText)
	return style.Render(joinHeaderColumns(width, statusStyle.Render(status), helpStyle.Render(helpText)))
}

func paneWidths(totalWidth int) (sidebarWidth, listWidth, contentWidth int) {
	availableWidth := totalWidth - 4
	if availableWidth < minSidebarWidth+minListWidth+minContentWidth {
		availableWidth = minSidebarWidth + minListWidth + minContentWidth
	}

	sidebarWidth = clampInt(int(float64(totalWidth)*0.18), minSidebarWidth, maxSidebarWidth)
	remainingWidth := availableWidth - sidebarWidth
	listWidth = clampInt(int(float64(remainingWidth)*0.38), minListWidth, maxListWidth)
	contentWidth = availableWidth - sidebarWidth - listWidth

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
