package config

import (
	"fmt"
	"path/filepath"
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

		for _, entry := range []struct {
			path string
			cmd  []string
		}{
			{prefix + ".password_cmd", account.PasswordCmd},
			{prefix + ".imap.password_cmd", account.IMAP.PasswordCmd},
			{prefix + ".smtp.password_cmd", account.SMTP.PasswordCmd},
			{prefix + ".oauth2.client_secret_cmd", account.OAuth2.ClientSecretCmd},
		} {
			if insecureLiteralCmd(entry.cmd) {
				issues = append(issues, insecureCmdIssue(entry.path))
			}
		}

		if usesOAuth2(&account) {
			issues = append(issues, validateOAuthAccount(account, prefix)...)
		} else if !hasConfigSecret(account) {
			issues = append(issues, ValidationIssue{
				Severity: "warning",
				Path:     prefix,
				Message:  "no password source is configured in the file",
				Hint:     "Use pstr auth set <account> or a password_cmd (inline plaintext passwords and env vars are no longer supported).",
			})
		}
	}

	for name, provider := range cfg.AI.Providers {
		if insecureLiteralCmd(provider.APIKeyCmd) {
			issues = append(issues, insecureCmdIssue(fmt.Sprintf("ai.providers.%s.api_key_cmd", name)))
		}
	}

	return issues
}

func validateOAuthAccount(account AccountConfig, prefix string) []ValidationIssue {
	issues := make([]ValidationIssue, 0, 3)
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
	if strings.TrimSpace(account.OAuth2.ClientSecret()) == "" && len(account.OAuth2.ClientSecretCmd) == 0 {
		issues = append(issues, ValidationIssue{
			Severity: "error",
			Path:     prefix + ".oauth2.client_secret",
			Message:  "OAuth2 client_secret is not available",
			Hint:     "Store it with `pstr auth set-secret <account>` or set client_secret_cmd.",
		})
	}
	return issues
}

// insecureLiteralCmd reports whether a *_cmd merely echoes/prints its arguments,
// which would expose a hardcoded literal secret in the process list (ps). A real
// secret command reads from a store and takes no secret on its command line.
func insecureLiteralCmd(cmd []string) bool {
	if len(cmd) < 2 {
		return false
	}
	switch strings.ToLower(filepath.Base(cmd[0])) {
	case "echo", "print", "printf":
		return true
	default:
		return false
	}
}

func insecureCmdIssue(path string) ValidationIssue {
	return ValidationIssue{
		Severity: "error",
		Path:     path,
		Message:  "command would expose a literal secret in the process list (ps)",
		Hint:     "Fetch the secret from a store (pass, security, secret-tool, gpg) or the OS keychain — never echo/print a literal.",
	}
}

func hasConfigSecret(account AccountConfig) bool {
	// Inline plaintext passwords are no longer read, so password_cmd is the only
	// in-file secret source (keychain and env vars live outside the config file).
	return len(account.PasswordCmd) > 0 ||
		len(account.IMAP.PasswordCmd) > 0 ||
		len(account.SMTP.PasswordCmd) > 0
}
