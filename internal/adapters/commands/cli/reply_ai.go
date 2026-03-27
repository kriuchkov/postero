package cli

import (
	"fmt"
	"strings"

	"github.com/go-faster/errors"
	"github.com/spf13/cobra"

	appcore "github.com/kriuchkov/postero/internal/app"
	"github.com/kriuchkov/postero/internal/config"
	coreerrors "github.com/kriuchkov/postero/internal/core/errors"
	"github.com/kriuchkov/postero/internal/core/models"
)

var (
	replyAIAccount     string
	replyAIBody        string
	replyAISubject     string
	replyAIInstruction string
	replyAITemplate    string
	replyAIVars        []string
	replyAIAll         bool
	replyAISend        bool
)

var replyAICmd = &cobra.Command{
	Use:   "ai [id]",
	Short: "Generate an AI-assisted reply",
	Long:  `Generate a reply or reply-all draft from a configured AI template, then save or send it through the regular Postero flow.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		id := args[0]
		service, cfg, err := newAIMessageService()
		if err != nil {
			return err
		}
		assistant, err := newAIAssistant(cfg)
		if err != nil {
			return err
		}

		original, err := service.GetMessage(commandContext(), id)
		if err != nil {
			return errors.Wrap(err, "load original message")
		}
		variables, err := parseTemplateVariables(replyAIVars)
		if err != nil {
			return err
		}
		generated, err := assistant.GenerateDraft(commandContext(), models.GenerateDraftRequest{
			Mode:        "reply",
			Template:    replyAITemplate,
			AccountID:   original.AccountID,
			Subject:     replyAISubject,
			Body:        replyAIBody,
			Instruction: replyAIInstruction,
			ReplyAll:    replyAIAll,
			Original:    original,
			Variables:   variables,
		})
		if err != nil {
			return err
		}

		var draft *models.Message
		if replyAIAll {
			draft, err = service.ReplyAllToMessage(commandContext(), id, generated.Body)
		} else {
			draft, err = service.ReplyToMessage(commandContext(), id, generated.Body)
		}
		if err != nil {
			return errors.Wrap(err, "create ai reply draft")
		}

		var selectedAccount config.AccountConfig
		useAccount := false
		if strings.TrimSpace(replyAIAccount) != "" {
			selectedAccount, useAccount = appcore.ResolveAccount(cfg, replyAIAccount)
			if !useAccount {
				return coreerrors.AccountNotFound(replyAIAccount)
			}
		}
		draft, err = updateDraftWithGeneratedSubject(service, draft, generated.Subject, selectedAccount, useAccount)
		if err != nil {
			return errors.Wrap(err, "update ai reply draft")
		}

		if replyAISend {
			if err := service.SendMessage(commandContext(), draft.ID); err != nil {
				return errors.Wrap(err, "send ai reply")
			}
			mode := "AI reply"
			if replyAIAll {
				mode = "AI reply-all"
			}
			fmt.Printf("Sent %s %s to %s\n", mode, draft.ID, strings.Join(draft.To, ", "))
			return nil
		}

		if replyAIAll {
			fmt.Printf("Saved AI reply-all draft %s\n", draft.ID)
			return nil
		}
		fmt.Printf("Saved AI reply draft %s\n", draft.ID)
		return nil
	},
}

func init() {
	replyAICmd.Flags().StringVar(&replyAIAccount, "account", "", "account name or email to send from")
	replyAICmd.Flags().StringVar(&replyAISubject, "subject", "", "subject hint passed to the template")
	replyAICmd.Flags().StringVar(&replyAIBody, "body", "", "body hint passed to the template")
	replyAICmd.Flags().StringVar(&replyAIInstruction, "instruction", "", "high-level reply instruction for the template")
	replyAICmd.Flags().StringVar(&replyAITemplate, "template", "", "ai template name from config")
	replyAICmd.Flags().StringSliceVar(&replyAIVars, "var", nil, "additional template variables as key=value")
	replyAICmd.Flags().BoolVar(&replyAIAll, "all", false, "reply to all recipients")
	replyAICmd.Flags().BoolVar(&replyAISend, "send", false, "send reply immediately")
	replyCmd.AddCommand(replyAICmd)
}
