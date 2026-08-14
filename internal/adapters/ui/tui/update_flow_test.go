package tui

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kriuchkov/postero/internal/core/models"
)

func TestComposeKeyEntersComposeState(t *testing.T) {
	m := testModel()

	updated := updateModel(t, m, keyRune('c'))

	assert.Equal(t, stateCompose, updated.state)
	assert.NotNil(t, updated.activeDraft)
	assert.Equal(t, 0, updated.focusIndex)
	assert.Empty(t, updated.activeDraft.Subject)
	assert.False(t, updated.composeEditing)
}

func TestReplyKeyBuildsReplyDraft(t *testing.T) {
	m := testModel()
	m.state = stateList

	updated := updateModel(t, m, keyRune('r'))

	assert.Equal(t, stateCompose, updated.state)
	assert.NotNil(t, updated.activeDraft)
	assert.Equal(t, 3, updated.focusIndex)
	assert.Contains(t, updated.activeDraft.Subject, "Re:")
	assert.NotEmpty(t, updated.activeDraft.To)
	assert.Contains(t, updated.activeDraft.Body, "wrote:")
	assert.False(t, updated.bodyInput.ShowLineNumbers)
	assert.Equal(t, "Reply", updated.composeTitle)
	assert.True(t, updated.composeEditing)
}

func TestReplyAllSetsComposeContext(t *testing.T) {
	m := testModel()
	m.state = stateList

	updated := updateModel(t, m, keyRune('R'))

	assert.Equal(t, "Reply all", updated.composeTitle)
	assert.Equal(t, "Type above the quoted message.", updated.composeHint)
}

func TestReplyStartsTypingAboveQuotedMessage(t *testing.T) {
	m := testModel()
	m.state = stateList

	updated := updateModel(t, m, keyRune('r'))
	updated = updateModel(t, updated, keyRune('O'))

	assert.Equal(t, stateCompose, updated.state)
	assert.NotNil(t, updated.activeDraft)
	assert.Contains(t, updated.activeDraft.Body, "On ")
	assert.True(t, strings.HasPrefix(updated.activeDraft.Body, "O"))
}

func TestEnterAdvancesComposeFocusBeforeBody(t *testing.T) {
	m := testModel()
	m.enterComposeState(
		&models.Message{AccountID: "personal", From: "me@example.com", To: []string{"user@example.com"}, Subject: "Hello", Body: "Body"},
		0,
	)

	updated := updateModel(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, 1, updated.focusIndex)
	assert.True(t, updated.composeEditing)

	updated = updateModel(t, updated, tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, 1, updated.focusIndex)
	assert.False(t, updated.composeEditing)

	updated = updateModel(t, updated, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, 1, updated.focusIndex)
	assert.True(t, updated.composeEditing)

	// Enter keeps writing mode active while advancing to the next field.
	updated = updateModel(t, updated, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, 2, updated.focusIndex)
	assert.True(t, updated.composeEditing)

	updated = updateModel(t, updated, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, 3, updated.focusIndex)
	assert.True(t, updated.composeEditing)

	updated = updateModel(t, updated, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, 3, updated.focusIndex)
	assert.True(t, updated.composeEditing)
}

func TestEnterInBodyAddsNewLineInsteadOfSending(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages(), composedDraftID: "draft-42"}
	m := testModelWithService(service)
	m.enterComposeState(&models.Message{To: []string{"user@example.com"}, Subject: "Hello", Body: "Body"}, 3)
	m.composeEditing = true
	m.applyComposeFocus()

	updated := updateModel(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	assert.Equal(t, stateCompose, updated.state)
	assert.Empty(t, service.sendCalls)
	assert.NotNil(t, updated.activeDraft)
	assert.Contains(t, updated.activeDraft.Body, "Body\n")
}

func TestEditKeyEntersWritingMode(t *testing.T) {
	m := testModel()
	m.enterComposeState(&models.Message{To: []string{"user@example.com"}, Subject: "Hello", Body: "Body"}, 1)

	updated := updateModel(t, m, keyRune('i'))

	assert.True(t, updated.composeEditing)
	assert.Equal(t, 1, updated.focusIndex)
}

func TestEscExitsWritingModeBeforeCancellingCompose(t *testing.T) {
	m := testModel()
	m.enterComposeState(&models.Message{To: []string{"user@example.com"}, Subject: "Hello", Body: "Body"}, 3)
	m.composeEditing = true
	m.applyComposeFocus()

	updated := updateModel(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, stateCompose, updated.state)
	assert.False(t, updated.composeEditing)
	assert.Equal(t, "Exited writing mode", updated.statusMessage)

	updated = updateModel(t, updated, tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, stateList, updated.state)
	assert.Equal(t, "Compose cancelled", updated.statusMessage)
}

func TestForwardSetsComposeContext(t *testing.T) {
	m := testModel()
	m.state = stateList

	updated := updateModel(t, m, keyRune('f'))

	assert.Equal(t, stateCompose, updated.state)
	assert.Equal(t, 1, updated.focusIndex)
	assert.Equal(t, "Forward", updated.composeTitle)
	assert.Equal(t, "Add recipients, then edit the forwarded message below.", updated.composeHint)
	assert.True(t, updated.composeEditing)
}

func TestForwardKeyBuildsForwardDraft(t *testing.T) {
	m := testModel()
	m.state = stateList

	updated := updateModel(t, m, keyRune('f'))

	assert.Equal(t, stateCompose, updated.state)
	assert.NotNil(t, updated.activeDraft)
	assert.Equal(t, 1, updated.focusIndex)
	assert.Contains(t, updated.activeDraft.Subject, "Fwd:")
	assert.Contains(t, updated.activeDraft.Body, "Forwarded message")
}

func TestReplyAllKeyBuildsReplyAllDraft(t *testing.T) {
	m := testModel()
	m.state = stateList

	updated := updateModel(t, m, keyRune('R'))

	assert.Equal(t, stateCompose, updated.state)
	assert.NotNil(t, updated.activeDraft)
	assert.Equal(t, 3, updated.focusIndex)
	assert.Contains(t, updated.activeDraft.Subject, "Re:")
	assert.Contains(t, updated.activeDraft.To[0], "sender1@example.com")
	assert.Contains(t, updated.activeDraft.Cc[0], "copy@example.com")
}

func TestComposeAccountCyclesWithArrowKeys(t *testing.T) {
	m := testModel()
	m.enterComposeState(&models.Message{AccountID: "personal", From: "me@example.com", Subject: "Hello"}, 0)

	updated := updateModel(t, m, tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, "work", updated.activeDraft.AccountID)
	assert.Equal(t, "work@example.com", updated.activeDraft.From)

	updated = updateModel(t, updated, tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, "personal", updated.activeDraft.AccountID)
	assert.Equal(t, "me@example.com", updated.activeDraft.From)
}

func TestComposeJKMovesFocusInNormalMode(t *testing.T) {
	m := testModel()
	m.enterComposeState(&models.Message{AccountID: "personal", From: "me@example.com", Subject: "Hello", Body: "Body"}, 0)

	updated := updateModel(t, m, keyRune('j'))
	assert.Equal(t, 1, updated.focusIndex)
	assert.False(t, updated.composeEditing)

	updated = updateModel(t, updated, keyRune('j'))
	assert.Equal(t, 2, updated.focusIndex)

	updated = updateModel(t, updated, keyRune('k'))
	assert.Equal(t, 1, updated.focusIndex)
}

func TestComposeHLCyclesAccountInNormalMode(t *testing.T) {
	m := testModel()
	m.enterComposeState(&models.Message{AccountID: "personal", From: "me@example.com", Subject: "Hello"}, 0)

	updated := updateModel(t, m, keyRune('l'))
	assert.Equal(t, "work", updated.activeDraft.AccountID)
	assert.Equal(t, "work@example.com", updated.activeDraft.From)

	updated = updateModel(t, updated, keyRune('h'))
	assert.Equal(t, "personal", updated.activeDraft.AccountID)
	assert.Equal(t, "me@example.com", updated.activeDraft.From)
}

func TestGGAndGMoveListAndSidebarLikeVim(t *testing.T) {
	m := testModel()
	m.state = stateSidebar
	m.sidebarCursor = len(m.sidebarItems) - 1

	updated := updateModel(t, m, keyRune('g'))
	assert.Equal(t, len(m.sidebarItems)-1, updated.sidebarCursor)

	updated = updateModel(t, updated, keyRune('g'))
	assert.Equal(t, 0, updated.sidebarCursor)

	updated = updateModel(t, updated, keyRune('G'))
	assert.Equal(t, len(updated.sidebarItems)-1, updated.sidebarCursor)

	updated.state = stateList
	updated.listCursor = len(updated.messages) - 1
	updated = updateModel(t, updated, keyRune('g'))
	assert.Equal(t, len(updated.messages)-1, updated.listCursor)

	updated = updateModel(t, updated, keyRune('g'))
	assert.Equal(t, 0, updated.listCursor)

	updated = updateModel(t, updated, keyRune('G'))
	assert.Equal(t, len(updated.messages)-1, updated.listCursor)
}

func TestZeroAndDollarJumpByPane(t *testing.T) {
	service := &messageServiceStub{inbox: sampleLongMessage()}
	m := testModelWithService(service)
	m.width = 120
	m.height = 24
	m.state = stateList
	m.listCursor = len(m.messages) - 1

	updated := updateModel(t, m, keyRune('0'))
	assert.Equal(t, 0, updated.listCursor)

	updated = updateModel(t, updated, keyRune('$'))
	assert.Equal(t, len(updated.messages)-1, updated.listCursor)

	updated.state = stateContent
	updated.syncContentViewport(true)
	updated = updateModel(t, updated, tea.KeyMsg{Type: tea.KeyPgDown})
	assert.Positive(t, updated.contentViewport.YOffset)

	updated = updateModel(t, updated, keyRune('0'))
	assert.True(t, updated.contentViewport.AtTop())

	updated = updateModel(t, updated, keyRune('$'))
	assert.True(t, updated.contentViewport.AtBottom())

	updated.enterComposeState(&models.Message{AccountID: "personal", From: "me@example.com", Subject: "Hello", Body: "Body"}, 2)
	updated = updateModel(t, updated, keyRune('0'))
	assert.Equal(t, 0, updated.focusIndex)

	updated = updateModel(t, updated, keyRune('$'))
	assert.Equal(t, 3, updated.focusIndex)
}

func TestListLoadsNextPageNearBottom(t *testing.T) {
	service := &messageServiceStub{inbox: pagedInboxMessages(65)}
	m := testModelWithService(service)
	m.state = stateList
	m.prepareFreshMessageFetch()

	initialLoad := m.fetchMessages()
	require.NotNil(t, initialLoad)
	loadedMsg, ok := resolveCmdForTests(initialLoad)().(messagesLoadedMsg)
	require.True(t, ok)
	m = updateModel(t, m, loadedMsg)

	require.Len(t, m.messages, m.listFetchPageSize())
	assert.True(t, m.hasMoreMessages)

	m.listCursor = len(m.messages) - m.listFetchNextThreshold()
	updated, cmd := updateModelWithCmd(t, m, keyRune('j'))
	require.NotNil(t, cmd)
	assert.True(t, updated.messagesLoading)

	appendedMsg, ok := cmd().(messagesLoadedMsg)
	require.True(t, ok)
	updated = updateModel(t, updated, appendedMsg)

	assert.Len(t, updated.messages, m.listFetchPageSize()*2)
	assert.Equal(t, m.listFetchPageSize()*2, updated.fetchOffset)
	assert.True(t, updated.hasMoreMessages)
	assert.False(t, updated.messagesLoading)
	assert.Equal(t, len(m.messages)-m.listFetchNextThreshold()+1, updated.listCursor)
	assert.Equal(t, "msg-031", updated.messages[m.listFetchPageSize()].ID)
	assert.Equal(t, m.listFetchPageSize(), appendedMsg.nextOffset-len(appendedMsg.messages))
}

