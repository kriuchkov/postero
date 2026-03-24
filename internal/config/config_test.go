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
	configDir := t.TempDir()
	t.Setenv("POSTERO_CONFIG_DIR", configDir)

	cfg, err := LoadConfig()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Verify defaults are set
	assert.Equal(t, "dark", cfg.Theme.Name)
	assert.Equal(t, filepath.Join(configDir, "config.yaml"), UsedConfigFile())
	assert.FileExists(t, filepath.Join(configDir, "config.yaml"))
}

func TestLoadConfigCreatesDefaultConfigFile(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	configDir := t.TempDir()
	t.Setenv("POSTERO_CONFIG_DIR", configDir)

	_, err := LoadConfig()
	require.NoError(t, err)

	configFile := filepath.Join(configDir, "config.yaml")
	content, readErr := os.ReadFile(configFile)
	require.NoError(t, readErr)

	assert.Contains(t, string(content), "backend: sqlite")
	assert.Contains(t, string(content), "name: dark")
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

func TestCanonicalProviderName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "gmail alias", input: "google", expected: "gmail"},
		{name: "outlook alias", input: "m365", expected: "outlook"},
		{name: "trim and lower", input: "  Yahoo ", expected: "yahoo"},
		{name: "unknown", input: "custom", expected: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, canonicalProviderName(test.input))
		})
	}
}

func TestInferredProviderFromEmail(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "gmail", input: "person@gmail.com", expected: "gmail"},
		{name: "outlook", input: "person@live.com", expected: "outlook"},
		{name: "icloud", input: "person@me.com", expected: "icloud"},
		{name: "invalid", input: "person-at-example.com", expected: ""},
		{name: "unknown domain", input: "person@example.com", expected: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, inferredProviderFromEmail(test.input))
		})
	}
}

func TestSupportsBuiltInOAuth2(t *testing.T) {
	assert.True(t, SupportsBuiltInOAuth2("gmail"))
	assert.True(t, SupportsBuiltInOAuth2("microsoft"))
	assert.False(t, SupportsBuiltInOAuth2("icloud"))
	assert.False(t, SupportsBuiltInOAuth2("custom"))
}

func TestPasswordCommandsForProtocol(t *testing.T) {
	account := &AccountConfig{
		PasswordCmd: []string{"shared-cmd"},
		IMAP: IMAPConfig{
			PasswordCmd: []string{"imap-cmd"},
		},
		SMTP: SMTPConfig{
			PasswordCmd: []string{"smtp-cmd"},
		},
	}

	assert.Equal(t, [][]string{{"imap-cmd"}, {"shared-cmd"}}, passwordCommandsForProtocol(account, "IMAP"))
	assert.Equal(t, [][]string{{"smtp-cmd"}, {"shared-cmd"}}, passwordCommandsForProtocol(account, "SMTP"))
	assert.Equal(t, [][]string{{"shared-cmd"}}, passwordCommandsForProtocol(account, ""))
}

func TestPasswordEnvKeys(t *testing.T) {
	assert.Equal(t,
		[]string{"POSTERO_WORK_IMAP_PASSWORD", "POSTERO_IMAP_PASSWORD", "POSTERO_WORK_PASSWORD"},
		passwordEnvKeys("WORK", "IMAP"),
	)
	assert.Equal(t,
		[]string{"POSTERO_SMTP_PASSWORD"},
		passwordEnvKeys("", "SMTP"),
	)
	assert.Equal(t,
		[]string{"POSTERO_WORK_PASSWORD"},
		passwordEnvKeys("WORK", ""),
	)
}
