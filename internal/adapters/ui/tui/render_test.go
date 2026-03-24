package tui

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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
	assert.NotContains(t, view, "Type above the quoted message")
}

func TestComposeViewKeepsTopHeaderVisible(t *testing.T) {
	m := testModel()
	m.width = 120
	m.height = 40
	m.enterComposeState(&models.MessageDTO{ID: "draft-1", AccountID: "personal", From: "me@example.com", Subject: "Hello", Body: "Body"}, 1)

	header := renderHeader(m, m.width)

	assert.Contains(t, header, "Postero")
	assert.Contains(t, header, "Edit Draft")
	assert.Contains(t, header, "ESC")
	assert.Contains(t, header, "back")
	assert.Contains(t, header, "CTRL+O")
	assert.Contains(t, header, "save")
	assert.Contains(t, header, "CTRL+X")
	assert.Contains(t, header, "send")
	assert.Contains(t, header, "I")
	assert.Contains(t, header, "edit")
	assert.Contains(t, header, "J/K")
	assert.Contains(t, header, "move")
	assert.NotContains(t, header, "H/L")
	assert.NotContains(t, header, "acct")
	assert.Contains(t, header, "O/O")
	assert.Contains(t, header, "body")
}

func TestComposeHeaderShowsAccountActionOnlyOnAccountFocus(t *testing.T) {
	m := testModel()
	m.width = 120
	m.height = 40
	m.enterComposeState(&models.MessageDTO{AccountID: "personal", From: "me@example.com", Subject: "Hello", Body: "Body"}, 0)

	header := renderHeader(m, m.width)
	assert.Contains(t, header, "H/L")
	assert.Contains(t, header, "acct")

	m.focusIndex = 1
	m.applyComposeFocus()
	header = renderHeader(m, m.width)
	assert.NotContains(t, header, "H/L")
	assert.NotContains(t, header, "acct")
}

func TestComposeHeaderHidesBodyActionInInsertMode(t *testing.T) {
	m := testModel()
	m.width = 120
	m.height = 40
	m.enterComposeState(&models.MessageDTO{AccountID: "personal", From: "me@example.com", Subject: "Hello", Body: "Body"}, 3)

	header := renderHeader(m, m.width)
	assert.Contains(t, header, "O/O")
	assert.Contains(t, header, "body")

	m.composeEditing = true
	m.applyComposeFocus()
	header = renderHeader(m, m.width)
	assert.NotContains(t, header, "O/O")
	assert.NotContains(t, header, "body")
}

func TestComposeHeaderContextHighlightsCurrentAction(t *testing.T) {
	m := testModel()
	m.enterComposeState(&models.MessageDTO{AccountID: "personal", From: "me@example.com", Subject: "Hello", Body: "Body"}, 0)

	context := currentComposeHeaderContext(m)
	assert.True(t, context.showAccount)
	assert.True(t, context.emphasizeAccount)
	assert.True(t, context.showBody)
	assert.False(t, context.emphasizeBody)
	assert.False(t, context.emphasizeMode)

	m.focusIndex = 2
	m.applyComposeFocus()
	context = currentComposeHeaderContext(m)
	assert.False(t, context.showAccount)
	assert.True(t, context.showBody)
	assert.False(t, context.emphasizeBody)
	assert.True(t, context.emphasizeMode)

	m.focusIndex = 3
	m.applyComposeFocus()
	context = currentComposeHeaderContext(m)
	assert.True(t, context.showBody)
	assert.True(t, context.emphasizeBody)
	assert.False(t, context.emphasizeMode)

	m.composeEditing = true
	m.applyComposeFocus()
	context = currentComposeHeaderContext(m)
	assert.False(t, context.showAccount)
	assert.False(t, context.showBody)
	assert.True(t, context.emphasizeMode)
}

