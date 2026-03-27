package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCommandRegistersExpectedSubcommands(t *testing.T) {
	expected := []string{
		"archive",
		"auth",
		"compose",
		"config",
		"delete",
		"forward",
		"list",
		"read",
		"reply",
		"search",
		"show",
		"spam",
		"star",
		"sync",
		"trash",
	}

	for _, name := range expected {
		cmd, _, err := rootCmd.Find([]string{name})
		require.NoError(t, err)
		if assert.NotNil(t, cmd) {
			assert.Equal(t, name, cmd.Name())
		}
	}
}

func TestComposeAndReplyRegisterAISubcommands(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"compose", "ai"})
	require.NoError(t, err)
	if assert.NotNil(t, cmd) {
		assert.Equal(t, "ai", cmd.Name())
	}

	cmd, _, err = rootCmd.Find([]string{"reply", "ai"})
	require.NoError(t, err)
	if assert.NotNil(t, cmd) {
		assert.Equal(t, "ai", cmd.Name())
	}
}
