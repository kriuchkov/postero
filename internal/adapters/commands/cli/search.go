package cli

import (
	"context"
	"fmt"

	"github.com/go-faster/errors"
	appcore "github.com/kriuchkov/postero/internal/app"
	"github.com/kriuchkov/postero/internal/core/models"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search emails",
	Long:  `Search emails by subject, sender, or content.`,
	Args:  cobra.MinimumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		query := args[0]
		service, _, err := appcore.NewMessageService()
		if err != nil {
			return err
		}

		messages, err := service.SearchMessages(context.Background(), models.SearchCriteria{Query: query, Limit: 50})
		if err != nil {
			return errors.Wrap(err, "search failed")
		}

		if len(messages) == 0 {
			fmt.Printf("No messages found matching: %s\n", query)
			return nil
		}

		fmt.Printf("🔍 Search results for: %s\n", query)
		fmt.Println("─────────────────────────────────────────────")
		for i, msg := range messages {
			fmt.Printf("%d. [%s] %s from %s\n", i+1, msg.ID, msg.Subject, msg.From)
		}
		fmt.Println("─────────────────────────────────────────────")
		fmt.Printf("Found %d messages\n", len(messages))

		return nil
	},
}