func TestComposeHeaderHidesMoveActionInInsertMode(t *testing.T) {
	m := testModel()
	m.width = 120
	m.height = 40
	m.enterComposeState(&models.MessageDTO{AccountID: "personal", From: "me@example.com", Subject: "Hello", Body: "Body"}, 2)

	header := renderHeader(m, m.width)
	assert.Contains(t, header, "J/K")
	assert.Contains(t, header, "move")

	m.composeEditing = true
	m.applyComposeFocus()
	header = renderHeader(m, m.width)
	assert.NotContains(t, header, "J/K")
	assert.NotContains(t, header, "move")
}

func TestComposeHeaderShowsSingleContextActionOnMediumWidth(t *testing.T) {
	m := testModel()
	m.width = 96
	m.height = 24
	m.enterComposeState(&models.MessageDTO{AccountID: "personal", From: "me@example.com", Subject: "Hello", Body: "Body"}, 0)

	header := renderHeader(m, m.width)
	assert.Contains(t, header, "H/L")
	assert.Contains(t, header, "acct")
	assert.NotContains(t, header, "J/K")
	assert.NotContains(t, header, "move")
	assert.NotContains(t, header, "O/O")

	m.focusIndex = 3
	m.applyComposeFocus()
	header = renderHeader(m, m.width)
	assert.Contains(t, header, "O/O")
	assert.Contains(t, header, "body")
	assert.NotContains(t, header, "H/L")
	assert.NotContains(t, header, "J/K")

	m.focusIndex = 2
	m.applyComposeFocus()
	header = renderHeader(m, m.width)
	assert.Contains(t, header, "J/K")
	assert.Contains(t, header, "move")
	assert.NotContains(t, header, "H/L")
	assert.NotContains(t, header, "O/O")
}

func TestComposeHeaderOmitsContextActionOnTightMediumWidth(t *testing.T) {
	m := testModel()
	m.width = 88
	m.height = 24
	m.enterComposeState(&models.MessageDTO{AccountID: "personal", From: "me@example.com", Subject: "Hello", Body: "Body"}, 3)

	header := renderHeader(m, m.width)
	assert.Contains(t, header, "CTRL+O")
	assert.Contains(t, header, "CTRL+X")
	assert.Contains(t, header, "I")
	assert.Contains(t, header, "edit")
	assert.NotContains(t, header, "J/K")
	assert.NotContains(t, header, "H/L")
	assert.NotContains(t, header, "O/O")
}

func TestComposeHeaderActionSpecsEmphasizeCurrentContext(t *testing.T) {
	m := testModel()
	m.width = 120
	m.enterComposeState(&models.MessageDTO{AccountID: "personal", From: "me@example.com", Subject: "Hello", Body: "Body"}, 0)

	actions := composeHeaderActionSpecs(m, m.width)
	assert.Contains(t, actions, composeHeaderActionSpec{key: "H/L", action: "acct", tone: composeActionSecondary, emphasize: true})
	assert.Contains(t, actions, composeHeaderActionSpec{key: "O/O", action: "body", tone: composeActionSecondary, emphasize: false})

	m.focusIndex = 2
	m.applyComposeFocus()
	actions = composeHeaderActionSpecs(m, m.width)
	assert.Contains(t, actions, composeHeaderActionSpec{key: "I", action: "edit", tone: composeActionSecondary, emphasize: true})

	m.composeEditing = true
	m.applyComposeFocus()
	actions = composeHeaderActionSpecs(m, m.width)
	assert.Contains(t, actions, composeHeaderActionSpec{key: "ENTER", action: "next", tone: composeActionSecondary, emphasize: true})
}

func TestCurrentListCursorModeTracksActiveAndPassivePane(t *testing.T) {
	m := testModel()
	m.listCursor = 1
	m.state = stateList

	assert.Equal(t, listCursorActive, currentListCursorMode(m, 1))
	assert.Equal(t, listCursorNone, currentListCursorMode(m, 0))

	m.state = stateContent
	assert.Equal(t, listCursorPassive, currentListCursorMode(m, 1))
}

