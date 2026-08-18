package app

import (
	"context"
	"slices"
	"testing"

	"github.com/go-faster/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kriuchkov/postero/internal/core/models"
)

type demoStoreStub struct {
	saved []*models.Message
}

func (s *demoStoreStub) GetByID(context.Context, string) (*models.Message, error) {
	return nil, errors.New("not implemented")
}
func (s *demoStoreStub) List(context.Context, int, int) ([]*models.Message, error) {
	return nil, nil
}
func (s *demoStoreStub) Count(context.Context, models.SearchCriteria) (int, error) {
	return 0, nil
}
func (s *demoStoreStub) Search(context.Context, models.SearchCriteria) ([]*models.Message, error) {
	return nil, nil
}
func (s *demoStoreStub) Save(_ context.Context, message *models.Message) error {
	s.saved = append(s.saved, message)
	return nil
}
func (s *demoStoreStub) Delete(context.Context, string) error     { return nil }
func (s *demoStoreStub) MarkAsRead(context.Context, string) error { return nil }
func (s *demoStoreStub) MarkAsSpam(context.Context, string) error { return nil }

func TestSeedDemoMailboxPopulatesStore(t *testing.T) {
	t.Parallel()
	store := &demoStoreStub{}

	messages, err := SeedDemoMailbox(context.Background(), store)
	require.NoError(t, err)
	require.NotEmpty(t, messages)
	assert.Len(t, store.saved, len(messages))

	var inbox, drafts int
	for _, msg := range store.saved {
		assert.Equal(t, models.DemoAccountName, msg.AccountID, "demo messages must be scoped to the demo account")
		switch {
		case slices.Contains(msg.Labels, "draft"):
			drafts++
		case slices.Contains(msg.Labels, "inbox"):
			inbox++
		}
	}
	assert.Positive(t, inbox, "demo mailbox should include inbox messages")
	assert.Positive(t, drafts, "demo mailbox should include a draft")
}

func TestSeedDemoMailboxRequiresStore(t *testing.T) {
	t.Parallel()
	_, err := SeedDemoMailbox(context.Background(), nil)
	require.Error(t, err)
}

// memDemoStore is a tiny in-memory store supporting Search-by-account and Delete.
type memDemoStore struct{ msgs map[string]*models.Message }

func newMemDemoStore() *memDemoStore { return &memDemoStore{msgs: map[string]*models.Message{}} }

func (s *memDemoStore) GetByID(_ context.Context, id string) (*models.Message, error) {
	if m, ok := s.msgs[id]; ok {
		return m, nil
	}
	return nil, errors.New("not found")
}
func (s *memDemoStore) List(context.Context, int, int) ([]*models.Message, error) { return nil, nil }
func (s *memDemoStore) Count(ctx context.Context, c models.SearchCriteria) (int, error) {
	out, err := s.Search(ctx, c)
	return len(out), err
}

func (s *memDemoStore) Search(_ context.Context, c models.SearchCriteria) ([]*models.Message, error) {
	out := make([]*models.Message, 0)
	for _, m := range s.msgs {
		if c.AccountID == "" || m.AccountID == c.AccountID {
			out = append(out, m)
		}
	}
	return out, nil
}
func (s *memDemoStore) Save(_ context.Context, m *models.Message) error { s.msgs[m.ID] = m; return nil }
func (s *memDemoStore) Delete(_ context.Context, id string) error       { delete(s.msgs, id); return nil }
func (s *memDemoStore) MarkAsRead(context.Context, string) error        { return nil }
func (s *memDemoStore) MarkAsSpam(context.Context, string) error        { return nil }

func TestPurgeDemoMailboxRemovesOnlyDemoMessages(t *testing.T) {
	t.Parallel()
	store := newMemDemoStore()
	ctx := context.Background()

	_, err := SeedDemoMailbox(ctx, store)
	require.NoError(t, err)
	require.NoError(t, store.Save(ctx, &models.Message{ID: "real-1", AccountID: "yandex", Labels: []string{"inbox"}}))

	removed, err := PurgeDemoMailbox(ctx, store)
	require.NoError(t, err)
	assert.Positive(t, removed, "demo messages must be removed")

	demo, _ := store.Search(ctx, models.SearchCriteria{AccountID: models.DemoAccountName})
	assert.Empty(t, demo, "no demo messages may remain")
	kept, _ := store.Search(ctx, models.SearchCriteria{AccountID: "yandex"})
	assert.Len(t, kept, 1, "real mail must be kept")
}
