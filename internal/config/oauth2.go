package config

import (
	"context"
	"encoding/json"

	"github.com/go-faster/errors"
	"github.com/zalando/go-keyring"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/microsoft"
)

const keyringService = "postero-oauth2"

func GetOAuthConfig(acc *OAuth2Config) *oauth2.Config {
	var endpoint oauth2.Endpoint

	switch canonicalProviderName(acc.Provider) {
	case "gmail":
		endpoint = google.Endpoint
	case "outlook":
		tenant := acc.TenantID
		if tenant == "" {
			tenant = "common"
		}
		endpoint = microsoft.AzureADEndpoint(tenant)
	default:
		endpoint = oauth2.Endpoint{}
	}

	redirectURL := acc.RedirectURL
	if redirectURL == "" {
		redirectURL = "http://localhost:8080" // or urn:ietf:wg:oauth:2.0:oob if supported
	}

	return &oauth2.Config{
		ClientID:     acc.ClientID,
		ClientSecret: acc.ClientSecret,
		Endpoint:     endpoint,
		Scopes:       acc.Scopes,
		RedirectURL:  redirectURL,
	}
}

func GetAuthCodeOptions(_ *OAuth2Config) []oauth2.AuthCodeOption {
	return []oauth2.AuthCodeOption{
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
	}
}

func GetToken(ctx context.Context, accountName string, cfg *OAuth2Config) (string, error) {
	data, err := keyring.Get(keyringService, accountName)
	if err != nil {
		return "", errors.Wrapf(err, "no oauth2 token found for account %s", accountName)
	}

	var token oauth2.Token
	if err := json.Unmarshal([]byte(data), &token); err != nil {
		return "", errors.Wrapf(err, "corrupted oauth2 token for account %s", accountName)
	}

	oauthConfig := GetOAuthConfig(cfg)
	tokenSource := oauthConfig.TokenSource(ctx, &token)

	newToken, err := tokenSource.Token()
	if err != nil {
		return "", errors.Wrap(err, "failed to refresh oauth2 token")
	}

	if newToken.AccessToken != token.AccessToken {
		newData, _ := json.Marshal(newToken)
		_ = keyring.Set(keyringService, accountName, string(newData))
	}

	return newToken.AccessToken, nil
}