func TestPaneTitleStyleHighlightsActivePane(t *testing.T) {
	m := testModel()
	m.state = stateContent

	active := paneTitleStyle(m, stateContent)
	inactive := paneTitleStyle(m, stateList)

	assert.NotEqual(t, inactive.GetForeground(), active.GetForeground())
	assert.True(t, active.GetBold())
	assert.True(t, inactive.GetBold())
}

func TestComposeViewCompactsHeaderActionsOnNarrowWidth(t *testing.T) {
	m := testModel()
	m.width = 78
	m.height = 24
	m.enterComposeState(&models.MessageDTO{ID: "draft-1", AccountID: "personal", From: "me@example.com", Subject: "Hello", Body: "Body"}, 1)

	header := renderHeader(m, m.width)

	assert.Contains(t, header, "^O")
	assert.Contains(t, header, "save")
	assert.Contains(t, header, "^X")
	assert.Contains(t, header, "send")
	assert.Contains(t, header, "I")
	assert.Contains(t, header, "edit")
	assert.NotContains(t, header, "ENTER")
	assert.NotContains(t, header, "next")
	assert.NotContains(t, header, "nl")
	assert.NotContains(t, header, "j/k Fields")
	assert.NotContains(t, header, "h/l Account")
	assert.NotContains(t, header, "o/O Body")
}

func TestReplyViewUsesCompactNormalModeHint(t *testing.T) {
	m := testModel()
	m.width = 72
	m.height = 24
	m.state = stateList
	m = updateModel(t, m, keyRune('r'))
	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyEsc})

	view := m.View()

	assert.Contains(t, view, "Normal. Enter/i/o/O edit.")
	assert.NotContains(t, view, "Navigation mode. Press Enter, i, o, or O to start editing from the current context.")
	assert.NotContains(t, view, "Normal mode.")
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
	m.state = stateList

	view := m.View()

	assert.Contains(t, view, "Ready")
	assert.Contains(t, view, "j/k move")
	assert.Contains(t, view, "enter/l read")
	assert.Contains(t, view, "gg/G")
	assert.Contains(t, view, "H/M/L")
}

func TestRenderFooterShowsErrorStatus(t *testing.T) {
	m := testModel()
	m.statusMessage = "sync failed"
	m.statusError = true

	footer := renderFooter(m, 100)

	assert.Contains(t, footer, "sync failed")
	assert.Contains(t, footer, "/ search")
}

func TestRenderFooterShowsComposeModeSpecificHelp(t *testing.T) {
	m := testModel()
	m.width = 140
	m.height = 30
	m.enterComposeState(&models.MessageDTO{AccountID: "personal", From: "me@example.com", Subject: "Hello", Body: "Body"}, 0)

	footer := renderFooter(m, 140)
	assert.Contains(t, footer, "h/l acct")
	assert.Contains(t, footer, "enter next")
	assert.Contains(t, footer, "ctrl+o save")

	m.composeEditing = true
	m.applyComposeFocus()
	footer = renderFooter(m, 140)
	assert.Contains(t, footer, "esc normal")
	assert.Contains(t, footer, "enter next")
	assert.Contains(t, footer, "ctrl+x send")
}

func TestRenderFooterShowsComposeFieldSpecificHelp(t *testing.T) {
	m := testModel()
	m.width = 140
	m.height = 30
	m.enterComposeState(&models.MessageDTO{AccountID: "personal", From: "me@example.com", Subject: "Hello", Body: "Body"}, 2)

	footer := renderFooter(m, 140)
	assert.Contains(t, footer, "enter/i edit")
	assert.Contains(t, footer, "o/O body")
	assert.NotContains(t, footer, "h/l acct")

	m.focusIndex = 3
	m.applyComposeFocus()
	footer = renderFooter(m, 140)
	assert.Contains(t, footer, "o/O body")
	assert.Contains(t, footer, "enter edit")
	assert.NotContains(t, footer, "h/l acct")

	m.composeEditing = true
	m.applyComposeFocus()
	footer = renderFooter(m, 140)
	assert.Contains(t, footer, "enter newline")
	assert.NotContains(t, footer, "enter next")
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
	m.openSearchPrompt()
	m.searchInput.SetValue("sender")
	m.searchQuery = "sender"
	m.applySearchFilter()

	view := m.View()

	assert.Contains(t, view, "/ Search")
	assert.Contains(t, view, ": Cmd")
	assert.Contains(t, view, "/ sender")
	assert.Contains(t, view, "live filter")
}

