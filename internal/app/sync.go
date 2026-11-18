package app

import (
	"context"
	"strings"

	"github.com/go-faster/errors"

	"github.com/kriuchkov/postero/internal/adapters/mail/imap"
	"github.com/kriuchkov/postero/internal/config"
	coreerrors "github.com/kriuchkov/postero/internal/core/errors"
	"github.com/kriuchkov/postero/internal/core/models"
	"github.com/kriuchkov/postero/internal/core/ports"
	"github.com/kriuchkov/postero/internal/services/syncer"
)

// NewAccountSyncer wires the sync service with the IMAP transport.
func NewAccountSyncer(store ports.MessageRepository) ports.AccountSyncer {
	return syncer.NewService(store, imap.NewRepository)
}

// mailboxSyncer binds the whole-mailbox sync capability to a store and config so
// the UI can drive it through the ports.MailboxSyncer boundary.
type mailboxSyncer struct {
	store ports.MessageRepository
	cfg   *config.Config
}

// NewMailboxSyncer returns a MailboxSyncer that syncs every configured account.
func NewMailboxSyncer(store ports.MessageRepository, cfg *config.Config) ports.MailboxSyncer {
	return &mailboxSyncer{store: store, cfg: cfg}
}

func (s *mailboxSyncer) SyncAll(ctx context.Context) ([]*models.Message, error) {
	return SyncAccounts(ctx, s.store, s.cfg, "")
}

// SyncAccounts resolves the matching accounts from config and pulls their
// INBOX messages into store. An empty selector syncs every configured account.
func SyncAccounts(ctx context.Context, store ports.MessageRepository, cfg *config.Config, selector string) ([]*models.Message, error) {
	if store == nil {
		return nil, errors.New("message repository is required")
	}
	targets, err := syncTargetsFromConfig(cfg, selector)
	if err != nil {
		return nil, err
	}
	// A real sync means real accounts exist; drop any leftover demo/sample data
	// so it never mixes with real mail.
	if _, err := PurgeDemoMailbox(ctx, store); err != nil {
		return nil, err
	}
	return NewAccountSyncer(store).SyncAccounts(ctx, targets)
}

// syncTargetsFromConfig resolves accounts and their credentials into
// transport-agnostic sync targets.
func syncTargetsFromConfig(cfg *config.Config, selector string) ([]models.SyncTarget, error) {
	if cfg == nil || len(cfg.Accounts) == 0 {
		return nil, errors.New("no accounts configured")
	}

	accounts := cfg.Accounts
	if strings.TrimSpace(selector) != "" {
		account, ok := ResolveAccount(cfg, selector)
		if !ok {
			return nil, coreerrors.AccountNotFound(selector)
		}
		accounts = []config.AccountConfig{account}
	}

	targets := make([]models.SyncTarget, 0, len(accounts))
	for _, account := range accounts {
		username, password := account.IMAPCredentials()
		if password == "" {
			return nil, coreerrors.PasswordNotConfigured(account.Name)
		}
		targets = append(targets, models.SyncTarget{
			AccountName: account.Name,
			Host:        account.IMAP.Host,
			Port:        account.IMAP.Port,
			Username:    username,
			Password:    password,
			AuthType:    account.IMAP.AuthType,
			TLS:         account.IMAP.TLS,
		})
	}
	return targets, nil
}
