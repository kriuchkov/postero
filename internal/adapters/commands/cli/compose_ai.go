package cli

import (
	"fmt"
	"strings"

	"github.com/go-faster/errors"
	"github.com/spf13/cobra"

	appcore "github.com/kriuchkov/postero/internal/app"
	coreerrors "github.com/kriuchkov/postero/internal/core/errors"
	"github.com/kriuchkov/postero/internal/core/models"
)

var (
	composeAIAccount     string
	composeAITo          []string
	composeAICc          []string
	composeAIBcc         []string
	composeAISubject     string
	composeAIBody        string
	composeAIAttach      []string
	composeAIInstruction string
	composeAITemplate    string
	composeAIVars        []string
	composeAISend        bool
)

var composeAICmd = &cobra.Command{
	Use:   "ai",
	Short: "Generate a draft with AI",
	Long:  `Generate a draft with a configured AI template and save or send it as a normal Postero message.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		service, cfg, err := newAIMessageService()
		if err != nil {
			return err
		}
		assistant, err := newAIAssistant(cfg)
		if err != nil {
			return err
		}

		account, ok := appcore.ResolveAccount(cfg, composeAIAccount)
		if !ok {
			return coreerrors.AccountNotFound(composeAIAccount)
		}
		attachments, err := loadComposeAttachments(composeAIAttach)
		if err != nil {
			return err
		}
		variables, err := parseTemplateVariables(composeAIVars)
		if err != nil {
			return err
		}
		generated, err := assistant.GenerateDraft(commandContext(), models.GenerateDraftRequest{
			Mode:        "compose",
			Template:    composeAITemplate,
			AccountID:   account.Name,
			From:        account.Email,
			To:          composeAITo,
			Cc:          composeAICc,
			Bcc:         composeAIBcc,
			Subject:     composeAISubject,
			Body:        composeAIBody,
			Instruction: composeAIInstruction,
			Variables:   variables,
		})
		if err != nil {
			return err
		}

		message, err := service.ComposeMessage(commandContext(), &models.CreateMessageRequest{
			AccountID:   account.Name,
			From:        account.Email,
			To:          composeAITo,
			Cc:          composeAICc,
			Bcc:         composeAIBcc,
			Subject:     firstNonEmpty(generated.Subject, composeAISubject),
			Body:        generated.Body,
			Labels:      []string{"draft"},
			Attachments: attachments,
		})
		if err != nil {
			return errors.Wrap(err, "compose ai draft")
		}

		if composeAISend {
			if err := service.SendMessage(commandContext(), message.ID); err != nil {
				return errors.Wrap(err, "send ai draft")
			}
			fmt.Printf("Sent AI draft %s to %s\n", message.ID, strings.Join(message.To, ", "))
			return nil
		}

		fmt.Printf("Saved AI draft %s\n", message.ID)
		return nil
	},
}

func init() {
	composeAICmd.Flags().StringVar(&composeAIAccount, "account", "", "account name or email to send from")
	composeAICmd.Flags().StringSliceVar(&composeAITo, "to", nil, "recipient addresses")
	composeAICmd.Flags().StringSliceVar(&composeAICc, "cc", nil, "cc recipient addresses")
	composeAICmd.Flags().StringSliceVar(&composeAIBcc, "bcc", nil, "bcc recipient addresses")
	composeAICmd.Flags().StringVar(&composeAISubject, "subject", "", "subject hint passed to the template")
	composeAICmd.Flags().StringVar(&composeAIBody, "body", "", "body hint passed to the template")
	composeAICmd.Flags().StringVar(&composeAIInstruction, "instruction", "", "high-level drafting instruction for the template")
	composeAICmd.Flags().StringVar(&composeAITemplate, "template", "", "ai template name from config")
	composeAICmd.Flags().StringSliceVar(&composeAIVars, "var", nil, "additional template variables as key=value")
	composeAICmd.Flags().StringSliceVar(&composeAIAttach, "attach", nil, "file path to attach; can be specified multiple times")
	composeAICmd.Flags().BoolVar(&composeAISend, "send", false, "send immediately instead of saving a draft")
	composeCmd.AddCommand(composeAICmd)
}
