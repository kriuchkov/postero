package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kriuchkov/postero/internal/core/models"
)

// adversarialMessages returns the message shapes that have historically broken
// the TUI: an HTML-only mail whose markup lives in the plain Body field, a huge
// HTML document, Cyrillic sender/subject, an unbroken over-long subject, a mail
// with attachments, and unread/draft/spam states. The self-check renders every
// one of these through the real View() and asserts invariants, so regressions
// are caught here instead of by eye.
func adversarialMessages() []*models.Message {
	base := time.Date(2026, 8, 3, 14, 23, 0, 0, time.UTC)
	return []*models.Message{
		{
			ID: "htmlplain-1", AccountID: "personal", ThreadID: "t1",
			Subject: "Электронный счёт за июль",
			From:    "Энергосбыт <billing@example.ru>",
			To:      []string{"me@example.com"},
			// Server delivered HTML inside the plain Body field.
			Body:   `<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 4.0 Transitional//EN"><html><head><style>td,p{color:red}</style></head><body><h1>Счёт за июль</h1><p>Сумма 1234 руб</p></body></html>`,
			Labels: []string{"inbox"}, Date: base, IsRead: false,
		},
		{
			ID: "bightml-1", AccountID: "personal", ThreadID: "t2",
			Subject: "Big HTML newsletter",
			From:    "News <news@example.com>",
			Body:    bigHTMLBody(),
			Labels:  []string{"inbox"}, Date: base, IsRead: true,
		},
		{
			ID: "cyr-1", AccountID: "personal", ThreadID: "t3",
			Subject: "Вход в аккаунт — подтверждение",
			From:    "Аккаунт <security@example.com>",
			Body:    "Здравствуйте! Кто-то вошёл в ваш аккаунт.",
			Labels:  []string{"inbox", "важное"}, Date: base, IsRead: false,
		},
		{
			ID: "longsubj-1", AccountID: "personal", ThreadID: "t4",
			Subject: strings.Repeat("VeryLongSubjectWithoutAnySpaces", 8),
			From:    "verylongsenderaddresswithoutspaces@some-really-long-domain.example.com",
			Body:    strings.Repeat("word ", 400),
			Labels:  []string{"inbox"}, Date: base, IsRead: true,
		},
		{
			ID: "att-1", AccountID: "personal", ThreadID: "t5",
			Subject: "Invoice attached", From: "billing@example.com",
			Body:        "Please find the invoice attached.",
			Attachments: []*models.Attachment{{Filename: "invoice.pdf", MimeType: "application/pdf", Size: 20480}},
			Labels:      []string{"inbox"}, Date: base, IsRead: true,
		},
		{
			ID: "draft-1", AccountID: "personal", ThreadID: "t6",
			Subject: "Draft in progress", From: "me@example.com",
			Body: "unfinished", Labels: []string{"drafts"}, Date: base, IsDraft: true,
		},
		{
			ID: "spam-1", AccountID: "personal", ThreadID: "t7",
			Subject: "You won!", From: "spam@example.com",
			Body: "<div>Click <a href='http://x'>here</a></div>", Labels: []string{"spam"}, Date: base, IsSpam: true,
		},
		{
			ID: "empty-1", AccountID: "personal", ThreadID: "t8",
			Subject: "", From: "", Body: "", Labels: []string{"inbox"}, Date: base, IsRead: true,
		},
	}
}

// rawHTMLMarkers are substrings that must never reach the screen — their presence
// means an HTML part was shown verbatim instead of being converted/stripped.
var rawHTMLMarkers = []string{
	"<!doctype", "<html", "<head", "<style", "<script",
	"<div", "<table", "<td", "<p>", "<br", "</", "color:red", "text/css",
}

func assertNoRawHTML(t *testing.T, where, rendered string) {
	t.Helper()
	clean := strings.ToLower(ansi.Strip(rendered))
	for _, marker := range rawHTMLMarkers {
		assert.NotContainsf(t, clean, marker, "%s leaked raw HTML/CSS marker %q", where, marker)
	}
}

// assertNoHorizontalOverflow guards the "поехала вёрстка" class of bug: no
// visible line may be wider than the terminal.
func assertNoHorizontalOverflow(t *testing.T, where string, rendered string, width int) {
	t.Helper()
	for i, line := range strings.Split(rendered, "\n") {
		if w := lipgloss.Width(line); w > width {
			t.Errorf("%s line %d overflows width: %d > %d\n  line=%q", where, i, w, width, ansi.Strip(line))
		}
	}
}

