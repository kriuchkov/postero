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
	forwardAccount string
	forwardTo      []string
	forwardSend    bool
)

var forwardCmd = &cobra.Command{
	Use:   "forward [id]",
	Short: "Forward an email",
	Long:  `Forward a specific email message.`,
	Args:  cobra.MinimumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		id := args[0]
		service, cfg, err := appcore.NewMessageService()
		if err != nil {
			return err
		}

		draft, err := service.ForwardMessage(context.Background(), id, forwardTo)
		if err != nil {
			return errors.Wrap(err, "forward message")
		}
		if strings.TrimSpace(forwardAccount) != "" {
			account, ok := appcore.ResolveAccount(cfg, forwardAccount)
			if !ok {
				return coreerrors.AccountNotFound(forwardAccount)
			}
			draft, err = service.UpdateDraft(context.Background(), draft.ID, &models.UpdateMessageRequest{
				AccountID: stringPtr(account.Name),
				From:      stringPtr(account.Email),
			})
			if err != nil {
				return errors.Wrap(err, "rebind forward draft")
			}
		}

		if forwardSend {
			if err := service.SendMessage(context.Background(), draft.ID); err != nil {
				return errors.Wrap(err, "send forward")
			}
			fmt.Printf("Sent forward %s to %s\n", draft.ID, strings.Join(draft.To, ", "))
			return nil
		}

		fmt.Printf("Saved forward draft %s\n", draft.ID)
		return nil
	},
}

func init() {
	forwardCmd.Flags().StringVar(&forwardAccount, "account", "", "account name or email to send from")
	forwardCmd.Flags().StringSliceVar(&forwardTo, "to", nil, "recipient addresses")
	forwardCmd.Flags().BoolVar(&forwardSend, "send", false, "send forward immediately")
}
