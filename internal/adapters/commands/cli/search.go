package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-faster/errors"
	"github.com/spf13/cobra"

	appcore "github.com/kriuchkov/postero/internal/app"
	"github.com/kriuchkov/postero/internal/core/models"
)

var (
	searchAccount string
	searchLabels  []string
	searchLimit   int
	searchOffset  int
	searchFormat  string
	searchUnread  bool
)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search emails",
	Long:  `Search emails by subject, sender, or content.`,
	Args:  cobra.MinimumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		query := strings.Join(args, " ")
		service, cfg, err := appcore.NewMessageService()
		if err != nil {
			return err
		}
		accountID, err := resolveAccountID(cfg, searchAccount)
		if err != nil {
			return err
		}

		criteria := models.SearchCriteria{
			Query:     query,
			AccountID: accountID,
			Labels:    append([]string{}, searchLabels...),
			Limit:     searchLimit,
			Offset:    searchOffset,
		}
		if searchUnread {
			isRead := false
			criteria.IsRead = &isRead
		}

		messages, err := service.SearchMessages(context.Background(), criteria)
		if err != nil {
			return errors.Wrap(err, "search failed")
		}

		if len(messages) == 0 {
			fmt.Printf("No messages found matching: %s\n", query)
			return nil
		}

		switch strings.ToLower(strings.TrimSpace(searchFormat)) {
		case "", outputFormatText:
			fmt.Printf("Search results for: %s\n", query)
			for _, msg := range messages {
				fmt.Println(renderMessageSummary(msg))
			}
			fmt.Printf("Found %d messages\n", len(messages))
			return nil
		case outputFormatJSON:
			return writeJSON(rootCmd.OutOrStdout(), messages)
		default:
			return errors.Errorf("unsupported format %q", searchFormat)
		}
	},
}

func init() {
	searchCmd.Flags().StringVar(&searchAccount, "account", "", "filter search by account name or email")
	searchCmd.Flags().StringSliceVar(&searchLabels, "label", nil, "limit search to messages with the given labels")
	searchCmd.Flags().IntVar(&searchLimit, "limit", 50, "maximum number of search results")
	searchCmd.Flags().IntVar(&searchOffset, "offset", 0, "number of search results to skip")
	searchCmd.Flags().StringVar(&searchFormat, "format", "text", "output format: text or json")
	searchCmd.Flags().BoolVar(&searchUnread, "unread", false, "only return unread messages")
}
