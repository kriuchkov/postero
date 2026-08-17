package tui

import (
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"

	"github.com/kriuchkov/postero/internal/core/models"
	"github.com/kriuchkov/postero/pkg/htmlmd"
)

// renderListCard renders one measured message card so
// the list viewport can pack variable-height rows without extra layout passes.
func renderListCard(m Model, msg *models.Message, contentWidth int, cursorMode listCursorMode) (string, int) {
	isSelected := cursorMode == listCursorActive
	isVisual := cursorMode == listCursorVisual
	highlighted := isSelected || isVisual

	border := lipgloss.NormalBorder()
	if highlighted {
		border.Left = "▌"
	}
	cardStyle := lipgloss.NewStyle().
		Width(contentWidth).
		Padding(0, 1).
		Border(border, false, false, false, true).
		BorderForeground(m.styles.Palette.Faint)
	switch {
	case isSelected:
		cardStyle = cardStyle.BorderForeground(m.styles.Palette.Primary)
	case isVisual || cursorMode == listCursorPassive:
		cardStyle = cardStyle.BorderForeground(m.styles.Palette.Secondary)
	}
	if highlighted {
		// A solid background makes the selected/hovered tile read as one block,
		// even over a translucent terminal. It is applied to the whole card and
		// to each row (below) as one style over plain text, so lipgloss fills it
		// with no gaps — never composed from separately styled substrings.
		//
		// BorderBackground is essential: the "▌" left-half-block glyph only paints
		// its left half with the accent colour; without a background the right half
		// of that cell is transparent, leaving a thin see-through sliver between the
		// accent bar and the fill on a translucent terminal.
		cardStyle = cardStyle.
			Background(m.styles.Palette.Surface).
			BorderBackground(m.styles.Palette.Surface)
	}

	cardInnerWidth := max(contentWidth-cardStyle.GetHorizontalFrameSize(), 1)

	// ── plain layout, shared by both paths (no inner ANSI) ─────────────────────
	sender := strings.TrimSpace(msg.From)
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
	unread := !msg.IsRead && !msg.IsDraft

	const unreadColWidth = 2 // dot + trailing space; also the shared left gutter
	dot := " "
	if unread {
		dot = "●"
	}
	senderMaxWidth := max(cardInnerWidth-lipgloss.Width(dateStr)-1-unreadColWidth, 5)
	if lipgloss.Width(sender) > senderMaxWidth {
		sender = truncateText(sender, senderMaxWidth)
	}
	gap := max(cardInnerWidth-unreadColWidth-lipgloss.Width(sender)-lipgloss.Width(dateStr), 1)

	subject := msg.Subject
	if subject == "" {
		subject = "(No Subject)"
	}
	tagText := listMessageTag(msg)
	tagPrefix := ""
	if tagText != "" {
		tagPrefix = "[" + tagText + "] "
	}
	// Subject and preview sit under the sender text, past the same fixed gutter
	// that holds the unread dot, so every row shares one left edge.
	subjectWidth := max(cardInnerWidth-unreadColWidth-lipgloss.Width(tagPrefix), 1)
	if lipgloss.Width(subject) > subjectWidth {
		subject = truncateText(subject, subjectWidth)
	}

	preview := previewLine(msg.Body, max(cardInnerWidth-unreadColWidth, 1))

	pieces := cardPieces{dot: dot, sender: sender, gap: gap, dateStr: dateStr, tagPrefix: tagPrefix, subject: subject, preview: preview}
	var rows []string
	if highlighted {
		rows = highlightedCardRows(m, cardInnerWidth, pieces, msg)
	} else {
		rows = plainCardRows(m, cardInnerWidth, cursorMode, unread, pieces, msg)
	}

	block := lipgloss.JoinVertical(lipgloss.Left, rows...)
	rendered := cardStyle.Render(block)
	// The card separator is a detached faint rule below the card, not a border
	// side: it stays the same in every cursor state, never touches the left
	// accent bar (no corner glyph), and is indented past the border column so it
	// underlines only the content.
	separator := lipgloss.NewStyle().
		Foreground(m.styles.Palette.Faint).
		Render(" " + strings.Repeat("─", max(lipgloss.Width(rendered)-1, 1)))
	rendered = lipgloss.JoinVertical(lipgloss.Left, rendered, separator)
	return rendered, lipgloss.Height(rendered)
}

// cardPieces holds the plain (unstyled) layout strings shared by both render paths.
type cardPieces struct {
	dot       string
	sender    string
	gap       int
	dateStr   string
	tagPrefix string
	subject   string
	preview   string
}

// cardGutter is the fixed left gutter shared by every card row: the unread dot
// occupies it on the sender line, and the subject/preview rows are indented by it
// so all three lines align on one left edge.
const cardGutter = "  "

// highlightedCardRows renders the selected/hovered card: each row is a single
// style over plain text, so the surface background fills solidly with no gaps.
func highlightedCardRows(m Model, width int, p cardPieces, msg *models.Message) []string {
	base := lipgloss.NewStyle().Width(width).Background(m.styles.Palette.Surface)
	strong := base.Foreground(m.styles.Palette.Highlight).Bold(true)
	rows := []string{
		strong.Render(p.dot + " " + p.sender + strings.Repeat(" ", p.gap) + p.dateStr),
		strong.Render(cardGutter + p.tagPrefix + p.subject),
	}
	if badges := messageStateBadges(msg); badges != "" {
		rows = append(rows, base.Foreground(m.styles.Palette.Primary).Render(cardGutter+badges))
	}
	return append(rows, base.Foreground(m.styles.Palette.SubText).Render(cardGutter+p.preview))
}

