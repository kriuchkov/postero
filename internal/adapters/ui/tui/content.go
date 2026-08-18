package tui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// readerLinkPattern matches, in one pass, either a Markdown link
// "[text](https://url)" (groups 1,2) or a remaining bare http(s) URL (group 3).
// A single pass avoids the second form matching the URL embedded inside the OSC 8
// escape emitted for the first.
var readerLinkPattern = regexp.MustCompile(`\[([^\]]*)\]\((https?://[^)\s]+)\)|(https?://[^\s)]+)`)

// collapsedURLLen is the length above which a bare URL is shown as just its host
// (with "/…" when it has a path), hiding the long query soup by default. The full
// URL always stays the hyperlink target, so a click still opens it in full.
const collapsedURLLen = 50

func collapseURL(raw string) string {
	if lipgloss.Width(raw) <= collapsedURLLen {
		return raw
	}
	s := strings.TrimPrefix(strings.TrimPrefix(raw, "https://"), "http://")
	host, _, hasPath := strings.Cut(s, "/")
	switch {
	case host == "":
		return truncateText(raw, collapsedURLLen)
	case hasPath:
		return host + "/…" // there is a path/query we are hiding
	default:
		return host
	}
}

// linkifyURLs makes URLs clickable in the reader, emitting each as an OSC 8
// hyperlink whose target is the full URL (so it stays openable in full even after
// soft-wrapping). By default the visible label is compact — a Markdown link shows
// just its text, a long bare URL just its host. When expand is true the label is
// the full URL instead, so it can be read and selected/copied. A distinct id per
// link groups wrapped fragments for hover.
func linkifyURLs(text string, expand bool) string {
	id := 0
	return readerLinkPattern.ReplaceAllStringFunc(text, func(match string) string {
		groups := readerLinkPattern.FindStringSubmatch(match)
		url, label := groups[3], ""
		if groups[2] != "" { // Markdown link: use its text, fall back to the host
			url, label = groups[2], strings.TrimSpace(groups[1])
		}
		switch {
		case expand:
			label = url
		case label == "":
			label = collapseURL(url)
		}
		id++
		return ansi.SetHyperlink(url, "id=pstr"+strconv.Itoa(id)) + label + ansi.ResetHyperlink()
	})
}

// markdownBoldPattern and markdownItalicPattern match "**bold**" and "*italic*"
// emphasis produced by the HTML→Markdown converter.
var (
	markdownBoldPattern   = regexp.MustCompile(`\*\*(.+?)\*\*`)
	markdownItalicPattern = regexp.MustCompile(`\*([^*\n]+?)\*`)
)

// styleEmphasis renders Markdown emphasis as ANSI in the reader: "**x**" becomes
// bold, "*x*" italic, and the markers themselves are dropped. It toggles only the
// bold/italic attributes (SGR 1/22 and 3/23), never a full reset, so it does not
// clobber the surrounding foreground colour.
func styleEmphasis(text string) string {
	text = markdownBoldPattern.ReplaceAllString(text, "\x1b[1m$1\x1b[22m")
	text = markdownItalicPattern.ReplaceAllString(text, "\x1b[3m$1\x1b[23m")
	return text
}

// wrapReaderBody makes clickable, styled, soft-wrapped reader text: URLs become
// OSC 8 hyperlinks, Markdown emphasis becomes ANSI bold/italic, then the whole
// body is wrapped to width (long tokens broken) so nothing runs off the right edge
// while links stay openable in full.
func wrapReaderBody(text string, width int, expand bool) string {
	// Width-hazard runes (variation selectors, ZWJ, tabs, stray \r) must go
	// before wrapping: a body line the wrapper measured as fitting would
	// otherwise render wider in the terminal and shift the whole frame.
	return wrapToWidth(styleEmphasis(linkifyURLs(sanitizeWidthText(text, true), expand)), width)
}

// wrapToWidth soft-wraps text to width, breaking overly long tokens (e.g. URLs)
// so the reader never runs a line off the right edge of the viewport.
func wrapToWidth(text string, width int) string {
	if width < 1 {
		return text
	}
	return ansi.Wrap(text, width, "")
}

// wrappedMessageBody returns the selected message body soft-wrapped to width. It
// serves the cache populated by syncContentViewport when it still matches, so
// View() (every keystroke/tick) does not re-wrap a large body each frame.
func (m Model) wrappedMessageBody(width int) string {
	if width > 0 && width == m.wrappedBodyWidth && m.wrappedBodyExpand == m.expandURLs &&
		m.currentMessageID() == m.renderedBodyID {
		return m.wrappedBody
	}
	return wrapReaderBody(m.currentMessageBody(), width, m.expandURLs)
}

