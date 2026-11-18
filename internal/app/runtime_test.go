package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kriuchkov/postero/internal/config"
)

func TestResolveAccount(t *testing.T) {
	cfg := &config.Config{
		Accounts: []config.AccountConfig{
			{Name: "work", Email: "work@example.com"},
			{Name: "personal", Email: "me@example.com"},
		},
	}

	// Test Empty Selector fallback
	acc, ok := ResolveAccount(cfg, "")
	assert.True(t, ok)
	assert.Equal(t, "work", acc.Name)

	// Test by Name
	acc, ok = ResolveAccount(cfg, "Personal")
	assert.True(t, ok)
	assert.Equal(t, "personal", acc.Name)

	// Test by email
	acc, ok = ResolveAccount(cfg, "work@example.com")
	assert.True(t, ok)
	assert.Equal(t, "work", acc.Name)

	// Test missing
	acc, ok = ResolveAccount(cfg, "missing")
	assert.False(t, ok)
	assert.Empty(t, acc.Name)

	// Test nil config
	_, ok = ResolveAccount(nil, "work")
	assert.False(t, ok)
}

func TestDefaultSender(t *testing.T) {
	cfg := &config.Config{
		Accounts: []config.AccountConfig{
			{Name: "first", Email: "first@example.com"},
		},
	}
	id, email := DefaultSender(cfg)
	assert.Equal(t, "first", id)
	assert.Equal(t, "first@example.com", email)

	id, email = DefaultSender(nil)
	assert.Empty(t, id)
	assert.Empty(t, email)
}