// plainCardRows renders a non-selected card with per-segment colours (no fill).
func plainCardRows(m Model, width int, cursorMode listCursorMode, unread bool, p cardPieces, msg *models.Message) []string {
	rowStyle := lipgloss.NewStyle().Width(width).MaxWidth(width)

	marker := "  "
	if unread {
		marker = lipgloss.NewStyle().Foreground(m.styles.Palette.Primary).Render("●") + " "
	}
	senderStyle := lipgloss.NewStyle().Bold(true).Faint(true).Foreground(m.styles.Palette.Highlight)
	if cursorMode == listCursorPassive || unread {
		senderStyle = senderStyle.Faint(false)
	}
	dateStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.SubText)
	row1 := marker + senderStyle.Render(p.sender) + strings.Repeat(" ", p.gap) + dateStyle.Render(p.dateStr)

	subjectStyle := lipgloss.NewStyle().Bold(true).Faint(true).Foreground(m.styles.Palette.Highlight)
	if cursorMode == listCursorPassive {
		subjectStyle = subjectStyle.Faint(false)
	}
	row2 := subjectStyle.Render(p.subject)
	if p.tagPrefix != "" {
		row2 = lipgloss.NewStyle().Foreground(m.styles.Palette.Primary).Render(p.tagPrefix) + row2
	}
	row2 = cardGutter + row2 // align subject under the sender text, past the gutter

	row3 := cardGutter + lipgloss.NewStyle().Foreground(m.styles.Palette.SubText).Render(p.preview)

	rows := []string{rowStyle.Render(row1), rowStyle.Render(row2)}
	if chips := renderMessageChips(msg, false); chips != "" {
		rows = append(rows, rowStyle.Render(cardGutter+chips))
	}
	return append(rows, rowStyle.Render(row3))
}

// messageStateBadges returns the plain state labels for a message (Draft/Spam/
// Archive) — the unread state is shown as an inline dot, not a badge.
func messageStateBadges(msg *models.Message) string {
	if msg == nil {
		return ""
	}
	badges := make([]string, 0, 3)
	if msg.IsDraft {
		badges = append(badges, "Draft")
	}
	if msg.IsSpam {
		badges = append(badges, "Spam")
	}
	if slices.Contains(msg.Labels, "archive") {
		badges = append(badges, "Archive")
	}
	return strings.Join(badges, "  ")
}

// previewLine builds the short, single-line card preview. It is deliberately
// cheap so it can run for every visible card each frame: it never scans more than
// a small bounded window of the body, strips HTML tags (and drops <style>/<script>
// content), and collapses whitespace. Full rendering lives in getFilteredBody.
func previewLine(body string, width int) string {
	maxRunes := width + 16 // a little slack before truncateText adds the ellipsis
	if htmlmd.LooksLikeHTML(body) {
		body = stripHTMLPreview(body, maxRunes)
	} else {
		body = collapseRunes(body, maxRunes)
	}
	// Drop Markdown emphasis/code markers so the one-line preview stays clean —
	// the reader renders "**bold**" as bold, but the list shows plain text.
	body = strings.NewReplacer("**", "", "`", "").Replace(body)
	if lipgloss.Width(body) > width {
		body = truncateText(body, width)
	}
	return body
}

// collapseRunes returns at most maxRunes runes of s with all whitespace collapsed
// to single spaces, scanning no further than needed.
func collapseRunes(s string, maxRunes int) string {
	var b strings.Builder
	runes := 0
	for _, r := range s {
		if r == '\n' || r == '\t' || r == '\r' {
			r = ' '
		}
		b.WriteRune(r)
		runes++
		if runes >= maxRunes {
			break
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// stripHTMLPreview extracts up to maxRunes runes of visible text from HTML,
// dropping tags and skipping <style>/<script> bodies. It is bounded by both
// maxRunes of output and an absolute byte cap so a huge document stays cheap.
func stripHTMLPreview(body string, maxRunes int) string {
	const scanCap = 8 << 10
	lower := strings.ToLower(body)
	var b strings.Builder
	inTag := false
	runes := 0
	for i := 0; i < len(body) && i < scanCap && runes < maxRunes; {
		switch {
		case strings.HasPrefix(lower[i:], "<style"):
			i = skipHTMLElement(lower, i, "</style>")
			continue
		case strings.HasPrefix(lower[i:], "<script"):
			i = skipHTMLElement(lower, i, "</script>")
			continue
		}
		r, size := utf8.DecodeRuneInString(body[i:])
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
			b.WriteByte(' ') // element boundaries become spaces
		case !inTag:
			b.WriteRune(r)
			runes++
		}
		i += size
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func skipHTMLElement(lower string, i int, closing string) int {
	if idx := strings.Index(lower[i:], closing); idx >= 0 {
		return i + idx + len(closing)
	}
	return len(lower)
}

func truncateText(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}
	if width == 1 {
		return "…"
	}
	runes := []rune(value)
	// The visible prefix is at most `width` cells, and every rune is ≥1 cell, so
	// we never need to consider more than the first `width` runes. Slicing here
	// keeps the shrink loop O(width) instead of O(len(value)²) — critical when a
	// card preview is built from a large (e.g. HTML) message body.
	if len(runes) > width {
		runes = runes[:width]
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
