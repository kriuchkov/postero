package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/go-faster/errors"
	"github.com/spf13/cobra"

	appcore "github.com/kriuchkov/postero/internal/app"
	"github.com/kriuchkov/postero/internal/core/models"
)

var readCmd = newUpdateMessageCommand(
	"read [id...]",
	"Mark messages as read",
	"Message marked as read",
	func(service messageServiceActions, id string) (*models.Message, error) {
		return service.MarkAsRead(context.Background(), id)
	},
)

var starCmd = newUpdateMessageCommand(
	"star [id...]",
	"Toggle the starred state of messages",
	"Updated star state",
	func(service messageServiceActions, id string) (*models.Message, error) {
		return service.ToggleStar(context.Background(), id)
	},
)

var archiveCmd = newUpdateMessageCommand(
	"archive [id...]",
	"Archive messages",
	"Archived message",
	func(service messageServiceActions, id string) (*models.Message, error) {
		return service.ArchiveMessage(context.Background(), id)
	},
)

var spamCmd = newUpdateMessageCommand(
	"spam [id...]",
	"Mark messages as spam",
	"Marked message as spam",
	func(service messageServiceActions, id string) (*models.Message, error) {
		return service.MarkAsSpam(context.Background(), id)
	},
)

var trashCmd = newUpdateMessageCommand(
	"trash [id...]",
	"Move messages to trash without deleting them permanently",
	"Moved message to trash",
	func(service messageServiceActions, id string) (*models.Message, error) {
		message, err := service.GetMessage(context.Background(), id)
		if err != nil {
			return nil, err
		}
		if message != nil && message.IsDeleted {
			return message, nil
		}
		return service.ToggleDelete(context.Background(), id)
	},
)

var deleteCmd = &cobra.Command{
	Use:   "delete [id...]",
	Short: "Permanently delete messages from the local store",
	Long:  `Delete removes messages from the local Postero store permanently. Use trash if you need a reversible mailbox action.`,
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ids, err := collectMessageIDs(cmd, args)
		if err != nil {
			return err
		}

		service, _, err := appcore.NewMessageService()
		if err != nil {
			return err
		}

		for _, id := range ids {
			if err := service.DeleteMessage(context.Background(), id); err != nil {
				return errors.Wrapf(err, "delete message %s", id)
			}
			if _, err := fmt.Fprintf(rootCmd.OutOrStdout(), "Deleted message permanently: %s\n", id); err != nil {
				return err
			}
		}
		return nil
	},
}

type messageServiceActions interface {
	GetMessage(ctx context.Context, id string) (*models.Message, error)
	MarkAsRead(ctx context.Context, id string) (*models.Message, error)
	ToggleStar(ctx context.Context, id string) (*models.Message, error)
	ToggleDelete(ctx context.Context, id string) (*models.Message, error)
	ArchiveMessage(ctx context.Context, id string) (*models.Message, error)
	MarkAsSpam(ctx context.Context, id string) (*models.Message, error)
	DeleteMessage(ctx context.Context, id string) error
}

func newUpdateMessageCommand(
	use string,
	short string,
	label string,
	action func(messageServiceActions, string) (*models.Message, error),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ids, err := collectMessageIDs(cmd, args)
			if err != nil {
				return err
			}
			return runMessageActionBatch(ids, label, action)
		},
	}
	addStdinIDsFlag(cmd)
	return cmd
}

func runMessageActionBatch(ids []string, label string, action func(messageServiceActions, string) (*models.Message, error)) error {
	service, _, err := appcore.NewMessageService()
	if err != nil {
		return err
	}

	for _, id := range ids {
		message, err := action(service, id)
		if err != nil {
			return errors.Wrapf(err, "update message %s", id)
		}
		if _, err := fmt.Fprintf(rootCmd.OutOrStdout(), "%s: %s\n", label, renderMessageSummary(message)); err != nil {
			return err
		}
	}
	return nil
}

func collectMessageIDs(cmd *cobra.Command, args []string) ([]string, error) {
	useStdin, err := cmd.Flags().GetBool("stdin-ids")
	if err != nil {
		return nil, err
	}

	ids := append([]string{}, args...)
	if useStdin {
		stdinIDs, err := parseMessageIDs(rootCmd.InOrStdin())
		if err != nil {
			return nil, err
		}
		ids = append(ids, stdinIDs...)
	}

	ids = normalizeMessageIDs(ids)
	if len(ids) == 0 {
		return nil, errors.New("provide at least one message ID via arguments or --stdin-ids")
	}
	return ids, nil
}

func parseMessageIDs(reader io.Reader) ([]string, error) {
	ids := make([]string, 0)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		ids = append(ids, strings.Fields(scanner.Text())...)
	}
	if err := scanner.Err(); err != nil {
		return nil, errors.Wrap(err, "read stdin ids")
	}
	return ids, nil
}

func normalizeMessageIDs(ids []string) []string {
	cleaned := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		cleaned = append(cleaned, trimmed)
	}
	return cleaned
}

func addStdinIDsFlag(cmd *cobra.Command) {
	cmd.Flags().Bool("stdin-ids", false, "read message IDs from stdin as whitespace-separated values")
}

func init() {
	addStdinIDsFlag(deleteCmd)
	rootCmd.AddCommand(readCmd)
	rootCmd.AddCommand(starCmd)
	rootCmd.AddCommand(archiveCmd)
	rootCmd.AddCommand(spamCmd)
	rootCmd.AddCommand(trashCmd)
	rootCmd.AddCommand(deleteCmd)
}
