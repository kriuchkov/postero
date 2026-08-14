package config

import (
	"context"
	stderrors "errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"github.com/zalando/go-keyring"
)

const (
	authTypeOAuth2       = "oauth2"
	storageBackendSQLite = "sqlite"
	// Keychain service names for at-rest secrets (the OAuth *token* store lives
	// in oauth2.go as keyringService = "postero-oauth2"). The exported ones are
	// used by the auth CLI to write these secrets.
	passwordKeyringService = "postero"
	// OAuthSecretKeyringService stores OAuth2 client secrets, keyed by account.
	OAuthSecretKeyringService = "postero-oauth2-secret" //nolint:gosec // G101: keychain service name, not a credential

	// AIKeyringService stores AI provider API keys, keyed by provider name.
	AIKeyringService = "postero-ai"
	providerGmail    = "gmail"
	providerOutlook  = "outlook"
	providerYahoo    = "yahoo"
	providerICloud   = "icloud"
	providerFastmail = "fastmail"
	providerYandex   = "yandex"
)

type providerPreset struct {
	imapHost      string
	imapPort      int
	imapTLS       bool
	smtpHost      string
	smtpPort      int
	smtpTLS       bool
	oauthProvider string
	oauthScopes   []string
	tenantID      string
	builtInOAuth2 bool
}

type Config struct {
	Accounts    []AccountConfig   `mapstructure:"accounts"     yaml:"accounts,omitempty"`
	Storage     StorageConfig     `mapstructure:"storage"      yaml:"storage,omitempty"`
	AI          AIConfig          `mapstructure:"ai"           yaml:"ai,omitempty"`
	Theme       ThemeConfig       `mapstructure:"theme"        yaml:"theme,omitempty"`
	TUI         TUIConfig         `mapstructure:"tui"          yaml:"tui,omitempty"`
	Keybindings KeybindingsConfig `mapstructure:"keybindings"  yaml:"keybindings,omitempty"`
	Filters     map[string]string `mapstructure:"filters"      yaml:"filters,omitempty"`
	DataPath    string            `mapstructure:"data_path"    yaml:"data_path,omitempty"`
	CustomFlags []string          `mapstructure:"custom_flags" yaml:"custom_flags,omitempty"`
}

type AIConfig struct {
	DefaultComposeTemplate string                      `mapstructure:"default_compose_template" yaml:"default_compose_template,omitempty"`
	DefaultReplyTemplate   string                      `mapstructure:"default_reply_template"   yaml:"default_reply_template,omitempty"`
	Providers              map[string]AIProviderConfig `mapstructure:"providers"                yaml:"providers,omitempty"`
	Templates              map[string]AITemplateConfig `mapstructure:"templates"                yaml:"templates,omitempty"`
}

type AIProviderConfig struct {
	Type      string   `mapstructure:"type"        yaml:"type,omitempty"`
	APIKeyCmd []string `mapstructure:"api_key_cmd" yaml:"api_key_cmd,omitempty"`
	BaseURL   string   `mapstructure:"base_url"    yaml:"base_url,omitempty"`
	Model     string   `mapstructure:"model"       yaml:"model,omitempty"`
	// Command is the argv of a local agent CLI for `type: command` / `openclaw`
	// providers (run without a shell; the prompt is passed on stdin).
	Command []string `mapstructure:"command" yaml:"command,omitempty"`

	// LegacyAPIKey captures a deprecated inline `api_key:` only so the loader can
	// warn and ignore it (yaml:"-" keeps it out of every write). API keys are
	// billable secrets — provide them via api_key_cmd or the OS keychain
	// (`pstr auth set-ai <provider>`).
	LegacyAPIKey string `mapstructure:"api_key" yaml:"-"`

	// resolvedAPIKey is filled at load from api_key_cmd or the keychain; never serialized.
	resolvedAPIKey string
}

// APIKey returns the resolved provider key (keychain or api_key_cmd). It is never
// read from the config file.
func (p AIProviderConfig) APIKey() string { return p.resolvedAPIKey }

