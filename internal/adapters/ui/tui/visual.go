package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// handleVisualKey implements vim-style visual (line) selection over the
// message list. Movement keys are left to the regular handlers so counts,
// gg/G, and H/M/L keep working while the selection is extended.
func (m *Model) handleVisualKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	if !m.visualActive {
		if m.state == stateList && isSingleRune(msg, 'V') {
			if len(m.messages) == 0 {
				m.setStatus("No messages to select")
				return true, nil
			}
			m.visualActive = true
			m.visualAnchor = m.listCursor
			m.setStatus("-- VISUAL --")
			return true, nil
		}
		return false, nil
	}

	switch {
	case keyMatches(msg, m.keys.Esc) || isSingleRune(msg, 'V') || keyMatches(msg, m.keys.Quit):
		// q leaves visual mode instead of quitting the app, so this handler
		// must run before the main key switch.
		m.exitVisualMode()
		m.setStatus("Visual mode cancelled")
		return true, nil
	case keyMatches(msg, m.keys.Delete):
		action := repeatableActionTrash
		if m.isTrashSelection() {
			action = repeatableActionDelete
		}
		return true, m.applyVisualAction(action)
	case keyMatches(msg, m.keys.Archive):
		return true, m.applyVisualAction(repeatableActionArchive)
	case keyMatches(msg, m.keys.Spam):
		return true, m.applyVisualAction(repeatableActionSpam)
	case keyMatches(msg, m.keys.MarkRead):
		return true, m.markVisualRangeRead()
	case keyMatches(msg, m.keys.Up) || keyMatches(msg, m.keys.Down) ||
		keyMatches(msg, m.keys.PageUp) || keyMatches(msg, m.keys.PageDown) ||
		keyMatches(msg, m.keys.HalfUp) || keyMatches(msg, m.keys.HalfDown):
		// Cursor movement extends the selection via the main switch.
		return false, nil
	default:
		// Swallow everything else (enter/compose/reply/focus switches) so the
		// selection cannot be abandoned accidentally.
		m.setStatus("-- VISUAL -- d/a/! act • m read • esc cancel")
		return true, nil
	}
}

func (m *Model) exitVisualMode() {
	m.visualActive = false
	m.visualAnchor = 0
}

// visualRange returns the inclusive selection bounds clamped to the list.
func visualRange(m Model) (int, int) {
	lo, hi := m.visualAnchor, m.listCursor
	if lo > hi {
		lo, hi = hi, lo
	}
	if lo < 0 {
		lo = 0
	}
	if last := len(m.messages) - 1; hi > last {
		hi = last
	}
	return lo, hi
}

// applyVisualAction runs a repeatable action over the selected range,
// reusing the count-based batch machinery (and its batch undo).
func (m *Model) applyVisualAction(action repeatableAction) tea.Cmd {
	lo, hi := visualRange(*m)
	m.listCursor = lo
	m.exitVisualMode()
	m.clearPendingCount()
	return m.applyRepeatableAction(action, hi-lo+1)
}

func (m *Model) markVisualRangeRead() tea.Cmd {
	lo, hi := visualRange(*m)
	for index := lo; index <= hi && index < len(m.messages); index++ {
		m.markMessageReadAt(context.Background(), index)
	}
	m.exitVisualMode()
	m.clearPendingCount()
	m.setStatus(fmt.Sprintf("%d messages marked as read", hi-lo+1))
	m.prepareFreshMessageFetch()
	return m.fetchMessagesForID(m.currentMessageID())
}
