package tui

import (
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

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
	sender := sanitizeCellText(strings.TrimSpace(msg.From))
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

	subject := sanitizeCellText(msg.Subject)
	if subject == "" {
		subject = "(No Subject)"
	}
	tagText := listMessageTag(msg)
	tagPrefix := ""
	if tagText != "" {
		tagPrefix = "[" + tagText + "] "
	}
	// When the list mixes mail from several accounts, each card names its owner
	// on the subject row's right edge — otherwise identical newsletters from two
	// inboxes are indistinguishable.
	accountTag := listCardAccountTag(m, msg)
	accountReserve := 0
	if accountTag != "" {
		accountReserve = lipgloss.Width(accountTag) + 1
	}
	// Subject and preview sit under the sender text, past the same fixed gutter
	// that holds the unread dot, so every row shares one left edge.
	subjectWidth := max(cardInnerWidth-unreadColWidth-lipgloss.Width(tagPrefix)-accountReserve, 1)
	if lipgloss.Width(subject) > subjectWidth {
		subject = truncateText(subject, subjectWidth)
	}

	preview := sanitizeCellText(previewLine(msg.Body, max(cardInnerWidth-unreadColWidth, 1)))

	pieces := cardPieces{dot: dot, sender: sender, gap: gap, dateStr: dateStr, tagPrefix: tagPrefix, subject: subject, preview: preview, accountTag: accountTag}
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
	dot        string
	sender     string
	gap        int
	dateStr    string
	tagPrefix  string
	subject    string
	preview    string
	accountTag string
}

// sanitizeCellText makes a string width-stable for cell-based layout by
// dropping runes that make the terminal's cell width diverge from the measured
// width. A row lipgloss measured as exactly fitting then renders one cell
// wider, wraps in the terminal, and shifts the whole frame — the classic
// "everything doubles while scrolling" corruption. One mismatched line is
// enough: the renderer keeps diffing against the wrong screen afterwards.
func sanitizeCellText(s string) string {
	return sanitizeWidthText(s, false)
}

// sanitizeWidthText is the shared implementation: cells flatten newlines to
// spaces, the reader body keeps them. Tabs become single spaces (a terminal
// expands \t to the next tab stop — up to 8 cells — while the width math
// counts 1), and \r plus all other control characters are dropped outright (a
// stray carriage return sends the terminal cursor back to column 0 and the
// rest of the line overwrites the pane next to it).
func sanitizeWidthText(s string, keepNewlines bool) string {
	changed := false
	for _, r := range s {
		if widthSafeRune(r, keepNewlines) != r {
			changed = true
			break
		}
	}
	if !changed {
		return s
	}
	return strings.Map(func(r rune) rune {
		return widthSafeRune(r, keepNewlines)
	}, s)
}

// widthSafeRune maps a rune to its width-stable replacement: itself when it is
// on the allowlist, a space for separator-like runes, or -1 (drop) otherwise.
//
// This is deliberately an ALLOWLIST, not a blocklist: every Unicode release
// (and every terminal) ships new glyphs whose rendered width disagrees with
// the measured width — variation selectors, narrow-classified emoji, tabs,
// stray carriage returns. Enumerating offenders is a losing game; enumerating
// the scripts and symbols whose cell width is actually predictable is short.
func widthSafeRune(r rune, keepNewlines bool) rune {
	switch r {
	case '\n':
		if keepNewlines {
			return '\n'
		}
		return ' '
	case '\t', '\u00A0': // tab and no-break space flatten to a plain space
		return ' '
	}
	if isWidthSafeRune(r) {
		return r
	}
	return -1
}

