package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/kriuchkov/postero/internal/core/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestViewRendersMessageContent(t *testing.T) {
	m := testModel()
	m.width = 120
	m.height = 40
	m.state = stateList

	view := m.View()

	assert.Contains(t, view, "Subject 1")
	assert.Contains(t, view, "sender1@example.com")
	assert.Contains(t, view, "Body 1")
}

func TestViewRendersStatusMessage(t *testing.T) {
	m := testModel()
	m.width = 120
	m.height = 40
	m.statusMessage = "Message sent"

	view := m.View()

	assert.Contains(t, view, "Message sent")
}

func TestViewShowsDraftEditActionForDraftSelection(t *testing.T) {
	m := testModel()
	m.width = 120
	m.height = 40
	m.messages = sampleDraftMessages()
	m.state = stateList

	view := m.View()

	assert.Contains(t, view, "Edit Draft")
}

func TestComposeViewRendersAccountSelector(t *testing.T) {
	m := testModel()
	m.width = 120
	m.height = 40
	m.enterComposeState(&models.MessageDTO{AccountID: "personal", From: "me@example.com", Subject: "Hello", Body: "Body"}, 0)

	view := m.View()

	assert.Contains(t, view, "Account:")
	assert.Contains(t, view, "personal <me@example.com>")
	assert.Contains(t, view, "h/l")
	assert.Contains(t, view, "Account")
}

func TestReplyViewRendersComposeContext(t *testing.T) {
	m := testModel()
	m.width = 120
	m.height = 40
	m.state = stateList
	m = updateModel(t, m, keyRune('r'))

	view := m.View()

	assert.Contains(t, view, "Reply")
	assert.Contains(t, view, "Type above the quoted message")
}

func TestComposeViewKeepsTopHeaderVisible(t *testing.T) {
	m := testModel()
	m.width = 120
	m.height = 40
	m.enterComposeState(&models.MessageDTO{ID: "draft-1", AccountID: "personal", From: "me@example.com", Subject: "Hello", Body: "Body"}, 1)

	view := m.View()

	assert.Contains(t, view, "Postero")
	assert.Contains(t, view, "Edit Draft")
	assert.Contains(t, view, "Esc Cancel")
	assert.Contains(t, view, "j/k Fields")
}

func TestViewFitsViewportWithManyDrafts(t *testing.T) {
	m := testModel()
	m.width = 120
	m.height = 24
	m.state = stateList
	m.sidebarCursor = 2
	m.messages = manyDraftMessages(30)

	view := m.View()

	assert.LessOrEqual(t, lipgloss.Height(view), m.height)
	assert.Contains(t, view, "Postero")
	assert.Contains(t, view, "Drafts")
}

func TestPaneWidthsClampSidebarWidth(t *testing.T) {
	sidebarWidth, _, _ := paneWidths(80)
	assert.GreaterOrEqual(t, sidebarWidth, minSidebarWidth)
	assert.LessOrEqual(t, sidebarWidth, maxSidebarWidth)

	sidebarWidth, _, _ = paneWidths(220)
	assert.GreaterOrEqual(t, sidebarWidth, minSidebarWidth)
	assert.LessOrEqual(t, sidebarWidth, maxSidebarWidth)
}

func TestViewRendersFixedFooterHelpBar(t *testing.T) {
	m := testModel()
	m.width = 120
	m.height = 30

	view := m.View()

	assert.Contains(t, view, "Ready")
	assert.Contains(t, view, "q")
	assert.Contains(t, view, "quit")
}

func TestRenderFooterShowsErrorStatus(t *testing.T) {
	m := testModel()
	m.statusMessage = "sync failed"
	m.statusError = true

	footer := renderFooter(m, 100)

	assert.Contains(t, footer, "sync failed")
	assert.Contains(t, footer, "q")
	assert.Contains(t, footer, "quit")
}

func TestListShowsScrollIndicatorForLongLists(t *testing.T) {
	m := testModel()
	m.messages = manyDraftMessages(30)

	list := renderList(m, 40, 20)

	assert.Contains(t, list, "▎")
}

func TestListHidesScrollIndicatorForShortLists(t *testing.T) {
	m := testModel()
	m.messages = sampleMessages()

	list := renderList(m, 40, 20)

	assert.NotContains(t, list, "▎")
}

func TestRenderMessageChipsUsesDifferentSelectedStyling(t *testing.T) {
	msg := &models.MessageDTO{IsRead: false, IsDraft: true, IsSpam: true, Labels: []string{"archive"}}

	plain := renderMessageChips(msg, false)
	selected := renderMessageChips(msg, true)

	assert.Contains(t, plain, "Unread")
	assert.Contains(t, plain, "Draft")
	assert.Contains(t, plain, "Spam")
	assert.Contains(t, plain, "Archive")
	assert.Contains(t, selected, "Unread")
	assert.Contains(t, selected, "Draft")
	assert.Contains(t, selected, "Spam")
	assert.Contains(t, selected, "Archive")
	assert.NotEqual(t, unreadChipStyle(false).GetBackground(), unreadChipStyle(true).GetBackground())
	assert.NotEqual(t, draftChipStyle(false).GetBackground(), draftChipStyle(true).GetBackground())
	assert.NotEqual(t, spamChipStyle(false).GetBackground(), spamChipStyle(true).GetBackground())
	assert.NotEqual(t, archiveChipStyle(false).GetBackground(), archiveChipStyle(true).GetBackground())
}

