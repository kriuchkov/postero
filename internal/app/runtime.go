package app

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-faster/errors"

	"github.com/kriuchkov/postero/internal/adapters/ai/gemini"
	"github.com/kriuchkov/postero/internal/adapters/ai/openai"
	"github.com/kriuchkov/postero/internal/adapters/mail/smtp"
	filestore "github.com/kriuchkov/postero/internal/adapters/storage/file"
	"github.com/kriuchkov/postero/internal/adapters/storage/sqlite"
	"github.com/kriuchkov/postero/internal/config"
	coreerrors "github.com/kriuchkov/postero/internal/core/errors"
	"github.com/kriuchkov/postero/internal/core/ports"
	"github.com/kriuchkov/postero/internal/services/assistant"
	"github.com/kriuchkov/postero/internal/services/message"
)

func LoadConfig() (*config.Config, error) {
	return config.LoadConfig()
}

func NewMessageRepository() (ports.MessageRepository, *config.Config, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, nil, errors.Wrap(err, "load config")
	}

	dataPath := cfg.DataPath
	if dataPath == "" {
		dataPath = filepath.Join(".", ".postero")
	}

	if err := os.MkdirAll(dataPath, 0o700); err != nil {
		return nil, nil, errors.Wrap(err, "create data directory")
	}

	var repo ports.MessageRepository
	backend := config.StorageBackend(cfg)
	switch backend {
	case "sqlite":
		repo, err = sqlite.NewRepository(filepath.Join(dataPath, "postero.db"))
		if err != nil {
			return nil, nil, errors.Wrap(err, "create sqlite repository")
		}
	case "file":
		repo, err = filestore.NewRepository(filepath.Join(dataPath, "messages"))
		if err != nil {
			return nil, nil, errors.Wrap(err, "create file repository")
		}
	default:
		return nil, nil, errors.Errorf("unsupported storage backend %q", backend)
	}

	return repo, cfg, nil
}

func NewMessageService() (ports.MessageService, *config.Config, error) {
	repo, cfg, err := NewMessageRepository()
	if err != nil {
		return nil, nil, err
	}

	return message.NewServiceWithSMTP(repo, smtpFactory(cfg)), cfg, nil
}

func NewDraftAssistant() (ports.DraftAssistant, *config.Config, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, nil, errors.Wrap(err, "load config")
	}
	service, err := NewDraftAssistantWithConfig(cfg)
	if err != nil {
		return nil, nil, err
	}
	return service, cfg, nil
}

func NewDraftAssistantWithConfig(cfg *config.Config) (ports.DraftAssistant, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}
	if len(cfg.AI.Providers) == 0 || len(cfg.AI.Templates) == 0 {
		return nil, errors.New("ai providers and templates must be configured")
	}

	httpClient := &http.Client{Timeout: 60 * time.Second}
	providers := make(map[string]ports.PromptCompletionProvider, len(cfg.AI.Providers))
	for name, providerCfg := range cfg.AI.Providers {
		switch strings.ToLower(strings.TrimSpace(providerCfg.Type)) {
		case "openai":
			providers[name] = openai.NewProvider(providerCfg, httpClient)
		case "gemini":
			providers[name] = gemini.NewProvider(providerCfg, httpClient)
		default:
			return nil, errors.Errorf("unsupported ai provider type %q for provider %q", providerCfg.Type, name)
		}
	}

	return assistant.NewService(cfg.AI, providers), nil
}

func DefaultSender(cfg *config.Config) (string, string) {
	account, ok := ResolveAccount(cfg, "")
	if !ok {
		return "", ""
	}
	return account.Name, account.Email
}

func ResolveAccount(cfg *config.Config, selector string) (config.AccountConfig, bool) {
	if cfg == nil || len(cfg.Accounts) == 0 {
		return config.AccountConfig{}, false
	}
	if strings.TrimSpace(selector) == "" {
		return cfg.Accounts[0], true
	}
	for _, account := range cfg.Accounts {
		if strings.EqualFold(account.Name, selector) || strings.EqualFold(account.Email, selector) {
			return account, true
		}
	}
	return config.AccountConfig{}, false
}

func smtpFactory(cfg *config.Config) func(accountID string) (ports.SMTPRepository, error) {
	return func(accountID string) (ports.SMTPRepository, error) {
		account, ok := ResolveAccount(cfg, accountID)
		if !ok {
			return nil, nil
		}

		repo := smtp.NewRepository()
		username, password := account.SMTPCredentials()
		if password == "" {
			return nil, coreerrors.PasswordNotConfigured(account.Name)
		}
		if err := repo.Connect(
			context.TODO(),
			account.SMTP.Host,
			account.SMTP.Port,
			username,
			password,
			account.SMTP.AuthType,
			account.SMTP.TLS,
		); err != nil {
			return nil, err
		}
		return repo, nil
	}
}
