package cli

import (
	"context"
	"fmt"

	"github.com/go-faster/errors"
	appcore "github.com/kriuchkov/postero/internal/app"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Print a mailbox snapshot",
	Long:  `Print a mailbox snapshot to standard output without starting the interactive TUI.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		service, _, err := appcore.NewMessageService()
		if err != nil {
			return err
		}

		messages, err := service.ListMessages(context.Background(), 25, 0)
		if err != nil {
			return errors.Wrap(err, "failed to list messages")
		}

		fmt.Println("📧 Inbox")
		fmt.Println("─────────────────────────────────────────────")
		for i, msg := range messages {
			status := " "
			if msg.IsStarred {
				status = "⭐"
			} else if !msg.IsRead {
				status = "•"
			}

			fmt.Printf("%d. %s [%s] %s\n", i+1, status, msg.From, msg.Subject)
			if msg.Subject != "" {
				fmt.Printf("   %s\n", msg.Body[:min(len(msg.Body), 60)])
			}
		}
		fmt.Println("─────────────────────────────────────────────")
		fmt.Printf("Total: %d messages\n", len(messages))

		return nil
	},
}
