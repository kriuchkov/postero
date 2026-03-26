package config

import (
	"context"
	stderrors "errors"
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
	providerGmail        = "gmail"
	providerOutlook      = "outlook"
	providerYahoo        = "yahoo"
	providerICloud       = "icloud"
	providerFastmail     = "fastmail"
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
	Theme       ThemeConfig       `mapstructure:"theme"        yaml:"theme,omitempty"`
	TUI         TUIConfig         `mapstructure:"tui"          yaml:"tui,omitempty"`
	Keybindings KeybindingsConfig `mapstructure:"keybindings"  yaml:"keybindings,omitempty"`
	Filters     map[string]string `mapstructure:"filters"      yaml:"filters,omitempty"`
	DataPath    string            `mapstructure:"data_path"    yaml:"data_path,omitempty"`
	CustomFlags []string          `mapstructure:"custom_flags" yaml:"custom_flags,omitempty"`
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
	Password    string       `mapstructure:"password"     yaml:"password,omitempty"`
	PasswordCmd []string     `mapstructure:"password_cmd" yaml:"password_cmd,omitempty"`
	OAuth2      OAuth2Config `mapstructure:"oauth2"       yaml:"oauth2,omitempty"`
}

type OAuth2Config struct {
	Provider     string   `mapstructure:"provider"      yaml:"provider,omitempty"`
	ClientID     string   `mapstructure:"client_id"     yaml:"client_id,omitempty"`
	ClientSecret string   `mapstructure:"client_secret" yaml:"client_secret,omitempty"`
	TenantID     string   `mapstructure:"tenant_id"     yaml:"tenant_id,omitempty"` // used mainly for microsoft
	Scopes       []string `mapstructure:"scopes"        yaml:"scopes,omitempty"`
	RedirectURL  string   `mapstructure:"redirect_url"  yaml:"redirect_url,omitempty"`
}

type IMAPConfig struct {
	Username    string   `mapstructure:"username"     yaml:"username,omitempty"`
	Password    string   `mapstructure:"password"     yaml:"password,omitempty"`
	PasswordCmd []string `mapstructure:"password_cmd" yaml:"password_cmd,omitempty"`
	AuthType    string   `mapstructure:"auth_type"    yaml:"auth_type,omitempty"` // e.g. "plain" (default), "oauth2"
	Host        string   `mapstructure:"host"         yaml:"host,omitempty"`
	Port        int      `mapstructure:"port"         yaml:"port,omitempty"`
	TLS         bool     `mapstructure:"tls"          yaml:"tls,omitempty"`
}

type SMTPConfig struct {
	Username    string   `mapstructure:"username"     yaml:"username,omitempty"`
	Password    string   `mapstructure:"password"     yaml:"password,omitempty"`
	PasswordCmd []string `mapstructure:"password_cmd" yaml:"password_cmd,omitempty"`
	AuthType    string   `mapstructure:"auth_type"    yaml:"auth_type,omitempty"` // e.g. "plain" (default), "oauth2"
	Host        string   `mapstructure:"host"         yaml:"host,omitempty"`
	Port        int      `mapstructure:"port"         yaml:"port,omitempty"`
	TLS         bool     `mapstructure:"tls"          yaml:"tls,omitempty"`
}

type ThemeConfig struct {
	Name      string `mapstructure:"name"      yaml:"name,omitempty"`
	Primary   string `mapstructure:"primary"   yaml:"primary,omitempty"`
	Secondary string `mapstructure:"secondary" yaml:"secondary,omitempty"`
	Text      string `mapstructure:"text"      yaml:"text,omitempty"`
	SubText   string `mapstructure:"sub_text"  yaml:"sub_text,omitempty"`
	Highlight string `mapstructure:"highlight" yaml:"highlight,omitempty"`
	Faint     string `mapstructure:"faint"     yaml:"faint,omitempty"`
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
	for index := range cfg.Accounts {
		applyAccountDefaults(&cfg.Accounts[index])
	}
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
	if account.IMAP.Password == "" {
		account.IMAP.Password = resolvePassword(account, "IMAP")
	}
	if account.SMTP.Password == "" {
		account.SMTP.Password = resolvePassword(account, "SMTP")
	}
	if account.Password == "" {
		account.Password = resolvePassword(account, "")
	}
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
	if account.OAuth2.Provider == "" && preset.oauthProvider != "" {
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

func usesOAuth2(account *AccountConfig) bool {
	if account == nil {
		return false
	}

	if account.IMAP.AuthType == authTypeOAuth2 || account.SMTP.AuthType == authTypeOAuth2 {
		return true
	}

	return account.OAuth2.ClientID != "" || account.OAuth2.ClientSecret != "" || account.OAuth2.Provider != ""
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
	default:
		return ""
	}
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
	return accountCredentials(a.IMAP.Username, a.IMAP.Password, a)
}

func (a AccountConfig) SMTPCredentials() (string, string) {
	return accountCredentials(a.SMTP.Username, a.SMTP.Password, a)
}

func accountCredentials(protocolUsername, protocolPassword string, account AccountConfig) (string, string) {
	username := firstNonEmpty(protocolUsername, account.Username, account.Email)
	password := firstNonEmpty(protocolPassword, account.Password)
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

	name := normalizedAccountName(account.Name)

	// Try to resolve using OS Keychain via Built-in Keyring integration
	if name != "" {
		if secret, err := keyring.Get("postero", account.Name); err == nil && secret != "" {
			return secret
		}
	}

	for _, key := range passwordEnvKeys(name, protocol) {
		if password := os.Getenv(key); password != "" {
			return password
		}
	}
	return os.Getenv("POSTERO_PASSWORD")
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

func passwordEnvKeys(name, protocol string) []string {
	keys := make([]string, 0, 3)
	if name != "" && protocol != "" {
		keys = append(keys, "POSTERO_"+name+"_"+protocol+"_PASSWORD")
	}
	if protocol != "" {
		keys = append(keys, "POSTERO_"+protocol+"_PASSWORD")
	}
	if name != "" {
		keys = append(keys, "POSTERO_"+name+"_PASSWORD")
	}
	return keys
}

func normalizedAccountName(name string) string {
	value := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(name), "-", "_"))
	return strings.ReplaceAll(value, " ", "_")
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
	default:
		return providerPreset{}, false
	}
}
