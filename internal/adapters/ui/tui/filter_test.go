package tui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kriuchkov/postero/internal/config"
	"github.com/kriuchkov/postero/internal/core/models"
)

func TestApplyFilterCmdPipesThroughCommand(t *testing.T) {
	t.Parallel()

	out, err := applyFilterCmd("tr a-z A-Z", "hello filter")
	require.NoError(t, err)
	assert.Equal(t, "HELLO FILTER", out)
}

func TestApplyFilterCmdEmptyCommandReturnsInput(t *testing.T) {
	t.Parallel()

	out, err := applyFilterCmd("   ", "unchanged")
	require.NoError(t, err)
	assert.Equal(t, "unchanged", out)
}

func TestApplyFilterCmdMissingBinaryFails(t *testing.T) {
	t.Parallel()

	_, err := applyFilterCmd("definitely-not-a-real-binary-42", "input")
	require.Error(t, err)
}

func TestGetFilteredBodyUsesConfiguredFilters(t *testing.T) {
	t.Parallel()

	m := Model{config: &config.Config{Filters: map[string]string{
		"text/html":  "tr a-z A-Z",
		"text/plain": "tr a-z A-Z",
	}}}

	// HTML filter wins when HTML content exists.
	msg := &models.Message{Body: "plain body", HTML: "<p>html</p>"}
	assert.Equal(t, "<P>HTML</P>", strings.TrimSpace(m.getFilteredBody(msg)))

	// Plain filter applies when there is no HTML.
	msg = &models.Message{Body: "plain body"}
	assert.Equal(t, "PLAIN BODY", strings.TrimSpace(m.getFilteredBody(msg)))
}

func TestGetFilteredBodyFallsBackWhenFilterFails(t *testing.T) {
	t.Parallel()

	m := Model{config: &config.Config{Filters: map[string]string{
		"text/plain": "definitely-not-a-real-binary-42",
	}}}

	msg := &models.Message{Body: "raw body"}
	assert.Equal(t, "raw body", m.getFilteredBody(msg))
}

func TestGetFilteredBodyDefaults(t *testing.T) {
	t.Parallel()

	var m Model // nil config
	assert.Equal(t, "body", m.getFilteredBody(&models.Message{Body: "body"}))
	// With no external filter, HTML is converted to readable Markdown (never raw tags).
	assert.Equal(t, "html", m.getFilteredBody(&models.Message{HTML: "<p>html</p>"}))
	assert.Equal(t, "No content.", m.getFilteredBody(&models.Message{}))
}

func TestGetFilteredBodyConvertsHTMLOnlyMessage(t *testing.T) {
	t.Parallel()

	var m Model
	out := m.getFilteredBody(&models.Message{HTML: "<h1>Title</h1><p>Hello <strong>world</strong></p>"})
	assert.Contains(t, out, "# Title")
	assert.Contains(t, out, "Hello **world**")
	assert.NotContains(t, out, "<")
}