func renderContent(m Model, width, height int) string {
	style := m.styles.Content.Width(width).Height(height)
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(m.styles.Palette.Highlight)

	if m.state == stateCompose {
		return style.Render(renderComposeContent(m, width, height, titleStyle))
	}

	if len(m.messages) == 0 || m.listCursor < 0 || m.listCursor >= len(m.messages) {
		empty := lipgloss.JoinVertical(
			lipgloss.Left,
			titleStyle.Render("Welcome to Postero"),
			lipgloss.NewStyle().Foreground(m.styles.Palette.SubText).Render("Choose a mailbox and select a message to start reading."),
		)
		return style.Render(lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, empty))
	}

	headerBlock, bodyWidth, bodyHeight := contentViewportLayout(m, width, height)
	bodyViewport := m.contentViewport
	bodyViewport.Width = bodyWidth
	bodyViewport.Height = bodyHeight
	bodyViewport.SetContent(m.wrappedMessageBody(bodyWidth))
	bodyViewport.Style = lipgloss.NewStyle().Foreground(m.styles.Palette.Text)
	if bodyViewport.Height < 1 {
		bodyViewport.Height = 1
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		headerBlock,
		bodyViewport.View(),
	)

	return style.Render(content)
}

func renderComposeContent(m Model, width, height int, titleStyle lipgloss.Style) string {
	if m.activeDraft == nil {
		return "Loading draft..."
	}

	labelStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.SubText).Width(8).Align(lipgloss.Right).MarginRight(1)
	modeStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.SubText)

	title, _ := composeContextText(m)
	headerLines := []string{titleStyle.Render(title)}
	headerLines = append(headerLines, modeStyle.Render(composeModeHint(m.composeEditing, width)))
	header := lipgloss.JoinVertical(lipgloss.Left, headerLines...)
	separator := lipgloss.NewStyle().
		Width(width-4).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(m.styles.Palette.Faint).
		MarginTop(1).
		MarginBottom(1).
		Render("")

	m.bodyInput.SetWidth(width - 4)
	m.bodyInput.SetHeight(height - 12)
	bodyView := composeBodyView(m)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"",
		lipgloss.JoinHorizontal(lipgloss.Top, labelStyle.Render("Account:"), composeAccountView(m)),
		lipgloss.JoinHorizontal(lipgloss.Top, labelStyle.Render("To:"), composeFieldView(m, 1, m.toInput.View())),
		lipgloss.JoinHorizontal(lipgloss.Top, labelStyle.Render("Subject:"), composeFieldView(m, 2, m.subjectInput.View())),
		separator,
		bodyView,
	)
}

func composeAccountView(m Model) string {
	accountStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.Text)
	accountHintStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.SubText)
	accountValue := m.composeAccountLabel()
	accountView := accountStyle.Render(accountValue)
	if m.focusIndex != 0 {
		return accountView
	}
	accountView = lipgloss.NewStyle().Foreground(m.styles.Palette.Primary).Bold(true).Render(accountValue)
	return lipgloss.JoinHorizontal(lipgloss.Left, accountView, accountHintStyle.Render("  h/l or ↑/↓ switch"))
}

func composeFieldView(m Model, focusIndex int, view string) string {
	if m.focusIndex != focusIndex {
		return view
	}
	return lipgloss.NewStyle().Foreground(m.styles.Palette.Primary).Render(view)
}

func composeBodyView(m Model) string {
	bodyView := m.bodyInput.View()
	if m.focusIndex != 3 {
		return bodyView
	}
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(m.styles.Palette.Primary).
		Render(bodyView)
}

func composeContextText(m Model) (string, string) {
	title := strings.TrimSpace(m.composeTitle)
	if title == "" {
		title = "Composer"
	}
	hint := strings.TrimSpace(m.composeHint)
	if hint == "" {
		hint = "Write now, save when ready."
	}
	return title, hint
}

func composeModeHint(composeEditing bool, width int) string {
	if composeEditing {
		candidates := []string{
			"Insert. Esc normal.",
			"Insert. Esc normal.",
		}
		for _, candidate := range candidates {
			if lipgloss.Width(candidate) <= max(width-4, 1) {
				return candidate
			}
		}
		return candidates[len(candidates)-1]
	}
	candidates := []string{
		"Normal. Enter/i/o/O edit.",
		"Normal. i/o/O edit.",
	}
	for _, candidate := range candidates {
		if lipgloss.Width(candidate) <= max(width-4, 1) {
			return candidate
		}
	}
	return candidates[len(candidates)-1]
}

