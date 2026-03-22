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

type Config struct {
	Accounts    []AccountConfig   `mapstructure:"accounts" yaml:"accounts,omitempty"`
	Storage     StorageConfig     `mapstructure:"storage" yaml:"storage,omitempty"`
	Theme       ThemeConfig       `mapstructure:"theme" yaml:"theme,omitempty"`
	Keybindings KeybindingsConfig `mapstructure:"keybindings" yaml:"keybindings,omitempty"`
	Filters     map[string]string `mapstructure:"filters" yaml:"filters,omitempty"`
	DataPath    string            `mapstructure:"data_path" yaml:"data_path,omitempty"`
	CustomFlags []string          `mapstructure:"custom_flags" yaml:"custom_flags,omitempty"`
}

type StorageConfig struct {
	Backend string `mapstructure:"backend" yaml:"backend,omitempty"`
}

type AccountConfig struct {
	Name        string       `mapstructure:"name" yaml:"name,omitempty"`
	Provider    string       `mapstructure:"provider" yaml:"provider,omitempty"`
	Email       string       `mapstructure:"email" yaml:"email,omitempty"`
	IMAP        IMAPConfig   `mapstructure:"imap" yaml:"imap,omitempty"`
	SMTP        SMTPConfig   `mapstructure:"smtp" yaml:"smtp,omitempty"`
	Username    string       `mapstructure:"username" yaml:"username,omitempty"`
	Password    string       `mapstructure:"password" yaml:"password,omitempty"`
	PasswordCmd []string     `mapstructure:"password_cmd" yaml:"password_cmd,omitempty"`
	OAuth2      OAuth2Config `mapstructure:"oauth2" yaml:"oauth2,omitempty"`
}

type OAuth2Config struct {
	Provider     string   `mapstructure:"provider" yaml:"provider,omitempty"`
	ClientID     string   `mapstructure:"client_id" yaml:"client_id,omitempty"`
	ClientSecret string   `mapstructure:"client_secret" yaml:"client_secret,omitempty"`
	TenantID     string   `mapstructure:"tenant_id" yaml:"tenant_id,omitempty"` // used mainly for microsoft
	Scopes       []string `mapstructure:"scopes" yaml:"scopes,omitempty"`
	RedirectURL  string   `mapstructure:"redirect_url" yaml:"redirect_url,omitempty"`
}

type IMAPConfig struct {
	Username    string   `mapstructure:"username" yaml:"username,omitempty"`
	Password    string   `mapstructure:"password" yaml:"password,omitempty"`
	PasswordCmd []string `mapstructure:"password_cmd" yaml:"password_cmd,omitempty"`
	AuthType    string   `mapstructure:"auth_type" yaml:"auth_type,omitempty"` // e.g. "plain" (default), "oauth2"
	Host        string   `mapstructure:"host" yaml:"host,omitempty"`
	Port        int      `mapstructure:"port" yaml:"port,omitempty"`
	TLS         bool     `mapstructure:"tls" yaml:"tls,omitempty"`
}

type SMTPConfig struct {
	Username    string   `mapstructure:"username" yaml:"username,omitempty"`
	Password    string   `mapstructure:"password" yaml:"password,omitempty"`
	PasswordCmd []string `mapstructure:"password_cmd" yaml:"password_cmd,omitempty"`
	AuthType    string   `mapstructure:"auth_type" yaml:"auth_type,omitempty"` // e.g. "plain" (default), "oauth2"
	Host        string   `mapstructure:"host" yaml:"host,omitempty"`
	Port        int      `mapstructure:"port" yaml:"port,omitempty"`
	TLS         bool     `mapstructure:"tls" yaml:"tls,omitempty"`
}

type ThemeConfig struct {
	Name      string `mapstructure:"name" yaml:"name,omitempty"`
	Primary   string `mapstructure:"primary" yaml:"primary,omitempty"`
	Secondary string `mapstructure:"secondary" yaml:"secondary,omitempty"`
	Text      string `mapstructure:"text" yaml:"text,omitempty"`
	SubText   string `mapstructure:"sub_text" yaml:"sub_text,omitempty"`
	Highlight string `mapstructure:"highlight" yaml:"highlight,omitempty"`
	Faint     string `mapstructure:"faint" yaml:"faint,omitempty"`
}