type AITemplateConfig struct {
	Mode         string  `mapstructure:"mode"          yaml:"mode,omitempty"`
	Provider     string  `mapstructure:"provider"      yaml:"provider,omitempty"`
	SystemPrompt string  `mapstructure:"system_prompt" yaml:"system_prompt,omitempty"`
	Prompt       string  `mapstructure:"prompt"        yaml:"prompt,omitempty"`
	Temperature  float64 `mapstructure:"temperature"   yaml:"temperature,omitempty"`
}

type StorageConfig struct {
	Backend string `mapstructure:"backend" yaml:"backend,omitempty"`
}

type AccountConfig struct {
	Name        string       `mapstructure:"name"         yaml:"name,omitempty"`
	Provider    string       `mapstructure:"provider"     yaml:"provider,omitempty"`
	Email       string       `mapstructure:"email"        yaml:"email,omitempty"`
	IMAP        IMAPConfig   `mapstructure:"imap"         yaml:"imap,omitempty"`
	SMTP        SMTPConfig   `mapstructure:"smtp"         yaml:"smtp,omitempty"`
	Username    string       `mapstructure:"username"     yaml:"username,omitempty"`
	PasswordCmd []string     `mapstructure:"password_cmd" yaml:"password_cmd,omitempty"`
	OAuth2      OAuth2Config `mapstructure:"oauth2"       yaml:"oauth2,omitempty"`

	// LegacyPassword captures a deprecated inline `password:` only so the loader
	// can warn and ignore it. It is never used for authentication (yaml:"-" keeps
	// it out of every SaveConfig write). Store secrets in the OS keychain or a
	// password_cmd instead.
	LegacyPassword string `mapstructure:"password" yaml:"-"`

	// Secrets resolved at load time (keychain, password_cmd, or OAuth token).
	// Kept unexported so SaveConfig can never write them back to disk.
	resolvedIMAPPassword string
	resolvedSMTPPassword string
	resolvedPassword     string
}

type OAuth2Config struct {
	Provider        string   `mapstructure:"provider"          yaml:"provider,omitempty"`
	ClientID        string   `mapstructure:"client_id"         yaml:"client_id,omitempty"`
	ClientSecretCmd []string `mapstructure:"client_secret_cmd" yaml:"client_secret_cmd,omitempty"`
	TenantID        string   `mapstructure:"tenant_id"         yaml:"tenant_id,omitempty"` // used mainly for microsoft
	Scopes          []string `mapstructure:"scopes"            yaml:"scopes,omitempty"`
	RedirectURL     string   `mapstructure:"redirect_url"      yaml:"redirect_url,omitempty"`

	// LegacyClientSecret captures a deprecated inline `client_secret:` only so the
	// loader can warn and ignore it (yaml:"-" keeps it out of every write). Store
	// it via client_secret_cmd or the OS keychain (`pstr auth set-secret <account>`).
	LegacyClientSecret string `mapstructure:"client_secret" yaml:"-"`
	// resolvedClientSecret is filled at load from client_secret_cmd or the keychain.
	resolvedClientSecret string
}

// ClientSecret returns the resolved OAuth2 client secret (keychain or
// client_secret_cmd). It is never read from the config file.
func (o OAuth2Config) ClientSecret() string { return o.resolvedClientSecret }

type IMAPConfig struct {
	Username    string   `mapstructure:"username"     yaml:"username,omitempty"`
	PasswordCmd []string `mapstructure:"password_cmd" yaml:"password_cmd,omitempty"`
	AuthType    string   `mapstructure:"auth_type"    yaml:"auth_type,omitempty"` // e.g. "plain" (default), "oauth2"
	Host        string   `mapstructure:"host"         yaml:"host,omitempty"`
	Port        int      `mapstructure:"port"         yaml:"port,omitempty"`
	TLS         bool     `mapstructure:"tls"          yaml:"tls,omitempty"`

	// LegacyPassword: deprecated inline `password:`, ignored (see AccountConfig).
	LegacyPassword string `mapstructure:"password" yaml:"-"`
}

