package app

import (
	"context"
	"slices"

	"github.com/go-faster/errors"

	"github.com/kriuchkov/postero/internal/adapters/mail/imap"
	"github.com/kriuchkov/postero/internal/core/models"
	"github.com/kriuchkov/postero/internal/core/ports"
)

// mailboxSeeder binds the demo-seed capability to a store so the UI can drive it
// through the ports.MailboxSeeder boundary.
type mailboxSeeder struct {
	store ports.MessageRepository
}

// NewMailboxSeeder returns a MailboxSeeder backed by the given store.
func NewMailboxSeeder(store ports.MessageRepository) ports.MailboxSeeder {
	return &mailboxSeeder{store: store}
}

func (s *mailboxSeeder) SeedDemo(ctx context.Context) ([]*models.Message, error) {
	return SeedDemoMailbox(ctx, s.store)
}

// SeedDemoMailbox loads sample messages into the local store so the app can be
// explored without connecting a real account. It returns the seeded messages.
func SeedDemoMailbox(ctx context.Context, store ports.MessageRepository) ([]*models.Message, error) {
	if store == nil {
		return nil, errors.New("message repository is required")
	}

	repo := imap.NewMockRepository()
	messages, err := repo.Fetch(ctx, "INBOX", 0)
	if err != nil {
		return nil, errors.Wrap(err, "fetch demo messages")
	}

	for _, msg := range messages {
		msg.AccountID = models.DemoAccountName
		mailbox := "inbox"
		if msg.IsDraft {
			mailbox = "draft"
		}
		if !slices.Contains(msg.Labels, mailbox) {
			msg.Labels = append(msg.Labels, mailbox)
		}
		if err := store.Save(ctx, msg); err != nil {
			return nil, errors.Wrapf(err, "persist demo message %s", msg.ID)
		}
	}

	return messages, nil
}

// PurgeDemoMailbox removes every message tagged with the demo account from the
// store. It runs before a real sync so sample data never mixes with real mail.
// It returns the number of messages removed.
func PurgeDemoMailbox(ctx context.Context, store ports.MessageRepository) (int, error) {
	if store == nil {
		return 0, errors.New("message repository is required")
	}
	messages, err := store.Search(ctx, models.SearchCriteria{AccountID: models.DemoAccountName, Limit: 100000})
	if err != nil {
		return 0, errors.Wrap(err, "search demo messages")
	}
	removed := 0
	for _, msg := range messages {
		if err := store.Delete(ctx, msg.ID); err != nil {
			return removed, errors.Wrapf(err, "delete demo message %s", msg.ID)
		}
		removed++
	}
	return removed, nil
}
