package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-faster/errors"
	appcore "github.com/kriuchkov/postero/internal/app"
	coreerrors "github.com/kriuchkov/postero/internal/core/errors"
	"github.com/kriuchkov/postero/internal/core/models"
	"github.com/spf13/cobra"
)

var (
	replyAccount string
	replyBody    string
	replySend    bool
)

var replyCmd = &cobra.Command{
	Use:   "reply [id]",
	Short: "Reply to an email",
	Long:  `Reply to a specific email message.`,
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		service, cfg, err := appcore.NewMessageService()
		if err != nil {
			return err
		}

		draft, err := service.ReplyToMessage(context.Background(), id, replyBody)
		if err != nil {
			return errors.Wrap(err, "reply to message")
		}
		if strings.TrimSpace(replyAccount) != "" {
			account, ok := appcore.ResolveAccount(cfg, replyAccount)
			if !ok {
				return coreerrors.AccountNotFound(replyAccount)
			}
			draft, err = service.UpdateDraft(context.Background(), draft.ID, &models.UpdateMessageRequest{
				AccountID: stringPtr(account.Name),
				From:      stringPtr(account.Email),
			})
			if err != nil {
				return errors.Wrap(err, "rebind reply draft")
			}
		}

		if replySend {
			if err := service.SendMessage(context.Background(), draft.ID); err != nil {
				return errors.Wrap(err, "send reply")
			}
			fmt.Printf("Sent reply %s to %s\n", draft.ID, strings.Join(draft.To, ", "))
			return nil
		}

		fmt.Printf("Saved reply draft %s\n", draft.ID)
		return nil
	},
}

func init() {
	replyCmd.Flags().StringVar(&replyAccount, "account", "", "account name or email to send from")
	replyCmd.Flags().StringVar(&replyBody, "body", "", "reply body")
	replyCmd.Flags().BoolVar(&replySend, "send", false, "send reply immediately")
}