type SMTPConfig struct {
	Username    string   `mapstructure:"username"     yaml:"username,omitempty"`
	PasswordCmd []string `mapstructure:"password_cmd" yaml:"password_cmd,omitempty"`
	AuthType    string   `mapstructure:"auth_type"    yaml:"auth_type,omitempty"` // e.g. "plain" (default), "oauth2"
	Host        string   `mapstructure:"host"         yaml:"host,omitempty"`
	Port        int      `mapstructure:"port"         yaml:"port,omitempty"`
	TLS         bool     `mapstructure:"tls"          yaml:"tls,omitempty"`

	// LegacyPassword: deprecated inline `password:`, ignored (see AccountConfig).
	LegacyPassword string `mapstructure:"password" yaml:"-"`
}

type ThemeConfig struct {
	Name      string `mapstructure:"name"      yaml:"name,omitempty"`
	Primary   string `mapstructure:"primary"   yaml:"primary,omitempty"`
	Secondary string `mapstructure:"secondary" yaml:"secondary,omitempty"`
	Text      string `mapstructure:"text"      yaml:"text,omitempty"`
	SubText   string `mapstructure:"sub_text"  yaml:"sub_text,omitempty"`
	Highlight string `mapstructure:"highlight" yaml:"highlight,omitempty"`
	Faint     string `mapstructure:"faint"     yaml:"faint,omitempty"`
	Surface   string `mapstructure:"surface"   yaml:"surface,omitempty"`
}

type TUIConfig struct {
	ListPageSize      int `mapstructure:"list_page_size"      yaml:"list_page_size,omitempty"`
	ListPrefetchAhead int `mapstructure:"list_prefetch_ahead" yaml:"list_prefetch_ahead,omitempty"`
	LoadingTickMS     int `mapstructure:"loading_tick_ms"     yaml:"loading_tick_ms,omitempty"`
}

type KeybindingsConfig struct {
	Quit     string `mapstructure:"quit"      yaml:"quit,omitempty"`
	Refresh  string `mapstructure:"refresh"   yaml:"refresh,omitempty"`
	Compose  string `mapstructure:"compose"   yaml:"compose,omitempty"`
	Reply    string `mapstructure:"reply"     yaml:"reply,omitempty"`
	Forward  string `mapstructure:"forward"   yaml:"forward,omitempty"`
	Search   string `mapstructure:"search"    yaml:"search,omitempty"`
	Delete   string `mapstructure:"delete"    yaml:"delete,omitempty"`
	MarkRead string `mapstructure:"mark_read" yaml:"mark_read,omitempty"`
}

func LoadConfig() (*Config, error) {
	configPath, err := ConfigFilePath()
	if err != nil {
		return nil, err
	}

	v := newConfigViper(configPath)
	if err := readOrCreateConfigFile(v, configPath); err != nil {
		return nil, err
	}

	cfg, err := decodeConfig(v)
	if err != nil {
		return nil, err
	}

	applyConfigDefaults(cfg)
	return cfg, nil
}

func newConfigViper(configPath string) *viper.Viper {
	configDir := filepath.Dir(configPath)

	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(configDir)
	v.AddConfigPath(".")

	v.SetEnvPrefix("POSTERO")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	setConfigDefaults(v, configDir)
	return v
}

func readOrCreateConfigFile(v *viper.Viper, configPath string) error {
	if err := v.ReadInConfig(); err != nil {
		var configNotFound viper.ConfigFileNotFoundError
		if !stderrors.As(err, &configNotFound) && !os.IsNotExist(err) {
			return err
		}

		configDir := filepath.Dir(configPath)
		if err := os.MkdirAll(configDir, 0o700); err != nil {
			return err
		}
		materializeEffectiveConfig(v)
		v.SetConfigPermissions(0o600) // owner-only; viper otherwise defaults to 0644
		if err := v.WriteConfigAs(configPath); err != nil {
			return err
		}
	} else {
		materializeEffectiveConfig(v)
	}

	return nil
}

