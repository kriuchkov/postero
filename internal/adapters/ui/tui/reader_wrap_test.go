package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kriuchkov/postero/internal/core/models"
)

// TestSelectedCardIsContiguousInList guards the regression where the highlighted
// card's full-width background made the list's outer Width() re-wrap each row,
// inserting a blank filled line between rows (segmented accent bar, patchy fill).
func TestSelectedCardIsContiguousInList(t *testing.T) {
	m := testModel()
	base := time.Date(2026, 8, 3, 14, 23, 0, 0, time.UTC)
	m.messages = []*models.Message{
		{ID: "m1", From: "Энергосбыт <billing@example.com>", Subject: "Электронный счёт за июль", Body: "Счёт", Date: base, IsRead: true},
		{ID: "m2", From: "someone@example.com", Subject: "Second", Body: "body", Date: base, IsRead: false},
	}
	m.allMessages = append([]*models.Message{}, m.messages...)
	m.state = stateList
	m.listCursor = 0

	list := renderList(m, 44, 20)
	lines := strings.Split(ansi.Strip(list), "\n")

	borderRows := make([]int, 0, 4)
	for i, line := range lines {
		if strings.Contains(line, "▌") {
			borderRows = append(borderRows, i)
		}
	}
	require.GreaterOrEqual(t, len(borderRows), 3, "the selected card must render its accent border on every row")
	for k := 1; k < len(borderRows); k++ {
		assert.Equalf(t, borderRows[k-1]+1, borderRows[k],
			"accent border rows must be contiguous (no blank gap row between card rows): %v", borderRows)
	}
}

// TestReaderWrapsLongProse guards that long prose lines are soft-wrapped to the
// viewport width instead of running off the right edge.
func TestReaderWrapsLongProse(t *testing.T) {
	longLine := strings.Repeat("some readable words here ", 30)
	m := testModel()
	m.width, m.height = 120, 40
	m.state = stateContent
	m.messages = []*models.Message{{
		ID: "wrap-1", From: "Sender <s@example.com>", Subject: "Prose",
		Body: longLine, Date: time.Now(),
	}}
	m.listCursor = 0
	m.syncContentViewport(true)

	_, _, contentWidth := paneWidths(m, m.width)
	_, bodyWidth, _ := contentViewportLayout(m, contentWidth, m.height)

	lines := strings.Split(m.wrappedMessageBody(bodyWidth), "\n")
	require.Greater(t, len(lines), 1, "long prose must wrap onto multiple lines")
	for i, line := range lines {
		assert.LessOrEqualf(t, lipgloss.Width(line), bodyWidth,
			"reader line %d exceeds viewport width (%d): %q", i, bodyWidth, line)
	}
}

// TestReaderCollapsesLongURL guards the default: a long bare URL is shown as just
// its host (hiding the long query), while the full URL stays the OSC 8 target so a
// click still opens it in full.
func TestReaderCollapsesLongURL(t *testing.T) {
	longURL := "https://links.newsletter.example.com/Link?messageId=" + strings.Repeat("A", 120)
	out := wrapReaderBody("Pay here: "+longURL+" thanks", 60, false)

	visible := ansi.Strip(out)
	assert.Contains(t, visible, "links.newsletter.example.com", "the host must stay visible")
	assert.NotContains(t, visible, strings.Repeat("A", 20), "the long query must be hidden from the visible text")
	assert.LessOrEqual(t, lipgloss.Width(visible), 60, "the collapsed line must fit without wrapping")

	// The full URL is still the hyperlink target, and the link is closed.
	assert.Contains(t, out, ";"+longURL+"\x07", "the full URL must remain the hyperlink target")
	assert.Contains(t, out, "\x1b]8;;\x07", "the hyperlink must be closed")
}

// TestReaderMarkdownLinkShowsAnchorText guards that a Markdown link collapses to
// its anchor text (not the raw URL), still targeting the full URL.
func TestReaderMarkdownLinkShowsAnchorText(t *testing.T) {
	url := "https://pay.example.com/printServ?anstype=print&params=" + strings.Repeat("B", 60)
	out := wrapReaderBody("[Оплатить]("+url+")", 60, false)

	visible := ansi.Strip(out)
	assert.Contains(t, visible, "Оплатить", "the anchor text must be shown")
	assert.NotContains(t, visible, "printServ", "the raw URL must be hidden from the visible text")
	assert.Contains(t, out, ";"+url+"\x07", "the anchor must target the full URL")
}