func TestLoadingTickAdvancesWhileMessagesLoading(t *testing.T) {
	m := testModel()
	m.messagesLoading = true
	m.loadingToken = 3
	m.loadingFrame = 0

	updatedAny, cmd := m.Update(loadingTickMsg{token: 3, frame: 1})
	updated := updatedAny.(Model)

	require.NotNil(t, cmd)
	assert.True(t, updated.messagesLoading)
	assert.Equal(t, 1, updated.loadingFrame)

	ignoredAny, ignoredCmd := updated.Update(loadingTickMsg{token: 2, frame: 2})
	ignored := ignoredAny.(Model)

	assert.Nil(t, ignoredCmd)
	assert.Equal(t, 1, ignored.loadingFrame)
}

func TestCtrlDUScrollsListLikeVim(t *testing.T) {
	service := &messageServiceStub{inbox: manyDraftMessages(20)}
	m := testModelWithService(service)
	m.messages = manyDraftMessages(20)
	m.state = stateList
	m.listCursor = 10

	updated := updateModel(t, m, tea.KeyMsg{Type: tea.KeyCtrlD})
	assert.Equal(t, 15, updated.listCursor)

	updated = updateModel(t, updated, tea.KeyMsg{Type: tea.KeyCtrlU})
	assert.Equal(t, 10, updated.listCursor)
}

func TestJKFallbackWorksEvenIfArrowBindingsAreStripped(t *testing.T) {
	m := testModel()
	m.state = stateList
	m.keys.Up = key.NewBinding(key.WithKeys("up"), key.WithHelp("up", "up"))
	m.keys.Down = key.NewBinding(key.WithKeys("down"), key.WithHelp("down", "down"))

	updated := updateModel(t, m, keyRune('j'))
	assert.Equal(t, 1, updated.listCursor)

	updated = updateModel(t, updated, keyRune('k'))
	assert.Equal(t, 0, updated.listCursor)
}

func TestComposeEnterSavesAndSendsDraft(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages(), composedDraftID: "draft-42"}
	m := testModelWithService(service)
	m.enterComposeState(&models.Message{To: []string{"user@example.com"}, Subject: "Hello", Body: "Body"}, 3)

	updated := updateModel(t, m, tea.KeyMsg{Type: tea.KeyCtrlX})

	require.Len(t, service.composeCalls, 1)
	assert.Equal(t, "Hello", service.composeCalls[0].Subject)
	assert.Equal(t, []string{"user@example.com"}, service.composeCalls[0].To)
	assert.Equal(t, []string{"draft-42"}, service.sendCalls)
	assert.Equal(t, stateList, updated.state)
	assert.Nil(t, updated.activeDraft)
	assert.Equal(t, "Message sent", updated.statusMessage)
	assert.False(t, updated.statusError)
}

func TestSaveDraftCreatesDraftAndReturnsToList(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages(), composedDraftID: "draft-42"}
	m := testModelWithService(service)
	m.enterComposeState(
		&models.Message{AccountID: "personal", From: "me@example.com", To: []string{"user@example.com"}, Subject: "Hello", Body: "Body"},
		1,
	)

	updatedAny, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	updated := updatedAny.(Model)

	require.Len(t, service.composeCalls, 1)
	assert.Equal(t, stateList, updated.state)
	assert.Nil(t, updated.activeDraft)
	assert.Equal(t, "Draft saved", updated.statusMessage)
	assert.False(t, updated.statusError)
	require.NotNil(t, cmd)
}

func TestSaveDraftUpdatesExistingDraft(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages()}
	m := testModelWithService(service)
	m.enterComposeState(
		&models.Message{
			ID:        "draft-1",
			AccountID: "personal",
			From:      "me@example.com",
			To:        []string{"user@example.com"},
			Subject:   "Hello",
			Body:      "Body",
		},
		1,
	)

	updatedAny, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	updated := updatedAny.(Model)

	assert.Empty(t, service.composeCalls)
	require.Len(t, service.updateDraftCalls, 1)
	assert.Equal(t, "draft-1", service.updateDraftCalls[0].id)
	assert.Equal(t, "Draft saved", updated.statusMessage)
	assert.False(t, updated.statusError)
	require.NotNil(t, cmd)
}

func TestEnterOnDraftOpensComposeForEditing(t *testing.T) {
	service := &messageServiceStub{inbox: sampleDraftMessages()}
	m := testModelWithService(service)
	m.messages = sampleDraftMessages()
	m.state = stateList

	updated := updateModel(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	assert.Equal(t, stateCompose, updated.state)
	assert.Equal(t, []string{"draft-1"}, service.markReadCalls)
	require.NotNil(t, updated.activeDraft)
	assert.Equal(t, "draft-1", updated.activeDraft.ID)
	assert.Equal(t, "Draft subject", updated.activeDraft.Subject)
	assert.Equal(t, 1, updated.focusIndex)
	assert.Equal(t, "Editing draft", updated.statusMessage)
	assert.False(t, updated.statusError)
	assert.True(t, updated.messages[0].IsRead)
}

func TestMovingSelectionMarksUnreadMessageAsRead(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages()}
	m := testModelWithService(service)
	m.state = stateList

	updated := updateModel(t, m, tea.KeyMsg{Type: tea.KeyDown})

	assert.Equal(t, []string{"msg-2"}, service.markReadCalls)
	assert.True(t, updated.messages[1].IsRead)
}

func TestOpeningContentMarksUnreadMessageAsRead(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages()}
	m := testModelWithService(service)
	m.state = stateList

	updated := updateModel(t, m, tea.KeyMsg{Type: tea.KeyRight})

	assert.Equal(t, stateContent, updated.state)
	assert.Equal(t, []string{"msg-1"}, service.markReadCalls)
	assert.True(t, updated.messages[0].IsRead)
}

func TestEnterOpensContentForRegularMessage(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages()}
	m := testModelWithService(service)
	m.state = stateList

	updated := updateModel(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	assert.Equal(t, stateContent, updated.state)
	assert.Equal(t, []string{"msg-1"}, service.markReadCalls)
	assert.True(t, updated.messages[0].IsRead)
}

func TestEnterOpensSelectedMailboxFromSidebar(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages()}
	m := testModelWithService(service)
	m.state = stateSidebar

	updatedAny, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := updatedAny.(Model)

	assert.Equal(t, stateList, updated.state)
	require.NotNil(t, cmd)
}

func TestContentViewportScrollsWithoutChangingSelection(t *testing.T) {
	service := &messageServiceStub{inbox: sampleLongMessage()}
	m := testModelWithService(service)
	m.width = 120
	m.height = 24
	m.state = stateContent
	m.syncContentViewport(true)

	updated := updateModel(t, m, tea.KeyMsg{Type: tea.KeyDown})

	assert.Equal(t, stateContent, updated.state)
	assert.Equal(t, 0, updated.listCursor)
	assert.Positive(t, updated.contentViewport.YOffset)
}

func TestSearchModeFiltersCurrentMailboxLive(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages()}
	m := testModelWithService(service)
	m.state = stateList

	updated := updateModel(t, m, keyRune('/'))
	assert.True(t, updated.searchActive)

	updated, cmd := updateModelWithCmd(t, updated, keyRune('2'))
	require.NotNil(t, cmd)
	assert.True(t, updated.searchDebouncing)
	assert.Empty(t, service.lastSearch.Query)

	updated, cmd = updateModelWithCmd(t, updated, searchDebounceMsg{token: updated.searchToken, query: updated.searchQuery})
	require.NotNil(t, cmd)
	updated = updateModel(t, updated, cmd())
	assert.True(t, updated.searchActive)
	assert.Equal(t, "2", updated.searchQuery)
	assert.False(t, updated.searchDebouncing)
	require.Len(t, updated.messages, 1)
	assert.Equal(t, "msg-2", updated.messages[0].ID)
	assert.Equal(t, "2", service.lastSearch.Query)

	updated = updateModel(t, updated, tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, updated.searchActive)
	assert.Equal(t, "2", updated.searchQuery)
	require.Len(t, updated.messages, 1)
	assert.Equal(t, "Search: 1 of 1 messages • n/N next/prev", updated.statusMessage)
}

func TestSlashFallbackOpensSearchEvenIfConfiguredBindingDiffers(t *testing.T) {
	m := testModel()
	m.state = stateList
	m.keys.Search = key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "search"))

	updated := updateModel(t, m, keyRune('/'))
	assert.True(t, updated.searchActive)
}

func TestSearchRepeatMovesAcrossFilteredResults(t *testing.T) {
	m := testModel()
	m.state = stateList
	m.searchQuery = "subject"
	m.searchInput.SetValue("subject")
	m.applySearchFilter()
	require.Len(t, m.messages, 2)

	updated := updateModel(t, m, keyRune('n'))
	assert.Equal(t, 1, updated.listCursor)
	assert.Equal(t, "msg-2", updated.messages[updated.listCursor].ID)

	updated = updateModel(t, updated, keyRune('N'))
	assert.Equal(t, 0, updated.listCursor)
	assert.Equal(t, "msg-1", updated.messages[updated.listCursor].ID)
}

func TestSearchResultsLoadNextPageNearBottom(t *testing.T) {
	service := &messageServiceStub{inbox: pagedInboxMessages(65)}
	for index := range service.inbox {
		service.inbox[index].Subject = "Critical match"
	}
	m := testModelWithService(service)
	m.state = stateList
	updated := updateModel(t, m, keyRune('/'))
	updated, cmd := updateModelWithCmd(t, updated, keyRune('c'))
	require.NotNil(t, cmd)
	for _, letter := range []rune{'r', 'i', 't', 'i', 'c', 'a', 'l'} {
		updated, cmd = updateModelWithCmd(t, updated, keyRune(letter))
		require.NotNil(t, cmd)
	}

	assert.True(t, updated.searchDebouncing)
	assert.Empty(t, service.lastSearch.Query)

	updated, cmd = updateModelWithCmd(t, updated, searchDebounceMsg{token: updated.searchToken, query: updated.searchQuery})
	require.NotNil(t, cmd)
	updated = updateModel(t, updated, cmd())

	require.Len(t, updated.messages, m.listFetchPageSize())
	assert.False(t, updated.messagesLoading)
	assert.False(t, updated.searchDebouncing)
	assert.True(t, updated.hasMoreMessages)
	assert.Equal(t, "critical", updated.searchQuery)
	assert.Equal(t, "critical", service.lastSearch.Query)

	updated = updateModel(t, updated, tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, updated.searchActive)

	updated.listCursor = len(updated.messages) - updated.listFetchNextThreshold()
	updated, cmd = updateModelWithCmd(t, updated, keyRune('j'))
	require.NotNil(t, cmd)
	assert.True(t, updated.messagesLoading)

	appendedMsg, ok := cmd().(messagesLoadedMsg)
	require.True(t, ok)
	updated = updateModel(t, updated, appendedMsg)

	require.Len(t, updated.messages, m.listFetchPageSize()*2)
	assert.Equal(t, "msg-031", updated.messages[m.listFetchPageSize()].ID)
	assert.False(t, updated.messagesLoading)
}

