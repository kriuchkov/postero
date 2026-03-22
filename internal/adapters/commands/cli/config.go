package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/go-faster/errors"
	appcore "github.com/kriuchkov/postero/internal/app"
	appconfig "github.com/kriuchkov/postero/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	configInitEmail  string
	configInitName   string
	configInitOAuth2 bool
	configInitOutput string
	configInitForce  bool
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Generate and validate Postero config",
	Long:  `Generate starter configuration snippets and validate existing Postero config files.`,
}

var configInitCmd = &cobra.Command{
	Use:   "init [provider]",
	Short: "Generate starter config for a provider",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		provider := strings.TrimSpace(strings.ToLower(args[0]))
		configDoc, err := buildInitConfig(provider, configInitEmail, configInitName, configInitOAuth2)
		if err != nil {
			return err
		}

		payload, err := yaml.Marshal(configDoc)
		if err != nil {
			return errors.Wrap(err, "marshal config")
		}

		if strings.TrimSpace(configInitOutput) == "" {
			_, err = cmd.OutOrStdout().Write(payload)
			return err
		}

		outputPath := configInitOutput
		if !filepath.IsAbs(outputPath) {
			cwd, cwdErr := os.Getwd()
			if cwdErr != nil {
				return errors.Wrap(cwdErr, "resolve output path")
			}
			outputPath = filepath.Join(cwd, outputPath)
		}

		if !configInitForce {
			if _, statErr := os.Stat(outputPath); statErr == nil {
				return errors.Errorf("output file %s already exists; use --force to overwrite", outputPath)
			}
		}

		if err := os.MkdirAll(filepath.Dir(outputPath), 0o700); err != nil {
			return errors.Wrap(err, "create config directory")
		}
		if err := os.WriteFile(outputPath, payload, 0o600); err != nil {
			return errors.Wrap(err, "write config file")
		}

		cmd.Printf("Wrote starter config to %s\n", outputPath)
		return nil
	},
}

var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate the current config file",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := appcore.LoadConfig()
		if err != nil {
			return err
		}

		usedConfigPath := strings.TrimSpace(appconfig.UsedConfigFile())
		persistentConfigPath, err := appconfig.ConfigFilePath()
		if err != nil {
			return errors.Wrap(err, "resolve config file path")
		}

		if usedConfigPath == "" {
			cmd.Println("Loaded config file: none (using environment variables and defaults)")
		} else {
			cmd.Printf("Loaded config file: %s\n", usedConfigPath)
		}
		cmd.Printf("Persistent config path: %s\n", persistentConfigPath)

		issues := appconfig.ValidateConfig(cfg)
		if len(issues) == 0 {
			cmd.Println("Config is valid.")
			return nil
		}

		hasErrors := false
		for _, issue := range issues {
			if issue.IsError() {
				hasErrors = true
			}
			cmd.Printf("[%s] %s: %s\n", strings.ToUpper(issue.Severity), issue.Path, issue.Message)
			if strings.TrimSpace(issue.Hint) != "" {
				cmd.Printf("  hint: %s\n", issue.Hint)
			}
		}

		if hasErrors {
			return errors.New("configuration validation failed")
		}
		return nil
	},
}

func buildInitConfig(provider, email, name string, oauth2 bool) (map[string]any, error) {
	provider = strings.TrimSpace(strings.ToLower(provider))
	canonical := map[string]string{
		"google":    "gmail",
		"gmail":     "gmail",
		"outlook":   "outlook",
		"microsoft": "outlook",
		"yahoo":     "yahoo",
		"icloud":    "icloud",
		"fastmail":  "fastmail",
	}[provider]
	if canonical == "" {
		return nil, errors.Errorf("unsupported provider %q", provider)
	}

	if strings.TrimSpace(email) == "" {
		email = placeholderEmail(canonical)
	}
	if strings.TrimSpace(name) == "" {
		name = canonical
	}

	if oauth2 && !appconfig.SupportsBuiltInOAuth2(canonical) {
		return nil, errors.Errorf("provider %q does not have a built-in OAuth2 preset", canonical)
	}
	if !oauth2 && (canonical == "gmail" || canonical == "outlook") {
		oauth2 = true
	}

	account := map[string]any{
		"name":     name,
		"provider": canonical,
		"email":    email,
	}
	if oauth2 {
		account["oauth2"] = map[string]any{
			"client_id":     "your-client-id",
			"client_secret": "your-client-secret",
		}
	} else {
		account["password"] = "your-app-password"
	}

	return map[string]any{
		"accounts": []map[string]any{account},
	}, nil
}

func placeholderEmail(provider string) string {
	switch provider {
	case "gmail":
		return "your.name@gmail.com"
	case "outlook":
		return "your.name@outlook.com"
	case "yahoo":
		return "your.name@yahoo.com"
	case "icloud":
		return "your.name@icloud.com"
	case "fastmail":
		return "your.name@fastmail.com"
	default:
		return "you@example.com"
	}
}

func init() {
	configInitCmd.Flags().StringVar(&configInitEmail, "email", "", "email address for the generated account")
	configInitCmd.Flags().StringVar(&configInitName, "name", "", "account name for the generated config")
	configInitCmd.Flags().BoolVar(&configInitOAuth2, "oauth2", false, "include an OAuth2 block when the provider supports it")
	configInitCmd.Flags().StringVar(&configInitOutput, "output", "", "write the generated config to a file instead of stdout")
	configInitCmd.Flags().BoolVar(&configInitForce, "force", false, "overwrite an existing output file")

	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configValidateCmd)
}
