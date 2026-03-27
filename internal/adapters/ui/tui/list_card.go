package tui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/kriuchkov/postero/internal/core/models"
)

// renderListCard renders one measured message card so
// the list viewport can pack variable-height rows without extra layout passes.
func renderListCard(m Model, msg *models.Message, contentWidth int, cursorMode listCursorMode) (string, int) {
	// Resolve the card chrome first so all inner row widths derive from the same frame.
	isSelected := cursorMode == listCursorActive
	border := lipgloss.NormalBorder()
	if isSelected {
		border.Left = "▌"
	}

	cardStyle := lipgloss.NewStyle().
		Width(contentWidth).
		Padding(0, 1).
		MarginBottom(1).
		Border(border, false, false, false, true).
		BorderForeground(m.styles.Palette.Faint)

	if isSelected {
		cardStyle = cardStyle.BorderForeground(m.styles.Palette.Primary)
	} else if cursorMode == listCursorPassive {
		cardStyle = cardStyle.BorderForeground(m.styles.Palette.Secondary)
	}

	cardInnerWidth := max(contentWidth-cardStyle.GetHorizontalFrameSize(), 1)

	// Row 1 keeps sender and date aligned to the outer edges of the card.
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

	senderStyle := lipgloss.NewStyle().Bold(true).Faint(true).Foreground(m.styles.Palette.Highlight)
	if isSelected || cursorMode == listCursorPassive {
		senderStyle = senderStyle.Faint(false)
	}

	dateStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.SubText)

	senderMaxWidth := cardInnerWidth - lipgloss.Width(dateStr) - 1
	if senderMaxWidth < 0 {
		senderMaxWidth = 5
	}
	if len(sender) > senderMaxWidth {
		sender = sender[:senderMaxWidth-1] + "…"
	}

	gap := max(cardInnerWidth-lipgloss.Width(sender)-lipgloss.Width(dateStr), 1)
	spacer := strings.Repeat(" ", gap)
	row1 := fmt.Sprintf("%s%s%s", senderStyle.Render(sender), spacer, dateStyle.Render(dateStr))

	// Row 2 reserves space for the optional custom tag before truncating the subject.
	subject := msg.Subject
	if subject == "" {
		subject = "(No Subject)"
	}

	tagText := listMessageTag(msg)
	subjectPrefix := ""
	tagStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.Primary)

	if tagText != "" {
		subjectPrefix = tagStyle.Render("[" + tagText + "] ")
	}
	subjectWidth := max(cardInnerWidth-lipgloss.Width(subjectPrefix), 1)
	if lipgloss.Width(subject) > subjectWidth {
		subject = truncateText(subject, subjectWidth)
	}
	subjectStyle := lipgloss.NewStyle().Bold(true).Faint(true).Foreground(m.styles.Palette.Highlight)
	if isSelected || cursorMode == listCursorPassive {
		subjectStyle = subjectStyle.Faint(false)
	}

	row2 := subjectPrefix + subjectStyle.Render(subject)

	// Row 3 is always the preview line, truncated to the card body width.
	preview := strings.ReplaceAll(msg.Body, "\n", " ")
	if lipgloss.Width(preview) > cardInnerWidth {
		preview = truncateText(preview, cardInnerWidth)
	}

	previewStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.SubText)
	row3 := previewStyle.Render(preview)

	// Chips are inserted between the subject and preview only when the message has state badges.
	chips := renderMessageChips(msg, isSelected)
	rowStyle := lipgloss.NewStyle().Width(cardInnerWidth).MaxWidth(cardInnerWidth)

	row1 = rowStyle.Render(row1)
	row2 = rowStyle.Render(row2)
	row3 = rowStyle.Render(row3)
	rows := []string{row1, row2}
	if chips != "" {
		rows = append(rows, rowStyle.Render(chips))
	}
	rows = append(rows, row3)

	// Render once and return the measured height so the list viewport can pack cards precisely.
	block := lipgloss.JoinVertical(lipgloss.Left, rows...)
	rendered := cardStyle.Render(block)
	return rendered, lipgloss.Height(rendered)
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

func renderMessageChips(msg *models.Message, selected bool) string {
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

func listMessageTag(msg *models.Message) string {
	if msg == nil {
		return ""
	}
	for _, label := range msg.Labels {
		trimmed := strings.TrimSpace(label)
		if trimmed == "" || isSystemMailboxLabel(trimmed) {
			continue
		}
		return strings.ReplaceAll(trimmed, "_", " ")
	}
	return ""
}

func isSystemMailboxLabel(label string) bool {
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "inbox", "sent", "draft", "drafts", "archive", "spam", "trash":
		return true
	default:
		return false
	}
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
