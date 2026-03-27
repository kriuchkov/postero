package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMessageIDsSplitsWhitespace(t *testing.T) {
	ids, err := parseMessageIDs(strings.NewReader("msg-1\nmsg-2 msg-3\n"))

	require.NoError(t, err)
	assert.Equal(t, []string{"msg-1", "msg-2", "msg-3"}, ids)
}

func TestNormalizeMessageIDsDeduplicatesAndTrims(t *testing.T) {
	ids := normalizeMessageIDs([]string{" msg-1 ", "", "msg-2", "msg-1"})

	assert.Equal(t, []string{"msg-1", "msg-2"}, ids)
}

func TestCollectMessageIDsRequiresInput(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	addStdinIDsFlag(cmd)

	_, err := collectMessageIDs(cmd, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "provide at least one message ID")
}

func TestCollectMessageIDsReadsFromStdinWhenRequested(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	addStdinIDsFlag(cmd)
	require.NoError(t, cmd.Flags().Set("stdin-ids", "true"))

	previousIn := rootCmd.InOrStdin()
	rootCmd.SetIn(strings.NewReader("msg-2\nmsg-3\n"))
	t.Cleanup(func() {
		rootCmd.SetIn(previousIn)
	})

	ids, err := collectMessageIDs(cmd, []string{"msg-1", "msg-2"})

	require.NoError(t, err)
	assert.Equal(t, []string{"msg-1", "msg-2", "msg-3"}, ids)
}

func TestNewUpdateMessageCommandAddsStdinFlag(t *testing.T) {
	cmd := newUpdateMessageCommand("fake [id...]", "fake", "done", nil)

	flag := cmd.Flags().Lookup("stdin-ids")
	require.NotNil(t, flag)
	assert.Equal(t, "false", flag.DefValue)
}
