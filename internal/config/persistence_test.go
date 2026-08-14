package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"
	"gopkg.in/yaml.v3"
)

// ── SaveConfig ────────────────────────────────────────────────────────────────

func TestSaveConfigNilReturnsError(t *testing.T) {
	err := SaveConfig(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestSaveConfigWritesYAML(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("POSTERO_CONFIG_DIR", dir)

	cfg := &Config{Storage: StorageConfig{Backend: "sqlite"}}
	require.NoError(t, SaveConfig(cfg))

	data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	require.NoError(t, err)

	var roundTrip Config
	require.NoError(t, yaml.Unmarshal(data, &roundTrip))
	assert.Equal(t, "sqlite", roundTrip.Storage.Backend)
}

func TestSaveConfigCreatesDirectory(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "nested", "config")
	t.Setenv("POSTERO_CONFIG_DIR", dir)

	require.NoError(t, SaveConfig(&Config{}))
	assert.FileExists(t, filepath.Join(dir, "config.yaml"))
}

// ── sameAccount ───────────────────────────────────────────────────────────────

func TestSameAccountByName(t *testing.T) {
	a := AccountConfig{Name: "work", Email: "w@example.com"}
	b := AccountConfig{Name: "WORK", Email: "other@example.com"}
	assert.True(t, sameAccount(a, b))
}

func TestSameAccountByEmail(t *testing.T) {
	a := AccountConfig{Name: "", Email: "alice@example.com"}
	b := AccountConfig{Name: "alice", Email: "ALICE@EXAMPLE.COM"}
	assert.True(t, sameAccount(a, b))
}

func TestSameAccountDifferent(t *testing.T) {
	a := AccountConfig{Name: "work", Email: "w@example.com"}
	b := AccountConfig{Name: "home", Email: "h@example.com"}
	assert.False(t, sameAccount(a, b))
}

func TestSameAccountEmptyNameMatchesByEmail(t *testing.T) {
	a := AccountConfig{Name: "  ", Email: "shared@example.com"}
	b := AccountConfig{Name: "  ", Email: "shared@example.com"}
	assert.True(t, sameAccount(a, b))
}

// ── mergeAccount ──────────────────────────────────────────────────────────────

func TestMergeAccountOverwritesNonEmptyFields(t *testing.T) {
	existing := AccountConfig{
		Name:        "work",
		Email:       "old@example.com",
		Provider:    "gmail",
		PasswordCmd: []string{"old-cmd"},
	}
	update := AccountConfig{
		Name:  "work",
		Email: "new@example.com",
	}
	merged := mergeAccount(existing, update)
	assert.Equal(t, "new@example.com", merged.Email)
	assert.Equal(t, []string{"old-cmd"}, merged.PasswordCmd) // empty update keeps existing
	assert.Equal(t, "gmail", merged.Provider)                // not in update → preserved
}

func TestMergeAccountPasswordCmdReplaces(t *testing.T) {
	existing := AccountConfig{PasswordCmd: []string{"old-cmd"}}
	update := AccountConfig{PasswordCmd: []string{"new-cmd", "arg"}}
	merged := mergeAccount(existing, update)
	assert.Equal(t, []string{"new-cmd", "arg"}, merged.PasswordCmd)
}

func TestMergeAccountPasswordCmdNotReplacedWhenEmpty(t *testing.T) {
	existing := AccountConfig{PasswordCmd: []string{"keep-me"}}
	update := AccountConfig{PasswordCmd: nil}
	merged := mergeAccount(existing, update)
	assert.Equal(t, []string{"keep-me"}, merged.PasswordCmd)
}

// ── mergeIMAP ─────────────────────────────────────────────────────────────────

func TestMergeIMAPOverwritesNonEmptyFields(t *testing.T) {
	existing := IMAPConfig{Host: "imap.old.com", Port: 993, Username: "old"}
	update := IMAPConfig{Host: "imap.new.com", Username: ""}
	merged := mergeIMAP(existing, update)
	assert.Equal(t, "imap.new.com", merged.Host)
	assert.Equal(t, "old", merged.Username) // empty update → keeps existing
	assert.Equal(t, 993, merged.Port)
}

func TestMergeIMAPPort(t *testing.T) {
	existing := IMAPConfig{Port: 143}
	update := IMAPConfig{Port: 993}
	merged := mergeIMAP(existing, update)
	assert.Equal(t, 993, merged.Port)
}

func TestMergeIMAPPortNotOverriddenByZero(t *testing.T) {
	existing := IMAPConfig{Port: 993}
	update := IMAPConfig{Port: 0}
	merged := mergeIMAP(existing, update)
	assert.Equal(t, 993, merged.Port)
}

func TestMergeIMAPTLSOnceSet(t *testing.T) {
	existing := IMAPConfig{TLS: false}
	update := IMAPConfig{TLS: true}
	merged := mergeIMAP(existing, update)
	assert.True(t, merged.TLS)
}

func TestMergeIMAPPasswordCmd(t *testing.T) {
	existing := IMAPConfig{PasswordCmd: []string{"old"}}
	update := IMAPConfig{PasswordCmd: []string{"new"}}
	merged := mergeIMAP(existing, update)
	assert.Equal(t, []string{"new"}, merged.PasswordCmd)
}

// ── mergeSMTP ─────────────────────────────────────────────────────────────────

func TestMergeSMTPOverwritesNonEmptyFields(t *testing.T) {
	existing := SMTPConfig{Host: "smtp.old.com", Port: 25}
	update := SMTPConfig{Host: "smtp.new.com", Port: 587}
	merged := mergeSMTP(existing, update)
	assert.Equal(t, "smtp.new.com", merged.Host)
	assert.Equal(t, 587, merged.Port)
}

func TestMergeSMTPPortNotOverriddenByZero(t *testing.T) {
	existing := SMTPConfig{Port: 465}
	update := SMTPConfig{Port: 0}
	merged := mergeSMTP(existing, update)
	assert.Equal(t, 465, merged.Port)
}

func TestMergeSMTPPasswordCmd(t *testing.T) {
	existing := SMTPConfig{PasswordCmd: []string{"old"}}
	update := SMTPConfig{PasswordCmd: []string{"gpg", "--decrypt"}}
	merged := mergeSMTP(existing, update)
	assert.Equal(t, []string{"gpg", "--decrypt"}, merged.PasswordCmd)
}

// ── mergeOAuth2 ───────────────────────────────────────────────────────────────

func TestMergeOAuth2OverwritesNonEmptyFields(t *testing.T) {
	existing := OAuth2Config{Provider: "google", ClientID: "old-id", ClientSecretCmd: []string{"echo", "old"}}
	update := OAuth2Config{ClientID: "new-id"}
	merged := mergeOAuth2(existing, update)
	assert.Equal(t, "new-id", merged.ClientID)
	assert.Equal(t, []string{"echo", "old"}, merged.ClientSecretCmd)
	assert.Equal(t, "google", merged.Provider)
}

func TestMergeOAuth2ScopesReplaced(t *testing.T) {
	existing := OAuth2Config{Scopes: []string{"email"}}
	update := OAuth2Config{Scopes: []string{"email", "profile"}}
	merged := mergeOAuth2(existing, update)
	assert.Equal(t, []string{"email", "profile"}, merged.Scopes)
}

func TestMergeOAuth2ScopesNotOverriddenWhenEmpty(t *testing.T) {
	existing := OAuth2Config{Scopes: []string{"email"}}
	update := OAuth2Config{Scopes: nil}
	merged := mergeOAuth2(existing, update)
	assert.Equal(t, []string{"email"}, merged.Scopes)
}

// ── UpsertAccount ─────────────────────────────────────────────────────────────

func TestUpsertAccountNilConfigCreatesNew(t *testing.T) {
	result := UpsertAccount(nil, AccountConfig{Name: "work", Email: "w@example.com"})
	require.Len(t, result.Accounts, 1)
	assert.Equal(t, "work", result.Accounts[0].Name)
}

func TestUpsertAccountAddsNewAccount(t *testing.T) {
	cfg := &Config{Accounts: []AccountConfig{{Name: "existing", Email: "e@example.com"}}}
	result := UpsertAccount(cfg, AccountConfig{Name: "new", Email: "n@example.com"})
	assert.Len(t, result.Accounts, 2)
}

func TestUpsertAccountMergesExistingByName(t *testing.T) {
	cfg := &Config{Accounts: []AccountConfig{{Name: "work", Email: "old@example.com", PasswordCmd: []string{"old-cmd"}}}}
	result := UpsertAccount(cfg, AccountConfig{Name: "WORK", Email: "new@example.com"})
	require.Len(t, result.Accounts, 1)
	assert.Equal(t, "new@example.com", result.Accounts[0].Email)
	assert.Equal(t, []string{"old-cmd"}, result.Accounts[0].PasswordCmd)
}

func TestUpsertAccountMergesExistingByEmail(t *testing.T) {
	cfg := &Config{Accounts: []AccountConfig{{Name: "", Email: "alice@example.com", PasswordCmd: []string{"pw-cmd"}}}}
	result := UpsertAccount(cfg, AccountConfig{Name: "alice", Email: "ALICE@EXAMPLE.COM"})
	require.Len(t, result.Accounts, 1)
	assert.Equal(t, "alice", result.Accounts[0].Name)
	assert.Equal(t, []string{"pw-cmd"}, result.Accounts[0].PasswordCmd)
}

func TestSaveConfigDoesNotPersistResolvedSecrets(t *testing.T) {
	t.Setenv("POSTERO_CONFIG_DIR", t.TempDir())
	keyring.MockInit()
	require.NoError(t, keyring.Set(passwordKeyringService, "work", "keychain-secret"))

	account := AccountConfig{
		Name:  "work",
		Email: "you@example.com",
		IMAP:  IMAPConfig{Host: "imap.example.com", Port: 993},
		SMTP:  SMTPConfig{Host: "smtp.example.com", Port: 587},
	}
	cfg := UpsertAccount(nil, account)

	// The runtime must see the resolved password from the keychain...
	_, password := cfg.Accounts[0].IMAPCredentials()
	require.Equal(t, "keychain-secret", password)

	// ...but the file on disk must never contain it.
	require.NoError(t, SaveConfig(cfg))
	path, err := ConfigFilePath()
	require.NoError(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "keychain-secret", "resolved secrets must not leak into config.yaml")
}

func TestInlinePasswordIsNeverPersistedOrUsed(t *testing.T) {
	t.Setenv("POSTERO_CONFIG_DIR", t.TempDir())

	cfg := UpsertAccount(nil, AccountConfig{
		Name:           "plain",
		Email:          "p@example.com",
		LegacyPassword: "inline-secret",
		IMAP:           IMAPConfig{Host: "h", Port: 1},
		SMTP:           SMTPConfig{Host: "h", Port: 2},
	})

	// A stray inline password is never used for authentication...
	_, password := cfg.Accounts[0].IMAPCredentials()
	assert.Empty(t, password, "inline plaintext passwords must not be used")

	// ...and can never be written back to disk.
	require.NoError(t, SaveConfig(cfg))
	path, err := ConfigFilePath()
	require.NoError(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "inline-secret", "plaintext passwords must never reach config.yaml")
}

func TestLoadConfigIgnoresLegacyInlinePassword(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("POSTERO_CONFIG_DIR", dir)
	content := []byte("accounts:\n" +
		"  - name: legacy\n" +
		"    email: legacy@example.com\n" +
		"    password: on-disk-secret\n" +
		"    imap: { host: h, port: 1 }\n" +
		"    smtp: { host: h, port: 2 }\n")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), content, 0o600))

	cfg, err := LoadConfig()
	require.NoError(t, err)
	require.Len(t, cfg.Accounts, 1)

	_, password := cfg.Accounts[0].IMAPCredentials()
	assert.Empty(t, password, "a plaintext password in the file must be ignored")
	assert.Empty(t, cfg.Accounts[0].LegacyPassword, "the ignored value must be cleared from memory")
}