func TestRenderSidebarShowsSectionsAndAccountLabels(t *testing.T) {
	m := testModel()
	m.sidebarItems = []string{"Inbox", "Drafts", "", "Accounts:", "  personal", "  work"}
	m.sidebarCursor = 4

	sidebar := renderSidebar(m, 24, 18)

	assert.Contains(t, sidebar, "Mailboxes")
	assert.Contains(t, sidebar, "Favorites")
	assert.Contains(t, sidebar, "Accounts")
	assert.Contains(t, sidebar, "◎ Inbox")
	assert.Contains(t, sidebar, "✎ Drafts")
	assert.Contains(t, sidebar, "• personal")
}

func TestSidebarItemStyleUsesDifferentSelectedAndMutedPalette(t *testing.T) {
	m := testModel()
	selected := sidebarItemStyle(m, true, false)
	plain := sidebarItemStyle(m, false, false)
	muted := sidebarItemStyle(m, false, true)

	assert.NotEqual(t, plain.GetBackground(), selected.GetBackground())
	assert.NotEqual(t, plain.GetForeground(), selected.GetForeground())
	assert.NotEqual(t, plain.GetForeground(), muted.GetForeground())
	assert.True(t, selected.GetBold())
	assert.False(t, plain.GetBold())
}

func TestRenderContentShowsComposeLoadingState(t *testing.T) {
	m := testModel()
	m.state = stateCompose
	m.activeDraft = nil

	content := renderContent(m, 60, 20)

	assert.Contains(t, content, "Loading draft")
}

func TestViewShowsInitialisingWithoutSize(t *testing.T) {
	m := testModel()

	assert.Equal(t, "Initialising...", m.View())
}

func TestRenderContentShowsEmptySelection(t *testing.T) {
	m := testModel()
	m.width = 120
	m.height = 40
	m.messages = nil

	content := renderContent(m, 40, 20)

	assert.Contains(t, content, "Welcome to Postero")
	assert.Contains(t, content, "Choose a mailbox and select a")
	assert.Contains(t, content, "message to start reading")
}

func TestViewShowsSearchModeAndFilterSummary(t *testing.T) {
	m := testModel()
	m.width = 120
	m.height = 40
	m.searchActive = true
	m.searchInput.Focus()
	m.searchInput.SetValue("sender")
	m.searchQuery = "sender"
	m.applySearchFilter()

	view := m.View()

	assert.Contains(t, view, "/ Search")
	assert.Contains(t, view, "Filter: sender")
	assert.Contains(t, view, "Search:")
}

func TestViewShowsUndoActionWhenPending(t *testing.T) {
	m := testModel()
	m.width = 120
	m.height = 40
	m.pendingUndo = &undoState{message: cloneMessageDTO(m.messages[0]), action: "trash", token: 1}

	view := m.View()

	assert.Contains(t, view, "u Undo")
}

func TestViewShowsActiveAccountScopeInHeader(t *testing.T) {
	m := testModel()
	m.width = 120
	m.height = 40
	m.sidebarItems = []string{"Inbox", "Sent", "", "Accounts:", "  Gmail"}
	m.sidebarCursor = 1
	m.activeAccountID = "Gmail"

	view := m.View()

	assert.Contains(t, view, "Sent • Gmail")
	assert.Contains(t, view, "Account: Gmail")
	assert.Contains(t, view, "Esc clears scope")
}

func TestListShowsSearchSpecificEmptyState(t *testing.T) {
	m := testModel()
	m.searchQuery = "missing"
	m.messages = nil
	m.allMessages = sampleMessages()

	list := renderList(m, 44, 18)

	assert.Contains(t, list, "No matches")
	assert.Contains(t, list, "Refine the filter")
	assert.Contains(t, list, "clear")
}

func TestRenderContentKeepsMessageHeaderVisibleWhileBodyIsScrolled(t *testing.T) {
	m := testModel()
	m.width = 120
	m.height = 24
	m.state = stateContent
	m.messages = sampleLongMessage()
	m.syncContentViewport(true)
	m.contentViewport.LineDown(8)

	content := renderContent(m, 60, 18)

	assert.Contains(t, content, "Long body message")
	assert.Contains(t, content, "From:")
	assert.Contains(t, content, "Mailbox:")
	assert.Contains(t, content, "ctrl+d/u")
	assert.Contains(t, content, "/40")
	assert.Contains(t, content, "Line 13:09 of a long message body")
	assert.NotContains(t, content, "Line 13:01 of a long message body")
}

func TestSelectedDraftReturnsCopyOfDraft(t *testing.T) {
	m := testModel()
	m.state = stateList
	m.messages = sampleDraftMessages()

	draft, ok := m.selectedDraft()
	require.True(t, ok)
	require.NotNil(t, draft)
	assert.Equal(t, "draft-1", draft.ID)

	draft.Subject = "Changed"
	assert.Equal(t, "Draft subject", m.messages[0].Subject)
}
