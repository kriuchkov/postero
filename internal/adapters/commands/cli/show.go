package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-faster/errors"
	"github.com/spf13/cobra"

	appcore "github.com/kriuchkov/postero/internal/app"
)

var (
	showFormat   string
	showMarkRead bool
)

var showCmd = &cobra.Command{
	Use:     "show [id]",
	Aliases: []string{"view"},
	Short:   "Show a full message",
	Long:    `Display a single message with headers, labels, attachments, and body content.`,
	Args:    cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		service, _, err := appcore.NewMessageService()
		if err != nil {
			return err
		}

		message, err := service.GetMessage(context.Background(), args[0])
		if err != nil {
			return errors.Wrap(err, "load message")
		}
		if showMarkRead {
			message, err = service.MarkAsRead(context.Background(), args[0])
			if err != nil {
				return errors.Wrap(err, "mark message as read")
			}
		}

		switch strings.ToLower(strings.TrimSpace(showFormat)) {
		case "", outputFormatText:
			_, err = fmt.Fprint(rootCmd.OutOrStdout(), renderMessageDetail(message))
			return err
		case outputFormatJSON:
			return writeJSON(rootCmd.OutOrStdout(), message)
		default:
			return errors.Errorf("unsupported format %q", showFormat)
		}
	},
}

func init() {
	showCmd.Flags().StringVar(&showFormat, "format", "text", "output format: text or json")
	showCmd.Flags().BoolVar(&showMarkRead, "mark-read", false, "mark the message as read before printing it")
	rootCmd.AddCommand(showCmd)
}