func TestSearchEscClearsFilterAndRestoresMailbox(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages()}
	m := testModelWithService(service)
	m.state = stateList
	m.searchQuery = "sender2"
	m.searchInput.SetValue("sender2")
	m.searchToken = 1
	m.prepareFreshMessageFetch()
	loadedMsg, ok := resolveCmdForTests(m.fetchMessages())().(messagesLoadedMsg)
	require.True(t, ok)
	m = updateModel(t, m, loadedMsg)
	require.Len(t, m.messages, 1)

	updated, cmd := updateModelWithCmd(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	require.NotNil(t, cmd)
	updated = updateModel(t, updated, cmd())

	assert.False(t, updated.searchActive)
	assert.Empty(t, updated.searchQuery)
	require.Len(t, updated.messages, 2)
	assert.Equal(t, "Search cleared", updated.statusMessage)
}

func TestContentViewportHalfPageDownWithoutChangingSelection(t *testing.T) {
	service := &messageServiceStub{inbox: sampleLongMessage()}
	m := testModelWithService(service)
	m.width = 120
	m.height = 24
	m.state = stateContent
	m.syncContentViewport(true)

	updated := updateModel(t, m, tea.KeyMsg{Type: tea.KeyCtrlD})

	assert.Equal(t, stateContent, updated.state)
	assert.Equal(t, 0, updated.listCursor)
	assert.GreaterOrEqual(t, updated.contentViewport.YOffset, 3)
	assert.Less(t, updated.contentViewport.YOffset, updated.contentViewport.TotalLineCount())
}

func TestContentViewportPageDownAndTopBottomShortcuts(t *testing.T) {
	service := &messageServiceStub{inbox: sampleLongMessage()}
	m := testModelWithService(service)
	m.width = 120
	m.height = 24
	m.state = stateContent
	m.syncContentViewport(true)

	updated := updateModel(t, m, tea.KeyMsg{Type: tea.KeyPgDown})
	assert.Equal(t, stateContent, updated.state)
	assert.Equal(t, 0, updated.listCursor)
	assert.Positive(t, updated.contentViewport.YOffset)

	updated = updateModel(t, updated, keyRune('G'))
	assert.True(t, updated.contentViewport.AtBottom())
	assert.Equal(t, 0, updated.listCursor)

	updated = updateModel(t, updated, keyRune('g'))
	assert.False(t, updated.contentViewport.AtTop())

	updated = updateModel(t, updated, keyRune('g'))
	assert.True(t, updated.contentViewport.AtTop())
	assert.Equal(t, 0, updated.listCursor)
}

func TestHMLJumpWithinVisibleListWindow(t *testing.T) {
	service := &messageServiceStub{inbox: manyDraftMessages(20)}
	m := testModelWithService(service)
	m.messages = manyDraftMessages(20)
	m.width = 120
	m.height = 24
	m.state = stateList
	m.listCursor = 5

	topJump := updateModel(t, m, keyRune('H'))
	assert.Equal(t, 1, topJump.listCursor)

	middleJump := updateModel(t, m, keyRune('M'))
	assert.Equal(t, 3, middleJump.listCursor)

	bottomJump := updateModel(t, m, keyRune('L'))
	assert.Equal(t, 5, bottomJump.listCursor)
}

func TestColonOpensCommandPromptAndRunsMailboxCommand(t *testing.T) {
	m := testModel()
	m.state = stateList

	updated := updateModel(t, m, keyRune(':'))
	assert.True(t, updated.commandActive)
	assert.False(t, updated.searchActive)
	assert.Equal(t, ": ", updated.searchInput.Prompt)

	updated.searchInput.SetValue("drafts")
	updatedAny, cmd := updated.Update(tea.KeyMsg{Type: tea.KeyEnter})
	run := updatedAny.(Model)

	assert.False(t, run.commandActive)
	assert.Equal(t, stateList, run.state)
	assert.Equal(t, 2, run.sidebarCursor)
	require.NotNil(t, cmd)
}

func TestColonOpensCommandPromptFromReplyMode(t *testing.T) {
	m := testModel()
	m.state = stateList
	m = updateModel(t, m, keyRune('r'))
	require.Equal(t, stateCompose, m.state)
	require.False(t, m.commandActive)

	updated := updateModel(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	updated = updateModel(t, updated, keyRune(':'))

	assert.True(t, updated.commandActive)
	assert.Equal(t, stateCompose, updated.state)
	assert.Equal(t, ": ", updated.searchInput.Prompt)

	updated.searchInput.SetValue("drafts")
	updatedAny, cmd := updated.Update(tea.KeyMsg{Type: tea.KeyEnter})
	run := updatedAny.(Model)

	assert.False(t, run.commandActive)
	assert.Equal(t, stateList, run.state)
	assert.Equal(t, 2, run.sidebarCursor)
	require.NotNil(t, cmd)
}

func TestUnknownCommandShowsError(t *testing.T) {
	m := testModel()
	m.commandActive = true
	m.searchInput.Prompt = ": "
	m.searchInput.SetValue("wat")

	updatedAny, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := updatedAny.(Model)

	assert.False(t, updated.commandActive)
	assert.Equal(t, "Unknown command: wat", updated.statusMessage)
	assert.True(t, updated.statusError)
	assert.Nil(t, cmd)
}

func TestCommandPromptUsesHistoryWithArrowKeys(t *testing.T) {
	m := testModel()
	m.commandHistory = []string{"compose", "drafts", "quit"}
	m.openCommandPrompt()
	m.searchInput.SetValue("ref")
	m.commandDraft = "ref"

	updated := updateModel(t, m, tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, "quit", updated.searchInput.Value())

	updated = updateModel(t, updated, tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, "drafts", updated.searchInput.Value())

	updated = updateModel(t, updated, tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, "quit", updated.searchInput.Value())

	updated = updateModel(t, updated, tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, "ref", updated.searchInput.Value())
}

func TestCommandPromptTabCompletesKnownCommands(t *testing.T) {
	m := testModel()
	m.openCommandPrompt()
	m.searchInput.SetValue("dr")

	updated := updateModel(t, m, tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, "drafts", updated.searchInput.Value())

	updated = updateModel(t, updated, tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, "drafts", updated.searchInput.Value())
}

func TestCommandPromptTabCompletesExtendedCommands(t *testing.T) {
	m := testModel()
	m.openCommandPrompt()
	m.searchInput.SetValue("ar")

	updated := updateModel(t, m, tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, "archive", updated.searchInput.Value())

	updated.searchInput.SetValue("he")
	updated = updateModel(t, updated, tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, "help", updated.searchInput.Value())
}

func TestCommandPromptTabCompletesAICommands(t *testing.T) {
	m := testModel()
	m.openCommandPrompt()
	m.searchInput.SetValue("compose-a")

	updated := updateModel(t, m, tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, "compose-ai", updated.searchInput.Value())

	updated.searchInput.SetValue("reply-all-a")
	updated = updateModel(t, updated, tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, "reply-all-ai", updated.searchInput.Value())
}

func TestCommandPromptTabCompletesEmptyToFirstCommand(t *testing.T) {
	m := testModel()
	m.openCommandPrompt()

	updated := updateModel(t, m, tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, "compose", updated.searchInput.Value())
}

func TestComposeOAndOOpenBodyInNormalMode(t *testing.T) {
	m := testModel()
	m.enterComposeState(&models.Message{AccountID: "personal", From: "me@example.com", Subject: "Hello", Body: "Body"}, 1)

	updated := updateModel(t, m, keyRune('o'))
	assert.True(t, updated.composeEditing)
	assert.Equal(t, 3, updated.focusIndex)
	assert.Contains(t, updated.activeDraft.Body, "Body\n")

	updated = updateModel(t, updated, tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, updated.composeEditing)

	updated = updateModel(t, updated, keyRune('O'))
	assert.True(t, updated.composeEditing)
	assert.Equal(t, 3, updated.focusIndex)
}

func TestComposeGJumpsToLastField(t *testing.T) {
	m := testModel()
	m.enterComposeState(&models.Message{AccountID: "personal", From: "me@example.com", Subject: "Hello", Body: "Body"}, 0)

	updated := updateModel(t, m, keyRune('G'))

	assert.Equal(t, 3, updated.focusIndex)
	assert.False(t, updated.composeEditing)
}

func TestComposeCommandRefusesToReplaceDirtyDraft(t *testing.T) {
	m := testModel()
	m.enterComposeState(&models.Message{AccountID: "personal", From: "me@example.com", Subject: "Hello", Body: "Body"}, 1)
	m.activeDraft.Body = "Body\nchanged"

	updated := updateModel(t, m, keyRune(':'))
	require.True(t, updated.commandActive)
	updated.searchInput.SetValue("compose")

	updatedAny, cmd := updated.Update(tea.KeyMsg{Type: tea.KeyEnter})
	run := updatedAny.(Model)

	assert.Nil(t, cmd)
	assert.Equal(t, stateCompose, run.state)
	require.NotNil(t, run.activeDraft)
	assert.Equal(t, "Body\nchanged", run.activeDraft.Body)
	assert.Equal(t, "Unsaved draft. Save, send, or cancel before leaving compose", run.statusMessage)
	assert.True(t, run.statusError)
}

func TestComposeAICommandUsesAssistantAndOpensCompose(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages()}
	assistant := &draftAssistantStub{response: &models.GeneratedDraft{Subject: "Kickoff", Body: "Here is a concise kickoff draft."}}
	m := testModelWithService(service)
	m.assistant = assistant

	m.openCommandPrompt()
	m.searchInput.SetValue("compose-ai --template compose-default Draft a kickoff note")

	updated, cmd := updateModelWithCmd(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	assert.Equal(t, "Generating AI draft...", updated.statusMessage)
	assert.True(t, updated.aiGenerating)
	assert.Equal(t, "AI draft", updated.aiLoadingLabel)

	updated = updateModel(t, updated, cmd())
	require.Len(t, assistant.requests, 1)
	assert.Equal(t, "compose", assistant.requests[0].Mode)
	assert.Equal(t, "compose-default", assistant.requests[0].Template)
	assert.Equal(t, "Draft a kickoff note", assistant.requests[0].Instruction)
	assert.Equal(t, stateCompose, updated.state)
	require.NotNil(t, updated.activeDraft)
	assert.Equal(t, "Kickoff", updated.activeDraft.Subject)
	assert.Equal(t, "Here is a concise kickoff draft.", updated.activeDraft.Body)
	assert.Equal(t, "AI Compose", updated.composeTitle)
	assert.True(t, updated.composeEditing)
	assert.False(t, updated.aiGenerating)
	assert.Equal(t, "AI draft ready", updated.statusMessage)
}

func TestReplyAICommandUsesSelectedMessageAndOpensReplyDraft(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages()}
	assistant := &draftAssistantStub{response: &models.GeneratedDraft{Subject: "Re: Subject 1", Body: "Thanks, this works for us."}}
	m := testModelWithService(service)
	m.assistant = assistant
	m.state = stateList

	m.openCommandPrompt()
	m.searchInput.SetValue("reply-ai Accept and confirm")

	updated, cmd := updateModelWithCmd(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	assert.Equal(t, "Generating AI reply...", updated.statusMessage)

	updated = updateModel(t, updated, cmd())
	require.Len(t, assistant.requests, 1)
	assert.Equal(t, "reply", assistant.requests[0].Mode)
	assert.Equal(t, "Accept and confirm", assistant.requests[0].Instruction)
	require.NotNil(t, assistant.requests[0].Original)
	assert.Equal(t, "msg-1", assistant.requests[0].Original.ID)
	assert.Equal(t, stateCompose, updated.state)
	require.NotNil(t, updated.activeDraft)
	assert.Equal(t, "AI Reply", updated.composeTitle)
	assert.Contains(t, updated.activeDraft.Body, "Thanks, this works for us.")
	assert.Contains(t, updated.activeDraft.Body, "wrote:")
	assert.True(t, updated.composeEditing)
	assert.Equal(t, "AI reply ready", updated.statusMessage)
}

func TestReplyAICommandAcceptsTemplateFlag(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages()}
	assistant := &draftAssistantStub{response: &models.GeneratedDraft{Subject: "Re: Subject 1", Body: "Confirmed."}}
	m := testModelWithService(service)
	m.assistant = assistant
	m.state = stateList

	m.openCommandPrompt()
	m.searchInput.SetValue("reply-ai --template reply-default Accept and confirm")

	updated, cmd := updateModelWithCmd(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	assert.True(t, updated.aiGenerating)

	updated = updateModel(t, updated, cmd())
	require.Len(t, assistant.requests, 1)
	assert.Equal(t, "reply-default", assistant.requests[0].Template)
	assert.Equal(t, "Accept and confirm", assistant.requests[0].Instruction)
	assert.False(t, updated.aiGenerating)
}

func TestComposeAICommandRejectsMissingTemplateValue(t *testing.T) {
	m := testModel()
	m.assistant = &draftAssistantStub{response: &models.GeneratedDraft{Body: "draft"}}
	m.openCommandPrompt()
	m.searchInput.SetValue("compose-ai --template")

	updated, cmd := updateModelWithCmd(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	assert.Nil(t, cmd)
	assert.True(t, updated.statusError)
	assert.Equal(t, "template name is required after --template", updated.statusMessage)
	assert.False(t, updated.aiGenerating)
}

func TestReplyAICommandRequiresSelection(t *testing.T) {
	assistant := &draftAssistantStub{response: &models.GeneratedDraft{Body: "Thanks"}}
	m := testModel()
	m.assistant = assistant
	m.messages = nil
	m.state = stateList
	m.openCommandPrompt()
	m.searchInput.SetValue("reply-ai")

	updated, cmd := updateModelWithCmd(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)

	updated = updateModel(t, updated, cmd())
	assert.True(t, updated.statusError)
	assert.Equal(t, "Select a message before generating an AI reply", updated.statusMessage)
}

func TestCountPrefixMovesListSelectionAndTargetsWithGGMotions(t *testing.T) {
	service := &messageServiceStub{inbox: pagedInboxMessages(8)}
	m := testModelWithService(service)
	m.messages = pagedInboxMessages(8)
	m.state = stateList

	updated := updateModel(t, m, keyRune('5'))
	updated = updateModel(t, updated, keyRune('j'))
	assert.Equal(t, 5, updated.listCursor)

	updated = updateModel(t, updated, keyRune('3'))
	updated = updateModel(t, updated, keyRune('g'))
	updated = updateModel(t, updated, keyRune('g'))
	assert.Equal(t, 2, updated.listCursor)

	updated = updateModel(t, updated, keyRune('6'))
	updated = updateModel(t, updated, keyRune('G'))
	assert.Equal(t, 5, updated.listCursor)
}

func TestCountPrefixMovesComposeFields(t *testing.T) {
	m := testModel()
	m.enterComposeState(&models.Message{AccountID: "personal", From: "me@example.com", Subject: "Hello", Body: "Body"}, 0)

	updated := updateModel(t, m, keyRune('2'))
	updated = updateModel(t, updated, keyRune('j'))
	assert.Equal(t, 2, updated.focusIndex)

	updated = updateModel(t, updated, keyRune('3'))
	updated = updateModel(t, updated, keyRune('g'))
	updated = updateModel(t, updated, keyRune('g'))
	assert.Equal(t, 2, updated.focusIndex)
}

func TestDeleteKeyTogglesDeleteAndRefreshesMessages(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages()}
	m := testModelWithService(service)
	m.state = stateList
	m.listCursor = 1

	updatedAny, cmd := m.Update(keyRune('d'))
	updated := updatedAny.(Model)

	assert.Equal(t, []string{"msg-2"}, service.toggleDeleteCalls)
	require.NotNil(t, cmd)

	reloaded := updateModel(t, updated, cmd())
	assert.NotEmpty(t, reloaded.messages)
	assert.Equal(t, 0, reloaded.listCursor)
	assert.Equal(t, "Message moved to trash. Press u to undo", updated.statusMessage)
	assert.False(t, updated.statusError)
}

func TestDeleteUndoRestoresMessageToMailbox(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages()}
	m := testModelWithService(service)
	m.state = stateList
	m.listCursor = 1

	deletedAny, fetchCmd := m.Update(keyRune('d'))
	deleted := deletedAny.(Model)
	require.NotNil(t, fetchCmd)
	reloaded := updateModel(t, deleted, fetchCmd())
	require.NotNil(t, reloaded.pendingUndo)

	undoneAny, undoFetchCmd := reloaded.Update(keyRune('u'))
	undone := undoneAny.(Model)
	assert.Nil(t, undone.pendingUndo)
	assert.Equal(t, "Undo applied", undone.statusMessage)
	require.NotNil(t, undoFetchCmd)

	restored := updateModel(t, undone, undoFetchCmd())
	assert.Len(t, restored.messages, 2)
	assert.Equal(t, "msg-2", restored.messages[restored.listCursor].ID)
	assert.False(t, service.inbox[1].IsDeleted)
}

