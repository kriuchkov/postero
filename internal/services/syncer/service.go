package syncer

import (
	"context"
	"strings"

	"github.com/go-faster/errors"

	"github.com/kriuchkov/postero/internal/core/models"
	"github.com/kriuchkov/postero/internal/core/ports"
)

const (
	defaultMailbox    = "INBOX"
	defaultFetchLimit = 50
)

// Service implements ports.AccountSyncer on top of an IMAP transport and the
// local message store.
type Service struct {
	store       ports.MessageRepository
	imapFactory func() ports.IMAPRepository
}

func NewService(store ports.MessageRepository, imapFactory func() ports.IMAPRepository) *Service {
	return &Service{store: store, imapFactory: imapFactory}
}

// SyncAccounts pulls every target mailbox and persists the fetched messages.
// It returns the synced messages so callers can report per-message details.
func (s *Service) SyncAccounts(ctx context.Context, targets []models.SyncTarget) ([]*models.Message, error) {
	if s.store == nil || s.imapFactory == nil {
		return nil, errors.New("syncer is not fully configured")
	}

	var synced []*models.Message
	for _, target := range targets {
		messages, err := s.syncTarget(ctx, target)
		if err != nil {
			return synced, err
		}
		synced = append(synced, messages...)
	}
	return synced, nil
}

func (s *Service) syncTarget(ctx context.Context, target models.SyncTarget) ([]*models.Message, error) {
	repo := s.imapFactory()
	if repo == nil {
		return nil, errors.New("imap repository factory returned nil")
	}

	if err := repo.Connect(
		ctx,
		target.Host,
		target.Port,
		target.Username,
		target.Password,
		target.AuthType,
		target.TLS,
	); err != nil {
		return nil, errors.Wrapf(err, "connect imap for %s", target.AccountName)
	}
	defer repo.Disconnect(ctx) //nolint:errcheck // best-effort cleanup after sync.

	mailbox := strings.TrimSpace(target.Mailbox)
	if mailbox == "" {
		mailbox = defaultMailbox
	}
	limit := target.Limit
	if limit <= 0 {
		limit = defaultFetchLimit
	}

	messages, err := repo.Fetch(ctx, mailbox, limit)
	if err != nil {
		return nil, errors.Wrapf(err, "fetch messages for %s", target.AccountName)
	}

	for _, msg := range messages {
		s.adoptMessage(msg, target.AccountName)
		s.mergeLocalState(ctx, msg)
	}
	if err := s.persist(ctx, target.AccountName, messages); err != nil {
		return nil, err
	}

	return messages, nil
}

// batchSaver is an optional capability: a store that can persist many messages
// in a single transaction. sqlite implements it; the JSON backend does not, and
// falls back to per-message Save.
type batchSaver interface {
	SaveBatch(ctx context.Context, messages []*models.Message) error
}

func (s *Service) persist(ctx context.Context, accountName string, messages []*models.Message) error {
	if bs, ok := s.store.(batchSaver); ok {
		if err := bs.SaveBatch(ctx, messages); err != nil {
			return errors.Wrapf(err, "persist synced messages for %s", accountName)
		}
		return nil
	}
	for _, msg := range messages {
		if err := s.store.Save(ctx, msg); err != nil {
			return errors.Wrapf(err, "persist synced message %s", msg.ID)
		}
	}
	return nil
}

// adoptMessage stamps a fetched message with the owning account and mailbox labels.
func (s *Service) adoptMessage(msg *models.Message, accountName string) {
	msg.AccountID = accountName
	// Message identifiers (both UID-derived "imap-" fallbacks and RFC Message-IDs)
	// are only unique within one account, so scope every ID by account. Otherwise
	// the same message delivered to two accounts (identical Message-ID) would
	// collide into a single row and flip account_id on each sync.
	if scoped := scopeMessageID(msg.ID, accountName); scoped != msg.ID {
		if msg.ThreadID == msg.ID {
			msg.ThreadID = scoped
		}
		msg.ID = scoped
	}
	if msg.IsDraft {
		msg.Labels = append(msg.Labels, "draft")
	} else {
		msg.Labels = append(msg.Labels, "inbox")
	}
}

// mergeLocalState carries local-only message state from the stored copy onto a
// freshly fetched one, so persisting the fetch doesn't undo the user's actions.
// IMAP is fetch-only for now — moving a message to trash, archiving, marking as
// spam/read/starred all happen purely in the local store and never reach the
// server — so without this merge every sync would resurrect trashed messages
// back into the inbox (the upsert overwrites labels and flags wholesale). The
// server stays authoritative for content; the local copy for state.
func (s *Service) mergeLocalState(ctx context.Context, msg *models.Message) {
	existing, err := s.store.GetByID(ctx, msg.ID)
	if err != nil || existing == nil {
		return
	}
	msg.IsDeleted = msg.IsDeleted || existing.IsDeleted
	msg.IsSpam = msg.IsSpam || existing.IsSpam
	msg.IsRead = msg.IsRead || existing.IsRead
	msg.IsStarred = msg.IsStarred || existing.IsStarred
	msg.Flags.Deleted = msg.IsDeleted
	msg.Flags.Junk = msg.IsSpam
	msg.Flags.Seen = msg.IsRead
	msg.Flags.Flagged = msg.IsStarred
	// Local labels are authoritative: they encode archive moves and user tags,
	// and the fetch path only ever stamps the mailbox label (adoptMessage), so
	// taking the stored set loses nothing from the server.
	if len(existing.Labels) > 0 {
		msg.Labels = append([]string{}, existing.Labels...)
	}
}

// scopeMessageID prefixes a message ID with the owning account so IDs stay unique
// across accounts. The "imap-" fallback prefix is kept at the front so downstream
// ID heuristics keep recognizing UID-derived messages.
func scopeMessageID(id, accountName string) string {
	if id == "" || accountName == "" {
		return id
	}
	if after, ok := strings.CutPrefix(id, "imap-"); ok {
		return "imap-" + accountName + "-" + after
	}
	return accountName + "-" + id
}
