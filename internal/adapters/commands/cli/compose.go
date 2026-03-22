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
	composeAccount string
	composeTo      []string
	composeCc      []string
	composeBcc     []string
	composeSubject string
	composeBody    string
	composeSend    bool
)

var composeCmd = &cobra.Command{
	Use:   "compose",
	Short: "Compose a new email",
	Long:  `Create and compose a new email message.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		service, cfg, err := appcore.NewMessageService()
		if err != nil {
			return err
		}

		account, ok := appcore.ResolveAccount(cfg, composeAccount)
		if !ok {
			return coreerrors.AccountNotFound(composeAccount)
		}
		message, err := service.ComposeMessage(context.Background(), &models.CreateMessageRequest{
			AccountID: account.Name,
			From:      account.Email,
			To:        composeTo,
			Cc:        composeCc,
			Bcc:       composeBcc,
			Subject:   composeSubject,
			Body:      composeBody,
			Labels:    []string{"draft"},
		})
		if err != nil {
			return errors.Wrap(err, "compose message")
		}

		if composeSend {
			if err := service.SendMessage(context.Background(), message.ID); err != nil {
				return errors.Wrap(err, "send message")
			}
			fmt.Printf("Sent message %s to %s\n", message.ID, strings.Join(message.To, ", "))
			return nil
		}

		fmt.Printf("Saved draft %s\n", message.ID)
		return nil
	},
}

func init() {
	composeCmd.Flags().StringVar(&composeAccount, "account", "", "account name or email to send from")
	composeCmd.Flags().StringSliceVar(&composeTo, "to", nil, "recipient addresses")
	composeCmd.Flags().StringSliceVar(&composeCc, "cc", nil, "cc recipient addresses")
	composeCmd.Flags().StringSliceVar(&composeBcc, "bcc", nil, "bcc recipient addresses")
	composeCmd.Flags().StringVar(&composeSubject, "subject", "", "message subject")
	composeCmd.Flags().StringVar(&composeBody, "body", "", "message body")
	composeCmd.Flags().BoolVar(&composeSend, "send", false, "send immediately instead of saving a draft")
}
