package tui

import (
	"context"
	"testing"

	"github.com/kriuchkov/postero/internal/core/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComposeAccountLabelFallsBackToSender(t *testing.T) {
	m := testModel()
	m.activeDraft = &models.MessageDTO{AccountID: "", From: "", Subject: "Hello"}
	m.defaultAcctID = "work"

	assert.Equal(t, "work <work@example.com>", m.composeAccountLabel())
}

func TestSelectedMessageReturnsFalseWithoutValidSelection(t *testing.T) {
	m := testModel()
	m.state = stateSidebar

	_, ok := m.selectedMessage()
	assert.False(t, ok)
}

func TestTrimRecipientsDropsEmptyValues(t *testing.T) {
	assert.Equal(t, []string{"one@example.com", "two@example.com"}, trimRecipients([]string{" one@example.com ", "", "   ", "two@example.com"}))
}

func TestPersistActiveDraftCreatesDraftWithFallbackSender(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages(), composedDraftID: "draft-new"}
	m := testModelWithService(service)
	m.defaultAcctID = "work"
	m.activeDraft = &models.MessageDTO{
		AccountID: "",
		From:      "",
		To:        []string{" user@example.com ", ""},
		Cc:        []string{" cc@example.com "},
		Bcc:       []string{"  "},
		Subject:   "  Hello  ",
		Body:      "Body",
	}

	draft, err := m.persistActiveDraft(context.Background())

	require.NoError(t, err)
	require.NotNil(t, draft)
	require.Len(t, service.composeCalls, 1)
	assert.Equal(t, "work", service.composeCalls[0].AccountID)
	assert.Equal(t, "work@example.com", service.composeCalls[0].From)
	assert.Equal(t, []string{"user@example.com"}, service.composeCalls[0].To)
	assert.Equal(t, []string{"cc@example.com"}, service.composeCalls[0].Cc)
	assert.Empty(t, service.composeCalls[0].Bcc)
	assert.Equal(t, "Hello", service.composeCalls[0].Subject)
	assert.Equal(t, "draft-new", draft.ID)
	assert.Equal(t, "work", draft.AccountID)
	assert.Equal(t, "work@example.com", draft.From)
}

func TestPersistActiveDraftUpdatePreservesLocalFallbackValues(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages()}
	m := testModelWithService(service)
	m.activeDraft = &models.MessageDTO{
		ID:        "draft-1",
		AccountID: "personal",
		From:      "me@example.com",
		To:        []string{" user@example.com "},
		Cc:        []string{" copy@example.com "},
		Bcc:       []string{" blind@example.com "},
		Subject:   "  Subject  ",
		Body:      "Draft body",
	}

	draft, err := m.persistActiveDraft(context.Background())

	require.NoError(t, err)
	require.NotNil(t, draft)
	require.Len(t, service.updateDraftCalls, 1)
	assert.Equal(t, "draft-1", service.updateDraftCalls[0].id)
	assert.Equal(t, "personal", draft.AccountID)
	assert.Equal(t, "me@example.com", draft.From)
	assert.Equal(t, "Subject", draft.Subject)
	assert.Equal(t, []string{"user@example.com"}, draft.To)
	assert.Equal(t, []string{"copy@example.com"}, draft.Cc)
	assert.Equal(t, []string{"blind@example.com"}, draft.Bcc)
	assert.Equal(t, "Draft body", draft.Body)
}

func TestSenderForAccountFallsBackToDefaultSender(t *testing.T) {
	m := testModel()

	assert.Equal(t, "me@example.com", m.senderForAccount("missing"))
	assert.Equal(t, "me@example.com", m.senderForAccount(""))
	assert.Equal(t, "work@example.com", m.senderForAccount("work"))
}

func TestCycleComposeAccountWrapsAcrossAccounts(t *testing.T) {
	m := testModel()
	m.activeDraft = &models.MessageDTO{AccountID: "personal", From: "me@example.com"}

	m.cycleComposeAccount(-1)
	assert.Equal(t, "work", m.activeDraft.AccountID)
	assert.Equal(t, "work@example.com", m.activeDraft.From)

	m.cycleComposeAccount(1)
	assert.Equal(t, "personal", m.activeDraft.AccountID)
	assert.Equal(t, "me@example.com", m.activeDraft.From)
}

func TestCycleComposeAccountNoOpWithoutDraft(t *testing.T) {
	m := testModel()
	m.activeDraft = nil

	m.cycleComposeAccount(1)

	assert.Nil(t, m.activeDraft)
}