// isWidthSafeRune reports whether a rune's terminal cell width is predictable:
// narrow scripts and punctuation that always render one cell, wide CJK that
// always renders two, and pictographs only when the width libraries already
// measure them as two cells (matching how terminals draw them).
func isWidthSafeRune(r rune) bool {
	switch {
	// Narrow, one cell everywhere: ASCII, Latin (incl. extended), Greek,
	// Cyrillic, Armenian, common punctuation, currency (₽), letterlike (№ ™).
	case r >= 0x20 && r <= 0x7E,
		r >= 0xA1 && r <= 0x2AF,
		r >= 0x370 && r <= 0x58F,
		r >= 0x1E00 && r <= 0x1FFF,
		r >= 0x2010 && r <= 0x2027,
		r >= 0x2030 && r <= 0x205E,
		r >= 0x20A0 && r <= 0x20CF,
		r >= 0x2100 && r <= 0x2134:
		return true
	// Also narrow and predictable, and common in real subject lines: arrows
	// (→), math operators (±, ≤), enclosed alphanumerics (①), box drawing and
	// block elements (─ ▌ — the TUI draws its own frame with these), and
	// geometric shapes (● ▶).
	case r >= 0x2190 && r <= 0x22FF,
		r >= 0x2460 && r <= 0x24FF,
		r >= 0x2500 && r <= 0x25FF:
		return true
	// Wide, two cells everywhere: kana, CJK ideographs, hangul, fullwidth forms.
	case r >= 0x3040 && r <= 0x30FF,
		r >= 0x3400 && r <= 0x9FFF,
		r >= 0xAC00 && r <= 0xD7A3,
		r >= 0xFF01 && r <= 0xFF60:
		return true
	// Pictographs and emoji: only when measured wide — a narrow-measured
	// pictograph (🏖 U+1F3D6, ❤ U+2764) is drawn two cells by macOS terminals
	// while the layout math places it as one, shifting the whole frame.
	case isPictographRune(r):
		return ansi.StringWidth(string(r)) == 2
	default:
		return false
	}
}

// isPictographRune reports whether a rune lives in an emoji/pictograph block —
// the glyphs whose terminal cell width agrees with the measured width least
// often. This is the single definition of that set: the sanitizer keeps only
// the ones already measured as two cells, and the width self-check measures the
// same set pessimistically. Two lists would drift apart.
func isPictographRune(r rune) bool {
	return (r >= 0x2300 && r <= 0x23FF) || // misc technical (⌚ ⏱)
		(r >= 0x2600 && r <= 0x27BF) || // misc symbols and dingbats (☀ ❤ ✈)
		(r >= 0x2B00 && r <= 0x2BFF) || // misc symbols and arrows (⭐ ⬛)
		(r >= 0x1F000 && r <= 0x1FAFF) || // emoji blocks (🏖 🎁)
		r == 0x303D || r == 0x3030 || // 〽 〰, emoji-presented CJK symbols
		(r >= 0xE000 && r <= 0xF8FF) // private use: width depends on the font
}

// listCardAccountTag names the card's owning account when the list can mix
// mail from several accounts: more than one account configured and no account
// scope active in the sidebar.
func listCardAccountTag(m Model, msg *models.Message) string {
	if msg == nil || len(m.accountNames) < 2 || strings.TrimSpace(m.activeAccountID) != "" {
		return ""
	}
	account := strings.TrimSpace(msg.AccountID)
	if account == "" {
		return ""
	}
	return "@" + truncateText(account, 14)
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
	subjectRow := strong.Render(cardGutter + p.tagPrefix + p.subject)
	if p.accountTag != "" {
		// Two runs instead of one, but both carry the surface background
		// explicitly, so the solid fill still has no gaps.
		text := cardGutter + p.tagPrefix + p.subject
		gap := max(width-lipgloss.Width(text)-lipgloss.Width(p.accountTag), 1)
		subjectRow = lipgloss.NewStyle().Background(m.styles.Palette.Surface).Foreground(m.styles.Palette.Highlight).Bold(true).
			Render(text+strings.Repeat(" ", gap)) +
			lipgloss.NewStyle().Background(m.styles.Palette.Surface).Foreground(m.styles.Palette.SubText).
				Render(p.accountTag)
	}
	rows := []string{
		strong.Render(p.dot + " " + p.sender + strings.Repeat(" ", p.gap) + p.dateStr),
		subjectRow,
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
	if p.accountTag != "" {
		used := len(cardGutter) + lipgloss.Width(p.tagPrefix) + lipgloss.Width(p.subject)
		gap := max(width-used-lipgloss.Width(p.accountTag), 1)
		row2 += strings.Repeat(" ", gap) +
			lipgloss.NewStyle().Foreground(m.styles.Palette.SubText).Render(p.accountTag)
	}

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
	// truncateText is the shared chokepoint for every single-line cell (card
	// rows, reader meta, popup fields) — sanitize here so no width-hazard rune
	// survives into a measured line.
	value = sanitizeCellText(value)
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