func TestArchiveUndoRestoresInboxLabel(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages()}
	m := testModelWithService(service)
	m.state = stateList
	m.listCursor = 0

	archivedAny, fetchCmd := m.Update(keyRune('a'))
	archived := archivedAny.(Model)
	require.NotNil(t, fetchCmd)
	reloaded := updateModel(t, archived, fetchCmd())
	require.NotNil(t, reloaded.pendingUndo)

	undoneAny, undoFetchCmd := reloaded.Update(keyRune('u'))
	undone := undoneAny.(Model)
	require.NotNil(t, undoFetchCmd)
	restored := updateModel(t, undone, undoFetchCmd())

	assert.Contains(t, restored.messages[restored.listCursor].Labels, "inbox")
	assert.NotContains(t, restored.messages[restored.listCursor].Labels, "archive")
	assert.False(t, service.inbox[0].IsDeleted)
	assert.False(t, service.inbox[0].IsSpam)
}

func TestUndoExpiresAndClearsPendingState(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages()}
	m := testModelWithService(service)
	m.state = stateList

	deletedAny, fetchCmd := m.Update(keyRune('d'))
	deleted := deletedAny.(Model)
	require.NotNil(t, fetchCmd)
	reloaded := updateModel(t, deleted, fetchCmd())
	require.NotNil(t, reloaded.pendingUndo)
	token := reloaded.pendingUndo.token

	expired := updateModel(t, reloaded, undoExpiredMsg{token: token})

	assert.Nil(t, expired.pendingUndo)
	assert.NotContains(t, expired.statusMessage, "Press u to undo")
}

func TestDeleteBottomItemKeepsCursorNearBottomAfterReload(t *testing.T) {
	service := &messageServiceStub{inbox: manyDraftMessages(5)}
	m := testModelWithService(service)
	m.sidebarCursor = 2
	m.state = stateList
	m.listCursor = 4

	updatedAny, cmd := m.Update(keyRune('d'))
	updated := updatedAny.(Model)

	require.NotNil(t, cmd)
	assert.Equal(t, 3, updated.listCursor)

	reloaded := updateModel(t, updated, cmd())
	require.Len(t, reloaded.messages, 4)
	assert.Equal(t, 3, reloaded.listCursor)
	assert.Equal(t, service.inbox[3].ID, reloaded.messages[reloaded.listCursor].ID)
}

func TestDeleteCanRemoveAutoSelectedNextMessage(t *testing.T) {
	service := &messageServiceStub{inbox: manyDraftMessages(5)}
	m := testModelWithService(service)
	m.sidebarCursor = 2
	m.state = stateList
	m.listCursor = 4

	firstAny, firstCmd := m.Update(keyRune('d'))
	first := firstAny.(Model)
	require.NotNil(t, firstCmd)
	require.Len(t, first.messages, 4)
	firstSelected, ok := first.selectedMessage()
	require.True(t, ok)

	secondAny, secondCmd := first.Update(keyRune('d'))
	second := secondAny.(Model)
	require.NotNil(t, secondCmd)
	require.Len(t, second.messages, 3)
	assert.Len(t, service.toggleDeleteCalls, 2)
	assert.NotEqual(t, service.toggleDeleteCalls[0], service.toggleDeleteCalls[1])
	assert.Equal(t, firstSelected.ID, service.toggleDeleteCalls[1])

	reloaded := updateModel(t, second, secondCmd())
	assert.Len(t, reloaded.messages, 3)
	assert.Equal(t, 2, reloaded.listCursor)
	assert.Equal(t, service.inbox[2].ID, reloaded.messages[reloaded.listCursor].ID)
}

func TestDeleteRemovesDraftFromDraftsMailbox(t *testing.T) {
	service := &messageServiceStub{inbox: sampleDraftMessages()}
	m := testModelWithService(service)
	m.sidebarCursor = 2
	m.state = stateList

	updatedAny, cmd := m.Update(keyRune('d'))
	updated := updatedAny.(Model)

	assert.Equal(t, []string{"draft-1"}, service.toggleDeleteCalls)
	require.NotNil(t, cmd)
	reloaded := updateModel(t, updated, cmd())
	assert.Empty(t, reloaded.messages)
	assert.Equal(t, "Message moved to trash. Press u to undo", updated.statusMessage)
	assert.True(t, service.inbox[0].IsDeleted)
}

func TestDeleteRemovesSentMessageFromSentMailbox(t *testing.T) {
	service := &messageServiceStub{inbox: sampleSentMessages()}
	m := testModelWithService(service)
	m.sidebarCursor = 1
	m.state = stateList

	updatedAny, cmd := m.Update(keyRune('d'))
	updated := updatedAny.(Model)

	assert.Equal(t, []string{"sent-1"}, service.toggleDeleteCalls)
	require.NotNil(t, cmd)
	reloaded := updateModel(t, updated, cmd())
	assert.Empty(t, reloaded.messages)
	assert.True(t, service.inbox[0].IsDeleted)
}

func TestDeleteRemovesSpamMessageFromSpamMailbox(t *testing.T) {
	service := &messageServiceStub{inbox: sampleSpamMessages()}
	m := testModelWithService(service)
	m.sidebarCursor = 5
	m.state = stateList

	updatedAny, cmd := m.Update(keyRune('d'))
	updated := updatedAny.(Model)

	assert.Equal(t, []string{"spam-1"}, service.toggleDeleteCalls)
	require.NotNil(t, cmd)
	reloaded := updateModel(t, updated, cmd())
	assert.Empty(t, reloaded.messages)
	assert.True(t, service.inbox[0].IsDeleted)
}

func TestDeleteInTrashPermanentlyRemovesMessage(t *testing.T) {
	service := &messageServiceStub{inbox: []*models.Message{{
		ID:        "trash-1",
		AccountID: "personal",
		Subject:   "Deleted message",
		From:      "sender@example.com",
		To:        []string{"me@example.com"},
		Body:      "Already in trash",
		Labels:    []string{"inbox"},
		IsDeleted: true,
	}}}
	m := testModelWithService(service)
	m.sidebarCursor = 4
	m.state = stateList

	updatedAny, cmd := m.Update(keyRune('d'))
	updated := updatedAny.(Model)

	assert.Equal(t, []string{"trash-1"}, service.deleteCalls)
	assert.Empty(t, service.toggleDeleteCalls)
	require.NotNil(t, cmd)
	reloaded := updateModel(t, updated, cmd())
	assert.Empty(t, reloaded.messages)
	assert.Equal(t, "Message permanently deleted. Press u to undo", updated.statusMessage)
	assert.Empty(t, service.inbox)
	assert.NotNil(t, reloaded.pendingUndo)

	undoneAny, undoFetchCmd := reloaded.Update(keyRune('u'))
	undone := undoneAny.(Model)
	require.NotNil(t, undoFetchCmd)
	restored := updateModel(t, undone, undoFetchCmd())
	assert.Len(t, restored.messages, 1)
	assert.Equal(t, "trash-1", restored.messages[0].ID)
	assert.True(t, restored.messages[0].IsDeleted)
}

