package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-faster/errors"
	"github.com/spf13/cobra"

	appcore "github.com/kriuchkov/postero/internal/app"
	coreerrors "github.com/kriuchkov/postero/internal/core/errors"
	"github.com/kriuchkov/postero/internal/core/models"
)

var (
	replyAccount string
	replyBody    string
	replyAll     bool
	replySend    bool
)

var replyCmd = &cobra.Command{
	Use:   "reply [id]",
	Short: "Reply to an email",
	Long:  `Reply to a specific email message.`,
	Args:  cobra.MinimumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		id := args[0]
		service, cfg, err := appcore.NewMessageService()
		if err != nil {
			return err
		}

		var draft *models.Message
		if replyAll {
			draft, err = service.ReplyAllToMessage(context.Background(), id, replyBody)
		} else {
			draft, err = service.ReplyToMessage(context.Background(), id, replyBody)
		}
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
			mode := "reply"
			if replyAll {
				mode = "reply-all"
			}
			fmt.Printf("Sent %s %s to %s\n", mode, draft.ID, strings.Join(draft.To, ", "))
			return nil
		}

		if replyAll {
			fmt.Printf("Saved reply-all draft %s\n", draft.ID)
			return nil
		}
		fmt.Printf("Saved reply draft %s\n", draft.ID)
		return nil
	},
}

func init() {
	replyCmd.Flags().StringVar(&replyAccount, "account", "", "account name or email to send from")
	replyCmd.Flags().StringVar(&replyBody, "body", "", "reply body")
	replyCmd.Flags().BoolVar(&replyAll, "all", false, "reply to all recipients")
	replyCmd.Flags().BoolVar(&replySend, "send", false, "send reply immediately")
}