func contentViewportLayout(m Model, width, height int) (string, int, int) {
	innerWidth := max(1, width-4)
	titleStyle := paneTitleStyle(m, stateContent).MarginBottom(1).Width(innerWidth)
	const metaLabelWidth = 8
	metaLabelStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.SubText).Width(metaLabelWidth)
	metaValueStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.Text)
	metaValueWidth := max(1, innerWidth-metaLabelWidth)
	hintStyle := paneSubtitleStyle(m, stateContent)
	statusStyle := lipgloss.NewStyle().Foreground(m.styles.Palette.Highlight).Background(m.styles.Palette.Faint).Padding(0, 1)
	renderHeader := func(label, value string) string {
		// Keep every meta row on a single line: a long From/To must not wrap and
		// overflow the pane width or push the body off-screen.
		return lipgloss.JoinHorizontal(lipgloss.Top,
			metaLabelStyle.Render(label+":"),
			metaValueStyle.Render(truncateText(value, metaValueWidth)),
		)
	}

	if len(m.messages) == 0 || m.listCursor < 0 || m.listCursor >= len(m.messages) || m.messages[m.listCursor] == nil {
		return "", innerWidth, max(1, height)
	}

	msg := m.messages[m.listCursor]
	// A long subject is truncated to one line so the header height stays fixed.
	subject := truncateText(msg.Subject, innerWidth)
	from := renderHeader("From", msg.From)
	to := renderHeader("To", strings.Join(msg.To, ", "))
	date := renderHeader("Date", msg.Date.Format("Mon, 02 Jan 2006 15:04"))
	mailbox := renderHeader("Mailbox", currentMailboxTitle(m))

	headerMeta := lipgloss.JoinVertical(lipgloss.Left, from, to, date, mailbox)
	separator := lipgloss.NewStyle().
		Width(innerWidth).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(m.styles.Palette.Faint).
		MarginTop(1).
		MarginBottom(1).
		Render("")
	bodyWidth := innerWidth
	bodyHeight := max(height-lipgloss.Height(lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render(subject),
		renderMessageChips(msg, false),
		headerMeta,
		separator,
	)), 1)
	statusText := contentViewportStatus(m.contentViewport.YOffset, bodyHeight, contentLineCount(m.wrappedMessageBody(bodyWidth)))
	statusView := statusStyle.Render(statusText)
	readerControls := joinHeaderColumns(
		bodyWidth,
		hintStyle.Render(contentViewportHint(bodyWidth, lipgloss.Width(statusView))),
		statusView,
	)

	headerBlock := lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render(subject),
		renderMessageChips(msg, false),
		headerMeta,
		readerControls,
		separator,
	)

	bodyHeight = max(height-lipgloss.Height(headerBlock), 1)

	return headerBlock, bodyWidth, bodyHeight
}

func contentViewportHint(width, statusWidth int) string {
	availableWidth := width - statusWidth - 2
	candidates := []string{
		"h back | j/k line | ctrl+d/u | gg/G",
		"j/k | ctrl+d/u | gg/G",
		"j/k | gg/G",
	}
	for _, candidate := range candidates {
		if lipgloss.Width(candidate) <= availableWidth {
			return candidate
		}
	}
	return "j/k"
}

func contentViewportStatus(offset, bodyHeight, totalLines int) string {
	if totalLines <= 0 {
		return "Empty"
	}
	firstLine := min(totalLines, offset+1)
	lastLine := min(totalLines, offset+max(1, bodyHeight))
	position := "Top"
	if offset > 0 {
		if lastLine >= totalLines {
			position = "Bottom"
		} else {
			position = fmt.Sprintf("%d%%", (lastLine*100)/totalLines)
		}
	}
	return fmt.Sprintf("%s • %d-%d/%d", position, firstLine, lastLine, totalLines)
}

func contentLineCount(content string) int {
	if content == "" {
		return 0
	}
	return strings.Count(content, "\n") + 1
}

func (m Model) composeAccountLabel() string {
	if m.activeDraft == nil {
		return ""
	}
	accountID := strings.TrimSpace(m.activeDraft.AccountID)
	if accountID == "" {
		accountID = m.defaultAcctID
	}
	from := strings.TrimSpace(m.senderForAccount(accountID))
	if from == "" {
		return accountID
	}
	if accountID == "" {
		return from
	}
	return accountID + " <" + from + ">"
}
