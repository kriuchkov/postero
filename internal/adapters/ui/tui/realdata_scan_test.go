package tui

import (
	"context"
	"os"
	"slices"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/rivo/uniseg"

	"github.com/kriuchkov/postero/internal/adapters/storage/sqlite"
)

// pessimisticWidth counts every pictographic-ish rune as at least 2 cells —
// the widest a terminal might render it — to flag lines that could exceed the
// terminal width even when the measured width says they fit.
func pessimisticWidth(s string) int {
	w := 0
	g := uniseg.NewGraphemes(s)
	for g.Next() {
		cw := g.Width()
		if slices.ContainsFunc(g.Runes(), isPictographRune) {
			if cw < 2 {
				cw = 2
			}
		}
		w += cw
	}
	return w
}

// TestRealMailboxWidthHazards is an opt-in self-check: point PSTR_DB at a copy
// of a real postero.db and it renders the full mailbox (list and reader, every
// cursor position, several terminal sizes), flagging any line that could render
// wider than the terminal — the root cause of frame-shift corruption where the
// sidebar and list "double up" during fast scrolling.
func TestRealMailboxWidthHazards(t *testing.T) {
	if os.Getenv("PSTR_DB") == "" {
		t.Skip("set PSTR_DB to a copy of a postero.db to run this scan")
	}
	repo, err := sqlite.NewRepository(os.Getenv("PSTR_DB"))
	if err != nil {
		t.Fatal(err)
	}
	msgs, err := repo.List(context.Background(), 200, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("loaded %d messages", len(msgs))

	for _, size := range []struct{ w, h int }{{240, 60}, {200, 50}, {120, 40}, {100, 35}, {90, 30}, {80, 25}} {
		m := testModel()
		m.width, m.height = size.w, size.h
		m.messages = msgs
		m.allMessages = msgs
		m.sidebarTagSource = msgs
		m.accountNames = []string{"personal", "work"}
		m.sidebarItems = []string{"Inbox", "Sent", "Drafts", "Archive", "Trash", "Spam", "", "Accounts:", "  personal", "  work"}
		for cursor := range msgs {
			m.listCursor = cursor
			for _, state := range []SessionState{stateList, stateContent} {
				m.state = state
				m.syncContentViewport(true)
				view := m.View()
				if h := lipgloss.Height(view); h > size.h {
					t.Errorf("size %dx%d cursor %d state %d: height %d > %d", size.w, size.h, cursor, state, h, size.h)
				}
				for i, line := range splitViewLines(view) {
					// Control characters other than SGR escapes corrupt the
					// terminal regardless of width: \t expands to a tab stop,
					// \r rewinds the cursor to column 0.
					for _, r := range ansi.Strip(line) {
						if r == '\t' || r == '\r' || (r < 0x20 && r != '\n') || r == 0x7F {
							t.Errorf("size %dx%d cursor %d state %d line %d: control rune U+%04X in output\n  %q",
								size.w, size.h, cursor, state, i, r, ansi.Strip(line))
							return
						}
					}
					lw, pw := lipgloss.Width(line), pessimisticWidth(ansi.Strip(line))
					if lw > size.w || pw > size.w {
						t.Errorf("size %dx%d cursor %d state %d line %d: measured=%d pessimistic=%d max=%d\n  %q",
							size.w, size.h, cursor, state, i, lw, pw, size.w, ansi.Strip(line))
						for _, r := range ansi.Strip(line) {
							if r > 0x2000 {
								t.Logf("    U+%04X %c", r, r)
							}
						}
						return
					}
				}
			}
		}
	}
}

func splitViewLines(s string) []string {
	lines := []string{}
	start := 0
	for i := range len(s) {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	return append(lines, s[start:])
}
