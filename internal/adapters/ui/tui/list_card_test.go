package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kriuchkov/postero/internal/core/models"
)

// forceColorProfile makes lipgloss emit ANSI colour codes deterministically so
// the tests can inspect the rendered background. It is not parallel-safe (global
// renderer state), so callers must not use t.Parallel.
func forceColorProfile(t *testing.T) {
	t.Helper()
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })
}

func cardMessage() *models.Message {
	return &models.Message{
		ID:      "card-1",
		From:    "Nikita Kryuchkov <nk@example.com>",
		Subject: "test subject",
		Body:    "hello world preview text",
		Date:    time.Date(2026, 7, 31, 21, 19, 0, 0, time.UTC),
		Labels:  []string{"inbox"},
		IsRead:  true,
	}
}

// TestSelectedCardHasSolidBackgroundOnEveryRow guards the regression where the
// hovered/selected tile fill was broken up by ANSI resets, leaving gaps that let
// the terminal background show through — including a transparent sliver at the
// accent border and interior transparent rows.
func TestSelectedCardHasSolidBackgroundOnEveryRow(t *testing.T) {
	forceColorProfile(t)
	m := testModel()
	surface := "48;5;" + string(m.styles.Palette.Surface)

	rendered, _ := renderListCard(m, cardMessage(), 44, listCursorActive)

	lines := strings.Split(strings.TrimRight(rendered, "\n"), "\n")
	require.NotEmpty(t, lines)

	// Every content row must carry the fill, AND the fill must be contiguous —
	// no transparent gap row between the first and last filled row.
	firstFilled, lastFilled := -1, -1
	for i, line := range lines {
		if strings.Contains(line, surface) {
			if firstFilled == -1 {
				firstFilled = i
			}
			lastFilled = i
		}
	}
	require.NotEqual(t, -1, firstFilled, "selected card must be filled")
	for i := firstFilled; i <= lastFilled; i++ {
		assert.Contains(t, lines[i], surface, "row %d is a transparent gap inside the filled card", i)
	}
	assert.GreaterOrEqual(t, lastFilled-firstFilled+1, 3, "sender, subject and preview rows must all be filled")

	// The accent border cell must also carry the fill, or the "▌" half-block leaves
	// a see-through sliver between the accent bar and the surface on a translucent
	// terminal. After the border escape sets the surface bg, "▌" follows it directly.
	assert.Contains(t, lines[firstFilled], surface+"m▌",
		"the accent border cell must carry the surface background (no transparent sliver)")

	// An unselected card must have no fill at all.
	unselected, _ := renderListCard(m, cardMessage(), 44, listCursorNone)
	assert.NotContains(t, unselected, surface, "unselected card must not be filled")
}

// TestSelectedCardKeepsSenderAndDateOnOneLine guards the "date поехало" regression:
// the sender and date must stay on a single, non-wrapping row within the card.
func TestSelectedCardKeepsSenderAndDateOnOneLine(t *testing.T) {
	forceColorProfile(t)
	m := testModel()

	rendered, _ := renderListCard(m, cardMessage(), 32, listCursorActive)
	lines := strings.Split(ansi.Strip(rendered), "\n")

	var dateRow string
	contentRows := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			contentRows++
		}
		if strings.Contains(line, "31/07") {
			dateRow = line
		}
	}
	require.NotEmpty(t, dateRow, "the date must be rendered")
	assert.Contains(t, dateRow, "Nikita", "sender and date must share one line (no wrap)")
	// Exactly sender, subject, preview and bottom-border rows — nothing wrapped
	// onto extra lines.
	assert.Equal(t, 4, contentRows, "no row may wrap onto an extra line")
}

// TestCardHeightStableAcrossStateAndSelection guards the layout jump the user saw
// when reading a message (unread badge vanished) or moving the cursor.
func TestCardHeightStableAcrossStateAndSelection(t *testing.T) {
	forceColorProfile(t)
	m := testModel()

	read := cardMessage()
	read.IsRead = true
	unread := cardMessage()
	unread.IsRead = false

	_, hReadSelected := renderListCard(m, read, 44, listCursorActive)
	_, hUnreadSelected := renderListCard(m, unread, 44, listCursorActive)
	_, hReadPlain := renderListCard(m, read, 44, listCursorNone)

	assert.Equal(t, hReadSelected, hUnreadSelected, "reading a message must not change the card height")
	assert.Equal(t, hReadSelected, hReadPlain, "selecting a card must not change its height")
}

func TestUnreadDotShownOnlyForUnread(t *testing.T) {
	m := testModel()

	unread := cardMessage()
	unread.IsRead = false
	read := cardMessage()
	read.IsRead = true

	unreadCard, _ := renderListCard(m, unread, 44, listCursorNone)
	readCard, _ := renderListCard(m, read, 44, listCursorNone)

	assert.Contains(t, ansi.Strip(unreadCard), "●", "unread messages show an inline dot")
	assert.NotContains(t, ansi.Strip(readCard), "●", "read messages have no dot")
}

