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

// TestCardRowsShareLeftGutterInAllStates guards the "лишние отступы" regression:
// in every cursor state and read/unread combination, the sender, subject and
// preview text must begin at the same left column — one shared gutter, not a
// sender row indented past the others.
func TestCardRowsShareLeftGutterInAllStates(t *testing.T) {
	m := testModel()

	modes := map[string]listCursorMode{
		"active":  listCursorActive,
		"visual":  listCursorVisual,
		"none":    listCursorNone,
		"passive": listCursorPassive,
	}

	for name, mode := range modes {
		for _, read := range []bool{true, false} {
			msg := &models.Message{
				ID:      "align-1",
				From:    "SENDERTOKEN <s@example.com>",
				Subject: "SUBJECTTOKEN",
				Body:    "PREVIEWTOKEN body text",
				Labels:  []string{"inbox"}, // no custom tag → subject has no [tag] prefix
				Date:    time.Date(2026, 7, 31, 9, 0, 0, 0, time.UTC),
				IsRead:  read,
			}

			card, _ := renderListCard(m, msg, 44, mode)
			lines := strings.Split(ansi.Strip(card), "\n")

			// col returns the display column (not byte offset) where token starts,
			// so a multi-byte unread dot "●" counts as one column like any glyph.
			col := func(token string) int {
				for _, line := range lines {
					if before, _, found := strings.Cut(line, token); found {
						return lipgloss.Width(before)
					}
				}
				return -1
			}

			senderCol := col("SENDERTOKEN")
			subjectCol := col("SUBJECTTOKEN")
			previewCol := col("PREVIEWTOKEN")

			where := name + "/read=" + boolLabel(read)
			require.NotEqualf(t, -1, senderCol, "%s: sender not rendered", where)
			require.NotEqualf(t, -1, subjectCol, "%s: subject not rendered", where)
			require.NotEqualf(t, -1, previewCol, "%s: preview not rendered", where)

			assert.Equalf(t, senderCol, subjectCol, "%s: subject must align with sender (no extra indent)", where)
			assert.Equalf(t, senderCol, previewCol, "%s: preview must align with sender (no extra indent)", where)
		}
	}
}

func boolLabel(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