func decodeConfig(v *viper.Viper) (*Config, error) {
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func applyConfigDefaults(cfg *Config) {
	applyAIDefaults(&cfg.AI)
	for index := range cfg.Accounts {
		warnAndClearLegacyPassword(&cfg.Accounts[index])
		warnAndClearLegacyClientSecret(&cfg.Accounts[index])
		applyAccountDefaults(&cfg.Accounts[index])
	}
}

// warnAndClearLegacyClientSecret rejects a deprecated inline `client_secret:`.
func warnAndClearLegacyClientSecret(account *AccountConfig) {
	if account == nil || account.OAuth2.LegacyClientSecret == "" {
		return
	}
	account.OAuth2.LegacyClientSecret = ""
	fmt.Fprintf(os.Stderr,
		"postero: ignoring inline plaintext oauth2 client_secret for %q — it is no longer used. "+
			"Store it with `pstr auth set-secret %s` or a client_secret_cmd.\n",
		firstNonEmpty(account.Name, account.Email, "account"), account.Name)
}

// warnAndClearLegacyPassword rejects a deprecated inline `password:` value: it is
// never used for authentication. The user is told to migrate to the keychain,
// password_cmd, or an environment variable, and the value is dropped in memory.
func warnAndClearLegacyPassword(account *AccountConfig) {
	if account == nil {
		return
	}
	hadInline := account.LegacyPassword != "" ||
		account.IMAP.LegacyPassword != "" ||
		account.SMTP.LegacyPassword != ""
	account.LegacyPassword = ""
	account.IMAP.LegacyPassword = ""
	account.SMTP.LegacyPassword = ""
	if !hadInline {
		return
	}
	name := firstNonEmpty(account.Name, account.Email, "account")
	fmt.Fprintf(os.Stderr,
		"postero: ignoring inline plaintext password for %q — it is no longer used. "+
			"Store the secret with `pstr auth set %s` or a password_cmd.\n",
		name, account.Name)
}

func UsedConfigFile() string {
	path, err := ConfigFilePath()
	if err != nil {
		return ""
	}
	return path
}

func setConfigDefaults(v *viper.Viper, configDir string) {
	v.SetDefault("storage.backend", storageBackendSQLite)
	v.SetDefault("theme.name", "dark")
	v.SetDefault("theme.primary", "#FF00FF")
	v.SetDefault("theme.secondary", "#00FFFF")
	v.SetDefault("theme.text", "#F6F6F6")
	v.SetDefault("theme.sub_text", "#B8B8B8")
	v.SetDefault("theme.highlight", "#FFFFFF")
	v.SetDefault("theme.faint", "#4A4A4A")
	v.SetDefault("tui.list_page_size", 30)
	v.SetDefault("tui.list_prefetch_ahead", 5)
	v.SetDefault("tui.loading_tick_ms", 120)
	v.SetDefault("data_path", filepath.Join(configDir, "data"))
}

func materializeEffectiveConfig(v *viper.Viper) {
	for _, key := range v.AllKeys() {
		v.Set(key, v.Get(key))
	}
}

func applyAccountDefaults(account *AccountConfig) {
	applyProviderDefaults(account)

	if account.Username == "" {
		account.Username = account.Email
	}
	if account.IMAP.Username == "" {
		account.IMAP.Username = account.Username
	}
	if account.SMTP.Username == "" {
		account.SMTP.Username = account.Username
	}
	// Secrets are never read from the config file; resolve them from the keychain,
	// a *_cmd, or the OAuth token exclusively.
	account.resolvedIMAPPassword = resolvePassword(account, "IMAP")
	account.resolvedSMTPPassword = resolvePassword(account, "SMTP")
	account.resolvedPassword = resolvePassword(account, "")
	account.OAuth2.resolvedClientSecret = resolveClientSecret(account)
}

// resolveClientSecret resolves an account's OAuth2 client secret from its
// client_secret_cmd or the OS keychain. Inline plaintext is never read.
func resolveClientSecret(account *AccountConfig) string {
	if secret := execPasswordCmd(account.OAuth2.ClientSecretCmd); secret != "" {
		return secret
	}
	if strings.TrimSpace(account.Name) != "" {
		if secret, err := keyring.Get(OAuthSecretKeyringService, account.Name); err == nil && secret != "" {
			return secret
		}
	}
	return ""
}

func applyAIDefaults(cfg *AIConfig) {
	if cfg == nil {
		return
	}

	for name, provider := range cfg.Providers {
		if provider.Type == "" {
			provider.Type = strings.ToLower(strings.TrimSpace(name))
		}
		switch strings.ToLower(strings.TrimSpace(provider.Type)) {
		case "openai":
			if provider.BaseURL == "" {
				provider.BaseURL = "https://api.openai.com/v1"
			}
		case "gemini":
			if provider.BaseURL == "" {
				provider.BaseURL = "https://generativelanguage.googleapis.com/v1beta"
			}
		case "anthropic", "claude":
			if provider.BaseURL == "" {
				provider.BaseURL = "https://api.anthropic.com/v1"
			}
		case "openclaw":
			if len(provider.Command) == 0 {
				provider.Command = []string{"openclaw", "agent", "exec", "--message-file", "-"}
			}
		}
		warnAndClearLegacyAPIKey(name, &provider)
		provider.resolvedAPIKey = resolveAPIKey(name, provider)
		cfg.Providers[name] = provider
	}
}

// resolveAPIKey resolves a provider's API key from its api_key_cmd or the OS
// keychain. Inline plaintext keys are never read.
func resolveAPIKey(providerName string, p AIProviderConfig) string {
	if key := execPasswordCmd(p.APIKeyCmd); key != "" {
		return key
	}
	if strings.TrimSpace(providerName) != "" {
		if secret, err := keyring.Get(AIKeyringService, providerName); err == nil && secret != "" {
			return secret
		}
	}
	return ""
}

func warnAndClearLegacyAPIKey(providerName string, p *AIProviderConfig) {
	if p == nil || p.LegacyAPIKey == "" {
		return
	}
	p.LegacyAPIKey = ""
	fmt.Fprintf(os.Stderr,
		"postero: ignoring inline plaintext api_key for AI provider %q — it is no longer used. "+
			"Store it with `pstr auth set-ai %s` or an api_key_cmd.\n",
		providerName, providerName)
}

func applyProviderDefaults(account *AccountConfig) {
	if account == nil {
		return
	}

	provider := canonicalProvider(account)
	preset, ok := providerPresetFor(provider)
	if !ok {
		return
	}

	applyIMAPNetworkDefaults(&account.IMAP, preset.imapHost, preset.imapPort, preset.imapTLS)
	applySMTPNetworkDefaults(&account.SMTP, preset.smtpHost, preset.smtpPort, preset.smtpTLS)
	// Only backfill the OAuth provider when the account actually opts into
	// OAuth2; otherwise an app-password gmail/outlook account would be forced
	// onto a broken oauth2 auth path.
	if account.OAuth2.Provider == "" && preset.oauthProvider != "" && accountOptsIntoOAuth2(account) {
		account.OAuth2.Provider = preset.oauthProvider
	}
	if usesOAuth2(account) {
		applyOAuthDefaults(account, authTypeOAuth2, preset.oauthScopes)
		if account.OAuth2.TenantID == "" && preset.tenantID != "" {
			account.OAuth2.TenantID = preset.tenantID
		}
	}
}

func applyIMAPNetworkDefaults(cfg *IMAPConfig, host string, port int, useTLS bool) {
	if cfg.Host == "" {
		cfg.Host = host
	}
	if cfg.Port == 0 {
		cfg.Port = port
	}
	if !cfg.TLS {
		cfg.TLS = useTLS
	}
}

func applySMTPNetworkDefaults(cfg *SMTPConfig, host string, port int, useTLS bool) {
	if cfg.Host == "" {
		cfg.Host = host
	}
	if cfg.Port == 0 {
		cfg.Port = port
	}
	if !cfg.TLS {
		cfg.TLS = useTLS
	}
}

func applyOAuthDefaults(account *AccountConfig, authType string, scopes []string) {
	if account.IMAP.AuthType == "" {
		account.IMAP.AuthType = authType
	}
	if account.SMTP.AuthType == "" {
		account.SMTP.AuthType = authType
	}
	if len(account.OAuth2.Scopes) == 0 {
		account.OAuth2.Scopes = scopes
	}
}

// accountOptsIntoOAuth2 reports whether the user explicitly configured OAuth2
// (credentials or auth_type), as opposed to a provider merely supporting it.
func accountOptsIntoOAuth2(account *AccountConfig) bool {
	if account == nil {
		return false
	}
	return account.OAuth2.ClientID != "" || len(account.OAuth2.ClientSecretCmd) > 0 ||
		account.IMAP.AuthType == authTypeOAuth2 || account.SMTP.AuthType == authTypeOAuth2
}

func usesOAuth2(account *AccountConfig) bool {
	if account == nil {
		return false
	}

	if account.IMAP.AuthType == authTypeOAuth2 || account.SMTP.AuthType == authTypeOAuth2 {
		return true
	}

	return account.OAuth2.ClientID != "" || len(account.OAuth2.ClientSecretCmd) > 0 || account.OAuth2.Provider != ""
}

func canonicalProvider(account *AccountConfig) string {
	if account == nil {
		return ""
	}

	for _, candidate := range []string{account.Provider, account.OAuth2.Provider, inferredProviderFromEmail(account.Email)} {
		if provider := canonicalProviderName(candidate); provider != "" {
			return provider
		}
	}

	return ""
}

func canonicalProviderName(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "gmail", "google":
		return providerGmail
	case "outlook", "microsoft", "office365", "m365":
		return providerOutlook
	case "yahoo", "ymail":
		return providerYahoo
	case "icloud", "me", "mac":
		return providerICloud
	case "fastmail":
		return providerFastmail
	case "yandex", "ya":
		return providerYandex
	default:
		return ""
	}
}

