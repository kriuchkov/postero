package tui

import (
	"context"
	"errors"
	"os"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"

	"github.com/kriuchkov/postero/internal/config"
	"github.com/kriuchkov/postero/internal/core/models"
)

func typeString(t *testing.T, m Model, value string) Model {
	t.Helper()
	return updateModel(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)})
}

// memStore is a minimal in-memory MessageRepository for wizard tests.
type memStore struct{ saved []*models.Message }

func (s *memStore) GetByID(context.Context, string) (*models.Message, error) {
	return nil, errors.New("not implemented")
}
func (s *memStore) List(context.Context, int, int) ([]*models.Message, error) {
	return nil, nil
}
func (s *memStore) Count(context.Context, models.SearchCriteria) (int, error) {
	return 0, nil
}
func (s *memStore) Search(context.Context, models.SearchCriteria) ([]*models.Message, error) {
	return nil, nil
}
func (s *memStore) Save(_ context.Context, m *models.Message) error {
	s.saved = append(s.saved, m)
	return nil
}
func (s *memStore) Delete(context.Context, string) error     { return nil }
func (s *memStore) MarkAsRead(context.Context, string) error { return nil }
func (s *memStore) MarkAsSpam(context.Context, string) error { return nil }

func setupTestModel(t *testing.T) Model {
	t.Helper()
	t.Setenv("POSTERO_CONFIG_DIR", t.TempDir())
	keyring.MockInit()

	service := &messageServiceStub{}
	m := testModelWithService(service)
	m.store = &memStore{}
	m.enterSetupState()
	return m
}

func stubKeyring(t *testing.T, err error) *[3]string {
	t.Helper()
	var captured [3]string
	original := keyringSetPassword
	keyringSetPassword = func(service, user, secret string) error {
		captured = [3]string{service, user, secret}
		return err
	}
	t.Cleanup(func() { keyringSetPassword = original })
	return &captured
}

func TestSetupWizardAddsKnownProviderAccount(t *testing.T) {
	captured := stubKeyring(t, nil)
	m := setupTestModel(t)
	require.Equal(t, stateSetup, m.state)

	m = typeString(t, m, "alice@gmail.com")
	assert.Equal(t, "gmail", m.setupProvider, "provider must be detected while typing")

	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, setupFieldPassword, m.setupFocus)

	m = typeString(t, m, "app-secret")
	updated, cmd := updateModelWithCmd(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	assert.Equal(t, stateList, updated.state, "wizard must land in the inbox")
	assert.Contains(t, updated.statusMessage, "alice@gmail.com added")
	assert.Equal(t, [3]string{"postero", "alice", "app-secret"}, *captured)
	require.NotNil(t, cmd, "a sync must start after setup")

	require.NotNil(t, updated.config)
	require.Len(t, updated.config.Accounts, 1)
	account := updated.config.Accounts[0]
	assert.Equal(t, "alice", account.Name)
	assert.Equal(t, "imap.gmail.com", account.IMAP.Host, "provider preset must be applied")
	assert.Equal(t, "smtp.gmail.com", account.SMTP.Host)
	assert.Contains(t, updated.sidebarItems, "  alice", "sidebar must show the new account")

	path, err := config.ConfigFilePath()
	require.NoError(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "app-secret", "secret must not reach the config file")
}