type KeybindingsConfig struct {
	Quit     string `mapstructure:"quit" yaml:"quit,omitempty"`
	Refresh  string `mapstructure:"refresh" yaml:"refresh,omitempty"`
	Compose  string `mapstructure:"compose" yaml:"compose,omitempty"`
	Reply    string `mapstructure:"reply" yaml:"reply,omitempty"`
	Forward  string `mapstructure:"forward" yaml:"forward,omitempty"`
	Search   string `mapstructure:"search" yaml:"search,omitempty"`
	Delete   string `mapstructure:"delete" yaml:"delete,omitempty"`
	MarkRead string `mapstructure:"mark_read" yaml:"mark_read,omitempty"`
}

func LoadConfig() (*Config, error) {
	configDir := os.Getenv("POSTERO_CONFIG_DIR")
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		configDir = filepath.Join(home, ".config", "postero")
	}

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(configDir)
	viper.AddConfigPath(".")

	viper.SetEnvPrefix("POSTERO")
	viper.AutomaticEnv()

	// Set defaults
	viper.SetDefault("storage.backend", "sqlite")
	viper.SetDefault("theme.name", "dark")
	viper.SetDefault("theme.primary", "#FF00FF")
	viper.SetDefault("theme.secondary", "#00FFFF")
	viper.SetDefault("theme.text", "#F6F6F6")
	viper.SetDefault("theme.sub_text", "#B8B8B8")
	viper.SetDefault("theme.highlight", "#FFFFFF")
	viper.SetDefault("theme.faint", "#4A4A4A")
	viper.SetDefault("data_path", filepath.Join(configDir, "data"))

	if err := viper.ReadInConfig(); err != nil {
		var configNotFound viper.ConfigFileNotFoundError
		if !stderrors.As(err, &configNotFound) {
			return nil, err
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	for index := range cfg.Accounts {
		applyAccountDefaults(&cfg.Accounts[index])
	}

	return &cfg, nil
}

func UsedConfigFile() string {
	return viper.ConfigFileUsed()
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
	switch provider {
	case "gmail":
		applyProviderNetworkDefaults(&account.IMAP, "imap.gmail.com", 993, true)
		applyProviderNetworkDefaults(&account.SMTP, "smtp.gmail.com", 587, true)
		if account.OAuth2.Provider == "" {
			account.OAuth2.Provider = "google"
		}
		if usesOAuth2(account) {
			applyOAuthDefaults(account, "oauth2", []string{"https://mail.google.com/"})
		}
	case "outlook":
		applyProviderNetworkDefaults(&account.IMAP, "outlook.office365.com", 993, true)
		applyProviderNetworkDefaults(&account.SMTP, "smtp.office365.com", 587, true)
		if account.OAuth2.Provider == "" {
			account.OAuth2.Provider = "microsoft"
		}
		if usesOAuth2(account) {
			applyOAuthDefaults(account, "oauth2", []string{
				"https://outlook.office.com/IMAP.AccessAsUser.All",
				"https://outlook.office.com/SMTP.Send",
				"offline_access",
			})
			if account.OAuth2.TenantID == "" {
				account.OAuth2.TenantID = "common"
			}
		}
	case "yahoo":
		applyProviderNetworkDefaults(&account.IMAP, "imap.mail.yahoo.com", 993, true)
		applyProviderNetworkDefaults(&account.SMTP, "smtp.mail.yahoo.com", 465, true)
	case "icloud":
		applyProviderNetworkDefaults(&account.IMAP, "imap.mail.me.com", 993, true)
		applyProviderNetworkDefaults(&account.SMTP, "smtp.mail.me.com", 587, true)
	case "fastmail":
		applyProviderNetworkDefaults(&account.IMAP, "imap.fastmail.com", 993, true)
		applyProviderNetworkDefaults(&account.SMTP, "smtp.fastmail.com", 465, true)
	}
}

func applyProviderNetworkDefaults(cfg interface{}, host string, port int, useTLS bool) {
	switch value := cfg.(type) {
	case *IMAPConfig:
		if value.Host == "" {
			value.Host = host
		}
		if value.Port == 0 {
			value.Port = port
		}
		if !value.TLS {
			value.TLS = useTLS
		}
	case *SMTPConfig:
		if value.Host == "" {
			value.Host = host
		}
		if value.Port == 0 {
			value.Port = port
		}
		if !value.TLS {
			value.TLS = useTLS
		}
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

	if account.IMAP.AuthType == "oauth2" || account.SMTP.AuthType == "oauth2" {
		return true
	}

	return account.OAuth2.ClientID != "" || account.OAuth2.ClientSecret != "" || account.OAuth2.Provider != ""
}

func canonicalProvider(account *AccountConfig) string {
	if account == nil {
		return ""
	}

	for _, candidate := range []string{account.Provider, account.OAuth2.Provider, inferredProviderFromEmail(account.Email)} {
		switch canonicalProviderName(candidate) {
		case "gmail":
			return "gmail"
		case "outlook":
			return "outlook"
		case "yahoo":
			return "yahoo"
		case "icloud":
			return "icloud"
		case "fastmail":
			return "fastmail"
		}
	}

	return ""
}

func canonicalProviderName(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "gmail", "google":
		return "gmail"
	case "outlook", "microsoft", "office365", "m365":
		return "outlook"
	case "yahoo", "ymail":
		return "yahoo"
	case "icloud", "me", "mac":
		return "icloud"
	case "fastmail":
		return "fastmail"
	default:
		return ""
	}
}

func inferredProviderFromEmail(email string) string {
	parts := strings.Split(strings.TrimSpace(strings.ToLower(email)), "@")
	if len(parts) != 2 {
		return ""
	}

	switch parts[1] {
	case "gmail.com", "googlemail.com":
		return "gmail"
	case "outlook.com", "hotmail.com", "live.com", "msn.com":
		return "outlook"
	case "yahoo.com", "ymail.com":
		return "yahoo"
	case "icloud.com", "me.com", "mac.com":
		return "icloud"
	case "fastmail.com":
		return "fastmail"
	default:
		return ""
	}
}

func SupportsBuiltInOAuth2(provider string) bool {
	switch canonicalProviderName(provider) {
	case "gmail", "outlook":
		return true
	default:
		return false
	}
}

func NormalizeProviderName(provider string) string {
	return canonicalProviderName(provider)
}

func StorageBackend(cfg *Config) string {
	if cfg == nil {
		return "sqlite"
	}
	backend := strings.TrimSpace(strings.ToLower(cfg.Storage.Backend))
	if backend == "" {
		return "sqlite"
	}
	return backend
}

func (a AccountConfig) IMAPCredentials() (string, string) {
	username := a.IMAP.Username
	if username == "" {
		username = a.Username
	}
	if username == "" {
		username = a.Email
	}
	password := a.IMAP.Password
	if password == "" {
		password = a.Password
	}
	return username, password
}

func (a AccountConfig) SMTPCredentials() (string, string) {
	username := a.SMTP.Username
	if username == "" {
		username = a.Username
	}
	if username == "" {
		username = a.Email
	}
	password := a.SMTP.Password
	if password == "" {
		password = a.Password
	}
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

	cmd := exec.Command(binary, cmdArgs[1:]...) //nolint:gosec // Command comes from the user's local config.
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

	// Try OAuth2 transparent refresh first if it's configured
	if protocol == "IMAP" && account.IMAP.AuthType == "oauth2" && account.OAuth2.ClientID != "" {
		if token, err := GetToken(context.Background(), account.Name, &account.OAuth2); err == nil {
			return token
		}
	}
	if protocol == "SMTP" && account.SMTP.AuthType == "oauth2" && account.OAuth2.ClientID != "" {
		if token, err := GetToken(context.Background(), account.Name, &account.OAuth2); err == nil {
			return token
		}
	}

	// Try to resolve using PasswordCmd if defined
	if protocol == "IMAP" && len(account.IMAP.PasswordCmd) > 0 {
		if pwd := execPasswordCmd(account.IMAP.PasswordCmd); pwd != "" {
			return pwd
		}
	}
	if protocol == "SMTP" && len(account.SMTP.PasswordCmd) > 0 {
		if pwd := execPasswordCmd(account.SMTP.PasswordCmd); pwd != "" {
			return pwd
		}
	}
	if len(account.PasswordCmd) > 0 {
		if pwd := execPasswordCmd(account.PasswordCmd); pwd != "" {
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

	if name != "" && protocol != "" {
		if password := os.Getenv("POSTERO_" + name + "_" + protocol + "_PASSWORD"); password != "" {
			return password
		}
	}
	if protocol != "" {
		if password := os.Getenv("POSTERO_" + protocol + "_PASSWORD"); password != "" {
			return password
		}
	}
	if name != "" {
		if password := os.Getenv("POSTERO_" + name + "_PASSWORD"); password != "" {
			return password
		}
	}
	return os.Getenv("POSTERO_PASSWORD")
}

func normalizedAccountName(name string) string {
	value := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(name), "-", "_"))
	return strings.ReplaceAll(value, " ", "_")
}
