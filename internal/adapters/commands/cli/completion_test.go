package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterCompletionsWiresCommands(t *testing.T) {
	registerCompletions()

	for _, cmd := range []struct {
		name string
		fn   any
	}{
		{"show", showCmd.ValidArgsFunction},
		{"read", readCmd.ValidArgsFunction},
		{"trash", trashCmd.ValidArgsFunction},
		{"delete", deleteCmd.ValidArgsFunction},
		{"reply", replyCmd.ValidArgsFunction},
		{"forward", forwardCmd.ValidArgsFunction},
	} {
		assert.NotNil(t, cmd.fn, "%s must complete message IDs", cmd.name)
	}

	fn, ok := listCmd.GetFlagCompletionFunc("account")
	require.True(t, ok, "list --account must have a completion function")
	require.NotNil(t, fn)
	fn, ok = listCmd.GetFlagCompletionFunc("mailbox")
	require.True(t, ok, "list --mailbox must have a completion function")
	require.NotNil(t, fn)
}

func TestCompletionTextCollapsesWhitespace(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "a b c", completionText("a\tb\nc"))
	assert.Equal(t, "Subject — sender", completionText("  Subject  —  sender "))
}
