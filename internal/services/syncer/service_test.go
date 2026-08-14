package syncer

import (
	"context"
	"testing"

	"github.com/go-faster/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kriuchkov/postero/internal/core/models"
	"github.com/kriuchkov/postero/internal/core/ports"
)

type imapStub struct {
	messages     []*models.Message
	connectErr   error
	connected    bool
	disconnected bool
	mailbox      string
	limit        int
}

func (s *imapStub) Connect(_ context.Context, _ string, _ int, _, _ string, _ string, _ bool) error {
	if s.connectErr != nil {
		return s.connectErr
	}
	s.connected = true
	return nil
}

func (s *imapStub) Disconnect(_ context.Context) error {
	s.disconnected = true
	return nil
}

func (s *imapStub) Fetch(_ context.Context, mailbox string, limit int) ([]*models.Message, error) {
	s.mailbox = mailbox
	s.limit = limit
	return s.messages, nil
}

func (s *imapStub) IsConnected() bool { return s.connected }

type storeStub struct {
	saved   []*models.Message
	saveErr error
}

func (s *storeStub) GetByID(context.Context, string) (*models.Message, error) {
	return nil, errors.New("not implemented")
}
func (s *storeStub) List(context.Context, int, int) ([]*models.Message, error) {
	return nil, nil
}
func (s *storeStub) Search(context.Context, models.SearchCriteria) ([]*models.Message, error) {
	return nil, nil
}
func (s *storeStub) Save(_ context.Context, message *models.Message) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.saved = append(s.saved, message)
	return nil
}
func (s *storeStub) Delete(context.Context, string) error     { return nil }
func (s *storeStub) MarkAsRead(context.Context, string) error { return nil }
func (s *storeStub) MarkAsSpam(context.Context, string) error { return nil }

func newTestService(store *storeStub, imap *imapStub) *Service {
	return NewService(store, func() ports.IMAPRepository { return imap })
}

func TestSyncAccountsAdoptsMessages(t *testing.T) {
	t.Parallel()

	imap := &imapStub{messages: []*models.Message{
		{ID: "imap-100-7", ThreadID: "imap-100-7"},
		{ID: "<rfc@example.com>", ThreadID: "<rfc@example.com>", IsDraft: true},
	}}
	store := &storeStub{}
	service := newTestService(store, imap)

	synced, err := service.SyncAccounts(context.Background(), []models.SyncTarget{{AccountName: "work"}})
	require.NoError(t, err)
	require.Len(t, synced, 2)

	// Every ID is scoped by account: UID-derived IDs keep the imap- prefix in
	// front, RFC Message-IDs are prefixed with the account name.
	assert.Equal(t, "imap-work-100-7", synced[0].ID)
	assert.Equal(t, "imap-work-100-7", synced[0].ThreadID)
	assert.Equal(t, []string{"inbox"}, synced[0].Labels)
	assert.Equal(t, "work-<rfc@example.com>", synced[1].ID)
	assert.Equal(t, "work-<rfc@example.com>", synced[1].ThreadID)
	assert.Equal(t, []string{"draft"}, synced[1].Labels)
	for _, msg := range synced {
		assert.Equal(t, "work", msg.AccountID)
	}

	assert.Len(t, store.saved, 2)
	assert.Equal(t, "INBOX", imap.mailbox)
	assert.Equal(t, defaultFetchLimit, imap.limit)
	assert.True(t, imap.disconnected)
}

func TestSyncAccountsScopesSharedMessageIDPerAccount(t *testing.T) {
	t.Parallel()

	// The same newsletter (identical RFC Message-ID) delivered to two accounts
	// must not collide into one row.
	shared := "<news@example.com>"
	workID := scopeMessageID(shared, "work")
	homeID := scopeMessageID(shared, "home")
	assert.NotEqual(t, workID, homeID)
	assert.Equal(t, "work-<news@example.com>", workID)
	assert.Equal(t, "home-<news@example.com>", homeID)
}

type batchStore struct {
	storeStub

	batches [][]*models.Message
}

func (s *batchStore) SaveBatch(_ context.Context, messages []*models.Message) error {
	s.batches = append(s.batches, messages)
	s.saved = append(s.saved, messages...)
	return nil
}

func TestSyncAccountsUsesBatchSaverWhenAvailable(t *testing.T) {
	t.Parallel()

	imap := &imapStub{messages: []*models.Message{
		{ID: "imap-1-1"},
		{ID: "imap-1-2"},
	}}
	store := &batchStore{}
	service := NewService(store, func() ports.IMAPRepository { return imap })

	_, err := service.SyncAccounts(context.Background(), []models.SyncTarget{{AccountName: "work"}})
	require.NoError(t, err)
	require.Len(t, store.batches, 1, "a batch-capable store must persist in one call")
	assert.Len(t, store.batches[0], 2)
}

func TestSyncAccountsHonorsTargetOverrides(t *testing.T) {
	t.Parallel()

	imap := &imapStub{}
	service := newTestService(&storeStub{}, imap)

	_, err := service.SyncAccounts(context.Background(), []models.SyncTarget{
		{AccountName: "work", Mailbox: "Archive", Limit: 10},
	})
	require.NoError(t, err)
	assert.Equal(t, "Archive", imap.mailbox)
	assert.Equal(t, 10, imap.limit)
}

func TestSyncAccountsPropagatesConnectError(t *testing.T) {
	t.Parallel()

	imap := &imapStub{connectErr: errors.New("dial failed")}
	service := newTestService(&storeStub{}, imap)

	_, err := service.SyncAccounts(context.Background(), []models.SyncTarget{{AccountName: "work"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connect imap for work")
}
