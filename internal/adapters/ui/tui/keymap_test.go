package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kriuchkov/postero/internal/config"
)

func TestParseBindingKeys(t *testing.T) {
	t.Parallel()

	fallback := []string{"q", "ctrl+c"}
	assert.Equal(t, fallback, parseBindingKeys("", fallback), "blank input keeps defaults")
	assert.Equal(t, fallback, parseBindingKeys("  ,  , ", fallback), "only separators keeps defaults")
	assert.Equal(t, []string{"x"}, parseBindingKeys("x", fallback))
	assert.Equal(t, []string{"x", "ctrl+q"}, parseBindingKeys(" x , ctrl+q ", fallback), "entries are trimmed")
}

func TestKeyMapFromConfigOverridesOnlyConfiguredActions(t *testing.T) {
	t.Parallel()

	bindings := keyMapFromConfig(config.KeybindingsConfig{
		Quit:   "x,ctrl+q",
		Search: "?",
	})

	assert.Equal(t, []string{"x", "ctrl+q"}, bindings.Quit.Keys())
	assert.Equal(t, []string{"?"}, bindings.Search.Keys())
	// Unconfigured actions keep their defaults.
	assert.Equal(t, []string{"c"}, bindings.Compose.Keys())
	assert.Equal(t, []string{"ctrl+h"}, bindings.CycleFocus.Keys())
}

func TestKeyMapFromConfigEmptyKeepsDefaults(t *testing.T) {
	t.Parallel()

	defaults := defaultKeyMap()
	bindings := keyMapFromConfig(config.KeybindingsConfig{})

	assert.Equal(t, defaults.Quit.Keys(), bindings.Quit.Keys())
	assert.Equal(t, defaults.Delete.Keys(), bindings.Delete.Keys())
	assert.Equal(t, defaults.MarkRead.Keys(), bindings.MarkRead.Keys())
}

func TestFullHelpIncludesAllGroups(t *testing.T) {
	t.Parallel()

	groups := defaultKeyMap().FullHelp()
	assert.NotEmpty(t, groups)
	total := 0
	for _, group := range groups {
		total += len(group)
	}
	assert.GreaterOrEqual(t, total, 20, "full help should surface the bulk of the bindings")
}
