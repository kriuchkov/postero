package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kriuchkov/postero/internal/core/models"
)

// TestMessageBodyIsCachedBetweenFrames guards the performance regression where
// View() re-parsed the selected message (HTML→markdown, and possibly an external
// filter subprocess) on every keystroke and spinner tick. The body must be
// computed once per selection in syncContentViewport and served from cache after.
func TestMessageBodyIsCachedBetweenFrames(t *testing.T) {
	m := testModel()
	m.width, m.height = 120, 40
	m.state = stateContent
	m.messages = []*models.Message{{
		ID:      "html-1",
		From:    "Sender <s@example.com>",
		Subject: "HTML mail",
		HTML:    "<h1>Original</h1><p>hello</p>",
	}}
	m.listCursor = 0

	m.syncContentViewport(true)
	first := m.currentMessageBody()
	require.Contains(t, first, "Original", "the HTML body must be converted to markdown once")

	// Mutating the underlying message WITHOUT re-syncing must not trigger a
	// re-parse: the cached body is served as long as it belongs to this message.
	m.messages[0].HTML = "<h1>Changed</h1>"
	assert.Equal(t, first, m.currentMessageBody(), "body must be served from cache, not re-parsed each call")

	// Selecting a different message invalidates the cache on the next sync.
	m.messages = append(m.messages, &models.Message{ID: "html-2", HTML: "<h1>Second</h1>"})
	m.listCursor = 1
	m.syncContentViewport(true)
	assert.Contains(t, m.currentMessageBody(), "Second",
		"switching messages must refresh the cached body")
}