func TestDeleteInTrashShowsErrorWhenPermanentDeleteFails(t *testing.T) {
	service := &messageServiceStub{inbox: []*models.Message{{
		ID:        "trash-1",
		AccountID: "personal",
		Subject:   "Deleted message",
		From:      "sender@example.com",
		To:        []string{"me@example.com"},
		Body:      "Already in trash",
		IsDeleted: true,
	}}, deleteErr: errors.New("delete failed")}
	m := testModelWithService(service)
	m.sidebarCursor = 4
	m.state = stateList

	updatedAny, cmd := m.Update(keyRune('d'))
	updated := updatedAny.(Model)

	assert.Nil(t, cmd)
	assert.Empty(t, service.deleteCalls)
	assert.Equal(t, "Permanent delete failed", updated.statusMessage)
	assert.True(t, updated.statusError)
	assert.Len(t, service.inbox, 1)
}

func TestArchiveKeyArchivesMessageAndRefreshesMessages(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages()}
	m := testModelWithService(service)
	m.state = stateList

	updatedAny, cmd := m.Update(keyRune('a'))
	updated := updatedAny.(Model)

	assert.Equal(t, []string{"msg-1"}, service.archiveCalls)
	require.NotNil(t, cmd)
	reloaded := updateModel(t, updated, cmd())
	assert.NotEmpty(t, reloaded.messages)
	assert.Equal(t, "Message archived. Press u to undo", updated.statusMessage)
	assert.False(t, updated.statusError)
}

func TestCountPrefixArchivesMultipleMessagesAndUndoRestoresBatch(t *testing.T) {
	service := &messageServiceStub{inbox: pagedInboxMessages(5)}
	m := testModelWithService(service)
	m.messages = append([]*models.Message{}, service.inbox...)
	m.state = stateList

	updated := updateModel(t, m, keyRune('2'))
	updatedAny, fetchCmd := updated.Update(keyRune('a'))
	updated = updatedAny.(Model)

	require.NotNil(t, fetchCmd)
	assert.Equal(t, []string{"msg-001", "msg-002"}, service.archiveCalls)
	assert.Equal(t, "2 messages archived. Press u to undo", updated.statusMessage)

	reloaded := updateModel(t, updated, fetchCmd())
	require.NotNil(t, reloaded.pendingUndo)
	require.Len(t, reloaded.pendingUndo.snapshots(), 2)
	require.Len(t, reloaded.messages, 3)

	undoneAny, undoFetchCmd := reloaded.Update(keyRune('u'))
	undone := undoneAny.(Model)
	require.NotNil(t, undoFetchCmd)
	restored := updateModel(t, undone, undoFetchCmd())

	assert.Equal(t, "Undo applied", undone.statusMessage)
	assert.Len(t, restored.messages, 5)
	assert.Contains(t, restored.messages[0].Labels, "inbox")
	assert.Contains(t, restored.messages[1].Labels, "inbox")
}

func TestDotRepeatsLastArchiveAction(t *testing.T) {
	service := &messageServiceStub{inbox: pagedInboxMessages(4)}
	m := testModelWithService(service)
	m.messages = append([]*models.Message{}, service.inbox...)
	m.state = stateList

	firstAny, firstFetchCmd := m.Update(keyRune('a'))
	first := firstAny.(Model)
	require.NotNil(t, firstFetchCmd)
	first = updateModel(t, first, firstFetchCmd())

	repeatedAny, repeatFetchCmd := first.Update(keyRune('.'))
	repeated := repeatedAny.(Model)
	require.NotNil(t, repeatFetchCmd)
	_ = updateModel(t, repeated, repeatFetchCmd())

	assert.Equal(t, []string{"msg-001", "msg-002"}, service.archiveCalls)
	assert.Equal(t, repeatableActionArchive, repeated.lastAction)
	assert.Equal(t, "Message archived. Press u to undo", repeated.statusMessage)
}

func TestSpamKeyMarksMessageAsSpamAndRefreshesMessages(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages()}
	m := testModelWithService(service)
	m.state = stateList

	updatedAny, cmd := m.Update(keyRune('!'))
	updated := updatedAny.(Model)

	assert.Equal(t, []string{"msg-1"}, service.spamCalls)
	require.NotNil(t, cmd)
	reloaded := updateModel(t, updated, cmd())
	assert.NotEmpty(t, reloaded.messages)
	assert.Equal(t, "Message marked as spam. Press u to undo", updated.statusMessage)
	assert.False(t, updated.statusError)
}

func TestArchiveKeyShowsErrorWhenArchivingFails(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages(), archiveErr: errors.New("archive failed")}
	m := testModelWithService(service)
	m.state = stateList

	updatedAny, cmd := m.Update(keyRune('a'))
	updated := updatedAny.(Model)

	assert.Nil(t, cmd)
	assert.Equal(t, "Archive failed", updated.statusMessage)
	assert.True(t, updated.statusError)
}

func TestSpamKeyShowsErrorWhenSpamUpdateFails(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages(), spamErr: errors.New("spam failed")}
	m := testModelWithService(service)
	m.state = stateList

	updatedAny, cmd := m.Update(keyRune('!'))
	updated := updatedAny.(Model)

	assert.Nil(t, cmd)
	assert.Equal(t, "Spam update failed", updated.statusMessage)
	assert.True(t, updated.statusError)
}

func TestDeleteKeyDoesNothingWhenDeleteFails(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages(), toggleDeleteErr: errors.New("delete failed")}
	m := testModelWithService(service)
	m.state = stateList

	updatedAny, cmd := m.Update(keyRune('d'))
	updated := updatedAny.(Model)

	assert.Empty(t, service.toggleDeleteCalls)
	assert.Nil(t, cmd)
	assert.Equal(t, stateList, updated.state)
	assert.Equal(t, 0, updated.listCursor)
	assert.Equal(t, "Delete failed", updated.statusMessage)
	assert.True(t, updated.statusError)
}

func TestTabAndShiftTabMoveComposeFocus(t *testing.T) {
	m := testModel()
	m.enterComposeState(&models.Message{To: []string{"user@example.com"}, Subject: "Hello", Body: "Body"}, 0)

	updated := updateModel(t, m, tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, 1, updated.focusIndex)

	updated = updateModel(t, updated, tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, 2, updated.focusIndex)

	updated = updateModel(t, updated, tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, 3, updated.focusIndex)

	updated = updateModel(t, updated, tea.KeyMsg{Type: tea.KeyShiftTab})
	assert.Equal(t, 2, updated.focusIndex)
}

func TestEscCancelsComposeAndReturnsToList(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages()}
	m := testModelWithService(service)
	m.enterComposeState(&models.Message{To: []string{"user@example.com"}, Subject: "Hello", Body: "Body"}, 1)

	updatedAny, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := updatedAny.(Model)

	assert.Equal(t, stateList, updated.state)
	assert.Nil(t, updated.activeDraft)
	assert.Equal(t, "Compose cancelled", updated.statusMessage)
	assert.False(t, updated.statusError)
	require.NotNil(t, cmd)
	msg := cmd()
	reloaded := updateModel(t, updated, msg)
	assert.NotEmpty(t, reloaded.messages)
}

func TestComposeSendErrorKeepsComposeState(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages(), sendErr: errors.New("send failed"), composedDraftID: "draft-42"}
	m := testModelWithService(service)
	m.enterComposeState(&models.Message{To: []string{"user@example.com"}, Subject: "Hello", Body: "Body"}, 3)

	updated := updateModel(t, m, tea.KeyMsg{Type: tea.KeyCtrlX})

	require.Len(t, service.composeCalls, 1)
	assert.Equal(t, stateCompose, updated.state)
	assert.NotNil(t, updated.activeDraft)
	assert.Empty(t, service.sendCalls)
	assert.Equal(t, "send failed", updated.statusMessage)
	assert.True(t, updated.statusError)
}

func TestSaveDraftErrorKeepsComposeState(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages(), composeErr: errors.New("save failed")}
	m := testModelWithService(service)
	m.enterComposeState(&models.Message{To: []string{"user@example.com"}, Subject: "Hello", Body: "Body"}, 3)

	updated := updateModel(t, m, tea.KeyMsg{Type: tea.KeyCtrlO})

	assert.Equal(t, stateCompose, updated.state)
	assert.NotNil(t, updated.activeDraft)
	assert.Equal(t, "save failed", updated.statusMessage)
	assert.True(t, updated.statusError)
}

func TestComposeCreateErrorKeepsComposeStateAndShowsError(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages(), composeErr: errors.New("compose failed")}
	m := testModelWithService(service)
	m.enterComposeState(&models.Message{To: []string{"user@example.com"}, Subject: "Hello", Body: "Body"}, 3)

	updated := updateModel(t, m, tea.KeyMsg{Type: tea.KeyCtrlX})

	assert.Equal(t, stateCompose, updated.state)
	assert.NotNil(t, updated.activeDraft)
	assert.Equal(t, "compose failed", updated.statusMessage)
	assert.True(t, updated.statusError)
}

func TestRightAndLeftChangePaneFocus(t *testing.T) {
	m := testModel()
	m.state = stateSidebar

	updated := updateModel(t, m, tea.KeyMsg{Type: tea.KeyRight})
	assert.Equal(t, stateList, updated.state)

	updated = updateModel(t, updated, tea.KeyMsg{Type: tea.KeyRight})
	assert.Equal(t, stateContent, updated.state)

	updated = updateModel(t, updated, tea.KeyMsg{Type: tea.KeyLeft})
	assert.Equal(t, stateList, updated.state)
}

func TestQuitReturnsQuitCommand(t *testing.T) {
	m := testModel()

	updatedAny, cmd := m.Update(keyRune('q'))
	updated := updatedAny.(Model)

	assert.Equal(t, m.state, updated.state)
	require.NotNil(t, cmd)
	assert.NotNil(t, cmd())
}

func TestSidebarNavigationFetchesMessages(t *testing.T) {
	service := &messageServiceStub{inbox: sampleSentMessages()}
	m := testModelWithService(service)
	m.state = stateSidebar

	updatedAny, cmd := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	updated := updatedAny.(Model)
	assert.Equal(t, 1, updated.sidebarCursor)
	require.NotNil(t, cmd)

	msg := cmd()
	reloaded := updateModel(t, updated, msg)
	assert.NotEmpty(t, reloaded.messages)
}

func TestFetchMessagesUsesLabelSelectionForAccountSection(t *testing.T) {
	service := &messageServiceStub{inbox: sampleAccountMessages()}
	m := testModelWithService(service)
	m.sidebarItems = []string{"Inbox", "", "Accounts:", "  Outlook", "  Gmail"}
	m.accountNames = []string{"Outlook", "Gmail"}
	m.sidebarCursor = 3

	cmd := m.fetchMessages()
	require.NotNil(t, cmd)
	loaded := updateModel(t, m, cmd())

	assert.Empty(t, service.lastLabelQuery)
	assert.Equal(t, "Outlook", service.lastSearch.AccountID)
	require.Len(t, loaded.messages, 1)
	assert.Equal(t, "outlook-1", loaded.messages[0].ID)
	assert.Equal(t, "Outlook", currentMailboxTitle(loaded))
}