// TestReaderKeepsShortURLVisible guards that a short URL is not needlessly
// collapsed — it stays readable in full.
func TestReaderKeepsShortURLVisible(t *testing.T) {
	url := "https://mos.ru/pay"
	out := wrapReaderBody("Open "+url, 60, false)
	assert.Contains(t, ansi.Strip(out), url, "a short URL should stay visible in full")
}

// TestReaderRendersBoldAndItalic guards that Markdown emphasis becomes ANSI
// bold/italic and the "**"/"*" markers are dropped from the visible text.
func TestReaderRendersBoldAndItalic(t *testing.T) {
	out := wrapReaderBody("A **bold word** and *italic* here", 80, false)

	assert.Contains(t, out, "\x1b[1m", "bold must be emitted as ANSI bold")
	assert.Contains(t, out, "\x1b[3m", "italic must be emitted as ANSI italic")
	// It must toggle attributes, not full-reset (which would clobber the colour).
	assert.NotContains(t, out, "\x1b[0m", "emphasis must not use a full SGR reset")

	visible := ansi.Strip(out)
	assert.NotContains(t, visible, "**", "the bold markers must be dropped")
	assert.Equal(t, "A bold word and italic here", strings.TrimSpace(visible))
}

// TestReaderBoldSurvivesWrapping guards that a bold span broken across a soft-wrap
// stays bold on every continuation line in the rendered reader (the viewport
// re-emits the active SGR per line).
func TestReaderBoldSurvivesWrapping(t *testing.T) {
	m := testModel()
	m.width, m.height = 80, 30
	m.state = stateContent
	m.messages = []*models.Message{{
		ID: "b", From: "S <s@x.com>", Subject: "T",
		Body: "**" + strings.Repeat("слово ", 30) + "**", Date: time.Now(),
	}}
	m.listCursor = 0
	m.syncContentViewport(true)

	view := renderContent(m, 60, 24)

	boldLines := 0
	for i, line := range strings.Split(view, "\n") {
		if !strings.Contains(ansi.Strip(line), "слово") {
			continue
		}
		assert.Containsf(t, line, "\x1b[1m", "wrapped bold line %d must remain bold", i)
		boldLines++
	}
	require.Greater(t, boldLines, 1, "the bold span must actually wrap across lines")
}

// TestReaderExpandShowsFullURL guards that the expanded mode reveals the full URL
// as visible (selectable/copyable) text, still wrapped to width.
func TestReaderExpandShowsFullURL(t *testing.T) {
	longURL := "https://links.newsletter.example.com/Link?messageId=" + strings.Repeat("A", 120)
	out := wrapReaderBody("Pay here: "+longURL+" thanks", 40, true)

	joined := strings.ReplaceAll(ansi.Strip(out), "\n", "")
	joined = strings.ReplaceAll(joined, " ", "")
	assert.Contains(t, joined, longURL, "expanded mode must show the full URL text")
	for i, line := range strings.Split(out, "\n") {
		assert.LessOrEqualf(t, lipgloss.Width(line), 40, "expanded line %d must still wrap to width", i)
	}
}

// TestToggleURLsKeyRevealsAndHidesInReader guards the reader key: 'U' flips
// between collapsed (host/anchor) and expanded (full URL) rendering.
func TestToggleURLsKeyRevealsAndHidesInReader(t *testing.T) {
	longURL := "https://links.newsletter.example.com/Link?messageId=" + strings.Repeat("A", 120)
	m := testModel()
	m.width, m.height = 120, 40
	m.state = stateContent
	m.messages = []*models.Message{{
		ID: "u1", From: "Sender <s@example.com>", Subject: "Pay",
		Body: "Pay here: " + longURL, Date: time.Now(),
	}}
	m.listCursor = 0
	m.syncContentViewport(true)

	_, _, contentWidth := paneWidths(m, m.width)
	_, bodyWidth, _ := contentViewportLayout(m, contentWidth, m.height)

	collapsed := ansi.Strip(m.wrappedMessageBody(bodyWidth))
	require.NotContains(t, collapsed, strings.Repeat("A", 30), "URLs start collapsed")

	m = updateModel(t, m, keyRune('U'))
	assert.True(t, m.expandURLs, "U enables full URLs")
	expanded := strings.ReplaceAll(ansi.Strip(m.wrappedMessageBody(bodyWidth)), "\n", "")
	assert.Contains(t, strings.ReplaceAll(expanded, " ", ""), longURL, "U reveals the full URL")

	m = updateModel(t, m, keyRune('U'))
	assert.False(t, m.expandURLs, "U again collapses URLs")
}
