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
	listAccount string
	listMailbox string
	listLabels  []string
	listLimit   int
	listOffset  int
	listFormat  string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Print a mailbox snapshot",
	Long:  `Print a mailbox snapshot to standard output without starting the interactive TUI.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		service, cfg, err := appcore.NewMessageService()
		if err != nil {
			return err
		}

		accountID, err := resolveAccountID(cfg, listAccount)
		if err != nil {
			return err
		}

		criteria, err := buildListCriteria(listMailbox, listLabels, accountID, listLimit, listOffset)
		if err != nil {
			return err
		}

		messages, err := service.SearchMessages(context.Background(), criteria)
		if err != nil {
			return errors.Wrap(err, "failed to list messages")
		}

		switch strings.ToLower(strings.TrimSpace(listFormat)) {
		case "", outputFormatText:
			mailbox := strings.TrimSpace(listMailbox)
			if mailbox == "" {
				mailbox = "inbox"
			}
			fmt.Printf("Mailbox: %s\n", mailbox)
			for _, msg := range messages {
				fmt.Println(renderMessageSummary(msg))
			}
			fmt.Printf("Total: %d messages\n", len(messages))
			return nil
		case outputFormatJSON:
			return writeJSON(rootCmd.OutOrStdout(), messages)
		default:
			return errors.Errorf("unsupported format %q", listFormat)
		}
	},
}

func init() {
	listCmd.Flags().StringVar(&listAccount, "account", "", "filter by account name or email")
	listCmd.Flags().StringVar(&listMailbox, "mailbox", "inbox", "mailbox to list: inbox, all, archive, drafts, sent, spam, trash, flagged")
	listCmd.Flags().StringSliceVar(&listLabels, "label", nil, "additional label filters")
	listCmd.Flags().IntVar(&listLimit, "limit", 25, "maximum number of messages to list")
	listCmd.Flags().IntVar(&listOffset, "offset", 0, "number of messages to skip before listing")
	listCmd.Flags().StringVar(&listFormat, "format", "text", "output format: text or json")
}
