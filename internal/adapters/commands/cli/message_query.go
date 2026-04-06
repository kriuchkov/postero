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
	criteria := models.SearchCriteria{
		AccountID: accountID,
		Labels:    append([]string{}, labels...),
		Limit:     limit,
		Offset:    offset,
	}

	switch strings.TrimSpace(strings.ToLower(mailbox)) {
	case "", "inbox":
		isDraft := false
		isSpam := false
		isDeleted := false
		criteria.IsDraft = &isDraft
		criteria.IsSpam = &isSpam
		criteria.IsDeleted = &isDeleted
		criteria.Labels = append(criteria.Labels, "inbox")
	case "all":
	case "archive":
		isDeleted := false
		criteria.IsDeleted = &isDeleted
		criteria.Labels = append(criteria.Labels, "archive")
	case "draft", "drafts":
		isDraft := true
		isDeleted := false
		criteria.IsDraft = &isDraft
		criteria.IsDeleted = &isDeleted
	case "sent":
		isDeleted := false
		criteria.IsDeleted = &isDeleted
		criteria.Labels = append(criteria.Labels, "sent")
	case "spam":
		isSpam := true
		criteria.IsSpam = &isSpam
	case "trash":
		isDeleted := true
		criteria.IsDeleted = &isDeleted
	case "flagged", "starred":
		isStarred := true
		isDeleted := false
		criteria.IsStarred = &isStarred
		criteria.IsDeleted = &isDeleted
	default:
		return models.SearchCriteria{}, errors.Errorf("unsupported mailbox %q", mailbox)
	}

	return criteria, nil
}
