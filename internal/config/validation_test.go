package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validAccount returns a minimal account that passes all validation checks.
func validAccount() AccountConfig {
	return AccountConfig{
		Name:        "alice",
		Email:       "alice@example.com",
		PasswordCmd: []string{"pass", "alice"},
		IMAP:        IMAPConfig{Host: "imap.example.com", Port: 993},
		SMTP:        SMTPConfig{Host: "smtp.example.com", Port: 587},
	}
}

func TestValidationIssueIsError(t *testing.T) {
	assert.True(t, ValidationIssue{Severity: "error"}.IsError())
	assert.True(t, ValidationIssue{Severity: "ERROR"}.IsError())
	assert.False(t, ValidationIssue{Severity: "warning"}.IsError())
	assert.False(t, ValidationIssue{Severity: ""}.IsError())
}

func TestValidateConfigNil(t *testing.T) {
	issues := ValidateConfig(nil)
	require.Len(t, issues, 1)
	assert.True(t, issues[0].IsError())
	assert.Equal(t, "config", issues[0].Path)
}

func TestValidateConfigNoAccounts(t *testing.T) {
	cfg := &Config{Storage: StorageConfig{Backend: "sqlite"}}
	issues := ValidateConfig(cfg)
	require.Len(t, issues, 1)
	assert.True(t, issues[0].IsError())
	assert.Equal(t, "accounts", issues[0].Path)
}

func TestValidateConfigUnsupportedBackend(t *testing.T) {
	cfg := &Config{
		Storage:  StorageConfig{Backend: "redis"},
		Accounts: []AccountConfig{validAccount()},
	}
	issues := ValidateConfig(cfg)
	errors := 0
	for _, i := range issues {
		if i.Path == "storage.backend" {
			errors++
		}
	}
	assert.Equal(t, 1, errors)
}

func TestValidateConfigValidAccountPasses(t *testing.T) {
	cfg := &Config{
		Storage:  StorageConfig{Backend: "sqlite"},
		Accounts: []AccountConfig{validAccount()},
	}
	issues := ValidateConfig(cfg)
	// Only the no-password-in-file warning may appear (account has Password set → no warning)
	for _, i := range issues {
		assert.False(t, i.IsError(), "unexpected error: %s — %s", i.Path, i.Message)
	}
}

func TestValidateConfigMissingAccountName(t *testing.T) {
	acc := validAccount()
	acc.Name = ""
	cfg := &Config{Storage: StorageConfig{Backend: "sqlite"}, Accounts: []AccountConfig{acc}}
	issues := ValidateConfig(cfg)
	paths := issuePaths(issues)
	assert.Contains(t, paths, "accounts[0].name")
}

func TestValidateConfigMissingEmail(t *testing.T) {
	acc := validAccount()
	acc.Email = ""
	cfg := &Config{Storage: StorageConfig{Backend: "sqlite"}, Accounts: []AccountConfig{acc}}
	issues := ValidateConfig(cfg)
	paths := issuePaths(issues)
	assert.Contains(t, paths, "accounts[0].email")
}

func TestValidateConfigMissingIMAPHost(t *testing.T) {
	acc := validAccount()
	acc.IMAP.Host = ""
	cfg := &Config{Storage: StorageConfig{Backend: "sqlite"}, Accounts: []AccountConfig{acc}}
	paths := issuePaths(ValidateConfig(cfg))
	assert.Contains(t, paths, "accounts[0].imap.host")
}

func TestValidateConfigMissingIMAPPort(t *testing.T) {
	acc := validAccount()
	acc.IMAP.Port = 0
	cfg := &Config{Storage: StorageConfig{Backend: "sqlite"}, Accounts: []AccountConfig{acc}}
	paths := issuePaths(ValidateConfig(cfg))
	assert.Contains(t, paths, "accounts[0].imap.port")
}

func TestValidateConfigMissingSMTPHost(t *testing.T) {
	acc := validAccount()
	acc.SMTP.Host = ""
	cfg := &Config{Storage: StorageConfig{Backend: "sqlite"}, Accounts: []AccountConfig{acc}}
	paths := issuePaths(ValidateConfig(cfg))
	assert.Contains(t, paths, "accounts[0].smtp.host")
}

