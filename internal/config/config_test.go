package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	t.Setenv("POSTERO_CONFIG_DIR", t.TempDir())

	cfg, err := LoadConfig()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	// Verify defaults are set
	assert.Equal(t, "dark", cfg.Theme.Name)
}

func TestLoadConfigAppliesProtocolCredentials(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	configDir := t.TempDir()
	t.Setenv("POSTERO_CONFIG_DIR", configDir)
	t.Setenv("POSTERO_WORK_IMAP_PASSWORD", "imap-secret")
	t.Setenv("POSTERO_WORK_SMTP_PASSWORD", "smtp-secret")

	configFile := filepath.Join(configDir, "config.yaml")
	content := []byte(`accounts:
  - name: "Work"
    email: "work@example.com"
    username: "mail-user"
    imap:
      host: "imap.example.com"
      port: 993
      tls: true
    smtp:
      host: "smtp.example.com"
      port: 587
      tls: true
`)
	require.NoError(t, os.WriteFile(configFile, content, 0o644))

	cfg, err := LoadConfig()
	require.NoError(t, err)
	require.Len(t, cfg.Accounts, 1)

	account := cfg.Accounts[0]
	imapUser, imapPass := account.IMAPCredentials()
	smtpUser, smtpPass := account.SMTPCredentials()

	assert.Equal(t, "mail-user", imapUser)
	assert.Equal(t, "imap-secret", imapPass)
	assert.Equal(t, "mail-user", smtpUser)
	assert.Equal(t, "smtp-secret", smtpPass)
}

func TestProtocolCredentialsFallbackToSharedPassword(t *testing.T) {
	account := AccountConfig{
		Name:     "Personal",
		Email:    "me@example.com",
		Username: "shared-user",
		Password: "shared-secret",
	}

	imapUser, imapPass := account.IMAPCredentials()
	smtpUser, smtpPass := account.SMTPCredentials()

	assert.Equal(t, "shared-user", imapUser)
	assert.Equal(t, "shared-secret", imapPass)
	assert.Equal(t, "shared-user", smtpUser)
	assert.Equal(t, "shared-secret", smtpPass)
}

func TestLoadConfigAppliesGmailProviderDefaults(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	configDir := t.TempDir()
	t.Setenv("POSTERO_CONFIG_DIR", configDir)

	configFile := filepath.Join(configDir, "config.yaml")
	content := []byte(`accounts:
  - name: "gmail"
    provider: "gmail"
    email: "your.name@gmail.com"
    oauth2:
      client_id: "client-id"
      client_secret: "client-secret"
`)
	require.NoError(t, os.WriteFile(configFile, content, 0o644))

	cfg, err := LoadConfig()
	require.NoError(t, err)
	require.Len(t, cfg.Accounts, 1)

	account := cfg.Accounts[0]
	assert.Equal(t, "imap.gmail.com", account.IMAP.Host)
	assert.Equal(t, 993, account.IMAP.Port)
	assert.True(t, account.IMAP.TLS)
	assert.Equal(t, "oauth2", account.IMAP.AuthType)
	assert.Equal(t, "smtp.gmail.com", account.SMTP.Host)
	assert.Equal(t, 587, account.SMTP.Port)
	assert.True(t, account.SMTP.TLS)
	assert.Equal(t, "oauth2", account.SMTP.AuthType)
	assert.Equal(t, "google", account.OAuth2.Provider)
	assert.Equal(t, []string{"https://mail.google.com/"}, account.OAuth2.Scopes)
	assert.Equal(t, "your.name@gmail.com", account.Username)
	assert.Equal(t, "your.name@gmail.com", account.IMAP.Username)
	assert.Equal(t, "your.name@gmail.com", account.SMTP.Username)
}

func TestLoadConfigInfersOutlookProviderDefaultsFromEmail(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	configDir := t.TempDir()
	t.Setenv("POSTERO_CONFIG_DIR", configDir)

	configFile := filepath.Join(configDir, "config.yaml")
	content := []byte(`accounts:
  - name: "outlook"
    email: "your.name@outlook.com"
    oauth2:
      client_id: "client-id"
      client_secret: "client-secret"
`)
	require.NoError(t, os.WriteFile(configFile, content, 0o644))

	cfg, err := LoadConfig()
	require.NoError(t, err)
	require.Len(t, cfg.Accounts, 1)

	account := cfg.Accounts[0]
	assert.Equal(t, "outlook.office365.com", account.IMAP.Host)
	assert.Equal(t, 993, account.IMAP.Port)
	assert.True(t, account.IMAP.TLS)
	assert.Equal(t, "oauth2", account.IMAP.AuthType)
	assert.Equal(t, "smtp.office365.com", account.SMTP.Host)
	assert.Equal(t, 587, account.SMTP.Port)
	assert.True(t, account.SMTP.TLS)
	assert.Equal(t, "oauth2", account.SMTP.AuthType)
	assert.Equal(t, "microsoft", account.OAuth2.Provider)
	assert.Equal(t, "common", account.OAuth2.TenantID)
	assert.Equal(t, []string{
		"https://outlook.office.com/IMAP.AccessAsUser.All",
		"https://outlook.office.com/SMTP.Send",
		"offline_access",
	}, account.OAuth2.Scopes)
}
