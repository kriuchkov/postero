package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultKeyMapDropsRedundantAliases(t *testing.T) {
	keys := defaultKeyMap()

	assert.Equal(t, []string{"pgup"}, keys.PageUp.Keys())
	assert.Equal(t, []string{"pgdown"}, keys.PageDown.Keys())
	assert.Equal(t, []string{"d", "delete"}, keys.Delete.Keys())
	assert.NotContains(t, keys.Delete.Keys(), "backspace")
	assert.NotContains(t, keys.PageUp.Keys(), "ctrl+b")
	assert.NotContains(t, keys.PageDown.Keys(), "ctrl+f")
}

func TestShortHelpShowsCoreActions(t *testing.T) {
	keys := defaultKeyMap()
	help := keys.ShortHelp()

	assert.Contains(t, help, keys.Enter)
	assert.Contains(t, help, keys.Search)
	assert.Contains(t, help, keys.Quit)
	assert.NotContains(t, help, keys.MarkRead)
}