func TestValidateConfigMissingSMTPPort(t *testing.T) {
	acc := validAccount()
	acc.SMTP.Port = 0
	cfg := &Config{Storage: StorageConfig{Backend: "sqlite"}, Accounts: []AccountConfig{acc}}
	paths := issuePaths(ValidateConfig(cfg))
	assert.Contains(t, paths, "accounts[0].smtp.port")
}

func TestValidateConfigNoPasswordWarning(t *testing.T) {
	acc := validAccount()
	acc.PasswordCmd = nil
	cfg := &Config{Storage: StorageConfig{Backend: "sqlite"}, Accounts: []AccountConfig{acc}}
	issues := ValidateConfig(cfg)
	// Should produce a warning (not error) about missing password source
	found := false
	for _, i := range issues {
		if i.Path == "accounts[0]" && !i.IsError() {
			found = true
		}
	}
	assert.True(t, found, "expected warning about missing password source")
}

func TestValidateConfigPasswordCmdSuppressesWarning(t *testing.T) {
	acc := validAccount()
	acc.PasswordCmd = []string{"pass", "email/alice"}
	cfg := &Config{Storage: StorageConfig{Backend: "sqlite"}, Accounts: []AccountConfig{acc}}
	issues := ValidateConfig(cfg)
	for _, i := range issues {
		assert.False(t, i.IsError(), "unexpected error: %s", i.Path)
		// no warning about missing password
		assert.NotEqual(t, "accounts[0]", i.Path)
	}
}

func TestValidateConfigOAuth2AccountMissingCredentials(t *testing.T) {
	acc := validAccount()
	acc.Email = "alice@gmail.com"
	acc.IMAP.AuthType = "oauth2"
	acc.SMTP.AuthType = "oauth2"
	// No client_id / client_secret set
	cfg := &Config{Storage: StorageConfig{Backend: "sqlite"}, Accounts: []AccountConfig{acc}}
	paths := issuePaths(ValidateConfig(cfg))
	assert.Contains(t, paths, "accounts[0].oauth2.client_id")
	assert.Contains(t, paths, "accounts[0].oauth2.client_secret")
}

func TestValidateConfigOAuth2UnsupportedProvider(t *testing.T) {
	acc := validAccount()
	acc.IMAP.AuthType = "oauth2"
	acc.SMTP.AuthType = "oauth2"
	acc.OAuth2 = OAuth2Config{
		Provider:        "unknown-provider",
		ClientID:        "id",
		ClientSecretCmd: []string{"echo", "secret"},
	}
	cfg := &Config{Storage: StorageConfig{Backend: "sqlite"}, Accounts: []AccountConfig{acc}}
	paths := issuePaths(ValidateConfig(cfg))
	assert.Contains(t, paths, "accounts[0].oauth2.provider")
}

func TestValidateConfigFileBackendIsValid(t *testing.T) {
	cfg := &Config{
		Storage:  StorageConfig{Backend: "file"},
		Accounts: []AccountConfig{validAccount()},
	}
	issues := ValidateConfig(cfg)
	for _, i := range issues {
		assert.NotEqual(t, "storage.backend", i.Path)
	}
}

func TestHasConfigSecretPasswordCmd(t *testing.T) {
	acc := AccountConfig{PasswordCmd: []string{"pass", "email"}}
	assert.True(t, hasConfigSecret(acc))
}

func TestHasConfigSecretIMAPPasswordCmd(t *testing.T) {
	acc := AccountConfig{IMAP: IMAPConfig{PasswordCmd: []string{"gpg"}}}
	assert.True(t, hasConfigSecret(acc))
}

func TestHasConfigSecretIgnoresInlinePassword(t *testing.T) {
	// A deprecated inline plaintext password is not a valid secret source.
	acc := AccountConfig{
		LegacyPassword: "s3cr3t",
		IMAP:           IMAPConfig{LegacyPassword: "imap-pass"},
		SMTP:           SMTPConfig{LegacyPassword: "smtp-pass"},
	}
	assert.False(t, hasConfigSecret(acc))
}

func TestHasConfigSecretNone(t *testing.T) {
	assert.False(t, hasConfigSecret(AccountConfig{}))
}

// issuePaths extracts all Path fields from issues.
func issuePaths(issues []ValidationIssue) []string {
	paths := make([]string, 0, len(issues))
	for _, i := range issues {
		paths = append(paths, i.Path)
	}
	return paths
}