func TestAccountSelectionShowsOnlyInboxMessagesForThatAccount(t *testing.T) {
	service := &messageServiceStub{inbox: sampleAccountMessages()}
	m := testModelWithService(service)
	m.sidebarItems = []string{"Inbox", "", "Accounts:", "  Outlook", "  Gmail"}
	m.accountNames = []string{"Outlook", "Gmail"}
	m.sidebarCursor = 4

	cmd := m.fetchMessages()
	require.NotNil(t, cmd)
	loaded := updateModel(t, m, cmd())

	assert.Equal(t, "Gmail", service.lastSearch.AccountID)
	require.Len(t, loaded.messages, 1)
	assert.Equal(t, "gmail-1", loaded.messages[0].ID)
	assert.Equal(t, "Gmail", loaded.messages[0].AccountID)
}

func TestAccountScopeCarriesAcrossMailboxSelection(t *testing.T) {
	service := &messageServiceStub{inbox: sampleAccountMessages()}
	m := testModelWithService(service)
	m.sidebarItems = []string{"Inbox", "Sent", "Drafts", "Archive", "Trash", "Spam", "", "Accounts:", "  Outlook", "  Gmail"}
	m.accountNames = []string{"Outlook", "Gmail"}
	m.sidebarCursor = 9

	loaded := updateModel(t, m, m.fetchMessages()())
	assert.Equal(t, "Gmail", loaded.activeAccountID)

	loaded.sidebarCursor = 1
	scoped := updateModel(t, loaded, loaded.fetchMessages()())

	assert.Equal(t, "Gmail", service.lastSearch.AccountID)
	require.Len(t, scoped.messages, 1)
	assert.Equal(t, "gmail-2", scoped.messages[0].ID)
	assert.Equal(t, "Sent • Gmail", currentMailboxTitle(scoped))
}

func TestEscClearsActiveAccountScope(t *testing.T) {
	service := &messageServiceStub{inbox: sampleAccountMessages()}
	m := testModelWithService(service)
	m.state = stateSidebar
	m.sidebarItems = []string{"Inbox", "Sent", "Drafts", "Archive", "Trash", "Spam", "", "Accounts:", "  Outlook", "  Gmail"}
	m.accountNames = []string{"Outlook", "Gmail"}
	m.sidebarCursor = 1
	m.activeAccountID = "Gmail"

	updatedAny, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := updatedAny.(Model)

	assert.Empty(t, updated.activeAccountID)
	assert.Equal(t, "Account scope cleared", updated.statusMessage)
	require.NotNil(t, cmd)

	reloaded := updateModel(t, updated, cmd())
	assert.Empty(t, service.lastSearch.AccountID)
	assert.Equal(t, "Sent", currentMailboxTitle(reloaded))
}

func TestTagHotkeySelectsSidebarTagAndLoadsMessages(t *testing.T) {
	service := &messageServiceStub{inbox: []*models.Message{
		{ID: "github-1", AccountID: "personal", Subject: "GitHub alert", Labels: []string{"inbox", "github"}},
		{ID: "work-1", AccountID: "personal", Subject: "Work update", Labels: []string{"inbox", "work"}},
	}}
	m := testModelWithService(service)
	m.state = stateSidebar

	updatedAny, cmd := m.Update(keyRune('g'))
	updated := updatedAny.(Model)

	assert.Equal(t, "github", updated.activeTagID)
	assert.Equal(t, "Tag selected: github", updated.statusMessage)
	require.NotNil(t, cmd)

	loaded := updateModel(t, updated, cmd())

	assert.Equal(t, "github", service.lastLabelQuery)
	assert.Equal(t, "github", loaded.activeTagID)
	require.Len(t, loaded.messages, 1)
	assert.Equal(t, "github-1", loaded.messages[0].ID)
	assert.Equal(t, "github", currentMailboxTitle(loaded))
	assert.Contains(t, renderSidebar(loaded, 24, 18), "> [G] github")
}

func TestSidebarArrowNavigationMovesIntoAndOutOfTags(t *testing.T) {
	service := &messageServiceStub{inbox: []*models.Message{
		{ID: "github-1", AccountID: "personal", Subject: "GitHub alert", Labels: []string{"inbox", "github"}},
		{ID: "work-1", AccountID: "personal", Subject: "Work update", Labels: []string{"inbox", "work"}},
	}}
	m := testModelWithService(service)
	m.state = stateSidebar
	m.sidebarCursor = len(m.sidebarItems) - 1

	updatedAny, cmd := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	updated := updatedAny.(Model)
	require.NotNil(t, cmd)
	loaded := updateModel(t, updated, cmd())

	assert.Equal(t, "github", loaded.activeTagID)
	assert.Equal(t, "github", service.lastLabelQuery)

	updatedAny, cmd = loaded.Update(tea.KeyMsg{Type: tea.KeyDown})
	updated = updatedAny.(Model)
	require.NotNil(t, cmd)
	loaded = updateModel(t, updated, cmd())

	assert.Equal(t, "work", loaded.activeTagID)
	assert.Equal(t, "work", service.lastLabelQuery)

	updatedAny, cmd = loaded.Update(tea.KeyMsg{Type: tea.KeyUp})
	updated = updatedAny.(Model)
	require.NotNil(t, cmd)
	loaded = updateModel(t, updated, cmd())

	assert.Equal(t, "github", loaded.activeTagID)

	updatedAny, cmd = loaded.Update(tea.KeyMsg{Type: tea.KeyUp})
	updated = updatedAny.(Model)
	require.NotNil(t, cmd)
	loaded = updateModel(t, updated, cmd())

	assert.Empty(t, loaded.activeTagID)
	assert.Equal(t, len(loaded.sidebarItems)-1, loaded.sidebarCursor)
}

func TestMessagesLoadedSetsEmptyMailboxStatus(t *testing.T) {
	m := testModel()
	m.statusError = false

	updated, _ := m.Update(messagesLoadedMsg{})
	model := updated.(Model)

	assert.Equal(t, "No messages found", model.statusMessage)
	assert.False(t, model.statusError)
}

func TestMessagesLoadedKeepsErrorStatus(t *testing.T) {
	m := testModel()
	m.statusMessage = "sync failed"
	m.statusError = true

	updated, _ := m.Update(messagesLoadedMsg{})
	model := updated.(Model)

	assert.Equal(t, "sync failed", model.statusMessage)
	assert.True(t, model.statusError)
}

func TestMarkReadKeyMarksMessageAsReadAndRefreshes(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages()}
	m := testModelWithService(service)
	m.state = stateList
	require.False(t, m.messages[0].IsRead)

	updated, cmd := updateModelWithCmd(t, m, keyRune('m'))

	assert.Equal(t, []string{"msg-1"}, service.markReadCalls)
	assert.True(t, updated.messages[0].IsRead)
	assert.Equal(t, "Message marked as read", updated.statusMessage)
	assert.False(t, updated.statusError)
	require.NotNil(t, cmd)
}

func TestMarkReadKeyDoesNotCallServiceWhenMessageAlreadyRead(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages()}
	m := testModelWithService(service)
	m.state = stateList
	m.messages[0].IsRead = true

	updated, cmd := updateModelWithCmd(t, m, keyRune('m'))

	assert.Empty(t, service.markReadCalls)
	assert.Equal(t, "Message marked as read", updated.statusMessage)
	require.NotNil(t, cmd)
}

func TestRefreshKeyReloadsCurrentMailbox(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages()}
	m := testModelWithService(service)
	m.state = stateList
	m.statusMessage = "old status"

	updated, cmd := updateModelWithCmd(t, m, tea.KeyMsg{Type: tea.KeyCtrlR})

	assert.Equal(t, "Mailbox refreshed", updated.statusMessage)
	assert.False(t, updated.statusError)
	require.NotNil(t, cmd)

	loaded := updateModel(t, updated, cmd())
	assert.Len(t, loaded.messages, 2)
}

func TestWindowSizeMsgSetsWidthAndHeight(t *testing.T) {
	m := testModel()

	updated := updateModel(t, m, tea.WindowSizeMsg{Width: 180, Height: 50})

	assert.Equal(t, 180, updated.width)
	assert.Equal(t, 50, updated.height)
}

func TestDownloadKeyShowsNoAttachmentsStatus(t *testing.T) {
	m := testModel()
	m.state = stateList

	updated := updateModel(t, m, keyRune('s'))

	assert.Equal(t, "No attachments to save", updated.statusMessage)
	assert.False(t, updated.statusError)
}

func TestDownloadKeyShowsSavedStatusWithAttachments(t *testing.T) {
	service := &messageServiceStub{inbox: []*models.Message{
		{
			ID: "att-1", AccountID: "personal", Subject: "Has attachment",
			From: "sender@example.com", To: []string{"me@example.com"},
			Labels: []string{"inbox"},
			Attachments: []*models.Attachment{
				{Filename: "report.pdf", Size: 2048, MimeType: "application/pdf", Data: []byte("pdf content")},
				{Filename: "photo.jpg", Size: 1024, MimeType: "image/jpeg", Data: []byte("jpg content")},
			},
		},
	}}
	tmp := t.TempDir()
	original := downloadsDir
	downloadsDir = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { downloadsDir = original })

	m := testModelWithService(service)
	m.state = stateList

	updated, cmd := updateModelWithCmd(t, m, keyRune('s'))
	assert.Equal(t, "Saving attachments…", updated.statusMessage)
	require.NotNil(t, cmd, "download must run asynchronously")

	done := updateModel(t, updated, cmd())
	assert.Contains(t, done.statusMessage, "Saved 2 attachment(s)")
	assert.False(t, done.statusError)

	entries, err := os.ReadDir(tmp)
	require.NoError(t, err)
	assert.Len(t, entries, 2, "both attachments must be written owner-only")
	for _, entry := range entries {
		info, statErr := entry.Info()
		require.NoError(t, statErr)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "attachments must be 0600")
	}
}

func TestQuitCommandFromCommandPromptReturnsQuitCmd(t *testing.T) {
	m := testModel()
	m.openCommandPrompt()
	m.searchInput.SetValue("q")

	updatedAny, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	run := updatedAny.(Model)

	assert.False(t, run.commandActive)
	require.NotNil(t, cmd)
	assert.NotNil(t, cmd())
}

func TestQuitCommandBlockedByDirtyComposeState(t *testing.T) {
	m := testModel()
	m.enterComposeState(&models.Message{AccountID: "personal", From: "me@example.com", Subject: "Hello", Body: "Body"}, 1)
	m.activeDraft.Body = "Body\nchanged"

	updated := updateModel(t, m, keyRune(':'))
	require.True(t, updated.commandActive)
	updated.searchInput.SetValue("quit")

	updatedAny, cmd := updated.Update(tea.KeyMsg{Type: tea.KeyEnter})
	run := updatedAny.(Model)

	assert.Nil(t, cmd)
	assert.Equal(t, stateCompose, run.state)
	assert.Equal(t, "Unsaved draft. Save, send, or cancel before leaving compose", run.statusMessage)
	assert.True(t, run.statusError)
}

func TestSidebarNavigationKeysWinOverTagHotkeys(t *testing.T) {
	service := &messageServiceStub{inbox: []*models.Message{
		{ID: "learning-1", AccountID: "personal", Subject: "Course", Labels: []string{"inbox", "learning"}},
		{ID: "junk-1", AccountID: "personal", Subject: "Junk-ish", Labels: []string{"inbox", "journal"}},
	}}
	m := testModelWithService(service)
	m.allMessages = service.inbox
	m.sidebarTagSource = service.inbox
	m.state = stateSidebar

	// l is bound to Right, so it must move focus to the list, not pick the l-tag.
	updated := updateModel(t, m, keyRune('l'))
	assert.Empty(t, updated.activeTagID)
	assert.Equal(t, stateList, updated.state)

	// j is bound to Down, so it must move the sidebar cursor, not pick the j-tag.
	m.state = stateSidebar
	updated = updateModel(t, m, keyRune('j'))
	assert.Empty(t, updated.activeTagID)
	assert.Equal(t, stateSidebar, updated.state)
	assert.Equal(t, 1, updated.sidebarCursor)
}

