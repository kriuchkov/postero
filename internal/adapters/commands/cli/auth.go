package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"syscall"

	"github.com/go-faster/errors"
	"github.com/spf13/cobra"
	"github.com/zalando/go-keyring"
	"golang.org/x/term"

	appcore "github.com/kriuchkov/postero/internal/app"
	"github.com/kriuchkov/postero/internal/config"
)

var (
	authBootstrapEmail        string
	authBootstrapProvider     string
	authBootstrapClientID     string
	authBootstrapClientSecret string
	authBootstrapTenantID     string
	authBootstrapName         string
	authAddLogin              bool
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage saved credentials",
	Long:  `Manage passwords stored securely in your operating system's native keychain.`,
}

var authSetCmd = &cobra.Command{
	Use:   "set [account_name]",
	Short: "Save a password for an account",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		account := args[0]

		password, err := readSecret(fmt.Sprintf("Enter password for account '%s': ", account))
		if err != nil {
			return err
		}

		if err := keyring.Set("postero", account, password); err != nil {
			return errors.Wrap(err, "failed to save password to keyring")
		}

		fmt.Printf("Password for '%s' saved successfully via OS Keychain.\n", account)
		return nil
	},
}

// readSecret prompts and reads a non-empty secret from the terminal without echo.
func readSecret(prompt string) (string, error) {
	fmt.Print(prompt)
	raw, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return "", errors.Wrap(err, "failed to read secret")
	}
	secret := strings.TrimSpace(string(raw))
	if secret == "" {
		return "", errors.New("secret cannot be empty")
	}
	return secret, nil
}

var authSetAICmd = &cobra.Command{
	Use:   "set-ai [provider]",
	Short: "Save an AI provider API key in the OS keychain",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		provider := strings.TrimSpace(args[0])
		if provider == "" {
			return errors.New("provider name cannot be empty")
		}
		secret, err := readSecret(fmt.Sprintf("Enter API key for AI provider '%s': ", provider))
		if err != nil {
			return err
		}
		if err := keyring.Set(config.AIKeyringService, provider, secret); err != nil {
			return errors.Wrap(err, "failed to save api key to keyring")
		}
		fmt.Printf("API key for AI provider '%s' saved to OS Keychain.\n", provider)
		return nil
	},
}

var authSetSecretCmd = &cobra.Command{
	Use:   "set-secret [account_name]",
	Short: "Save an OAuth2 client secret in the OS keychain",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		account := strings.TrimSpace(args[0])
		if account == "" {
			return errors.New("account name cannot be empty")
		}
		secret, err := readSecret(fmt.Sprintf("Enter OAuth2 client secret for account '%s': ", account))
		if err != nil {
			return err
		}
		if err := keyring.Set(config.OAuthSecretKeyringService, account, secret); err != nil {
			return errors.Wrap(err, "failed to save client secret to keyring")
		}
		fmt.Printf("OAuth2 client secret for '%s' saved to OS Keychain.\n", account)
		return nil
	},
}

var authDelCmd = &cobra.Command{
	Use:   "delete [account_name]",
	Short: "Delete a saved password for an account",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		account := args[0]

		err := keyring.Delete("postero", account)
		// Try deleting oauth2 token and client-secret keys as well.
		_ = keyring.Delete("postero-oauth2", account)
		_ = keyring.Delete(config.OAuthSecretKeyringService, account)

		if err != nil {
			if errors.Is(err, keyring.ErrNotFound) {
				fmt.Printf("No credentials found for '%s'.\n", account)
				return nil
			}
			return errors.Wrap(err, "failed to delete password")
		}

		fmt.Printf("Credentials for '%s' deleted successfully.\n", account)
		return nil
	},
}

var authLoginCmd = &cobra.Command{
	Use:   "login [account_name]",
	Short: "Perform interactive login (for OAuth2)",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		accountName := args[0]
		cfg, err := appcore.LoadConfig()
		if err != nil {
			return errors.Wrap(err, "failed to load config")
		}

		account, ok := appcore.ResolveAccount(cfg, accountName)
		if !ok || account.OAuth2.ClientID == "" || account.OAuth2.ClientSecret() == "" {
			account, err = ensureOAuthAccountConfig(cfg, accountName, account, ok)
			if err != nil {
				return err
			}
		}

		ctx := context.Background()
		return runOAuthLogin(ctx, accountName, account)
	},
}

