package config

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"
	"golang.org/x/oauth2"
)

func TestGetTokenWithMockAuth(t *testing.T) {
	keyring.MockInit() // Use mock keyring

	// Start a mock OAuth2 server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		assert.Equal(t, "POST", r.Method)

		response := oauth2.Token{
			AccessToken:  "mock-access-token-new",
			TokenType:    "Bearer",
			RefreshToken: "mock-refresh-token",
			Expiry:       time.Now().Add(1 * time.Hour),
		}

		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// We need to override the endpoint dynamically just for this test
	mockEndpoint := oauth2.Endpoint{
		AuthURL:  server.URL + "/auth",
		TokenURL: server.URL + "/token",
	}

	// Pre-seed an expired token
	expiredToken := &oauth2.Token{
		AccessToken:  "expired-token",
		RefreshToken: "mock-refresh-token",
		Expiry:       time.Now().Add(-1 * time.Hour), // Expired
	}
	expiredData, _ := json.Marshal(expiredToken)
	err := keyring.Set(keyringService, "test-account", string(expiredData))
	require.NoError(t, err)

	oauthCfg := &oauth2.Config{
		ClientID:     "my-client",
		ClientSecret: "my-secret",
		Endpoint:     mockEndpoint,
	}

	tokenSource := oauthCfg.TokenSource(context.Background(), expiredToken)
	newToken, err := tokenSource.Token()
	require.NoError(t, err)

	assert.Equal(t, "mock-access-token-new", newToken.AccessToken)
}

func TestGetTokenWithFreshToken(t *testing.T) {
	keyring.MockInit()

	freshToken := &oauth2.Token{
		AccessToken:  "fresh-access-token",
		RefreshToken: "mock-refresh-token",
		Expiry:       time.Now().Add(1 * time.Hour), // Valid for 1 hour
	}
	freshData, _ := json.Marshal(freshToken)
	err := keyring.Set(keyringService, "fresh-account", string(freshData))
	require.NoError(t, err)

	cfg := &OAuth2Config{
		Provider: "gmail",
	}

	// This should not hit any network since the token is valid
	token, err := GetToken(context.Background(), "fresh-account", cfg)
	require.NoError(t, err)
	assert.Equal(t, "fresh-access-token", token)
}

func TestGetTokenNoToken(t *testing.T) {
	keyring.MockInit()

	cfg := &OAuth2Config{
		Provider: "gmail",
	}

	_, err := GetToken(context.Background(), "no-token-account", cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no oauth2 token found")
}
