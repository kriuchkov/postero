package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-faster/errors"
	"github.com/kriuchkov/postero/internal/adapters/mail/imap"
	appcore "github.com/kriuchkov/postero/internal/app"
	"github.com/kriuchkov/postero/internal/config"
	coreerrors "github.com/kriuchkov/postero/internal/core/errors"
	"github.com/kriuchkov/postero/internal/core/ports"
	"github.com/spf13/cobra"
)

var syncAccountName string

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Synchronize emails with IMAP server",
	Long:  `Fetch and synchronize emails from configured IMAP accounts.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		store, cfg, err := appcore.NewMessageRepository()
		if err != nil {
			return err
		}
		if cfg == nil || len(cfg.Accounts) == 0 {
			return syncMockMessages(ctx, store)
		}

		accounts := cfg.Accounts
		if strings.TrimSpace(syncAccountName) != "" {
			account, ok := appcore.ResolveAccount(cfg, syncAccountName)
			if !ok {
				return coreerrors.AccountNotFound(syncAccountName)
			}
			accounts = []config.AccountConfig{account}
		}

		total := 0
		for _, account := range accounts {
			count, err := syncAccount(ctx, store, account)
			if err != nil {
				return err
			}
			total += count
		}

		fmt.Printf("Synced %d emails into the local store\n", total)
		return nil
	},
}

func syncAccount(ctx context.Context, store ports.MessageRepository, account config.AccountConfig) (int, error) {
	username, password := account.IMAPCredentials()
	if password == "" {
		return 0, coreerrors.PasswordNotConfigured(account.Name)
	}

	repo := imap.NewRepository()
	if err := repo.Connect(ctx, account.IMAP.Host, account.IMAP.Port, username, password, account.IMAP.AuthType, account.IMAP.TLS); err != nil {
		return 0, errors.Wrapf(err, "connect imap for %s", account.Name)
	}
	defer repo.Disconnect(ctx) //nolint:errcheck

	messages, err := repo.Fetch(ctx, "INBOX", 50)
	if err != nil {
		return 0, errors.Wrapf(err, "fetch messages for %s", account.Name)
	}

	for _, msg := range messages {
		msg.AccountID = account.Name
		if msg.IsDraft {
			msg.Labels = append(msg.Labels, "draft")
		} else {
			msg.Labels = append(msg.Labels, "inbox")
		}
		if err := store.Save(ctx, msg); err != nil {
			return 0, errors.Wrapf(err, "persist synced message %s", msg.ID)
		}
	}

	for _, msg := range messages {
		fmt.Printf("  - [%s] %s from %s (%s)\n", msg.ID, msg.Subject, msg.From, account.Name)
	}

	return len(messages), nil
}

func syncMockMessages(ctx context.Context, store ports.MessageRepository) error {
	repo := imap.NewMockRepository()
	if err := repo.Connect(ctx, "imap.gmail.com", 993, "user", "pass", "plain", true); err != nil {
		return errors.Wrap(err, "failed to connect mock imap")
	}
	defer repo.Disconnect(ctx) //nolint:errcheck

	messages, err := repo.Fetch(ctx, "INBOX", 50)
	if err != nil {
		return errors.Wrap(err, "failed to fetch mock messages")
	}
	for _, msg := range messages {
		msg.AccountID = "mock"
		msg.Labels = append(msg.Labels, "inbox")
		if err := store.Save(ctx, msg); err != nil {
			return errors.Wrapf(err, "persist mock message %s", msg.ID)
		}
	}
	fmt.Printf("Synced %d mock emails into the local store\n", len(messages))
	return nil
}

func init() {
	syncCmd.Flags().StringVar(&syncAccountName, "account", "", "sync only the specified account name or email")
}

func stringPtr(value string) *string {
	return &value
}
