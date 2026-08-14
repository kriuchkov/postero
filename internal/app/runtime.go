package app

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-faster/errors"

	"github.com/kriuchkov/postero/internal/adapters/ai/aiutil"
	"github.com/kriuchkov/postero/internal/adapters/ai/anthropic"
	"github.com/kriuchkov/postero/internal/adapters/ai/command"
	"github.com/kriuchkov/postero/internal/adapters/ai/gemini"
	"github.com/kriuchkov/postero/internal/adapters/ai/openai"
	"github.com/kriuchkov/postero/internal/adapters/mail/smtp"
	filestore "github.com/kriuchkov/postero/internal/adapters/storage/file"
	"github.com/kriuchkov/postero/internal/adapters/storage/sqlite"
	"github.com/kriuchkov/postero/internal/config"
	coreerrors "github.com/kriuchkov/postero/internal/core/errors"
	"github.com/kriuchkov/postero/internal/core/models"
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
	service, _, cfg, err := NewMessageServiceWithStore()
	return service, cfg, err
}

// NewMessageServiceWithStore also exposes the backing repository so callers
// (e.g. the TUI sync flow) can persist messages through the same store.
func NewMessageServiceWithStore() (ports.MessageService, ports.MessageRepository, *config.Config, error) {
	repo, cfg, err := NewMessageRepository()
	if err != nil {
		return nil, nil, nil, err
	}

	return message.NewServiceWithSMTP(repo, smtpFactory(cfg)), repo, cfg, nil
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
		opts := aiProviderOptions(providerCfg)
		switch strings.ToLower(strings.TrimSpace(providerCfg.Type)) {
		case "openai":
			providers[name] = openai.NewProvider(opts, httpClient)
		case "gemini":
			providers[name] = gemini.NewProvider(opts, httpClient)
		case "anthropic", "claude":
			providers[name] = anthropic.NewProvider(opts, httpClient)
		case "command", "openclaw":
			providers[name] = command.NewProvider(opts)
		default:
			return nil, errors.Errorf("unsupported ai provider type %q for provider %q", providerCfg.Type, name)
		}
	}

	return assistant.NewService(aiSettingsFromConfig(cfg.AI), providers), nil
}

// aiProviderOptions maps the infrastructure provider config into the adapter-owned
// transport options, keeping the provider adapters free of the config package.
func aiProviderOptions(cfg config.AIProviderConfig) aiutil.Options {
	return aiutil.Options{BaseURL: cfg.BaseURL, APIKey: cfg.APIKey(), Command: cfg.Command}
}

// aiSettingsFromConfig maps the infrastructure AI config into the provider-neutral
// domain settings the assistant service consumes, keeping the service free of any
// dependency on the config package.
func aiSettingsFromConfig(cfg config.AIConfig) models.AISettings {
	settings := models.AISettings{
		DefaultComposeTemplate: cfg.DefaultComposeTemplate,
		DefaultReplyTemplate:   cfg.DefaultReplyTemplate,
		Providers:              make(map[string]models.AIProviderSettings, len(cfg.Providers)),
		Templates:              make(map[string]models.AITemplate, len(cfg.Templates)),
	}
	for name, provider := range cfg.Providers {
		settings.Providers[name] = models.AIProviderSettings{Model: provider.Model}
	}
	for name, template := range cfg.Templates {
		settings.Templates[name] = models.AITemplate{
			Mode:         template.Mode,
			Provider:     template.Provider,
			SystemPrompt: template.SystemPrompt,
			Prompt:       template.Prompt,
			Temperature:  template.Temperature,
		}
	}
	return settings
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
