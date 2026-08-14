package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kriuchkov/postero/internal/core/models"
)

func newTestRepository(t *testing.T) *Repository {
	t.Helper()
	repo, err := NewRepository(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	return repo.(*Repository)
}

func TestSaveRoundTripsAttachments(t *testing.T) {
	t.Parallel()
	repo := newTestRepository(t)
	ctx := context.Background()

	message := &models.Message{
		ID:      "att-1",
		Subject: "With attachment",
		To:      []string{"rcpt@example.com"},
		Body:    "body",
		Attachments: []*models.Attachment{
			{Filename: "report.txt", MimeType: "text/plain", Size: 7, Data: []byte("payload")},
		},
	}
	require.NoError(t, repo.Save(ctx, message))

	loaded, err := repo.GetByID(ctx, "att-1")
	require.NoError(t, err)
	require.Len(t, loaded.Attachments, 1)
	assert.Equal(t, "report.txt", loaded.Attachments[0].Filename)
	assert.Equal(t, "text/plain", loaded.Attachments[0].MimeType)
	assert.Equal(t, []byte("payload"), loaded.Attachments[0].Data)
}

func TestSaveIsIdempotentByID(t *testing.T) {
	t.Parallel()
	repo := newTestRepository(t)
	ctx := context.Background()

	var seeded int
	require.NoError(t, repo.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM messages").Scan(&seeded))

	message := &models.Message{ID: "stable-1", Subject: "First", Body: "body"}
	require.NoError(t, repo.Save(ctx, message))
	message.Subject = "Updated"
	require.NoError(t, repo.Save(ctx, message))

	var count int
	require.NoError(t, repo.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM messages").Scan(&count))
	assert.Equal(t, seeded+1, count, "saving the same id twice must not create duplicates")

	loaded, err := repo.GetByID(ctx, "stable-1")
	require.NoError(t, err)
	assert.Equal(t, "Updated", loaded.Subject)
}

func TestSaveBatchPersistsAllInOneTransaction(t *testing.T) {
	t.Parallel()
	repo := newTestRepository(t)
	ctx := context.Background()

	batch := []*models.Message{
		{ID: "batch-1", Subject: "One", Body: "a"},
		{ID: "batch-2", Subject: "Two", Body: "b"},
		{ID: "batch-3", Subject: "Three", Body: "c"},
	}
	require.NoError(t, repo.SaveBatch(ctx, batch))

	for _, want := range batch {
		loaded, err := repo.GetByID(ctx, want.ID)
		require.NoError(t, err)
		assert.Equal(t, want.Subject, loaded.Subject)
	}

	// Idempotent: re-saving the same batch must not duplicate rows.
	require.NoError(t, repo.SaveBatch(ctx, batch))
	var count int
	require.NoError(t, repo.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM messages").Scan(&count))
	assert.Equal(t, len(batch), count)

	assert.NoError(t, repo.SaveBatch(ctx, nil), "empty batch is a no-op")
}

func TestListAndSearchSkipAttachmentBlobs(t *testing.T) {
	t.Parallel()
	repo := newTestRepository(t)
	ctx := context.Background()

	require.NoError(t, repo.Save(ctx, &models.Message{
		ID:      "att-2",
		Subject: "With attachment",
		Body:    "body",
		Labels:  []string{"inbox"},
		Attachments: []*models.Attachment{
			{Filename: "report.txt", MimeType: "text/plain", Data: []byte("payload")},
		},
	}))

	listed, err := repo.List(ctx, 10, 0)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Nil(t, listed[0].Attachments, "list rows must stay summaries")

	found, err := repo.Search(ctx, models.SearchCriteria{Query: "attachment", Limit: 10})
	require.NoError(t, err)
	require.Len(t, found, 1)
	assert.Nil(t, found[0].Attachments, "search rows must stay summaries")

	full, err := repo.GetByID(ctx, "att-2")
	require.NoError(t, err)
	require.Len(t, full.Attachments, 1)
	assert.Equal(t, []byte("payload"), full.Attachments[0].Data)
}

func TestSaveWithNilAttachmentsKeepsStoredBlobs(t *testing.T) {
	t.Parallel()
	repo := newTestRepository(t)
	ctx := context.Background()

	require.NoError(t, repo.Save(ctx, &models.Message{
		ID:      "att-3",
		Subject: "Original",
		Attachments: []*models.Attachment{
			{Filename: "keep.txt", MimeType: "text/plain", Data: []byte("keep me")},
		},
	}))

	// Simulate restoring a summary snapshot (loaded without attachments).
	require.NoError(t, repo.Save(ctx, &models.Message{ID: "att-3", Subject: "Restored"}))

	full, err := repo.GetByID(ctx, "att-3")
	require.NoError(t, err)
	assert.Equal(t, "Restored", full.Subject)
	require.Len(t, full.Attachments, 1, "nil attachments must not wipe stored blobs")
	assert.Equal(t, "keep.txt", full.Attachments[0].Filename)

	// An explicit empty slice clears them.
	require.NoError(t, repo.Save(ctx, &models.Message{ID: "att-3", Subject: "Cleared", Attachments: []*models.Attachment{}}))
	full, err = repo.GetByID(ctx, "att-3")
	require.NoError(t, err)
	assert.Empty(t, full.Attachments)
}
