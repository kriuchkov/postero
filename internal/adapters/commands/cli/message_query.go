package cli

import (
	"strings"

	"github.com/go-faster/errors"

	appcore "github.com/kriuchkov/postero/internal/app"
	"github.com/kriuchkov/postero/internal/config"
	"github.com/kriuchkov/postero/internal/core/models"
)

func resolveAccountID(cfg *config.Config, selector string) (string, error) {
	if strings.TrimSpace(selector) == "" {
		return "", nil
	}
	account, ok := appcore.ResolveAccount(cfg, selector)
	if !ok {
		return "", errors.Errorf("account %q not found", selector)
	}
	return account.Name, nil
}

func buildListCriteria(mailbox string, labels []string, accountID string, limit, offset int) (models.SearchCriteria, error) {
	name := strings.TrimSpace(mailbox)
	if name == "" {
		name = string(models.MailboxInbox)
	}
	// Mailbox membership is defined in the domain; the CLI only picks a mailbox
	// and adds its own label filters and pagination on top.
	box, ok := models.ParseMailbox(name)
	if !ok {
		return models.SearchCriteria{}, errors.Errorf("unsupported mailbox %q", mailbox)
	}

	criteria := box.Criteria(accountID)
	criteria.Labels = append(criteria.Labels, labels...)
	criteria.Limit = limit
	criteria.Offset = offset
	return criteria, nil
}
