package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadComposeAttachmentsReadsFileMetadata(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "notes.txt")
	require.NoError(t, os.WriteFile(path, []byte("hello"), 0o600))

	attachments, err := loadComposeAttachments([]string{path})

	require.NoError(t, err)
	require.Len(t, attachments, 1)
	assert.Equal(t, "notes.txt", attachments[0].Filename)
	assert.Equal(t, int64(5), attachments[0].Size)
	assert.Equal(t, "text/plain; charset=utf-8", attachments[0].MimeType)
	assert.Equal(t, []byte("hello"), attachments[0].Data)
}

func TestLoadComposeAttachmentsReturnsErrorForMissingFile(t *testing.T) {
	_, err := loadComposeAttachments([]string{"/definitely/missing/file.txt"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "read attachment")
}
