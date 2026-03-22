package config

import (
	"fmt"
	"strings"
)

type ValidationIssue struct {
	Severity string
	Path     string
	Message  string
	Hint     string
}

func (issue ValidationIssue) IsError() bool {
	return strings.EqualFold(issue.Severity, "error")
}

func ValidateConfig(cfg *Config) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	if cfg == nil {
		return append(issues, ValidationIssue{
			Severity: "error",
			Path:     "config",
			Message:  "configuration is nil",
			Hint:     "Create a config file with pstr config init <provider>.",
		})
	}

	backend := StorageBackend(cfg)
	if backend != "sqlite" && backend != "file" {
		issues = append(issues, ValidationIssue{
			Severity: "error",
			Path:     "storage.backend",
			Message:  fmt.Sprintf("unsupported storage backend %q", cfg.Storage.Backend),
			Hint:     "Use sqlite or file.",
		})
	}

	if len(cfg.Accounts) == 0 {
		issues = append(issues, ValidationIssue{
			Severity: "error",
			Path:     "accounts",
			Message:  "no accounts configured",
			Hint:     "Add at least one account or generate a starter config with pstr config init gmail.",
		})
		return issues
	}

	for index := range cfg.Accounts {
		account := cfg.Accounts[index]
		prefix := fmt.Sprintf("accounts[%d]", index)

		if strings.TrimSpace(account.Name) == "" {
			issues = append(issues, ValidationIssue{
				Severity: "error",
				Path:     prefix + ".name",
				Message:  "account name is required",
				Hint:     "Set a short unique name such as gmail or work.",
			})
		}
		if strings.TrimSpace(account.Email) == "" {
			issues = append(issues, ValidationIssue{
				Severity: "error",
				Path:     prefix + ".email",
				Message:  "account email is required",
				Hint:     "Set the full sender address for this account.",
			})
		}
		if strings.TrimSpace(account.IMAP.Host) == "" {
			issues = append(issues, ValidationIssue{
				Severity: "error",
				Path:     prefix + ".imap.host",
				Message:  "IMAP host is missing",
				Hint:     "Set provider to a supported preset or fill imap.host manually.",
			})
		}
		if account.IMAP.Port == 0 {
			issues = append(issues, ValidationIssue{
				Severity: "error",
				Path:     prefix + ".imap.port",
				Message:  "IMAP port is missing",
				Hint:     "Set provider to a supported preset or fill imap.port manually.",
			})
		}
		if strings.TrimSpace(account.SMTP.Host) == "" {
			issues = append(issues, ValidationIssue{
				Severity: "error",
				Path:     prefix + ".smtp.host",
				Message:  "SMTP host is missing",
				Hint:     "Set provider to a supported preset or fill smtp.host manually.",
			})
		}
		if account.SMTP.Port == 0 {
			issues = append(issues, ValidationIssue{
				Severity: "error",
				Path:     prefix + ".smtp.port",
				Message:  "SMTP port is missing",
				Hint:     "Set provider to a supported preset or fill smtp.port manually.",
			})
		}

		if usesOAuth2(&account) {
			provider := account.OAuth2.Provider
			if provider == "" {
				provider = account.Provider
			}
			if !SupportsBuiltInOAuth2(provider) {
				issues = append(issues, ValidationIssue{
					Severity: "error",
					Path:     prefix + ".oauth2.provider",
					Message:  fmt.Sprintf("built-in OAuth2 is not supported for provider %q", provider),
					Hint:     "Use gmail/google or outlook/microsoft, or switch the account to app-password/password_cmd auth.",
				})
			}
			if strings.TrimSpace(account.OAuth2.ClientID) == "" {
				issues = append(issues, ValidationIssue{
					Severity: "error",
					Path:     prefix + ".oauth2.client_id",
					Message:  "OAuth2 client_id is missing",
					Hint:     "Create an OAuth app for the provider and set client_id.",
				})
			}
			if strings.TrimSpace(account.OAuth2.ClientSecret) == "" {
				issues = append(issues, ValidationIssue{
					Severity: "error",
					Path:     prefix + ".oauth2.client_secret",
					Message:  "OAuth2 client_secret is missing",
					Hint:     "Create an OAuth app for the provider and set client_secret.",
				})
			}
		} else if !hasConfigSecret(account) {
			issues = append(issues, ValidationIssue{
				Severity: "warning",
				Path:     prefix,
				Message:  "no password source is configured in the file",
				Hint:     "Use pstr auth set <account>, password_cmd, environment variables, or inline passwords.",
			})
		}
	}

	return issues
}

func hasConfigSecret(account AccountConfig) bool {
	return strings.TrimSpace(account.Password) != "" ||
		len(account.PasswordCmd) > 0 ||
		strings.TrimSpace(account.IMAP.Password) != "" ||
		len(account.IMAP.PasswordCmd) > 0 ||
		strings.TrimSpace(account.SMTP.Password) != "" ||
		len(account.SMTP.PasswordCmd) > 0
}