func TestGmailAppPasswordAccountStaysPlainAuth(t *testing.T) {
	t.Setenv("POSTERO_CONFIG_DIR", t.TempDir())

	cfg := UpsertAccount(nil, AccountConfig{Name: "alice", Email: "alice@gmail.com", PasswordCmd: []string{"echo", "app-pw"}})
	account := cfg.Accounts[0]

	assert.Equal(t, "imap.gmail.com", account.IMAP.Host, "network preset still applies")
	assert.NotEqual(t, "oauth2", account.IMAP.AuthType, "app-password account must not be forced onto oauth2")
	assert.NotEqual(t, "oauth2", account.SMTP.AuthType)
	assert.Empty(t, account.OAuth2.Provider, "oauth provider must not be backfilled without credentials")
}

func TestGmailOAuthAccountKeepsOAuthDefaults(t *testing.T) {
	t.Setenv("POSTERO_CONFIG_DIR", t.TempDir())

	cfg := UpsertAccount(nil, AccountConfig{
		Name:   "alice",
		Email:  "alice@gmail.com",
		OAuth2: OAuth2Config{ClientID: "id"},
	})
	account := cfg.Accounts[0]

	assert.Equal(t, "oauth2", account.IMAP.AuthType, "explicit oauth credentials keep the oauth2 flow")
	assert.Equal(t, "google", account.OAuth2.Provider)
	assert.NotEmpty(t, account.OAuth2.Scopes)
}
