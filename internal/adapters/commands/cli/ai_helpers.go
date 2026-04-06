package cli

import (
	"context"
	"strings"

	"github.com/go-faster/errors"

	appcore "github.com/kriuchkov/postero/internal/app"
	"github.com/kriuchkov/postero/internal/config"
	"github.com/kriuchkov/postero/internal/core/models"
	"github.com/kriuchkov/postero/internal/core/ports"
)

var (
	newAIMessageService = appcore.NewMessageService
	newAIAssistant      = appcore.NewDraftAssistantWithConfig
)

func parseTemplateVariables(values []string) (map[string]string, error) {
	if len(values) == 0 {
		return map[string]string{}, nil
	}
	variables := make(map[string]string, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key, rawValue, ok := strings.Cut(trimmed, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return nil, errors.Errorf("invalid template variable %q, expected key=value", value)
		}
		variables[strings.TrimSpace(key)] = strings.TrimSpace(rawValue)
	}
	if len(variables) == 0 {
		return map[string]string{}, nil
	}
	return variables, nil
}

func commandContext() context.Context {
	if ctx := rootCmd.Context(); ctx != nil {
		return ctx
	}
	return context.Background()
}

func updateDraftWithGeneratedSubject(
	service ports.MessageService,
	draft *models.Message,
	subject string,
	account config.AccountConfig,
	useAccount bool,
) (*models.Message, error) {
	request := &models.UpdateMessageRequest{}
	shouldUpdate := false
	if strings.TrimSpace(subject) != "" {
		request.Subject = stringPtr(strings.TrimSpace(subject))
		shouldUpdate = true
	}
	if useAccount {
		request.AccountID = stringPtr(account.Name)
		request.From = stringPtr(account.Email)
		shouldUpdate = true
	}
	if !shouldUpdate {
		return draft, nil
	}
	return service.UpdateDraft(commandContext(), draft.ID, request)
}
