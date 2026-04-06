package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/go-faster/errors"
	"gopkg.in/yaml.v3"
)

//nolint:revive // exported name is kept for package-level clarity.
func ConfigFilePath() (string, error) {
	configDir := os.Getenv("POSTERO_CONFIG_DIR")
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configDir = filepath.Join(home, ".config", "postero")
	}
	return filepath.Join(configDir, "config.yaml"), nil
}

func SaveConfig(cfg *Config) error {
	if cfg == nil {
		return errors.New("config is nil")
	}
	path, err := ConfigFilePath()
	if err != nil {
		return errors.Wrap(err, "resolve config path")
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return errors.Wrap(err, "marshal config")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return errors.Wrap(err, "create config directory")
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return errors.Wrap(err, "write config file")
	}
	return nil
}

func UpsertAccount(cfg *Config, update AccountConfig) *Config {
	if cfg == nil {
		cfg = &Config{}
	}
	merged := update
	for index := range cfg.Accounts {
		candidate := cfg.Accounts[index]
		if sameAccount(candidate, update) {
			merged = mergeAccount(candidate, update)
			cfg.Accounts[index] = merged
			applyAccountDefaults(&cfg.Accounts[index])
			return cfg
		}
	}
	applyAccountDefaults(&merged)
	cfg.Accounts = append(cfg.Accounts, merged)
	return cfg
}

func sameAccount(left, right AccountConfig) bool {
	if strings.TrimSpace(left.Name) != "" && strings.EqualFold(left.Name, right.Name) {
		return true
	}
	if strings.TrimSpace(left.Email) != "" && strings.EqualFold(left.Email, right.Email) {
		return true
	}
	return false
}

func mergeAccount(existing, update AccountConfig) AccountConfig {
	merged := existing
	mergeString := func(current, next string) string {
		if strings.TrimSpace(next) != "" {
			return next
		}
		return current
	}

	merged.Name = mergeString(merged.Name, update.Name)
	merged.Provider = mergeString(merged.Provider, update.Provider)
	merged.Email = mergeString(merged.Email, update.Email)
	merged.Username = mergeString(merged.Username, update.Username)
	merged.Password = mergeString(merged.Password, update.Password)
	if len(update.PasswordCmd) > 0 {
		merged.PasswordCmd = append([]string{}, update.PasswordCmd...)
	}

	merged.IMAP = mergeIMAP(merged.IMAP, update.IMAP)
	merged.SMTP = mergeSMTP(merged.SMTP, update.SMTP)
	merged.OAuth2 = mergeOAuth2(merged.OAuth2, update.OAuth2)
	return merged
}

func mergeIMAP(existing, update IMAPConfig) IMAPConfig {
	merged := existing
	if strings.TrimSpace(update.Username) != "" {
		merged.Username = update.Username
	}
	if strings.TrimSpace(update.Password) != "" {
		merged.Password = update.Password
	}
	if len(update.PasswordCmd) > 0 {
		merged.PasswordCmd = append([]string{}, update.PasswordCmd...)
	}
	if strings.TrimSpace(update.AuthType) != "" {
		merged.AuthType = update.AuthType
	}
	if strings.TrimSpace(update.Host) != "" {
		merged.Host = update.Host
	}
	if update.Port != 0 {
		merged.Port = update.Port
	}
	if update.TLS {
		merged.TLS = true
	}
	return merged
}

func mergeSMTP(existing, update SMTPConfig) SMTPConfig {
	merged := existing
	if strings.TrimSpace(update.Username) != "" {
		merged.Username = update.Username
	}
	if strings.TrimSpace(update.Password) != "" {
		merged.Password = update.Password
	}
	if len(update.PasswordCmd) > 0 {
		merged.PasswordCmd = append([]string{}, update.PasswordCmd...)
	}
	if strings.TrimSpace(update.AuthType) != "" {
		merged.AuthType = update.AuthType
	}
	if strings.TrimSpace(update.Host) != "" {
		merged.Host = update.Host
	}
	if update.Port != 0 {
		merged.Port = update.Port
	}
	if update.TLS {
		merged.TLS = true
	}
	return merged
}

func mergeOAuth2(existing, update OAuth2Config) OAuth2Config {
	merged := existing
	if strings.TrimSpace(update.Provider) != "" {
		merged.Provider = update.Provider
	}
	if strings.TrimSpace(update.ClientID) != "" {
		merged.ClientID = update.ClientID
	}
	if strings.TrimSpace(update.ClientSecret) != "" {
		merged.ClientSecret = update.ClientSecret
	}
	if strings.TrimSpace(update.TenantID) != "" {
		merged.TenantID = update.TenantID
	}
	if len(update.Scopes) > 0 {
		merged.Scopes = append([]string{}, update.Scopes...)
	}
	if strings.TrimSpace(update.RedirectURL) != "" {
		merged.RedirectURL = update.RedirectURL
	}
	return merged
}
