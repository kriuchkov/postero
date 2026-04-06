package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildInitConfigDefaultsGmailToOAuth2(t *testing.T) {
	configDoc, err := buildInitConfig("google", "", "", false)

	require.NoError(t, err)
	accounts := configDoc["accounts"].([]map[string]any)
	require.Len(t, accounts, 1)
	account := accounts[0]
	assert.Equal(t, "gmail", account["provider"])
	assert.Equal(t, "gmail", account["name"])
	assert.Equal(t, "your.name@gmail.com", account["email"])
	_, hasOAuth := account["oauth2"]
	_, hasPassword := account["password"]
	assert.True(t, hasOAuth)
	assert.False(t, hasPassword)
}

func TestBuildInitConfigUsesPasswordForNonOAuthProvider(t *testing.T) {
	configDoc, err := buildInitConfig("fastmail", "me@example.com", "work", false)

	require.NoError(t, err)
	account := configDoc["accounts"].([]map[string]any)[0]
	assert.Equal(t, "work", account["name"])
	assert.Equal(t, "me@example.com", account["email"])
	assert.Equal(t, "your-app-password", account["password"])
	_, hasOAuth := account["oauth2"]
	assert.False(t, hasOAuth)
}

func TestBuildInitConfigRejectsUnsupportedProvider(t *testing.T) {
	_, err := buildInitConfig("unknown", "", "", false)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported provider")
}

func TestBuildInitConfigRejectsOAuth2ForProviderWithoutPreset(t *testing.T) {
	_, err := buildInitConfig("fastmail", "", "", true)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not have a built-in OAuth2 preset")
}

func TestPlaceholderEmailFallback(t *testing.T) {
	assert.Equal(t, "you@example.com", placeholderEmail("custom"))
}