// TestCardHasNoInterCardMargin guards the tighter list: a card is exactly its
// content rows plus the bottom-border separator (4, or 5 with a state chip),
// with no blank margin row that would space the cards apart.
func TestCardHasNoInterCardMargin(t *testing.T) {
	m := testModel()

	plain := &models.Message{ID: "p", From: "a@example.com", Subject: "S", Body: "B", Date: cardMessage().Date, IsRead: true}
	_, h := renderListCard(m, plain, 44, listCursorNone)
	assert.Equal(t, 4, h, "a plain card is sender+subject+preview+border with no margin")
	assert.Equal(t, 4, listCardHeight(plain), "the height estimate must match the render")

	chipped := &models.Message{ID: "c", From: "a@example.com", Subject: "S", Body: "B", Date: cardMessage().Date, IsDraft: true}
	_, hc := renderListCard(m, chipped, 44, listCursorNone)
	assert.Equal(t, 5, hc, "a card with a state chip adds exactly one row")
	assert.Equal(t, 5, listCardHeight(chipped), "the height estimate must match the render")
}

// TestCardShowsAccountTagInMixedList: with several accounts and no account
// scope, each card names its owner; scoping to one account (or having a single
// account) drops the tag.
func TestCardShowsAccountTagInMixedList(t *testing.T) {
	m := testModel()
	m.accountNames = []string{"personal", "work"}
	msg := cardMessage()
	msg.AccountID = "work"

	for _, mode := range []listCursorMode{listCursorNone, listCursorActive} {
		rendered, _ := renderListCard(m, msg, 44, mode)
		assert.Contains(t, ansi.Strip(rendered), "@work", "mixed list must name the owning account (mode %v)", mode)
	}

	m.activeAccountID = "work"
	rendered, _ := renderListCard(m, msg, 44, listCursorNone)
	assert.NotContains(t, ansi.Strip(rendered), "@work", "account-scoped list needs no tag")

	m.activeAccountID = ""
	m.accountNames = []string{"work"}
	rendered, _ = renderListCard(m, msg, 44, listCursorNone)
	assert.NotContains(t, ansi.Strip(rendered), "@work", "single-account list needs no tag")
}

// TestCardStripsEmojiVariationSelectors guards the frame-shift bug: VS16/VS15
// make terminals render a glyph wider or narrower than lipgloss measures it, so
// a subject like "🏖️" wraps the row and shifts the whole frame during scroll.
func TestCardStripsEmojiVariationSelectors(t *testing.T) {
	m := testModel()
	msg := cardMessage()
	msg.Subject = "Важное \u2764\ufe0f письмо \U0001F381"
	msg.From = "Отель \U0001F3D6\ufe0f <hotel@example.com>"
	msg.Body = "Пляж \U0001F3D6\ufe0f ждёт \u200d\u200b"

	rendered, _ := renderListCard(m, msg, 44, listCursorNone)
	clean := ansi.Strip(rendered)
	assert.NotContains(t, clean, "\ufe0f", "variation selectors must be stripped from card text")
	assert.NotContains(t, clean, "\u200d", "zero-width joiners must be stripped from card text")
	assert.NotContains(t, clean, "\u200b", "zero-width spaces must be stripped from card text")
	// Narrow-measured pictographs (❤ U+2764, 🏖 U+1F3D6) render two cells on
	// macOS terminals while measuring one — they must go entirely.
	assert.NotContains(t, clean, "\u2764", "narrow-measured pictographs must be dropped")
	assert.NotContains(t, clean, "\U0001F3D6", "narrow-measured pictographs must be dropped")
	assert.Contains(t, clean, "\U0001F381", "wide-measured emoji are width-stable and survive")
}

// TestSanitizeWidthTextDropsControlRunes: tabs expand to a tab stop and \r
// rewinds the cursor — both corrupt the frame no matter what the width math
// says, so no control rune may survive into rendered text.
func TestSanitizeWidthTextDropsControlRunes(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "a bc", sanitizeCellText("a\tb\rc"), "tab becomes a space, carriage return is dropped")
	assert.Equal(t, "one two", sanitizeCellText("one\ntwo"))
	assert.Equal(t, "line1\nline2 x", sanitizeWidthText("line1\r\nline2\tx", true))
	assert.Equal(t, "clean", sanitizeWidthText("clean", true))
}

// TestSanitizeKeepsPredictableSymbols: the width allowlist must not eat glyphs
// that render exactly one cell everywhere — arrows, box drawing and geometric
// shapes are common in real subject lines, and the TUI draws its own frame with
// them.
func TestSanitizeKeepsPredictableSymbols(t *testing.T) {
	t.Parallel()

	for _, keep := range []string{"\u2192", "\u2500", "\u258c", "\u25cf", "\u2264", "\u2116", "\u20bd"} {
		assert.Equal(t, keep, sanitizeCellText(keep), "%q renders one predictable cell and must survive", keep)
	}
	for _, drop := range []string{"\u2764", "\U0001F3D6", "\ufe0f", "\u200d"} {
		assert.Empty(t, sanitizeCellText(drop), "%q has an unpredictable width and must be dropped", drop)
	}
	// Wide-measured emoji stay: measured width and rendered width agree.
	assert.Equal(t, "\U0001F381", sanitizeCellText("\U0001F381"))
}

func TestMessageStateBadges(t *testing.T) {
	t.Parallel()
	assert.Empty(t, messageStateBadges(&models.Message{}))
	assert.Equal(t, "Draft", messageStateBadges(&models.Message{IsDraft: true}))
	assert.Equal(t, "Spam", messageStateBadges(&models.Message{IsSpam: true}))
	assert.Equal(t, "Archive", messageStateBadges(&models.Message{Labels: []string{"archive"}}))
	assert.Equal(t, "Draft  Spam  Archive",
		messageStateBadges(&models.Message{IsDraft: true, IsSpam: true, Labels: []string{"archive"}}))
	assert.Empty(t, messageStateBadges(nil))
}