var authAddCmd = &cobra.Command{
	Use:   "add [provider]",
	Short: "Create or update an account config entry",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		provider := config.NormalizeProviderName(args[0])
		if provider == "" {
			return errors.Errorf("unsupported provider %q", args[0])
		}
		if strings.TrimSpace(authBootstrapEmail) == "" {
			return errors.New("--email is required")
		}

		cfg, err := appcore.LoadConfig()
		if err != nil {
			return errors.Wrap(err, "failed to load config")
		}

		accountName := authBootstrapName
		if strings.TrimSpace(accountName) == "" {
			accountName = provider
		}

		account := config.AccountConfig{
			Name:     accountName,
			Provider: provider,
			Email:    authBootstrapEmail,
		}
		if config.SupportsBuiltInOAuth2(provider) {
			account.OAuth2 = config.OAuth2Config{
				ClientID: authBootstrapClientID,
				TenantID: authBootstrapTenantID,
			}
		}

		// The client secret goes to the OS keychain, never the config file.
		if err := persistOAuthClientSecret(accountName, authBootstrapClientSecret); err != nil {
			return err
		}

		config.UpsertAccount(cfg, account)
		if err := config.SaveConfig(cfg); err != nil {
			return err
		}
		path, pathErr := config.ConfigFilePath()
		if pathErr == nil {
			fmt.Printf("Account %q saved to %s\n", accountName, path)
		}

		if authAddLogin {
			if !config.SupportsBuiltInOAuth2(provider) {
				return errors.Errorf("provider %q does not support built-in OAuth2 login", provider)
			}
			if strings.TrimSpace(authBootstrapClientID) == "" || strings.TrimSpace(authBootstrapClientSecret) == "" {
				return errors.New("--client-id and --client-secret are required when using --login")
			}
			// Re-resolve so the client secret just stored in the keychain is loaded.
			resolved, resolvedOK := appcore.ResolveAccount(cfg, accountName)
			if !resolvedOK {
				return errors.Errorf("failed to resolve saved account %q", accountName)
			}
			return runOAuthLogin(context.Background(), accountName, resolved)
		}
		return nil
	},
}

// persistOAuthClientSecret stores an OAuth2 client secret in the OS keychain.
// Empty secrets are a no-op. The secret is never written to the config file.
func persistOAuthClientSecret(accountName, secret string) error {
	if strings.TrimSpace(secret) == "" {
		return nil
	}
	if err := keyring.Set(config.OAuthSecretKeyringService, accountName, secret); err != nil {
		return errors.Wrap(err, "failed to store oauth2 client secret in keychain")
	}
	return nil
}

func ensureOAuthAccountConfig(
	cfg *config.Config,
	accountName string,
	existing config.AccountConfig,
	exists bool,
) (config.AccountConfig, error) {
	provider := config.NormalizeProviderName(firstNonEmpty(authBootstrapProvider, existing.Provider, existing.OAuth2.Provider, accountName))
	if provider == "" {
		return config.AccountConfig{}, errors.Errorf(
			"account %q is not configured for OAuth2; rerun with --provider, --email, --client-id, and --client-secret or use pstr auth add <provider>",
			accountName,
		)
	}
	if !config.SupportsBuiltInOAuth2(provider) {
		return config.AccountConfig{}, errors.Errorf("provider %q does not support built-in OAuth2 login", provider)
	}
	email := firstNonEmpty(authBootstrapEmail, existing.Email)
	if strings.TrimSpace(email) == "" {
		return config.AccountConfig{}, errors.New("--email is required to bootstrap a new OAuth2 account")
	}
	name := firstNonEmpty(authBootstrapName, existing.Name, accountName)
	clientID := firstNonEmpty(authBootstrapClientID, existing.OAuth2.ClientID)
	clientSecret := firstNonEmpty(authBootstrapClientSecret, existing.OAuth2.ClientSecret())
	if strings.TrimSpace(clientID) == "" || strings.TrimSpace(clientSecret) == "" {
		return config.AccountConfig{}, errors.New("--client-id and --client-secret are required to bootstrap OAuth2 login")
	}

	// The client secret goes to the OS keychain, never the config file.
	if err := persistOAuthClientSecret(name, clientSecret); err != nil {
		return config.AccountConfig{}, err
	}

	account := config.AccountConfig{
		Name:     name,
		Provider: provider,
		Email:    email,
		Username: firstNonEmpty(existing.Username, email),
		OAuth2: config.OAuth2Config{
			ClientID:    clientID,
			TenantID:    firstNonEmpty(authBootstrapTenantID, existing.OAuth2.TenantID),
			RedirectURL: existing.OAuth2.RedirectURL,
		},
	}
	if exists {
		account.IMAP = existing.IMAP
		account.SMTP = existing.SMTP
		account.PasswordCmd = append([]string{}, existing.PasswordCmd...)
	}
	config.UpsertAccount(cfg, account)
	if err := config.SaveConfig(cfg); err != nil {
		return config.AccountConfig{}, err
	}

	updated, ok := appcore.ResolveAccount(cfg, account.Name)
	if !ok {
		return config.AccountConfig{}, errors.Errorf("failed to resolve saved account %q", account.Name)
	}
	return updated, nil
}