func TestViewShowsCommandModePrompt(t *testing.T) {
	m := testModel()
	m.width = 120
	m.height = 40
	m.openCommandPrompt()
	m.searchInput.SetValue("drafts")

	view := m.View()

	assert.Contains(t, view, "drafts")
	assert.Contains(t, view, ":")
	assert.NotContains(t, view, "enter run")
	assert.NotContains(t, view, "command mode")
}

func TestRenderFooterShowsCommandLineAtBottom(t *testing.T) {
	m := testModel()
	m.width = 120
	m.height = 40
	m.openCommandPrompt()
	m.searchInput.SetValue("compose")

	footer := renderFooter(m, 120)

	assert.Contains(t, footer, "compose")
	assert.Contains(t, footer, ":")
	assert.NotContains(t, footer, "enter run")
	assert.NotContains(t, footer, "esc cancel")
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

func TestSelectedListCardKeepsAlignedRowWidths(t *testing.T) {
	m := testModel()
	m.width = 120
	m.height = 24
	m.state = stateList
	m.messages = sampleDraftMessages()
	m.listCursor = 0

	list := renderList(m, 44, 18)
	clean := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(list, "")
	lines := strings.Split(clean, "\n")
	matched := make([]string, 0, 4)
	for _, line := range lines {
		if strings.Contains(line, "tester@test.local") || strings.Contains(line, "(No Subject)") || strings.Contains(line, "Draft") {
			matched = append(matched, line)
		}
	}
	require.Len(t, matched, 3)
	firstWidth := lipgloss.Width(matched[0])
	assert.Equal(t, firstWidth, lipgloss.Width(matched[1]))
	assert.Equal(t, firstWidth, lipgloss.Width(matched[2]))
}

func TestSelectedListCardWithoutChipsDoesNotRenderBlankHighlightedRow(t *testing.T) {
	m := testModel()
	m.width = 120
	m.height = 24
	m.state = stateList
	m.messages = []*models.MessageDTO{{
		ID:        "msg-clean",
		AccountID: "personal",
		ThreadID:  "thread-clean",
		Subject:   "Subject 1",
		From:      "sender1@example.com",
		To:        []string{"me@example.com"},
		Body:      "Body 1",
		Labels:    []string{"inbox"},
		IsRead:    true,
		Date:      sampleMessages()[0].Date,
	}}
	m.listCursor = 0

	list := renderList(m, 44, 18)
	clean := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(list, "")
	lines := strings.Split(clean, "\n")

	selectedBlock := make([]string, 0, 4)
	collecting := false
	for _, line := range lines {
		if strings.Contains(line, "sender1@example.com") {
			collecting = true
		}
		if collecting {
			selectedBlock = append(selectedBlock, line)
			if strings.Contains(line, "Body 1") {
				break
			}
		}
	}

	require.Len(t, selectedBlock, 3)
	assert.Contains(t, selectedBlock[0], "sender1@example.com")
	assert.Contains(t, selectedBlock[1], "Subject 1")
	assert.Contains(t, selectedBlock[2], "Body 1")
}

func TestRenderContentKeepsMessageHeaderVisibleWhileBodyIsScrolled(t *testing.T) {
	m := testModel()
	m.width = 120
	m.height = 24
	m.state = stateContent
	m.messages = sampleLongMessage()
	m.syncContentViewport(true)
	m.contentViewport.SetYOffset(8)

	content := renderContent(m, 60, 18)

	assert.Contains(t, content, "Long body message")
	assert.Contains(t, content, "From:")
	assert.Contains(t, content, "Mailbox:")
	assert.Contains(t, content, "ctrl+d/u")
	assert.Contains(t, content, "0/$")
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
