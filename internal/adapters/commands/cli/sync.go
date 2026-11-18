package cli

import (
	"context"
	"fmt"

	"github.com/go-faster/errors"
	"github.com/spf13/cobra"

	appcore "github.com/kriuchkov/postero/internal/app"
)

var syncAccountName string

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Synchronize emails with IMAP server",
	Long:  `Fetch and synchronize emails from configured IMAP accounts.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx := context.Background()
		store, cfg, err := appcore.NewMessageRepository()
		if err != nil {
			return err
		}
		if cfg == nil || len(cfg.Accounts) == 0 {
			return errors.New("no accounts configured — launch pstr to run the setup wizard, " +
				"add one with pstr auth add <provider>, or press ctrl+d in the TUI for a demo inbox")
		}

		messages, err := appcore.SyncAccounts(ctx, store, cfg, syncAccountName)
		if err != nil {
			return err
		}

		for _, msg := range messages {
			fmt.Printf("  - [%s] %s from %s (%s)\n", msg.ID, msg.Subject, msg.From, msg.AccountID)
		}
		fmt.Printf("Synced %d emails into the local store\n", len(messages))
		return nil
	},
}

func init() {
	syncCmd.Flags().StringVar(&syncAccountName, "account", "", "sync only the specified account name or email")
}