// TestTUISelfCheckRendersAllStatesWithoutLeaksOrOverflow drives every adversarial
// message through the list and reading views at several terminal sizes and
// asserts the invariants. This is the automated replacement for eyeballing.
func TestTUISelfCheckRendersAllStatesWithoutLeaksOrOverflow(t *testing.T) {
	msgs := adversarialMessages()
	sizes := []struct{ w, h int }{{120, 40}, {96, 30}, {72, 24}}

	for _, size := range sizes {
		for i := range msgs {
			m := testModel()
			m.width, m.height = size.w, size.h
			m.messages = append([]*models.Message{}, msgs...)
			m.allMessages = append([]*models.Message{}, msgs...)
			m.listCursor = i

			// List view (browsing).
			m.state = stateList
			m.syncContentViewport(true)
			listView := m.View()
			where := fmt.Sprintf("list[%dx%d,cursor=%s]", size.w, size.h, msgs[i].ID)
			assertNoRawHTML(t, where, listView)
			assertNoHorizontalOverflow(t, where, listView, size.w)
			assert.LessOrEqualf(t, lipgloss.Height(listView), size.h, "%s exceeds terminal height", where)

			// Reading view (content pane focused, body scrolled a bit).
			m.state = stateContent
			m.syncContentViewport(true)
			m.contentViewport.SetYOffset(3)
			readView := m.View()
			where = fmt.Sprintf("read[%dx%d,msg=%s]", size.w, size.h, msgs[i].ID)
			assertNoRawHTML(t, where, readView)
			assertNoHorizontalOverflow(t, where, readView, size.w)
			assert.LessOrEqualf(t, lipgloss.Height(readView), size.h, "%s exceeds terminal height", where)
		}
	}
}

// TestTUISelfCheckComposeViewFitsTerminal renders the composer (new message and
// reply, in normal and insert mode) at several sizes and asserts it never
// overflows the terminal width or height.
func TestTUISelfCheckComposeViewFitsTerminal(t *testing.T) {
	sizes := []struct{ w, h int }{{120, 40}, {100, 30}, {110, 24}}
	longBody := strings.Repeat("This is a long line without hard breaks that must wrap cleanly. ", 20)

	for _, size := range sizes {
		for _, editing := range []bool{false, true} {
			m := testModel()
			m.width, m.height = size.w, size.h
			m.enterComposeState(&models.Message{
				AccountID: "personal", From: "me@example.com",
				Subject: "Re: " + strings.Repeat("VeryLongSubjectToken", 6),
				Body:    longBody,
			}, 3)
			m.composeEditing = editing
			m.applyComposeFocus()

			view := m.View()
			where := fmt.Sprintf("compose[%dx%d,editing=%v]", size.w, size.h, editing)
			assertNoHorizontalOverflow(t, where, view, size.w)
			assert.LessOrEqualf(t, lipgloss.Height(view), size.h, "%s exceeds terminal height", where)
		}
	}
}

// TestTUISelfCheckKeyDrivingDoesNotPanic replays real keystrokes over the
// adversarial mailbox to exercise the Update paths (navigation, open, back,
// jumps) and ensure none of these inputs panic or corrupt layout.
func TestTUISelfCheckKeyDrivingDoesNotPanic(t *testing.T) {
	m := testModel()
	m.width, m.height = 100, 30
	m.messages = adversarialMessages()
	m.allMessages = append([]*models.Message{}, m.messages...)
	m.state = stateList
	m.listCursor = 0

	keys := []tea.KeyMsg{
		keyRune('j'), keyRune('j'), keyRune('k'),
		keyRune('G'), keyRune('g'), keyRune('g'),
		keyRune('l'),               // open reading view
		keyRune('j'), keyRune('k'), // scroll body
		{Type: tea.KeyEsc}, keyRune('h'), // back to list
		keyRune('3'), keyRune('j'), // counted motion
	}
	for idx, key := range keys {
		m = updateModel(t, m, key)
		view := m.View()
		assertNoRawHTML(t, fmt.Sprintf("keydrive step %d", idx), view)
		assertNoHorizontalOverflow(t, fmt.Sprintf("keydrive step %d", idx), view, m.width)
		require.LessOrEqual(t, lipgloss.Height(view), m.height)
	}
}

// TestTUISelfCheckRenderStaysFast guards against per-frame O(n²)/re-parse
// regressions: rendering the full View() many times over a mailbox that
// contains a ~120 KB HTML message must stay well under a second in aggregate.
func TestTUISelfCheckRenderStaysFast(t *testing.T) {
	m := testModel()
	m.width, m.height = 120, 40
	m.messages = adversarialMessages()
	m.allMessages = append([]*models.Message{}, m.messages...)
	m.state = stateList
	m.listCursor = 1 // the big HTML message
	m.syncContentViewport(true)

	start := time.Now()
	const frames = 60 // ~ a second of interaction at 60fps
	for range frames {
		_ = m.View()
	}
	elapsed := time.Since(start)

	assert.Lessf(t, elapsed, 750*time.Millisecond,
		"%d frames took %s — rendering is too slow (per-frame heavy work?)", frames, elapsed)
}
