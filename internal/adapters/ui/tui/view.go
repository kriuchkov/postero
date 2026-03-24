package tui

import (
	"strings"

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
	helpText := footerHelpText(m, width)
	helpStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.SubText)
	return style.Render(joinHeaderColumns(width, statusStyle.Render(status), helpStyle.Render(helpText)))
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
			"type to filter • enter apply • esc clear • n/N move through results",
			"enter apply • esc clear • n/N move",
		}
	}

	hasSearch := strings.TrimSpace(m.searchQuery) != ""

	switch m.state {
	case stateSidebar:
		if hasSearch {
			return []string{
				"j/k move • enter/l open • n/N results • gg/G • 0/$ • h/l panes • / search • : commands",
				"j/k • enter/l • n/N • gg/G • 0/$ • / search • : cmd",
			}
		}
		return []string{
			"j/k move • enter/l open • gg/G • 0/$ • H/M/L • h/l panes • / search • : commands",
			"j/k • enter/l • gg/G • 0/$ • H/M/L • / search • : cmd",
		}
	case stateList:
		if hasSearch {
			return []string{
				"j/k move • n/N next hit • enter/l read • gg/G • 0/$ • H/M/L • ctrl+d/u • / search • : commands",
				"j/k • n/N • enter/l • gg/G • 0/$ • H/M/L • ctrl+d/u • / search • : cmd",
				"j/k • n/N • enter/l • gg/G • ctrl+d/u • /",
			}
		}
		return []string{
			"j/k move • enter/l read • gg/G • 0/$ • H/M/L • ctrl+d/u • r/R/f • a/!/d • / search • : commands",
			"j/k • enter/l • gg/G • 0/$ • H/M/L • ctrl+d/u • r/f • a/!/d • / search • : cmd",
			"j/k • enter/l • gg/G • ctrl+d/u • r/f • d • /",
		}
	case stateContent:
		if hasSearch {
			return []string{
				"j/k scroll • n/N next hit • gg/G • 0/$ • ctrl+d/u • h back • / search • : commands",
				"j/k • n/N • gg/G • 0/$ • ctrl+d/u • h • / search • : cmd",
				"j/k • n/N • gg/G • ctrl+d/u • h",
			}
		}
		return []string{
			"j/k scroll • gg/G • 0/$ • ctrl+d/u • h back • / search • : commands • r reply • f forward",
			"j/k • gg/G • 0/$ • ctrl+d/u • h • / search • : cmd",
			"j/k • gg/G • ctrl+d/u • h",
		}
	case stateCompose:
		return []string{m.help.ShortHelpView(m.keys.ShortHelp())}
	default:
		return []string{m.help.ShortHelpView(m.keys.ShortHelp())}
	}
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
