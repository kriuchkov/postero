package tui

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"

	"github.com/kriuchkov/postero/internal/core/models"
)

func renderSidebar(m Model, width, height int) string {
	style := m.styles.Sidebar.Width(width).Height(height)
	bodyWidth := max(width-style.GetHorizontalFrameSize(), 1)
	lines := []string{}

	folders, accounts := sidebarRows(m)
	tags := sidebarTags(m)

	lines = append(lines, renderSidebarSection(m, bodyWidth, "Accounts", accounts)...)
	lines = append(lines, "")
	lines = append(lines, renderSidebarSection(m, bodyWidth, "Folders", folders)...)
	lines = append(lines, "")
	lines = append(lines, renderSidebarSection(m, bodyWidth, "Tags", tags)...)

	return style.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

type sidebarSectionRow struct {
	index  int
	value  string
	label  string
	prefix string
	kind   sidebarRowKind
	count  int
	active bool
	hotkey rune
}

type sidebarRowKind int

const (
	sidebarRowFolder sidebarRowKind = iota
	sidebarRowAccount
	sidebarRowTag
)

func sidebarRows(m Model) ([]sidebarSectionRow, []sidebarSectionRow) {
	folders := make([]sidebarSectionRow, 0, len(m.sidebarItems))
	accounts := make([]sidebarSectionRow, 0, len(m.sidebarItems))
	accountNumber := 1
	for index, item := range m.sidebarItems {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" || trimmed == "Accounts:" {
			continue
		}
		if strings.HasPrefix(item, "  ") {
			accounts = append(accounts, sidebarSectionRow{
				index:  index,
				value:  trimmed,
				label:  trimmed,
				prefix: fmt.Sprintf("[%d]", accountNumber),
				kind:   sidebarRowAccount,
				active: strings.EqualFold(trimmed, strings.TrimSpace(m.activeAccountID)),
			})
			accountNumber++
			continue
		}
		folders = append(folders, sidebarSectionRow{
			index: index,
			value: trimmed,
			label: trimmed,
			kind:  sidebarRowFolder,
			count: sidebarMailboxCount(m, trimmed),
		})
	}
	return folders, accounts
}

func renderSidebarSection(m Model, width int, title string, rows []sidebarSectionRow) []string {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(m.styles.Palette.SubText)
	dividerStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.Faint)
	lines := []string{
		headerStyle.Width(width).Render(strings.ToUpper(title)),
		dividerStyle.Width(width).Render(strings.Repeat("─", width)),
	}
	if len(rows) == 0 {
		emptyStyle := sidebarItemStyle(m, false, true).Width(width)
		return append(lines, emptyStyle.Render("  none"))
	}
	for _, row := range rows {
		lines = append(lines, renderSidebarRow(m, row, width))
	}
	return lines
}

func renderSidebarRow(m Model, row sidebarSectionRow, width int) string {
	selected := row.index >= 0 && row.index == m.sidebarCursor
	muted := row.kind == sidebarRowAccount
	style := sidebarItemStyle(m, selected, muted).Width(width)
	if row.active && !selected {
		style = style.Foreground(m.styles.Palette.Primary).Bold(true)
	}

	marker := " "
	if selected || (row.kind == sidebarRowTag && row.active) {
		marker = ">"
	}

	label := row.label
	if row.kind == sidebarRowFolder && row.count > 0 {
		label = fmt.Sprintf("%s (%d)", label, row.count)
	}
	if row.prefix != "" {
		label = row.prefix + " " + label
	}

	line := fmt.Sprintf("%s %s", marker, label)
	return style.Render(truncateSidebarLine(line, width))
}