func TestEscOnUnsavedComposeWarnsBeforeDiscarding(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages()}
	m := testModelWithService(service)
	m.enterComposeState(&models.Message{To: []string{"user@example.com"}, Subject: "Hello", Body: "Body"}, 1)

	// Simulate the user editing the draft after opening the composer.
	m.activeDraft.Body = "Body with new content"

	updated := updateModel(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, stateCompose, updated.state)
	assert.NotNil(t, updated.activeDraft)
	assert.Contains(t, updated.statusMessage, "Unsaved draft")

	updated = updateModel(t, updated, tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, stateList, updated.state)
	assert.Nil(t, updated.activeDraft)
	assert.Equal(t, "Compose cancelled", updated.statusMessage)
}

func TestNonEscKeyDisarmsComposeDiscard(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages()}
	m := testModelWithService(service)
	m.enterComposeState(&models.Message{To: []string{"user@example.com"}, Subject: "Hello", Body: "Body"}, 1)
	m.activeDraft.Body = "Body with new content"

	updated := updateModel(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	assert.Contains(t, updated.statusMessage, "Unsaved draft")

	// Any other key keeps composing and disarms the pending discard.
	updated = updateModel(t, updated, keyRune('j'))
	assert.Equal(t, stateCompose, updated.state)
	assert.False(t, updated.composeDiscardArmed)

	updated = updateModel(t, updated, tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, stateCompose, updated.state, "first esc after disarm must warn again")
	assert.Contains(t, updated.statusMessage, "Unsaved draft")
}

func TestGotoChordsSwitchMailbox(t *testing.T) {
	cases := []struct {
		key  rune
		item string
	}{
		{'i', "Inbox"},
		{'s', "Sent"},
		{'d', "Drafts"},
		{'a', "Archive"},
		{'t', "Trash"},
		{'!', "Spam"},
	}
	for _, tc := range cases {
		t.Run(string(tc.key), func(t *testing.T) {
			service := &messageServiceStub{inbox: sampleMessages()}
			m := testModelWithService(service)
			m.state = stateList

			updated := updateModel(t, m, keyRune('g'))
			assert.Equal(t, "g", updated.pendingMotion)

			updated, cmd := updateModelWithCmd(t, updated, keyRune(tc.key))
			assert.Equal(t, stateList, updated.state)
			assert.Equal(t, tc.item, strings.TrimSpace(updated.sidebarItems[updated.sidebarCursor]))
			assert.Equal(t, "Switched to "+tc.item, updated.statusMessage)
			assert.Empty(t, updated.pendingMotion)
			require.NotNil(t, cmd, "goto must reload the list")
		})
	}
}

func TestGotoWorksFromContentState(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages()}
	m := testModelWithService(service)
	m.state = stateContent

	updated := updateModel(t, m, keyRune('g'))
	updated, cmd := updateModelWithCmd(t, updated, keyRune('s'))

	assert.Equal(t, stateList, updated.state)
	assert.Equal(t, "Sent", strings.TrimSpace(updated.sidebarItems[updated.sidebarCursor]))
	require.NotNil(t, cmd)
}

func TestGDDoesNotDeleteMessage(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages()}
	m := testModelWithService(service)
	m.state = stateList
	m.messages = service.inbox
	before := len(m.messages)

	updated := updateModel(t, m, keyRune('g'))
	updated = updateModel(t, updated, keyRune('d'))

	assert.Empty(t, service.toggleDeleteCalls, "gd must switch to Drafts, not trash the message")
	assert.Len(t, m.messages, before)
	assert.Equal(t, "Drafts", strings.TrimSpace(updated.sidebarItems[updated.sidebarCursor]))
}

func TestGThenUnknownKeyFallsThrough(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages()}
	m := testModelWithService(service)
	m.state = stateList
	m.messages = service.inbox
	m.listCursor = 0

	updated := updateModel(t, m, keyRune('g'))
	updated = updateModel(t, updated, keyRune('j'))

	assert.Empty(t, updated.pendingMotion)
	assert.Equal(t, 1, updated.listCursor, "g followed by j must fall through to plain movement")
}

func TestCountThenGotoDropsCount(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages()}
	m := testModelWithService(service)
	m.state = stateList

	updated := updateModel(t, m, keyRune('3'))
	updated = updateModel(t, updated, keyRune('g'))
	updated, cmd := updateModelWithCmd(t, updated, keyRune('i'))

	assert.Equal(t, "Inbox", strings.TrimSpace(updated.sidebarItems[updated.sidebarCursor]))
	assert.Empty(t, updated.pendingCount)
	require.NotNil(t, cmd)
}

func TestStarSearchesBySenderEmail(t *testing.T) {
	service := &messageServiceStub{inbox: []*models.Message{
		{ID: "m1", From: "Alice Example <alice@example.com>", Subject: "Hello", Labels: []string{"inbox"}},
		{ID: "m2", From: "alice@example.com", Subject: "Second", Labels: []string{"inbox"}},
	}}
	m := testModelWithService(service)
	m.state = stateList
	m.messages = service.inbox
	m.listCursor = 0

	updated, cmd := updateModelWithCmd(t, m, keyRune('*'))

	assert.Equal(t, "alice@example.com", updated.searchQuery)
	require.NotNil(t, cmd)
	loaded := updateModel(t, updated, cmd())
	assert.Contains(t, loaded.statusMessage, "messages from alice@example.com")
}

func TestHashSearchesBySubjectWithoutRePrefix(t *testing.T) {
	service := &messageServiceStub{inbox: []*models.Message{
		{ID: "m1", From: "bob@example.com", Subject: "Re: Fwd: Quarterly report", Labels: []string{"inbox"}},
	}}
	m := testModelWithService(service)
	m.state = stateList
	m.messages = service.inbox
	m.listCursor = 0

	updated, cmd := updateModelWithCmd(t, m, keyRune('#'))

	assert.Equal(t, "Quarterly report", updated.searchQuery)
	require.NotNil(t, cmd)
	loaded := updateModel(t, updated, cmd())
	assert.Contains(t, loaded.statusMessage, "matching Quarterly report")
}

func TestYankChordsCopyToClipboard(t *testing.T) {
	var captured string
	original := clipboardWriteAll
	clipboardWriteAll = func(text string) error { captured = text; return nil }
	t.Cleanup(func() { clipboardWriteAll = original })

	service := &messageServiceStub{inbox: []*models.Message{
		{ID: "m1", From: "Carol <carol@example.com>", Subject: "Agenda", Body: "Full body text", Labels: []string{"inbox"}},
	}}
	newModel := func() Model {
		m := testModelWithService(service)
		m.state = stateList
		m.messages = service.inbox
		m.listCursor = 0
		return m
	}

	m := updateModel(t, newModel(), keyRune('y'))
	assert.Equal(t, "y", m.pendingMotion)
	m = updateModel(t, m, keyRune('y'))
	assert.Equal(t, "Full body text", captured)
	assert.Equal(t, "Yanked message body", m.statusMessage)

	m = updateModel(t, updateModel(t, newModel(), keyRune('y')), keyRune('s'))
	assert.Equal(t, "Agenda", captured)
	assert.Equal(t, "Yanked subject", m.statusMessage)

	m = updateModel(t, updateModel(t, newModel(), keyRune('y')), keyRune('f'))
	assert.Equal(t, "carol@example.com", captured)
	assert.Equal(t, "Yanked carol@example.com", m.statusMessage)
}

func TestYankClipboardErrorShowsError(t *testing.T) {
	original := clipboardWriteAll
	clipboardWriteAll = func(string) error { return errors.New("no clipboard") }
	t.Cleanup(func() { clipboardWriteAll = original })

	service := &messageServiceStub{inbox: sampleMessages()}
	m := testModelWithService(service)
	m.state = stateList
	m.messages = service.inbox

	updated := updateModel(t, m, keyRune('y'))
	updated = updateModel(t, updated, keyRune('y'))

	assert.True(t, updated.statusError)
	assert.Equal(t, "Clipboard unavailable", updated.statusMessage)
}

func TestYankUnknownSuffixFallsThroughToMovement(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages()}
	m := testModelWithService(service)
	m.state = stateList
	m.messages = service.inbox
	m.listCursor = 0

	updated := updateModel(t, m, keyRune('y'))
	updated = updateModel(t, updated, keyRune('j'))

	assert.Empty(t, updated.pendingMotion)
	assert.Equal(t, 1, updated.listCursor, "y followed by j must fall through to movement")
}

func TestYankOnEmptyListReportsNothing(t *testing.T) {
	original := clipboardWriteAll
	clipboardWriteAll = func(string) error { t.Fatal("clipboard must not be touched"); return nil }
	t.Cleanup(func() { clipboardWriteAll = original })

	service := &messageServiceStub{}
	m := testModelWithService(service)
	m.state = stateList

	updated := updateModel(t, m, keyRune('y'))
	updated = updateModel(t, updated, keyRune('y'))

	assert.Equal(t, "Nothing to yank", updated.statusMessage)
}

func visualTestMessages() []*models.Message {
	return []*models.Message{
		{ID: "v1", AccountID: "personal", Subject: "One", Labels: []string{"inbox"}, IsRead: true},
		{ID: "v2", AccountID: "personal", Subject: "Two", Labels: []string{"inbox"}, IsRead: true},
		{ID: "v3", AccountID: "personal", Subject: "Three", Labels: []string{"inbox"}},
		{ID: "v4", AccountID: "personal", Subject: "Four", Labels: []string{"inbox"}},
	}
}

func visualTestModel(service *messageServiceStub) Model {
	m := testModelWithService(service)
	m.state = stateList
	m.messages = service.inbox
	m.allMessages = service.inbox
	m.listCursor = 0
	return m
}

func TestVisualModeEntersAndHighlightsRange(t *testing.T) {
	service := &messageServiceStub{inbox: visualTestMessages()}
	m := visualTestModel(service)

	updated := updateModel(t, m, keyRune('V'))
	assert.True(t, updated.visualActive)
	assert.Equal(t, 0, updated.visualAnchor)

	updated = updateModel(t, updated, keyRune('j'))
	updated = updateModel(t, updated, keyRune('j'))
	assert.Equal(t, 2, updated.listCursor)

	assert.Equal(t, listCursorVisual, currentListCursorMode(updated, 0))
	assert.Equal(t, listCursorVisual, currentListCursorMode(updated, 1))
	assert.Equal(t, listCursorActive, currentListCursorMode(updated, 2))
	assert.Equal(t, listCursorNone, currentListCursorMode(updated, 3))
}

func TestVisualModeEscVAndQExitWithoutAction(t *testing.T) {
	for _, exit := range []tea.KeyMsg{{Type: tea.KeyEsc}, keyRune('V'), keyRune('q')} {
		service := &messageServiceStub{inbox: visualTestMessages()}
		m := visualTestModel(service)

		updated := updateModel(t, m, keyRune('V'))
		updated, cmd := updateModelWithCmd(t, updated, exit)

		assert.False(t, updated.visualActive)
		assert.Nil(t, cmd, "exit must not quit the app or trigger actions")
		assert.Empty(t, service.toggleDeleteCalls)
	}
}

