package cli

import (
	"context"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-faster/errors"
	"github.com/spf13/cobra"

	appcore "github.com/kriuchkov/postero/internal/app"
	coreerrors "github.com/kriuchkov/postero/internal/core/errors"
	"github.com/kriuchkov/postero/internal/core/models"
)

var (
	composeAccount string
	composeTo      []string
	composeCc      []string
	composeBcc     []string
	composeSubject string
	composeBody    string
	composeAttach  []string
	composeSend    bool
)

var composeCmd = &cobra.Command{
	Use:   "compose",
	Short: "Compose a new email",
	Long:  `Create and compose a new email message.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		service, cfg, err := appcore.NewMessageService()
		if err != nil {
			return err
		}

		account, ok := appcore.ResolveAccount(cfg, composeAccount)
		if !ok {
			return coreerrors.AccountNotFound(composeAccount)
		}
		attachments, err := loadComposeAttachments(composeAttach)
		if err != nil {
			return err
		}
		message, err := service.ComposeMessage(context.Background(), &models.CreateMessageRequest{
			AccountID:   account.Name,
			From:        account.Email,
			To:          composeTo,
			Cc:          composeCc,
			Bcc:         composeBcc,
			Subject:     composeSubject,
			Body:        composeBody,
			Labels:      []string{"draft"},
			Attachments: attachments,
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
	composeCmd.Flags().StringSliceVar(&composeAttach, "attach", nil, "file path to attach; can be specified multiple times")
	composeCmd.Flags().BoolVar(&composeSend, "send", false, "send immediately instead of saving a draft")
}

func loadComposeAttachments(paths []string) ([]*models.Attachment, error) {
	attachments := make([]*models.Attachment, 0, len(paths))
	for _, path := range paths {
		trimmed := strings.TrimSpace(path)
		if trimmed == "" {
			continue
		}
		data, err := os.ReadFile(trimmed)
		if err != nil {
			return nil, errors.Wrapf(err, "read attachment %s", trimmed)
		}
		filename := filepath.Base(trimmed)
		mimeType := mime.TypeByExtension(strings.ToLower(filepath.Ext(filename)))
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		attachments = append(attachments, &models.Attachment{
			Filename: filename,
			Size:     int64(len(data)),
			MimeType: mimeType,
			Data:     data,
		})
	}
	if len(attachments) == 0 {
		return nil, nil
	}
	return attachments, nil
}