func TestSetupWizardUnknownProviderAsksForHosts(t *testing.T) {
	stubKeyring(t, nil)
	m := setupTestModel(t)

	m = typeString(t, m, "user@corp.dev")
	assert.Empty(t, m.setupProvider)
	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // -> password
	m = typeString(t, m, "pw")
	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // -> imap host
	assert.Equal(t, setupFieldIMAPHost, m.setupFocus)
	m = typeString(t, m, "mail.corp.dev")
	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // -> smtp host
	m = typeString(t, m, "smtp.corp.dev")
	updated, _ := updateModelWithCmd(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	assert.Equal(t, stateList, updated.state)
	require.Len(t, updated.config.Accounts, 1)
	account := updated.config.Accounts[0]
	assert.Equal(t, "mail.corp.dev", account.IMAP.Host)
	assert.Equal(t, 993, account.IMAP.Port)
	assert.True(t, account.IMAP.TLS)
	assert.Equal(t, "smtp.corp.dev", account.SMTP.Host)
	assert.Equal(t, 587, account.SMTP.Port)
}

func TestSetupWizardRequiresKeychainAndNeverStoresPlaintext(t *testing.T) {
	stubKeyring(t, errors.New("no keychain"))
	m := setupTestModel(t)

	m = typeString(t, m, "bob@icloud.com")
	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = typeString(t, m, "should-never-persist")
	updated, _ := updateModelWithCmd(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	// Without a keychain the wizard refuses rather than writing plaintext.
	assert.Equal(t, stateSetup, updated.state, "wizard must not proceed without a keychain")
	assert.Contains(t, updated.setupErr, "keychain")
	if updated.config != nil {
		assert.Empty(t, updated.config.Accounts, "no account may be saved when the secret cannot be stored securely")
	}

	path, err := config.ConfigFilePath()
	require.NoError(t, err)
	if data, readErr := os.ReadFile(path); readErr == nil {
		assert.NotContains(t, string(data), "should-never-persist", "a typed password must never reach the config file")
	}
}

func TestSetupWizardValidatesInput(t *testing.T) {
	stubKeyring(t, nil)
	m := setupTestModel(t)

	m = typeString(t, m, "not-an-email")
	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, stateSetup, m.state)
	assert.Contains(t, m.setupErr, "valid email")

	m = typeString(t, m, "@gmail.com") // now "not-an-email@gmail.com"
	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // empty password
	assert.Equal(t, stateSetup, m.state)
	assert.Contains(t, m.setupErr, "password")
}

func TestSetupWizardEscSkips(t *testing.T) {
	stubKeyring(t, nil)
	m := setupTestModel(t)

	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, stateSidebar, m.state)
	assert.Contains(t, m.statusMessage, ":setup")
}

func TestSetupWizardDemoButtonLoadsSampleInbox(t *testing.T) {
	m := setupTestModel(t)
	require.Equal(t, stateSetup, m.state)

	updated, cmd := updateModelWithCmd(t, m, tea.KeyMsg{Type: tea.KeyCtrlD})
	assert.Equal(t, stateList, updated.state)
	assert.Contains(t, updated.statusMessage, "Demo mode")
	assert.Equal(t, []string{"demo"}, updated.accountNames, "demo sets an in-memory account")
	assert.Contains(t, updated.sidebarItems, "  demo")
	assert.False(t, updated.canSyncAccounts(), "demo must not attempt a real IMAP sync")
	require.NotNil(t, cmd, "demo seeding must run")

	// Draining the seed command yields a demoSeededMsg that reports the count.
	seeded := updateModel(t, updated, cmd())
	assert.Contains(t, seeded.statusMessage, "sample messages loaded")
}

func TestDemoCommandFromListEntersDemo(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages()}
	m := testModelWithService(service)
	m.state = stateList

	m = updateModel(t, m, keyRune(':'))
	for _, r := range "demo" {
		m = updateModel(t, m, keyRune(r))
	}
	updated, cmd := updateModelWithCmd(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	assert.Equal(t, stateList, updated.state)
	assert.Equal(t, []string{"demo"}, updated.accountNames)
	require.NotNil(t, cmd)
}

func TestSetupCommandReopensWizard(t *testing.T) {
	stubKeyring(t, nil)
	service := &messageServiceStub{inbox: sampleMessages()}
	m := testModelWithService(service)
	m.state = stateList

	m = updateModel(t, m, keyRune(':'))
	for _, r := range "setup" {
		m = updateModel(t, m, keyRune(r))
	}
	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	assert.Equal(t, stateSetup, m.state)
	assert.NotEmpty(t, renderSetupView(m), "setup view must render")
}