func runOAuthLogin(ctx context.Context, accountName string, account config.AccountConfig) error {
	if strings.TrimSpace(account.OAuth2.ClientID) == "" {
		return fmt.Errorf("account %q is not configured for OAuth2 (missing client_id)", accountName)
	}

	oauthConfig := config.GetOAuthConfig(&account.OAuth2)
	if strings.TrimSpace(oauthConfig.ClientID) == "" || oauthConfig.Endpoint.AuthURL == "" {
		return fmt.Errorf("account %q does not have a complete built-in OAuth2 configuration", accountName)
	}

	// 1. Give the user a URL
	url := oauthConfig.AuthCodeURL("state-token", config.GetAuthCodeOptions(&account.OAuth2)...)
	fmt.Printf("Visit the URL below to authorize Postero:\n\n%s\n\n", url)

	// 2. Wait for the Auth code
	fmt.Printf("Enter the authorization code: ")
	var code string
	if _, err := fmt.Scan(&code); err != nil {
		return errors.Wrap(err, "failed to read authorization code")
	}

	code = strings.TrimSpace(code)

	// 3. Exchange
	token, err := oauthConfig.Exchange(ctx, code)
	if err != nil {
		return errors.Wrap(err, "failed to exchange token")
	}

	// 4. Save to keyring
	data, err := json.Marshal(token)
	if err != nil {
		return errors.Wrap(err, "failed to marshal token")
	}

	if err := keyring.Set("postero-oauth2", accountName, string(data)); err != nil {
		return errors.Wrap(err, "failed to save oauth2 token to keyring")
	}

	fmt.Printf("OAuth2 token for '%s' successfully acquired and saved to Keychain.\n", accountName)
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func init() {
	authAddCmd.Flags().StringVar(&authBootstrapEmail, "email", "", "account email address")
	authAddCmd.Flags().StringVar(&authBootstrapName, "name", "", "account name")
	authAddCmd.Flags().StringVar(&authBootstrapClientID, "client-id", "", "OAuth2 client ID")
	authAddCmd.Flags().StringVar(&authBootstrapClientSecret, "client-secret", "", "OAuth2 client secret")
	authAddCmd.Flags().StringVar(&authBootstrapTenantID, "tenant-id", "", "OAuth2 tenant ID for Microsoft accounts")
	authAddCmd.Flags().BoolVar(&authAddLogin, "login", false, "start the OAuth2 login flow immediately after saving the account")

	authLoginCmd.Flags().StringVar(&authBootstrapProvider, "provider", "", "provider preset to use when bootstrapping a missing account")
	authLoginCmd.Flags().
		StringVar(&authBootstrapEmail, "email", "", "account email address to save when bootstrapping a missing OAuth2 account")
	authLoginCmd.Flags().StringVar(&authBootstrapName, "name", "", "account name to save when bootstrapping a missing OAuth2 account")
	authLoginCmd.Flags().StringVar(&authBootstrapClientID, "client-id", "", "OAuth2 client ID to save when bootstrapping a missing account")
	authLoginCmd.Flags().
		StringVar(&authBootstrapClientSecret, "client-secret", "", "OAuth2 client secret to save when bootstrapping a missing account")
	authLoginCmd.Flags().StringVar(&authBootstrapTenantID, "tenant-id", "", "OAuth2 tenant ID for Microsoft accounts")

	authCmd.AddCommand(authSetCmd)
	authCmd.AddCommand(authSetAICmd)
	authCmd.AddCommand(authSetSecretCmd)
	authCmd.AddCommand(authDelCmd)
	authCmd.AddCommand(authAddCmd)
	authCmd.AddCommand(authLoginCmd)
}
