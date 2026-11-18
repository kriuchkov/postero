package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestYandexProviderPreset(t *testing.T) {
	assert.Equal(t, providerYandex, ProviderForEmail("me@yandex.ru"))
	assert.Equal(t, providerYandex, ProviderForEmail("me@ya.ru"))
	assert.Equal(t, providerYandex, NormalizeProviderName("yandex"))

	cfg := UpsertAccount(nil, AccountConfig{Name: "ya", Email: "me@yandex.com", Provider: "yandex"})
	account := cfg.Accounts[0]
	assert.Equal(t, "imap.yandex.com", account.IMAP.Host)
	assert.Equal(t, 993, account.IMAP.Port)
	assert.True(t, account.IMAP.TLS)
	assert.Equal(t, "smtp.yandex.com", account.SMTP.Host)
	assert.Equal(t, 465, account.SMTP.Port)
	assert.True(t, account.SMTP.TLS)
}

func TestValidateConfigRejectsLiteralSecretCmd(t *testing.T) {
	cfg := &Config{
		Storage: StorageConfig{Backend: "sqlite"},
		Accounts: []AccountConfig{{
			Name:        "work",
			Email:       "work@example.com",
			IMAP:        IMAPConfig{Host: "imap.example.com", Port: 993},
			SMTP:        SMTPConfig{Host: "smtp.example.com", Port: 587},
			PasswordCmd: []string{"echo", "hunter2"},
		}},
	}

	var found bool
	for _, issue := range ValidateConfig(cfg) {
		if issue.Path == "accounts[0].password_cmd" && issue.IsError() {
			found = true
			assert.Contains(t, issue.Message, "process list")
		}
	}
	assert.True(t, found, "an echo/printf password_cmd must be a validation error")
}

func TestInsecureLiteralCmd(t *testing.T) {
	t.Parallel()
	assert.True(t, insecureLiteralCmd([]string{"echo", "secret"}))
	assert.True(t, insecureLiteralCmd([]string{"/bin/printf", "secret"}))
	assert.True(t, insecureLiteralCmd([]string{"print", "secret"}))
	assert.False(t, insecureLiteralCmd([]string{"pass", "show", "email/work"}))
	assert.False(t, insecureLiteralCmd([]string{"cat", "/run/secrets/x"}))
	assert.False(t, insecureLiteralCmd([]string{"echo"}), "no literal argument")
	assert.False(t, insecureLiteralCmd(nil))
}

func TestAutoCreatedConfigIsOwnerOnly(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("POSTERO_CONFIG_DIR", dir)

	_, err := LoadConfig()
	require.NoError(t, err)

	info, err := os.Stat(filepath.Join(dir, "config.yaml"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "auto-created config must not be world-readable")
}