func TestVisualModeDeleteTrashesRangeAndUndoRestores(t *testing.T) {
	service := &messageServiceStub{inbox: visualTestMessages()}
	m := visualTestModel(service)

	updated := updateModel(t, m, keyRune('V'))
	updated = updateModel(t, updated, keyRune('j'))
	updated = updateModel(t, updated, keyRune('j'))
	updated, cmd := updateModelWithCmd(t, updated, keyRune('d'))

	assert.Equal(t, []string{"v1", "v2", "v3"}, service.toggleDeleteCalls)
	assert.False(t, updated.visualActive)
	assert.Contains(t, updated.statusMessage, "3 messages moved to trash")
	require.NotNil(t, updated.pendingUndo)
	assert.Len(t, updated.pendingUndo.snapshots(), 3)
	require.NotNil(t, cmd)

	restored := updateModel(t, updated, keyRune('u'))
	assert.Equal(t, "Undo applied", restored.statusMessage)
	assert.Nil(t, restored.pendingUndo)
}

func TestVisualModeArchiveAndSpamRange(t *testing.T) {
	service := &messageServiceStub{inbox: visualTestMessages()}
	m := visualTestModel(service)
	updated := updateModel(t, m, keyRune('V'))
	updated = updateModel(t, updated, keyRune('j'))
	updated, _ = updateModelWithCmd(t, updated, keyRune('a'))
	assert.Equal(t, []string{"v1", "v2"}, service.archiveCalls)
	assert.False(t, updated.visualActive)

	service = &messageServiceStub{inbox: visualTestMessages()}
	m = visualTestModel(service)
	updated = updateModel(t, m, keyRune('V'))
	updated = updateModel(t, updated, keyRune('j'))
	updated, _ = updateModelWithCmd(t, updated, keyRune('!'))
	assert.Equal(t, []string{"v1", "v2"}, service.spamCalls)
	assert.False(t, updated.visualActive)
}

func TestVisualModeReverseSelection(t *testing.T) {
	service := &messageServiceStub{inbox: visualTestMessages()}
	m := visualTestModel(service)
	m.listCursor = 2

	updated := updateModel(t, m, keyRune('V'))
	updated = updateModel(t, updated, keyRune('k'))
	updated = updateModel(t, updated, keyRune('k'))
	_, _ = updateModelWithCmd(t, updated, keyRune('d'))

	assert.Equal(t, []string{"v1", "v2", "v3"}, service.toggleDeleteCalls, "reverse selection must act from the top of the range")
}

func TestVisualModeMarkReadRange(t *testing.T) {
	service := &messageServiceStub{inbox: visualTestMessages()}
	m := visualTestModel(service)
	m.listCursor = 2

	updated := updateModel(t, m, keyRune('V'))
	updated = updateModel(t, updated, keyRune('j'))
	updated, cmd := updateModelWithCmd(t, updated, keyRune('m'))

	assert.ElementsMatch(t, []string{"v3", "v4"}, service.markReadCalls)
	assert.False(t, updated.visualActive)
	assert.Contains(t, updated.statusMessage, "2 messages marked as read")
	require.NotNil(t, cmd)
}

func TestVisualModeGExtendsToBottom(t *testing.T) {
	service := &messageServiceStub{inbox: visualTestMessages()}
	m := visualTestModel(service)

	updated := updateModel(t, m, keyRune('V'))
	updated = updateModel(t, updated, keyRune('G'))
	_, _ = updateModelWithCmd(t, updated, keyRune('d'))

	assert.Len(t, service.toggleDeleteCalls, 4, "V+G+d must trash the whole list")
}

func TestVisualModeBlocksEnterAndCompose(t *testing.T) {
	service := &messageServiceStub{inbox: visualTestMessages()}
	m := visualTestModel(service)

	updated := updateModel(t, m, keyRune('V'))
	for _, blocked := range []tea.KeyMsg{{Type: tea.KeyEnter}, keyRune('c'), keyRune('r')} {
		updated = updateModel(t, updated, blocked)
		assert.Equal(t, stateList, updated.state)
		assert.True(t, updated.visualActive)
	}
	assert.Nil(t, updated.activeDraft)
}

func TestVisualModeUnavailableOutsideList(t *testing.T) {
	service := &messageServiceStub{inbox: visualTestMessages()}
	for _, state := range []SessionState{stateSidebar, stateContent} {
		m := visualTestModel(service)
		m.state = state
		updated := updateModel(t, m, keyRune('V'))
		assert.False(t, updated.visualActive)
	}
}

func TestVisualModeEmptyList(t *testing.T) {
	service := &messageServiceStub{}
	m := testModelWithService(service)
	m.state = stateList

	updated := updateModel(t, m, keyRune('V'))
	assert.False(t, updated.visualActive)
	assert.Equal(t, "No messages to select", updated.statusMessage)
}

func TestVisualAnchorClampsOnReload(t *testing.T) {
	service := &messageServiceStub{inbox: visualTestMessages()}
	m := visualTestModel(service)
	m.listCursor = 3
	m = updateModel(t, m, keyRune('V'))
	require.True(t, m.visualActive)

	smaller := visualTestMessages()[:2]
	m = updateModel(t, m, messagesLoadedMsg{messages: smaller})
	assert.True(t, m.visualActive)
	assert.LessOrEqual(t, m.visualAnchor, len(m.messages)-1)

	m = updateModel(t, m, messagesLoadedMsg{messages: nil})
	assert.False(t, m.visualActive, "empty reload must leave visual mode")
}

func TestApplyRepeatableActionAdvancesWhenMessageStays(t *testing.T) {
	// In a view where the acted-on message stays visible, a count must touch
	// successive rows instead of hammering the same one.
	service := &messageServiceStub{inbox: []*models.Message{
		{ID: "s1", AccountID: "personal", Subject: "One", Labels: []string{"inbox"}},
		{ID: "s2", AccountID: "personal", Subject: "Two", Labels: []string{"inbox"}},
		{ID: "s3", AccountID: "personal", Subject: "Three", Labels: []string{"inbox"}},
	}}
	m := visualTestModel(service)

	cmd := m.applyRepeatableAction(repeatableActionSpam, 3)
	_ = cmd

	assert.Equal(t, []string{"s1", "s2", "s3"}, service.spamCalls)
}

func composeCommandModel(service *messageServiceStub) Model {
	m := testModelWithService(service)
	m.enterComposeState(&models.Message{AccountID: "personal", To: []string{"user@example.com"}, Subject: "Hello", Body: "Body"}, 1)
	return m
}

func runComposeCommand(t *testing.T, m Model, command string) (Model, tea.Cmd) {
	t.Helper()
	m = updateModel(t, m, keyRune(':'))
	for _, r := range command {
		m = updateModel(t, m, keyRune(r))
	}
	return updateModelWithCmd(t, m, tea.KeyMsg{Type: tea.KeyEnter})
}

func TestComposeCommandWSavesAndStays(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages(), composedDraftID: "draft-1"}
	m := composeCommandModel(service)
	m.activeDraft.Body = "Edited body"

	updated, _ := runComposeCommand(t, m, "w")

	assert.Equal(t, stateCompose, updated.state, ":w must keep the composer open")
	assert.Equal(t, "Draft saved", updated.statusMessage)
	require.Len(t, service.composeCalls, 1)
}

func TestComposeCommandWQSavesAndCloses(t *testing.T) {
	for _, command := range []string{"wq", "x"} {
		service := &messageServiceStub{inbox: sampleMessages(), composedDraftID: "draft-1"}
		m := composeCommandModel(service)
		m.activeDraft.Body = "Edited body"

		updated, cmd := runComposeCommand(t, m, command)

		assert.Equal(t, stateList, updated.state, command)
		assert.Equal(t, "Draft saved", updated.statusMessage)
		require.Len(t, service.composeCalls, 1, command)
		require.NotNil(t, cmd)
	}
}

func TestComposeCommandQWithUnsavedBlocks(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages()}
	m := composeCommandModel(service)
	m.activeDraft.Body = "Edited body"

	updated, _ := runComposeCommand(t, m, "q")

	assert.Equal(t, stateCompose, updated.state)
	assert.True(t, updated.statusError)
	assert.Contains(t, updated.statusMessage, "Unsaved draft")
}

func TestComposeCommandQAfterWCloses(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages(), composedDraftID: "draft-1"}
	m := composeCommandModel(service)
	m.activeDraft.Body = "Edited body"

	updated, _ := runComposeCommand(t, m, "w")
	require.Equal(t, stateCompose, updated.state)

	updated, cmd := runComposeCommand(t, updated, "q")
	assert.Equal(t, stateList, updated.state, ":q after :w must close the composer")
	require.NotNil(t, cmd)
}

func TestComposeCommandQBangDiscards(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages()}
	m := composeCommandModel(service)
	m.activeDraft.Body = "Edited body"

	updated, cmd := runComposeCommand(t, m, "q!")

	assert.Equal(t, stateList, updated.state)
	assert.Equal(t, "Draft discarded", updated.statusMessage)
	assert.Empty(t, service.composeCalls, ":q! must not save the draft")
	require.NotNil(t, cmd)
}

func TestComposeCommandSendSendsDraft(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages(), composedDraftID: "draft-9"}
	m := composeCommandModel(service)
	m.activeDraft.Body = "Edited body"

	updated, cmd := runComposeCommand(t, m, "send")

	assert.Equal(t, stateList, updated.state)
	assert.Equal(t, "Message sent", updated.statusMessage)
	assert.Equal(t, []string{"draft-9"}, service.sendCalls)
	require.NotNil(t, cmd)
}

func TestCommandCandidatesDependOnState(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages()}
	m := testModelWithService(service)
	assert.Contains(t, m.commandPromptCandidates(), "inbox")
	assert.NotContains(t, m.commandPromptCandidates(), "wq")

	m = composeCommandModel(service)
	assert.Contains(t, m.commandPromptCandidates(), "wq")
	assert.NotContains(t, m.commandPromptCandidates(), "inbox")
}

func TestCommandWOutsideComposeFails(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages()}
	m := testModelWithService(service)
	m.state = stateList

	updated, _ := runComposeCommand(t, m, "w")

	assert.True(t, updated.statusError)
	assert.Equal(t, "No draft to save", updated.statusMessage)
}

func TestCommandQOutsideComposeQuits(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages()}
	m := testModelWithService(service)
	m.state = stateList

	m = updateModel(t, m, keyRune(':'))
	m = updateModel(t, m, keyRune('q'))
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	require.NotNil(t, cmd)
	assert.Equal(t, tea.Quit(), cmd())
}

func TestCtrlHCyclesFocusThroughPanes(t *testing.T) {
	service := &messageServiceStub{inbox: sampleMessages()}
	m := testModelWithService(service)
	m.messages = service.inbox
	m.state = stateSidebar

	updated, cmd := updateModelWithCmd(t, m, tea.KeyMsg{Type: tea.KeyCtrlH})
	assert.Equal(t, stateList, updated.state, "sidebar -> list")
	require.NotNil(t, cmd, "entering the list must reload messages")

	updated = updateModel(t, updated, tea.KeyMsg{Type: tea.KeyCtrlH})
	assert.Equal(t, stateContent, updated.state, "list -> reader")

	updated = updateModel(t, updated, tea.KeyMsg{Type: tea.KeyCtrlH})
	assert.Equal(t, stateSidebar, updated.state, "reader -> sidebar (wrap)")
}

func TestCtrlHInVisualModeStaysInList(t *testing.T) {
	service := &messageServiceStub{inbox: visualTestMessages()}
	m := visualTestModel(service)

	updated := updateModel(t, m, keyRune('V'))
	updated = updateModel(t, updated, tea.KeyMsg{Type: tea.KeyCtrlH})

	assert.Equal(t, stateList, updated.state, "visual selection must not lose the list pane")
	assert.True(t, updated.visualActive)
}
