package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildInitConfigDefaultsGmailToOAuth2(t *testing.T) {
	configDoc, guidance, err := buildInitConfig("google", "", "", false)

	require.NoError(t, err)
	accounts := configDoc["accounts"].([]map[string]any)
	require.Len(t, accounts, 1)
	account := accounts[0]
	assert.Equal(t, "gmail", account["provider"])
	assert.Equal(t, "gmail", account["name"])
	assert.Equal(t, "your.name@gmail.com", account["email"])
	oauth, hasOAuth := account["oauth2"].(map[string]any)
	assert.True(t, hasOAuth)
	_, hasPassword := account["password"]
	assert.False(t, hasPassword)
	_, hasClientSecret := oauth["client_secret"]
	assert.False(t, hasClientSecret, "the client secret must not be written to the generated config")
	assert.Contains(t, guidance, "pstr auth login gmail")
}

func TestBuildInitConfigNonOAuthProviderOmitsSecret(t *testing.T) {
	configDoc, guidance, err := buildInitConfig("fastmail", "me@example.com", "work", false)

	require.NoError(t, err)
	account := configDoc["accounts"].([]map[string]any)[0]
	assert.Equal(t, "work", account["name"])
	assert.Equal(t, "me@example.com", account["email"])
	_, hasPassword := account["password"]
	assert.False(t, hasPassword, "no plaintext password may be written to the generated config")
	_, hasOAuth := account["oauth2"]
	assert.False(t, hasOAuth)
	assert.Contains(t, guidance, "pstr auth set work")
}

func TestBuildInitConfigSupportsYandex(t *testing.T) {
	configDoc, guidance, err := buildInitConfig("yandex", "", "", false)

	require.NoError(t, err)
	account := configDoc["accounts"].([]map[string]any)[0]
	assert.Equal(t, "yandex", account["provider"])
	assert.Equal(t, "your.name@yandex.com", account["email"])
	assert.Contains(t, guidance, "pstr auth set yandex")
}

func TestBuildInitConfigRejectsUnsupportedProvider(t *testing.T) {
	_, _, err := buildInitConfig("unknown", "", "", false)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported provider")
}

func TestBuildInitConfigRejectsOAuth2ForProviderWithoutPreset(t *testing.T) {
	_, _, err := buildInitConfig("fastmail", "", "", true)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not have a built-in OAuth2 preset")
}

func TestPlaceholderEmailFallback(t *testing.T) {
	assert.Equal(t, "you@example.com", placeholderEmail("custom"))
}