// ProviderForEmail reports the provider preset inferred from the email
// domain, or "" when the domain is unknown.
func ProviderForEmail(email string) string {
	return inferredProviderFromEmail(email)
}

func inferredProviderFromEmail(email string) string {
	_, domain, ok := strings.Cut(strings.TrimSpace(strings.ToLower(email)), "@")
	if !ok {
		return ""
	}

	switch domain {
	case "gmail.com", "googlemail.com":
		return providerGmail
	case "outlook.com", "hotmail.com", "live.com", "msn.com":
		return providerOutlook
	case "yahoo.com", "ymail.com":
		return providerYahoo
	case "icloud.com", "me.com", "mac.com":
		return providerICloud
	case "fastmail.com":
		return providerFastmail
	case "yandex.com", "yandex.ru", "ya.ru", "yandex.by", "yandex.kz", "yandex.ua":
		return providerYandex
	default:
		return ""
	}
}

func SupportsBuiltInOAuth2(provider string) bool {
	preset, ok := providerPresetFor(canonicalProviderName(provider))
	return ok && preset.builtInOAuth2
}

func NormalizeProviderName(provider string) string {
	return canonicalProviderName(provider)
}

func StorageBackend(cfg *Config) string {
	if cfg == nil {
		return storageBackendSQLite
	}
	backend := strings.TrimSpace(strings.ToLower(cfg.Storage.Backend))
	if backend == "" {
		return storageBackendSQLite
	}
	return backend
}