func sidebarTags(m Model) []sidebarSectionRow {
	tagCounts := make(map[string]int)
	source := m.sidebarTagSource
	if len(source) == 0 {
		source = m.allMessages
	}
	for _, msg := range source {
		if msg == nil {
			continue
		}
		for _, label := range msg.Labels {
			if sidebarSystemLabel(label) {
				continue
			}
			tagCounts[label]++
		}
	}
	if len(tagCounts) == 0 {
		return nil
	}
	ordered := make([]string, 0, len(tagCounts))
	for label := range tagCounts {
		ordered = append(ordered, label)
	}
	sort.Slice(ordered, func(i, j int) bool {
		return strings.ToLower(ordered[i]) < strings.ToLower(ordered[j])
	})
	assignedHotkeys := make(map[rune]struct{}, len(ordered))

	rows := make([]sidebarSectionRow, 0, len(ordered))
	for _, label := range ordered {
		hotkey := sidebarTagHotkey(label, assignedHotkeys)
		rows = append(rows, sidebarSectionRow{
			index:  -1,
			value:  label,
			label:  strings.ReplaceAll(label, "_", " "),
			prefix: "[" + strings.ToUpper(string(hotkey)) + "]",
			kind:   sidebarRowTag,
			count:  tagCounts[label],
			active: strings.EqualFold(strings.TrimSpace(m.activeTagID), strings.TrimSpace(label)),
			hotkey: hotkey,
		})
	}
	return rows
}

func findSidebarTagByHotkey(m Model, key rune) (string, bool) {
	needle := unicode.ToLower(key)
	for _, row := range sidebarTags(m) {
		if unicode.ToLower(row.hotkey) == needle {
			return row.value, true
		}
	}
	return "", false
}

func sidebarMailboxCount(m Model, mailbox string) int {
	count := 0
	for _, msg := range m.allMessages {
		if msg == nil {
			continue
		}
		if sidebarMessageInMailbox(msg, mailbox) {
			count++
		}
	}
	return count
}

func sidebarItemStyle(m Model, selected, muted bool) lipgloss.Style {
	style := lipgloss.NewStyle().Padding(0, 1).Foreground(m.styles.Palette.Text)
	if muted {
		style = style.Foreground(m.styles.Palette.SubText)
	}
	if selected {
		style = style.
			Foreground(m.styles.Palette.Highlight).
			Background(m.styles.Palette.Primary).
			Bold(true)
	}
	return style
}

func sidebarMessageInMailbox(msg *models.Message, mailbox string) bool {
	switch mailbox {
	case "Inbox":
		return sidebarHasLabel(msg.Labels, "inbox") && !msg.IsDeleted && !msg.IsSpam && !msg.IsDraft
	case "Sent":
		return sidebarHasLabel(msg.Labels, "sent") && !msg.IsDeleted
	case "Drafts":
		return msg.IsDraft && !msg.IsDeleted
	case "Archive":
		return sidebarHasLabel(msg.Labels, "archive") && !msg.IsDeleted
	case "Trash":
		return msg.IsDeleted
	case "Spam":
		return msg.IsSpam
	default:
		return false
	}
}

func sidebarHasLabel(labels []string, target string) bool {
	for _, label := range labels {
		if strings.EqualFold(strings.TrimSpace(label), target) {
			return true
		}
	}
	return false
}

func sidebarSystemLabel(label string) bool {
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "inbox", "sent", "archive", "trash", "spam", "draft", "drafts":
		return true
	default:
		return false
	}
}

func sidebarTagHotkey(label string, assigned map[rune]struct{}) rune {
	for _, candidate := range sidebarTagHotkeyCandidates(label) {
		key := unicode.ToLower(candidate)
		if _, exists := assigned[key]; exists {
			continue
		}
		assigned[key] = struct{}{}
		return unicode.ToUpper(candidate)
	}
	for candidate := 'a'; candidate <= 'z'; candidate++ {
		if _, exists := assigned[candidate]; exists {
			continue
		}
		assigned[candidate] = struct{}{}
		return unicode.ToUpper(candidate)
	}
	return '?'
}

func sidebarTagHotkeyCandidates(label string) []rune {
	seen := map[rune]struct{}{}
	candidates := make([]rune, 0, len(label))
	for _, r := range label {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			continue
		}
		lower := unicode.ToLower(r)
		if _, exists := seen[lower]; exists {
			continue
		}
		seen[lower] = struct{}{}
		candidates = append(candidates, unicode.ToUpper(r))
	}
	if len(candidates) == 0 {
		return []rune{'?'}
	}
	return candidates
}

func truncateSidebarLine(line string, width int) string {
	if width <= 0 {
		return ""
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(line)
}
