package file

import (
	"context"
	"testing"
	"time"

	coreerrors "github.com/kriuchkov/postero/internal/core/errors"
	"github.com/kriuchkov/postero/internal/core/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRepository(t *testing.T) {
	repo, err := NewRepository(t.TempDir())
	require.NoError(t, err)
	assert.NotNil(t, repo)
}

func TestRepositorySaveGetListAndSearch(t *testing.T) {
	repoAny, err := NewRepository(t.TempDir())
	require.NoError(t, err)
	repo := repoAny.(*Repository)
	ctx := context.Background()

	older := &models.Message{
		ID:        "msg-old",
		AccountID: "gmail",
		Subject:   "Older message",
		From:      "old@example.com",
		To:        []string{"me@example.com"},
		Body:      "old body",
		Labels:    []string{"inbox"},
		Date:      time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC),
	}
	newer := &models.Message{
		ID:        "msg-new",
		AccountID: "gmail",
		Subject:   "Sprint update",
		From:      "lead@example.com",
		To:        []string{"me@example.com"},
		Body:      "project status",
		Labels:    []string{"inbox", "work"},
		Date:      time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC),
	}

	require.NoError(t, repo.Save(ctx, older))
	require.NoError(t, repo.Save(ctx, newer))

	loaded, err := repo.GetByID(ctx, newer.ID)
	require.NoError(t, err)
	assert.Equal(t, newer.Subject, loaded.Subject)

	listed, err := repo.List(ctx, 10, 0)
	require.NoError(t, err)
	require.Len(t, listed, 2)
	assert.Equal(t, newer.ID, listed[0].ID)
	assert.Equal(t, older.ID, listed[1].ID)

	results, err := repo.Search(ctx, models.SearchCriteria{Subject: "sprint", AccountID: "gmail", Labels: []string{"work"}})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, newer.ID, results[0].ID)
}

func TestRepositoryMutationsAndDelete(t *testing.T) {
	repoAny, err := NewRepository(t.TempDir())
	require.NoError(t, err)
	repo := repoAny.(*Repository)
	ctx := context.Background()

	message := &models.Message{
		ID:        "msg-1",
		AccountID: "personal",
		Subject:   "Hello",
		From:      "sender@example.com",
		To:        []string{"me@example.com"},
		Body:      "body",
		Date:      time.Now(),
	}
	require.NoError(t, repo.Save(ctx, message))

	require.NoError(t, repo.MarkAsRead(ctx, message.ID))
	require.NoError(t, repo.MarkAsSpam(ctx, message.ID))

	loaded, err := repo.GetByID(ctx, message.ID)
	require.NoError(t, err)
	assert.True(t, loaded.IsRead)
	assert.True(t, loaded.IsSpam)
	assert.True(t, loaded.Flags.Seen)
	assert.True(t, loaded.Flags.Junk)

	require.NoError(t, repo.Delete(ctx, message.ID))
	_, err = repo.GetByID(ctx, message.ID)
	require.ErrorIs(t, err, coreerrors.ErrMessageNotFound)
}