func (a AccountConfig) IMAPCredentials() (string, string) {
	return accountCredentials(a.IMAP.Username, a.resolvedIMAPPassword, a)
}

func (a AccountConfig) SMTPCredentials() (string, string) {
	return accountCredentials(a.SMTP.Username, a.resolvedSMTPPassword, a)
}

func accountCredentials(protocolUsername, protocolPassword string, account AccountConfig) (string, string) {
	username := firstNonEmpty(protocolUsername, account.Username, account.Email)
	password := firstNonEmpty(protocolPassword, account.resolvedPassword)
	return username, password
}

func execPasswordCmd(cmdArgs []string) string {
	if len(cmdArgs) == 0 {
		return ""
	}

	binary, err := exec.LookPath(cmdArgs[0])
	if err != nil {
		return ""
	}

	cmd := exec.CommandContext(context.Background(), binary, cmdArgs[1:]...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSuffix(strings.TrimSpace(string(out)), "\n")
}

func resolvePassword(account *AccountConfig, protocol string) string {
	if account == nil {
		return ""
	}

	if token := resolveOAuth2Token(account, protocol); token != "" {
		return token
	}

	for _, candidate := range passwordCommandsForProtocol(account, protocol) {
		if pwd := execPasswordCmd(candidate); pwd != "" {
			return pwd
		}
	}

	// OS keychain, written by `pstr auth set` or the setup wizard. This is the
	// only at-rest secret store Postero uses — passwords never come from the
	// config file or environment variables.
	if strings.TrimSpace(account.Name) != "" {
		if secret, err := keyring.Get(passwordKeyringService, account.Name); err == nil && secret != "" {
			return secret
		}
	}
	return ""
}

func resolveOAuth2Token(account *AccountConfig, protocol string) string {
	if account.OAuth2.ClientID == "" {
		return ""
	}

	switch protocol {
	case "IMAP":
		if account.IMAP.AuthType != authTypeOAuth2 {
			return ""
		}
	case "SMTP":
		if account.SMTP.AuthType != authTypeOAuth2 {
			return ""
		}
	default:
		return ""
	}

	if token, err := GetToken(context.Background(), account.Name, &account.OAuth2); err == nil {
		return token
	}

	return ""
}

func passwordCommandsForProtocol(account *AccountConfig, protocol string) [][]string {
	commands := make([][]string, 0, 2)
	switch protocol {
	case "IMAP":
		if len(account.IMAP.PasswordCmd) > 0 {
			commands = append(commands, account.IMAP.PasswordCmd)
		}
	case "SMTP":
		if len(account.SMTP.PasswordCmd) > 0 {
			commands = append(commands, account.SMTP.PasswordCmd)
		}
	}
	if len(account.PasswordCmd) > 0 {
		commands = append(commands, account.PasswordCmd)
	}
	return commands
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func providerPresetFor(provider string) (providerPreset, bool) {
	switch provider {
	case providerGmail:
		return providerPreset{
			imapHost:      "imap.gmail.com",
			imapPort:      993,
			imapTLS:       true,
			smtpHost:      "smtp.gmail.com",
			smtpPort:      587,
			smtpTLS:       true,
			oauthProvider: "google",
			oauthScopes:   []string{"https://mail.google.com/"},
			builtInOAuth2: true,
		}, true
	case providerOutlook:
		return providerPreset{
			imapHost:      "outlook.office365.com",
			imapPort:      993,
			imapTLS:       true,
			smtpHost:      "smtp.office365.com",
			smtpPort:      587,
			smtpTLS:       true,
			oauthProvider: "microsoft",
			oauthScopes: []string{
				"https://outlook.office.com/IMAP.AccessAsUser.All",
				"https://outlook.office.com/SMTP.Send",
				"offline_access",
			},
			tenantID:      "common",
			builtInOAuth2: true,
		}, true
	case providerYahoo:
		return providerPreset{
			imapHost: "imap.mail.yahoo.com",
			imapPort: 993,
			imapTLS:  true,
			smtpHost: "smtp.mail.yahoo.com",
			smtpPort: 465,
			smtpTLS:  true,
		}, true
	case providerICloud:
		return providerPreset{
			imapHost: "imap.mail.me.com",
			imapPort: 993,
			imapTLS:  true,
			smtpHost: "smtp.mail.me.com",
			smtpPort: 587,
			smtpTLS:  true,
		}, true
	case providerFastmail:
		return providerPreset{
			imapHost: "imap.fastmail.com",
			imapPort: 993,
			imapTLS:  true,
			smtpHost: "smtp.fastmail.com",
			smtpPort: 465,
			smtpTLS:  true,
		}, true
	case providerYandex:
		return providerPreset{
			imapHost: "imap.yandex.com",
			imapPort: 993,
			imapTLS:  true,
			smtpHost: "smtp.yandex.com",
			smtpPort: 465,
			smtpTLS:  true,
		}, true
	default:
		return providerPreset{}, false
	}
}
