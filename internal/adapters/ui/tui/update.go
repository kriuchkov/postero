package tui

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"os"
	"path/filepath"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kriuchkov/postero/internal/core/models"
	"github.com/kriuchkov/postero/pkg/compose"
)

//nolint:nestif // central TUI state machine is intentionally structured as a single dispatcher.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	if m.commandActive {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch {
			case keyMatches(msg, m.keys.Esc):
				m.closeCommandPrompt()
				m.setStatus("Command cancelled")
				return m, nil
			case keyMatches(msg, m.keys.Enter):
				return m.executeCommandPrompt()
			case msg.Type == tea.KeyUp:
				m.commandHistoryPrev()
				return m, nil
			case msg.Type == tea.KeyDown:
				m.commandHistoryNext()
				return m, nil
			case msg.Type == tea.KeyTab:
				m.completeCommandPrompt()
				return m, nil
			}
		}

		m.searchInput, cmd = m.searchInput.Update(msg)
		m.commandDraft = m.searchInput.Value()
		m.commandHistoryIx = -1
		return m, cmd
	}

	if m.state == stateCompose {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch {
			case !m.composeEditing && isSingleRune(msg, ':'):
				m.openCommandPrompt()
				return m, nil
			case key.Matches(msg, m.keys.Esc) && m.composeEditing:
				m.clearPendingCount()
				m.composeEditing = false
				m.applyComposeFocus()
				m.setStatus("Exited writing mode")
				return m, nil
			case key.Matches(msg, m.keys.Esc):
				m.state = stateList
				m.resetComposeState()
				m.setStatus("Compose cancelled")
				return m, m.fetchMessages()
			case !m.composeEditing && m.captureCountPrefix(msg):
				return m, nil
			case !m.composeEditing && m.handleComposeMotion(msg):
				return m, nil
			case !m.composeEditing && isSingleRune(msg, 'o'):
				m.clearPendingCount()
				m.openComposeBody(false)
				return m, nil
			case !m.composeEditing && isSingleRune(msg, 'O'):
				m.clearPendingCount()
				m.openComposeBody(true)
				return m, nil
			case matchesUp(msg, m.keys.Up) && !m.composeEditing:
				count := m.consumeCount()
				if m.focusIndex == 0 && msg.Type == tea.KeyUp {
					m.cycleComposeAccount(-count)
					return m, nil
				}
				m.moveComposeFocus(-count)
				return m, nil
			case matchesDown(msg, m.keys.Down) && !m.composeEditing:
				count := m.consumeCount()
				if m.focusIndex == 0 && msg.Type == tea.KeyDown {
					m.cycleComposeAccount(count)
					return m, nil
				}
				m.moveComposeFocus(count)
				return m, nil
			case matchesLeft(msg, m.keys.Left) && !m.composeEditing && m.focusIndex == 0:
				m.cycleComposeAccount(-m.consumeCount())
				return m, nil
			case matchesRight(msg, m.keys.Right) && !m.composeEditing && m.focusIndex == 0:
				m.cycleComposeAccount(m.consumeCount())
				return m, nil
			case key.Matches(msg, m.keys.Enter) && !m.composeEditing && m.focusIndex == 0:
				m.clearPendingCount()
				m.focusIndex++
				m.composeEditing = true
				m.applyComposeFocus()
				return m, nil
			case key.Matches(msg, m.keys.Edit) && !m.composeEditing && m.focusIndex > 0:
				m.clearPendingCount()
				m.composeEditing = true
				m.applyComposeFocus()
				m.clearStatus()
				return m, nil
			case key.Matches(msg, m.keys.Enter) && !m.composeEditing && m.focusIndex > 0:
				m.clearPendingCount()
				m.composeEditing = true
				m.applyComposeFocus()
				m.clearStatus()
				return m, nil
			case key.Matches(msg, m.keys.Enter) && m.composeEditing && m.focusIndex < 3:
				m.composeEditing = false
				if m.focusIndex < 3 {
					m.focusIndex++
				}
				m.applyComposeFocus()
				return m, nil
			case key.Matches(msg, m.keys.SaveDraft) && m.activeDraft != nil:
				m.clearPendingCount()
				if _, err := m.persistActiveDraft(context.Background()); err != nil {
					m.setError(err.Error())
					return m, nil
				}
				m.state = stateList
				m.resetComposeState()
				m.setStatus("Draft saved")
				return m, m.fetchMessages()
			case key.Matches(msg, m.keys.Send) && m.activeDraft != nil:
				m.clearPendingCount()
				draft, err := m.persistActiveDraft(context.Background())
				if err != nil {
					m.setError(err.Error())
					return m, nil
				}
				if draft == nil || draft.ID == "" {
					m.setError("draft was not saved")
					return m, nil
				}
				if err := m.service.SendMessage(context.Background(), draft.ID); err != nil {
					m.setError(err.Error())
					return m, nil
				}
				m.state = stateList
				m.resetComposeState()
				m.setStatus("Message sent")
				return m, m.fetchMessages()
			}

			if msg.String() == "tab" && !m.composeEditing {
				m.clearPendingCount()
				m.focusIndex = (m.focusIndex + 1) % 4
				m.applyComposeFocus()
				return m, nil
			}
			if msg.String() == "shift+tab" && !m.composeEditing {
				m.clearPendingCount()
				m.focusIndex--
				if m.focusIndex < 0 {
					m.focusIndex = 3
				}
				m.applyComposeFocus()
				return m, nil
			}
		}

		if !m.composeEditing {
			return m, nil
		}

		var cmds []tea.Cmd
		switch m.focusIndex {
		case 0:
			return m, nil
		case 1:
			m.toInput, cmd = m.toInput.Update(msg)
			cmds = append(cmds, cmd)
			if m.activeDraft != nil {
				m.activeDraft.To = trimRecipients(strings.Split(m.toInput.Value(), ","))
			}
		case 2:
			m.subjectInput, cmd = m.subjectInput.Update(msg)
			cmds = append(cmds, cmd)
			if m.activeDraft != nil {
				m.activeDraft.Subject = m.subjectInput.Value()
			}
		case 3:
			m.bodyInput, cmd = m.bodyInput.Update(msg)
			cmds = append(cmds, cmd)
			if m.activeDraft != nil {
				m.activeDraft.Body = m.bodyInput.Value()
			}
		}

		return m, tea.Batch(cmds...)
	}

	if m.searchActive {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch {
			case keyMatches(msg, m.keys.Esc):
				clearCmd := m.clearSearch()
				m.setStatus("Search cleared")
				return m, clearCmd
			case keyMatches(msg, m.keys.Enter):
				m.searchActive = false
				m.searchInput.Blur()
				if strings.TrimSpace(m.searchQuery) == "" {
					m.setStatus("Search cleared")
					return m, nil
				}
				if m.searchDebouncing {
					m.searchDebouncing = false
					m.prepareFreshMessageFetch()
					m.messages = nil
					m.allMessages = nil
					m.listCursor = 0
					return m, m.fetchMessages()
				}
				m.setStatus(m.searchStatusMessage())
				return m, nil
			}

			previousQuery := m.searchQuery
			m.searchInput, cmd = m.searchInput.Update(msg)
			m.searchQuery = strings.TrimSpace(m.searchInput.Value())
			if m.searchQuery == previousQuery {
				return m, cmd
			}
			m.searchToken++
			m.searchDebouncing = strings.TrimSpace(m.searchQuery) != ""
			if m.searchQuery == "" {
				m.searchDebouncing = false
				m.statusMessage = ""
				m.statusError = false
				m.prepareFreshMessageFetch()
				m.messages = nil
				m.allMessages = nil
				m.listCursor = 0
				return m, m.fetchMessages()
			}
			return m, m.searchDebounceCmd()
		case messagesLoadedMsg, loadingTickMsg, undoExpiredMsg, tea.WindowSizeMsg, searchDebounceMsg:
			// Let async runtime messages flow to the main dispatcher below.
		default:
			m.searchInput, cmd = m.searchInput.Update(msg)
			return m, cmd
		}
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.state != stateSidebar || msg.Type != tea.KeyRunes || len(msg.Runes) != 1 || !sidebarConsumesTagHotkey(m, msg.Runes[0]) {
			if m.captureCountPrefix(msg) {
				return m, nil
			}
			if handled, motionCmd := m.handleBrowseMotion(msg); handled {
				return m, motionCmd
			}
		}

		switch {
		case keyMatches(msg, m.keys.Quit):
			return m, tea.Quit
		case isSingleRune(msg, ':'):
			m.openCommandPrompt()
			return m, nil
		case keyMatches(msg, m.keys.Refresh):
			m.clearPendingCount()
			m.setStatus("Mailbox refreshed")
			m.prepareFreshMessageFetch()
			return m, m.fetchMessages()
		case m.state == stateSidebar && strings.TrimSpace(m.activeTagID) != "" && keyMatches(msg, m.keys.Esc):
			m.activeTagID = ""
			m.setStatus("Tag filter cleared")
			m.prepareFreshMessageFetch()
			return m, m.fetchMessages()
		case keyMatches(msg, m.keys.Esc) && m.state == stateSidebar && strings.TrimSpace(m.activeAccountID) != "":
			if _, ok := m.selectedAccountID(); ok {
				m.sidebarCursor = 0
			}
			m.activeAccountID = ""
			m.setStatus("Account scope cleared")
			m.prepareFreshMessageFetch()
			return m, m.fetchMessages()
		case m.state == stateSidebar && msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && sidebarConsumesTagHotkey(m, msg.Runes[0]):
			if tagID, ok := findSidebarTagByHotkey(m, msg.Runes[0]); ok {
				m.activeTagID = tagID
				m.setStatus("Tag selected: " + strings.ReplaceAll(tagID, "_", " "))
				m.prepareFreshMessageFetch()
				return m, m.fetchMessages()
			}
		case keyMatches(msg, m.keys.Undo) && m.pendingUndo != nil:
			m.clearPendingCount()
			snapshots := m.pendingUndo.snapshots()
			m.pendingUndo = nil
			for _, snapshot := range snapshots {
				if _, err := m.service.RestoreMessage(context.Background(), cloneMessage(snapshot)); err != nil {
					m.setError("Undo failed")
					return m, nil
				}
			}
			m.setStatus("Undo applied")
			if len(snapshots) > 0 {
				return m, m.fetchMessagesForID(snapshots[0].ID)
			}
			return m, m.fetchMessages()
		case matchesRepeat(msg, m.keys.Repeat) && m.lastAction != repeatableActionNone:
			return m, m.applyRepeatableAction(m.lastAction, m.consumeCount())
		case matchesSearch(msg, m.keys.Search):
			m.openSearchPrompt()
			m.setStatus("Search the current mailbox")
			return m, nil
		case keyMatches(msg, m.keys.MarkRead):
			m.clearPendingCount()
			if len(m.messages) == 0 || m.listCursor < 0 || m.listCursor >= len(m.messages) {
				return m, nil
			}
			selected := m.messages[m.listCursor]
			if selected == nil {
				return m, nil
			}
			m.markSelectedMessageRead(context.Background())
			m.setStatus("Message marked as read")
			m.prepareFreshMessageFetch()
			return m, m.fetchMessagesForID(selected.ID)
		case keyMatches(msg, m.keys.Esc) && strings.TrimSpace(m.searchQuery) != "":
			clearCmd := m.clearSearch()
			m.setStatus("Search cleared")
			return m, clearCmd
		case keyMatches(msg, m.keys.Enter):
			m.clearPendingCount()
			if m.state == stateSidebar {
				oldState := m.state
				m.nextFocus()
				if oldState == stateSidebar && m.state == stateList {
					m.prepareFreshMessageFetch()
					return m, m.fetchMessages()
				}
				return m, nil
			}
			if draft, ok := m.selectedDraft(); ok {
				m.markSelectedMessageRead(context.Background())
				m.enterComposeState(draft, 1)
				m.setComposeContext("Draft", "Resume where you left off.")
				m.composeEditing = false
				m.setStatus("Editing draft")
				return m, nil
			}
			if m.state == stateList {
				if _, ok := m.selectedMessage(); ok {
					m.nextFocus()
					m.markSelectedMessageRead(context.Background())
					m.syncContentViewport(true)
					return m, nil
				}
			}
		case matchesUp(msg, m.keys.Up):
			count := m.consumeCount()
			if m.state == stateSidebar {
				return m.sidebarMoveBy(-count)
			}
			if m.state == stateList {
				if m.listCursor > 0 {
					m.listCursor = max(0, m.listCursor-count)
					m.markSelectedMessageRead(context.Background())
					m.syncContentViewport(true)
				}
				return m, m.maybeFetchMoreMessages()
			}
			m.contentViewport.SetYOffset(max(m.contentViewport.YOffset-count, 0))
		case matchesDown(msg, m.keys.Down):
			count := m.consumeCount()
			if m.state == stateSidebar {
				return m.sidebarMoveBy(count)
			}
			if m.state == stateList {
				if m.listCursor < len(m.messages)-1 {
					m.listCursor = min(len(m.messages)-1, m.listCursor+count)
					m.markSelectedMessageRead(context.Background())
					m.syncContentViewport(true)
				}
				return m, m.maybeFetchMoreMessages()
			}
			m.contentViewport.SetYOffset(m.contentViewport.YOffset + count)
		case matchesRight(msg, m.keys.Right):
			count := m.consumeCount()
			oldState := m.state
			for range count {
				m.nextFocus()
			}
			if oldState == stateSidebar && m.state == stateList {
				m.prepareFreshMessageFetch()
				return m, m.fetchMessages()
			}
			if oldState == stateList && m.state == stateContent {
				m.markSelectedMessageRead(context.Background())
				m.syncContentViewport(true)
			}
		case matchesLeft(msg, m.keys.Left):
			count := m.consumeCount()
			for range count {
				m.prevFocus()
			}
		case keyMatches(msg, m.keys.PageUp):
			count := m.consumeCount()
			if m.state == stateSidebar {
				return m.sidebarMoveBy(-10 * count)
			}
			if m.state == stateList {
				if len(m.messages) == 0 {
					return m, nil
				}
				m.listCursor = max(0, m.listCursor-(10*count))
				m.markSelectedMessageRead(context.Background())
				m.syncContentViewport(true)
				return m, m.maybeFetchMoreMessages()
			}
			for range count {
				m.contentViewport.PageUp()
			}
		case keyMatches(msg, m.keys.PageDown):
			count := m.consumeCount()
			if m.state == stateSidebar {
				return m.sidebarMoveBy(10 * count)
			}
			if m.state == stateList {
				if len(m.messages) == 0 {
					return m, nil
				}
				m.listCursor = min(len(m.messages)-1, m.listCursor+(10*count))
				m.markSelectedMessageRead(context.Background())
				m.syncContentViewport(true)
				return m, m.maybeFetchMoreMessages()
			}
			for range count {
				m.contentViewport.PageDown()
			}
		case keyMatches(msg, m.keys.HalfUp):
			count := m.consumeCount()
			if m.state == stateSidebar {
				return m.sidebarMoveBy(-5 * count)
			}
			if m.state == stateList {
				if len(m.messages) == 0 {
					return m, nil
				}
				m.listCursor = max(0, m.listCursor-(5*count))
				m.markSelectedMessageRead(context.Background())
				m.syncContentViewport(true)
				return m, m.maybeFetchMoreMessages()
			}
			for range count {
				m.contentViewport.HalfPageUp()
			}
		case keyMatches(msg, m.keys.HalfDown):
			count := m.consumeCount()
			if m.state == stateSidebar {
				return m.sidebarMoveBy(5 * count)
			}
			if m.state == stateList {
				if len(m.messages) == 0 {
					return m, nil
				}
				m.listCursor = min(len(m.messages)-1, m.listCursor+(5*count))
				m.markSelectedMessageRead(context.Background())
				m.syncContentViewport(true)
				return m, m.maybeFetchMoreMessages()
			}
			for range count {
				m.contentViewport.HalfPageDown()
			}
		case keyMatches(msg, m.keys.Delete):
			action := repeatableActionTrash
			if m.isTrashSelection() {
				action = repeatableActionDelete
			}
			return m, m.applyRepeatableAction(action, m.consumeCount())
		case keyMatches(msg, m.keys.Archive):
			return m, m.applyRepeatableAction(repeatableActionArchive, m.consumeCount())
		case keyMatches(msg, m.keys.Spam):
			return m, m.applyRepeatableAction(repeatableActionSpam, m.consumeCount())
		case keyMatches(msg, m.keys.Download):
			m.clearPendingCount()
			if selected, ok := m.selectedMessage(); ok {
				if len(selected.Attachments) > 0 {
					go func(msg *models.Message) {
						for _, att := range msg.Attachments {
							if len(att.Data) == 0 {
								continue
							}
							safeName := filepath.Base(att.Filename)
							if safeName == "" || safeName == "." || safeName == "/" {
								safeName = "unnamed_attachment"
							}
							dlPath := filepath.Join(os.Getenv("HOME"), "Downloads", safeName)
							if err := os.WriteFile(dlPath, att.Data, 0o600); err != nil {
								continue
							}
						}
					}(&selected)
					m.setStatus(fmt.Sprintf("Saved %d attachments to ~/Downloads", len(selected.Attachments)))
				} else {
					m.setStatus("No attachments to save")
				}
				return m, nil
			}
		case matchesReply(msg, m.keys.Reply):
			m.clearPendingCount()
			if selected, ok := m.selectedMessage(); ok {
				draft := compose.BuildReply(&selected, compose.ReplyOptions{Self: []string{m.senderForAccount(selected.AccountID)}})
				m.enterComposeState(&models.Message{AccountID: selected.AccountID, From: m.senderForAccount(selected.AccountID), ThreadID: draft.ThreadID, Subject: draft.Subject, To: draft.To, Cc: draft.Cc, Body: draft.Body}, 3)
				m.setComposeContext("Reply", "Type above the quoted message.")
				m.composeEditing = true
				m.moveReplyCursorToStart()
				m.clearStatus()
			}
		case matchesReplyAll(msg, m.keys.ReplyAll):
			m.clearPendingCount()
			if selected, ok := m.selectedMessage(); ok {
				draft := compose.BuildReply(&selected, compose.ReplyOptions{ReplyAll: true, Self: []string{m.senderForAccount(selected.AccountID)}})
				m.enterComposeState(&models.Message{AccountID: selected.AccountID, From: m.senderForAccount(selected.AccountID), ThreadID: draft.ThreadID, Subject: draft.Subject, To: draft.To, Cc: draft.Cc, Body: draft.Body}, 3)
				m.setComposeContext("Reply all", "Type above the quoted message.")
				m.composeEditing = true
				m.moveReplyCursorToStart()
				m.clearStatus()
			}
		case matchesForward(msg, m.keys.Forward):
			m.clearPendingCount()
			if selected, ok := m.selectedMessage(); ok {
				draft := compose.BuildForward(&selected, nil, "")
				m.enterComposeState(&models.Message{AccountID: selected.AccountID, From: m.senderForAccount(selected.AccountID), ThreadID: draft.ThreadID, Subject: draft.Subject, To: draft.To, Body: draft.Body}, 1)
				m.setComposeContext("Forward", "Add recipients, then edit the forwarded message below.")
				m.composeEditing = true
				m.applyComposeFocus()
				m.clearStatus()
			}
		case matchesCompose(msg, m.keys.Compose):
			m.clearPendingCount()
			m.enterComposeState(&models.Message{AccountID: m.defaultAcctID, From: m.defaultFrom, Subject: "", To: []string{}, Body: ""}, 0)
			m.setComposeContext("Composer", "Write now, save when ready.")
			m.composeEditing = false
			m.clearStatus()
		}
	case aiDraftGeneratedMsg:
		m.finishAIGeneration()
		if msg.draft == nil {
			return m, nil
		}
		m.enterComposeState(msg.draft, msg.focusIndex)
		m.setComposeContext(msg.title, msg.hint)
		m.composeEditing = msg.composeEditing
		m.applyComposeFocus()
		if msg.moveCursorTop {
			m.moveReplyCursorToStart()
		}
		m.setStatus(msg.status)
		return m, nil
	case aiDraftFailedMsg:
		m.finishAIGeneration()
		if msg.err != nil {
			m.setError(msg.err.Error())
		}
		return m, nil
	case messagesLoadedMsg:
		if msg.scopeKey != "" && msg.scopeKey != m.currentMessageScopeKey() {
			return m, nil
		}
		m.activeAccountID = strings.TrimSpace(msg.activeAccountID)
		m.activeTagID = strings.TrimSpace(msg.activeTagID)
		m.searchDebouncing = false
		m.messagesLoading = false
		m.loadingFrame = 0
		m.fetchOffset = msg.nextOffset
		m.hasMoreMessages = msg.hasMore
		if msg.appendPage {
			m.allMessages = mergeMessages(m.allMessages, msg.messages)
			if strings.TrimSpace(msg.activeTagID) == "" || len(m.sidebarTagSource) == 0 {
				m.sidebarTagSource = mergeMessages(m.sidebarTagSource, msg.messages)
			}
		} else {
			m.allMessages = append([]*models.Message{}, msg.messages...)
			if strings.TrimSpace(msg.activeTagID) == "" || len(m.sidebarTagSource) == 0 {
				m.sidebarTagSource = append([]*models.Message{}, msg.messages...)
			}
		}
		if msg.appendPage {
			selectedID := m.currentMessageID()
			m.messages = append([]*models.Message{}, m.allMessages...)
			m.restoreListCursor(selectedID)
		} else {
			m.messages = append([]*models.Message{}, m.allMessages...)
			m.applyLoadedCursor(msg.targetCursor, msg.targetID)
		}
		if len(m.messages) == 0 && !m.statusError {
			m.setStatus("No messages found")
		}
		m.syncContentViewport(true)
		return m, m.undoCountdownCmd()
	case searchDebounceMsg:
		if msg.token != m.searchToken || !m.searchDebouncing || strings.TrimSpace(msg.query) != strings.TrimSpace(m.searchQuery) {
			return m, nil
		}
		m.searchDebouncing = false
		m.prepareFreshMessageFetch()
		m.messages = nil
		m.allMessages = nil
		m.listCursor = 0
		return m, m.fetchMessages()
	case loadingTickMsg:
		if !m.messagesLoading || msg.token != m.loadingToken {
			return m, nil
		}
		m.loadingFrame = msg.frame
		return m, m.loadingTickCmd()
	case aiLoadingTickMsg:
		if !m.aiGenerating || msg.token != m.aiLoadingToken {
			return m, nil
		}
		m.aiLoadingFrame = msg.frame
		return m, m.aiLoadingTickCmd()
	case undoExpiredMsg:
		if m.pendingUndo != nil && m.pendingUndo.token == msg.token {
			m.pendingUndo = nil
			if strings.Contains(m.statusMessage, "Press u to undo") {
				m.clearStatus()
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.syncContentViewport(false)
	}

	return m, nil
}

func (m *Model) enterComposeState(draft *models.Message, focusIndex int) {
	m.state = stateCompose
	m.activeDraft = draft
	m.composeBaseline = cloneMessage(draft)
	m.pendingMotion = ""
	m.pendingCount = ""
	m.toInput.SetValue(strings.Join(draft.To, ", "))
	m.subjectInput.SetValue(draft.Subject)
	m.bodyInput.SetValue(draft.Body)
	m.setComposeContext(defaultComposeContext(draft))
	m.composeEditing = false
	m.focusIndex = focusIndex
	m.applyComposeFocus()
}

func (m *Model) setComposeContext(title, hint string) {
	m.composeTitle = title
	m.composeHint = hint
}

func defaultComposeContext(draft *models.Message) (string, string) {
	if draft != nil && strings.TrimSpace(draft.ID) != "" {
		return "Draft", "Resume where you left off."
	}
	return "Composer", "Write now, save when ready."
}

func (m *Model) applyComposeFocus() {
	switch m.focusIndex {
	case 0:
		m.toInput.Blur()
		m.subjectInput.Blur()
		m.bodyInput.Blur()
	case 1:
		m.toInput.Focus()
		m.subjectInput.Blur()
		m.bodyInput.Blur()
	case 2:
		m.toInput.Blur()
		m.subjectInput.Focus()
		m.bodyInput.Blur()
	default:
		m.toInput.Blur()
		m.subjectInput.Blur()
		m.bodyInput.Focus()
	}
}

func (m *Model) openComposeBody(atTop bool) {
	m.focusIndex = 3
	m.composeEditing = true
	m.applyComposeFocus()
	if atTop {
		m.bodyInput.CursorStart()
		m.clearStatus()
		return
	}
	body := m.bodyInput.Value()
	if body != "" && !strings.HasSuffix(body, "\n") {
		body += "\n"
		m.bodyInput.SetValue(body)
		if m.activeDraft != nil {
			m.activeDraft.Body = body
		}
	}
	m.bodyInput.CursorEnd()
	m.clearStatus()
}

func (m *Model) moveReplyCursorToStart() {
	m.bodyInput.CursorStart()
	lineCount := strings.Count(m.bodyInput.Value(), "\n") + 1
	for range lineCount {
		m.bodyInput.CursorUp()
	}
}

func (m *Model) moveComposeFocus(step int) {
	if step == 0 {
		return
	}
	m.focusIndex += step
	if m.focusIndex < 0 {
		m.focusIndex = 0
	}
	if m.focusIndex > 3 {
		m.focusIndex = 3
	}
	m.applyComposeFocus()
}

func isSingleRune(msg tea.KeyMsg, value rune) bool {
	return msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == value
}

func matchesUp(msg tea.KeyMsg, binding key.Binding) bool {
	return keyMatches(msg, binding) || isSingleRune(msg, 'k')
}

func matchesDown(msg tea.KeyMsg, binding key.Binding) bool {
	return keyMatches(msg, binding) || isSingleRune(msg, 'j')
}

func matchesLeft(msg tea.KeyMsg, binding key.Binding) bool {
	return keyMatches(msg, binding) || isSingleRune(msg, 'h')
}

func matchesRight(msg tea.KeyMsg, binding key.Binding) bool {
	return keyMatches(msg, binding) || isSingleRune(msg, 'l')
}

func matchesSearch(msg tea.KeyMsg, binding key.Binding) bool {
	return keyMatches(msg, binding) || isSingleRune(msg, '/')
}

func matchesCompose(msg tea.KeyMsg, binding key.Binding) bool {
	return keyMatches(msg, binding) || isSingleRune(msg, 'c')
}

func matchesReply(msg tea.KeyMsg, binding key.Binding) bool {
	return keyMatches(msg, binding) || isSingleRune(msg, 'r')
}

func matchesReplyAll(msg tea.KeyMsg, binding key.Binding) bool {
	return keyMatches(msg, binding) || isSingleRune(msg, 'R')
}

func matchesForward(msg tea.KeyMsg, binding key.Binding) bool {
	return keyMatches(msg, binding) || isSingleRune(msg, 'f')
}

func matchesRepeat(msg tea.KeyMsg, binding key.Binding) bool {
	return keyMatches(msg, binding) || isSingleRune(msg, '.')
}

func (m *Model) captureCountPrefix(msg tea.KeyMsg) bool {
	if msg.Type != tea.KeyRunes || len(msg.Runes) != 1 {
		return false
	}
	r := msg.Runes[0]
	if r < '0' || r > '9' {
		return false
	}
	if r == '0' && m.pendingCount == "" {
		return false
	}
	m.pendingCount += string(r)
	return true
}

func (m *Model) clearPendingCount() {
	m.pendingCount = ""
}

func (m *Model) consumeCount() int {
	value := m.consumeExplicitCount()
	if value > 0 {
		return value
	}
	return 1
}

func (m *Model) consumeExplicitCount() int {
	if strings.TrimSpace(m.pendingCount) == "" {
		return 0
	}
	value, err := strconv.Atoi(m.pendingCount)
	m.pendingCount = ""
	if err != nil || value <= 0 {
		return 0
	}
	return value
}

func composeDraftHasContent(draft *models.Message) bool {
	if draft == nil {
		return false
	}
	if strings.TrimSpace(draft.Subject) != "" || strings.TrimSpace(draft.Body) != "" {
		return true
	}
	return len(trimRecipients(draft.To)) > 0 || len(trimRecipients(draft.Cc)) > 0 || len(trimRecipients(draft.Bcc)) > 0
}

func composeDraftEqual(left, right *models.Message) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return strings.EqualFold(strings.TrimSpace(left.AccountID), strings.TrimSpace(right.AccountID)) &&
		strings.TrimSpace(left.From) == strings.TrimSpace(right.From) &&
		strings.TrimSpace(left.Subject) == strings.TrimSpace(right.Subject) &&
		left.Body == right.Body &&
		slices.Equal(trimRecipients(left.To), trimRecipients(right.To)) &&
		slices.Equal(trimRecipients(left.Cc), trimRecipients(right.Cc)) &&
		slices.Equal(trimRecipients(left.Bcc), trimRecipients(right.Bcc))
}

func (m Model) hasUnsavedComposeChanges() bool {
	if m.state != stateCompose || m.activeDraft == nil {
		return false
	}
	if m.composeBaseline == nil {
		return composeDraftHasContent(m.activeDraft)
	}
	return !composeDraftEqual(m.composeBaseline, m.activeDraft)
}

func (m *Model) resetComposeState() {
	m.activeDraft = nil
	m.composeBaseline = nil
	m.composeEditing = false
	m.pendingMotion = ""
	m.pendingCount = ""
}

func (m *Model) jumpToComposeField(index int) {
	if index < 0 {
		index = 0
	}
	if index > 3 {
		index = 3
	}
	m.focusIndex = index
	m.applyComposeFocus()
}

func sidebarConsumesTagHotkey(m Model, key rune) bool {
	_, ok := findSidebarTagByHotkey(m, key)
	return ok
}

func (m *Model) sidebarMoveBy(delta int) (Model, tea.Cmd) {
	if delta == 0 {
		return *m, nil
	}
	steps := delta
	if steps < 0 {
		steps = -steps
	}
	direction := 1
	if delta < 0 {
		direction = -1
	}
	moved := false
	for range steps {
		if !m.moveSidebarSelectionOne(direction) {
			break
		}
		moved = true
	}
	if !moved {
		return *m, nil
	}
	return *m, m.fetchMessages()
}

func (m *Model) moveSidebarSelectionOne(step int) bool {
	tags := sidebarTags(*m)
	if step > 0 {
		return m.moveSidebarSelectionForward(tags)
	}
	return m.moveSidebarSelectionBackward(tags)
}

func (m *Model) moveSidebarSelectionForward(tags []sidebarSectionRow) bool {
	if strings.TrimSpace(m.activeTagID) != "" {
		index := sidebarActiveTagIndex(*m, tags)
		if index >= 0 && index < len(tags)-1 {
			m.activeTagID = tags[index+1].value
			return true
		}
		return false
	}
	if m.sidebarCursor < len(m.sidebarItems)-1 {
		m.sidebarCursor++
		return true
	}
	if len(tags) > 0 {
		m.activeTagID = tags[0].value
		return true
	}
	return false
}

func (m *Model) moveSidebarSelectionBackward(tags []sidebarSectionRow) bool {
	if strings.TrimSpace(m.activeTagID) != "" {
		index := sidebarActiveTagIndex(*m, tags)
		if index > 0 {
			m.activeTagID = tags[index-1].value
			return true
		}
		if index == 0 {
			m.activeTagID = ""
			if len(m.sidebarItems) > 0 {
				m.sidebarCursor = len(m.sidebarItems) - 1
			}
			return true
		}
		m.activeTagID = ""
		return len(m.sidebarItems) > 0
	}
	if m.sidebarCursor > 0 {
		m.sidebarCursor--
		return true
	}
	return false
}

func (m *Model) sidebarJumpToTop() (Model, tea.Cmd) {
	if len(m.sidebarItems) == 0 {
		return *m, nil
	}
	m.activeTagID = ""
	m.sidebarCursor = 0
	return *m, m.fetchMessages()
}

func (m *Model) sidebarJumpToBottom() (Model, tea.Cmd) {
	tags := sidebarTags(*m)
	if len(tags) > 0 {
		m.activeTagID = tags[len(tags)-1].value
		return *m, m.fetchMessages()
	}
	if len(m.sidebarItems) == 0 {
		return *m, nil
	}
	m.activeTagID = ""
	m.sidebarCursor = len(m.sidebarItems) - 1
	return *m, m.fetchMessages()
}

func sidebarActiveTagIndex(m Model, tags []sidebarSectionRow) int {
	active := strings.TrimSpace(m.activeTagID)
	for index, row := range tags {
		if strings.EqualFold(strings.TrimSpace(row.value), active) {
			return index
		}
	}
	return -1
}

func (m *Model) handleComposeMotion(msg tea.KeyMsg) bool {
	if m.pendingMotion == "g" && !isSingleRune(msg, 'g') {
		m.pendingMotion = ""
	}

	switch {
	case isSingleRune(msg, 'g'):
		if m.pendingMotion == "g" {
			m.pendingMotion = ""
			if count := m.consumeExplicitCount(); count > 0 {
				m.jumpToComposeField(count - 1)
			} else {
				m.jumpToComposeField(0)
			}
			return true
		}
		m.pendingMotion = "g"
		return true
	case keyMatches(msg, m.keys.Top):
		m.pendingMotion = ""
		if count := m.consumeExplicitCount(); count > 0 {
			m.jumpToComposeField(count - 1)
		} else {
			m.jumpToComposeField(0)
		}
		return true
	case keyMatches(msg, m.keys.Bottom) || isSingleRune(msg, 'G'):
		m.pendingMotion = ""
		if count := m.consumeExplicitCount(); count > 0 {
			m.jumpToComposeField(count - 1)
		} else {
			m.jumpToComposeField(3)
		}
		return true
	case isSingleRune(msg, '0'):
		m.pendingMotion = ""
		m.jumpToComposeField(0)
		return true
	case isSingleRune(msg, '$'):
		m.pendingMotion = ""
		m.jumpToComposeField(3)
		return true
	default:
		return false
	}
}

func (m *Model) handleBrowseMotion(msg tea.KeyMsg) (bool, tea.Cmd) {
	if m.pendingMotion == "g" && !isSingleRune(msg, 'g') {
		m.pendingMotion = ""
	}

	switch {
	case isSingleRune(msg, 'n'):
		return true, m.repeatSearch(m.consumeCount())
	case isSingleRune(msg, 'N'):
		return true, m.repeatSearch(-m.consumeCount())
	case isSingleRune(msg, 'H'):
		m.clearPendingCount()
		return m.jumpToListViewport(listViewportTop)
	case isSingleRune(msg, 'M'):
		m.clearPendingCount()
		return m.jumpToListViewport(listViewportMiddle)
	case isSingleRune(msg, 'L'):
		m.clearPendingCount()
		return m.jumpToListViewport(listViewportBottom)
	case isSingleRune(msg, 'g'):
		if m.pendingMotion == "g" {
			m.pendingMotion = ""
			if m.state == stateList || m.state == stateContent {
				if count := m.consumeExplicitCount(); count > 0 {
					return true, m.jumpToIndexedPosition(count - 1)
				}
			}
			return true, m.jumpToTop()
		}
		m.pendingMotion = "g"
		return true, nil
	case keyMatches(msg, m.keys.Top):
		m.pendingMotion = ""
		if m.state == stateList || m.state == stateContent {
			if count := m.consumeExplicitCount(); count > 0 {
				return true, m.jumpToIndexedPosition(count - 1)
			}
		}
		return true, m.jumpToTop()
	case keyMatches(msg, m.keys.Bottom) || isSingleRune(msg, 'G'):
		m.pendingMotion = ""
		if m.state == stateList || m.state == stateContent {
			if count := m.consumeExplicitCount(); count > 0 {
				return true, m.jumpToIndexedPosition(count - 1)
			}
		}
		return true, m.jumpToBottom()
	case isSingleRune(msg, '0'):
		m.pendingMotion = ""
		return true, m.jumpToTop()
	case isSingleRune(msg, '$'):
		m.pendingMotion = ""
		return true, m.jumpToBottom()
	default:
		return false, nil
	}
}

type listViewportJump int

const (
	listViewportTop listViewportJump = iota
	listViewportMiddle
	listViewportBottom
)

func (m *Model) repeatSearch(step int) tea.Cmd {
	if strings.TrimSpace(m.searchQuery) == "" || len(m.messages) == 0 || step == 0 {
		return nil
	}
	if m.listCursor < 0 || m.listCursor >= len(m.messages) {
		m.listCursor = 0
	}
	if len(m.messages) > 1 {
		m.listCursor = (m.listCursor + step + len(m.messages)) % len(m.messages)
	}
	m.markSelectedMessageRead(context.Background())
	m.syncContentViewport(true)
	m.setStatus(m.searchStatusMessage())
	return m.maybeFetchMoreMessages()
}

func (m *Model) jumpToListViewport(target listViewportJump) (bool, tea.Cmd) {
	if m.state != stateList || len(m.messages) == 0 {
		return false, nil
	}
	listHeight := m.height
	if listHeight <= 0 {
		listHeight = 20
	}
	start, end := listWindowRange(*m, listHeight)
	if end <= start {
		return true, nil
	}
	jumpIndex := start
	switch target {
	case listViewportTop:
		jumpIndex = start
	case listViewportMiddle:
		jumpIndex = start + (end-start-1)/2
	case listViewportBottom:
		jumpIndex = end - 1
	}
	m.listCursor = jumpIndex
	m.markSelectedMessageRead(context.Background())
	m.syncContentViewport(true)
	return true, nil
}

func (m *Model) jumpToTop() tea.Cmd {
	switch m.state {
	case stateSidebar:
		_, cmd := m.sidebarJumpToTop()
		return cmd
	case stateList:
		if len(m.messages) == 0 {
			return nil
		}
		m.listCursor = 0
		m.markSelectedMessageRead(context.Background())
		m.syncContentViewport(true)
		return nil
	case stateContent:
		m.contentViewport.GotoTop()
		return nil
	case stateCompose:
		m.jumpToComposeField(0)
		return nil
	}
	return nil
}

func (m *Model) jumpToBottom() tea.Cmd {
	switch m.state {
	case stateSidebar:
		_, cmd := m.sidebarJumpToBottom()
		return cmd
	case stateList:
		if len(m.messages) == 0 {
			return nil
		}
		m.listCursor = len(m.messages) - 1
		m.markSelectedMessageRead(context.Background())
		m.syncContentViewport(true)
		return nil
	case stateContent:
		m.contentViewport.GotoBottom()
		return nil
	case stateCompose:
		m.jumpToComposeField(3)
		return nil
	}
	return nil
}

func (m *Model) jumpToIndexedPosition(index int) tea.Cmd {
	if index < 0 {
		index = 0
	}
	switch m.state {
	case stateList:
		if len(m.messages) == 0 {
			return nil
		}
		m.listCursor = min(len(m.messages)-1, index)
		m.markSelectedMessageRead(context.Background())
		m.syncContentViewport(true)
		return m.maybeFetchMoreMessages()
	case stateContent:
		lineCount := max(contentLineCount(m.currentMessageBody()), 1)
		m.contentViewport.SetYOffset(min(lineCount-1, index))
		return nil
	case stateCompose:
		m.jumpToComposeField(index)
		return nil
	case stateSidebar:
		return nil
	}
	return nil
}

func (m Model) selectedMessage() (models.Message, bool) {
	if m.state < stateList || len(m.messages) == 0 || m.listCursor >= len(m.messages) {
		return models.Message{}, false
	}
	msg := m.messages[m.listCursor]
	attachments := make([]*models.Attachment, 0, len(msg.Attachments))
	for _, attachment := range msg.Attachments {
		attachments = append(attachments, &models.Attachment{
			Filename: attachment.Filename,
			Size:     attachment.Size,
			MimeType: attachment.MimeType,
			Data:     append([]byte{}, attachment.Data...),
		})
	}
	return models.Message{
		ID:          msg.ID,
		AccountID:   msg.AccountID,
		Subject:     msg.Subject,
		From:        msg.From,
		To:          append([]string{}, msg.To...),
		Cc:          append([]string{}, msg.Cc...),
		Bcc:         append([]string{}, msg.Bcc...),
		Body:        msg.Body,
		HTML:        msg.HTML,
		Date:        msg.Date,
		ThreadID:    msg.ThreadID,
		IsRead:      msg.IsRead,
		IsSpam:      msg.IsSpam,
		IsDraft:     msg.IsDraft,
		IsStarred:   msg.IsStarred,
		IsDeleted:   msg.IsDeleted,
		Labels:      append([]string{}, msg.Labels...),
		Size:        msg.Size,
		Attachments: attachments,
	}, true
}

func (m Model) isTrashSelection() bool {
	if len(m.messages) == 0 || m.listCursor < 0 || m.listCursor >= len(m.messages) {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(currentMailboxTitle(m)), "Trash") {
		return true
	}
	selected := m.messages[m.listCursor]
	return selected != nil && selected.IsDeleted
}

func (m Model) selectedDraft() (*models.Message, bool) {
	if m.state < stateList || len(m.messages) == 0 || m.listCursor >= len(m.messages) {
		return nil, false
	}
	selected := m.messages[m.listCursor]
	if selected == nil || !selected.IsDraft {
		return nil, false
	}

	clone := &models.Message{
		ID:        selected.ID,
		AccountID: selected.AccountID,
		From:      selected.From,
		Subject:   selected.Subject,
		To:        append([]string{}, selected.To...),
		Cc:        append([]string{}, selected.Cc...),
		Bcc:       append([]string{}, selected.Bcc...),
		Body:      selected.Body,
		ThreadID:  selected.ThreadID,
		IsRead:    selected.IsRead,
		IsDraft:   selected.IsDraft,
	}
	if strings.TrimSpace(clone.From) == "" {
		clone.From = m.senderForAccount(clone.AccountID)
	}
	return clone, true
}

func (m *Model) markSelectedMessageRead(ctx context.Context) {
	if m.service == nil || len(m.messages) == 0 || m.listCursor < 0 || m.listCursor >= len(m.messages) {
		return
	}
	selected := m.messages[m.listCursor]
	if selected == nil || selected.IsRead {
		return
	}
	updated, err := m.service.MarkAsRead(ctx, selected.ID)
	if err != nil {
		return
	}
	if updated != nil {
		m.messages[m.listCursor] = updated
		return
	}
	selected.IsRead = true
	m.messages[m.listCursor] = selected
}

func trimRecipients(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func (m *Model) nextFocus() {
	if m.state < stateContent {
		m.state++
	}
}

func (m *Model) prevFocus() {
	if m.state > stateSidebar {
		m.state--
	}
}

func (m *Model) setStatus(message string) {
	m.statusMessage = message
	m.statusError = false
}

func (m *Model) setError(message string) {
	m.statusMessage = message
	m.statusError = true
}

func (m *Model) clearStatus() {
	m.statusMessage = ""
	m.statusError = false
}

//nolint:nestif // draft persistence has two explicit branches: create and update.
func (m *Model) persistActiveDraft(ctx context.Context) (*models.Message, error) {
	if m.activeDraft == nil {
		return nil, errors.New("active draft is nil")
	}

	accountID := strings.TrimSpace(m.activeDraft.AccountID)
	if accountID == "" {
		accountID = m.defaultAcctID
	}
	from := strings.TrimSpace(m.activeDraft.From)
	if from == "" {
		from = m.senderForAccount(accountID)
	}

	to := trimRecipients(m.activeDraft.To)
	cc := trimRecipients(m.activeDraft.Cc)
	bcc := trimRecipients(m.activeDraft.Bcc)
	subject := strings.TrimSpace(m.activeDraft.Subject)
	body := m.activeDraft.Body

	if strings.TrimSpace(m.activeDraft.ID) == "" {
		created, err := m.service.ComposeMessage(ctx, &models.CreateMessageRequest{
			AccountID: accountID,
			From:      from,
			To:        to,
			Cc:        cc,
			Bcc:       bcc,
			Subject:   subject,
			Body:      body,
			Labels:    []string{"draft"},
		})
		if err != nil {
			return nil, err
		}
		if created != nil {
			if created.AccountID == "" {
				created.AccountID = accountID
			}
			if created.From == "" {
				created.From = from
			}
		}
		m.activeDraft = created
		m.composeBaseline = cloneMessage(created)
		return created, nil
	}

	updated, err := m.service.UpdateDraft(ctx, m.activeDraft.ID, &models.UpdateMessageRequest{
		AccountID: &accountID,
		From:      &from,
		Subject:   &subject,
		To:        &to,
		Cc:        &cc,
		Bcc:       &bcc,
		Body:      &body,
	})
	if err != nil {
		return nil, err
	}
	if updated != nil {
		if updated.AccountID == "" {
			updated.AccountID = accountID
		}
		if updated.From == "" {
			updated.From = from
		}
		if updated.Subject == "" {
			updated.Subject = subject
		}
		if len(updated.To) == 0 {
			updated.To = to
		}
		if len(updated.Cc) == 0 {
			updated.Cc = cc
		}
		if len(updated.Bcc) == 0 {
			updated.Bcc = bcc
		}
		if updated.Body == "" {
			updated.Body = body
		}
	}
	m.activeDraft = updated
	m.composeBaseline = cloneMessage(updated)
	return updated, nil
}

func (m *Model) removeMessageAtCursor() {
	if m.listCursor < 0 || m.listCursor >= len(m.messages) {
		m.listCursor = 0
		return
	}

	m.messages = append(m.messages[:m.listCursor], m.messages[m.listCursor+1:]...)
	if len(m.messages) == 0 {
		m.listCursor = 0
		return
	}
	if m.listCursor >= len(m.messages) {
		m.listCursor = len(m.messages) - 1
	}
}

func (m *Model) syncContentViewport(reset bool) {
	body := m.currentMessageBody()
	if body == "" {
		m.contentMessageID = ""
		m.contentViewport.SetContent("")
		m.contentViewport.YOffset = 0
		return
	}

	_, _, contentWidth := paneWidths(m.width)
	_, bodyWidth, bodyHeight := contentViewportLayout(*m, contentWidth, max(0, m.height))
	if bodyWidth < 1 {
		bodyWidth = contentWidth - 4
	}
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	m.contentViewport.Width = bodyWidth
	m.contentViewport.Height = bodyHeight
	currentID := m.currentMessageID()
	messageChanged := currentID != m.contentMessageID
	m.contentViewport.SetContent(body)
	if reset || messageChanged {
		m.contentViewport.GotoTop()
	}
	m.contentMessageID = currentID
}

func (m Model) currentMessageBody() string {
	if len(m.messages) == 0 || m.listCursor < 0 || m.listCursor >= len(m.messages) {
		return ""
	}
	msg := m.messages[m.listCursor]
	if msg == nil {
		return ""
	}

	body := m.getFilteredBody(msg)

	if len(msg.Attachments) > 0 {
		body += "\n\n---\nAttachments:\n"
		var bodySb960 strings.Builder
		for _, att := range msg.Attachments {
			sizeKB := att.Size / 1024
			if sizeKB == 0 {
				sizeKB = 1
			}
			bodySb960.WriteString(fmt.Sprintf("- □ %s (%d KB)\n", att.Filename, sizeKB))
		}
		body += bodySb960.String()
	}

	return body
}

func (m Model) currentMessageID() string {
	if len(m.messages) == 0 || m.listCursor < 0 || m.listCursor >= len(m.messages) {
		return ""
	}
	msg := m.messages[m.listCursor]
	if msg == nil {
		return ""
	}
	return msg.ID
}

func (m Model) senderForAccount(accountID string) string {
	accountID = strings.TrimSpace(accountID)
	if accountID != "" {
		if email := strings.TrimSpace(m.accountEmails[accountID]); email != "" {
			return email
		}
	}
	return m.defaultFrom
}

func (m *Model) cycleComposeAccount(step int) {
	if m.activeDraft == nil || len(m.accountNames) == 0 || step == 0 {
		return
	}

	current := strings.TrimSpace(m.activeDraft.AccountID)
	index := 0
	for i, accountName := range m.accountNames {
		if strings.EqualFold(accountName, current) {
			index = i
			break
		}
	}

	index = (index + step) % len(m.accountNames)
	if index < 0 {
		index += len(m.accountNames)
	}

	m.activeDraft.AccountID = m.accountNames[index]
	m.activeDraft.From = m.senderForAccount(m.activeDraft.AccountID)
	m.clearStatus()
}

func (m *Model) applySearchFilter() {
	selectedID := m.currentMessageID()
	query := strings.ToLower(strings.TrimSpace(m.searchQuery))
	if query == "" {
		m.messages = append([]*models.Message{}, m.allMessages...)
		m.restoreListCursor(selectedID)
		m.syncContentViewport(true)
		return
	}

	filtered := make([]*models.Message, 0, len(m.allMessages))
	for _, msg := range m.allMessages {
		if msg == nil {
			continue
		}
		if messageMatchesSearch(msg, query) {
			filtered = append(filtered, msg)
		}
	}
	m.messages = filtered
	m.restoreListCursor(selectedID)
	m.syncContentViewport(true)
}

func (m *Model) openSearchPrompt() {
	m.commandActive = false
	m.searchActive = true
	m.searchDebouncing = false
	m.pendingMotion = ""
	m.pendingCount = ""
	m.searchInput.Prompt = "/ "
	m.searchInput.Placeholder = "subject, sender, body"
	m.applySearchInputStyles(false)
	m.searchInput.SetValue(m.searchQuery)
	m.searchInput.CursorEnd()
	m.searchInput.Focus()
}

func (m *Model) openCommandPrompt() {
	m.searchActive = false
	m.commandActive = true
	m.pendingMotion = ""
	m.pendingCount = ""
	m.searchInput.Prompt = ": "
	m.searchInput.Placeholder = commandPromptPlaceholder()
	m.applySearchInputStyles(true)
	m.commandDraft = ""
	m.commandHistoryIx = -1
	m.searchInput.SetValue("")
	m.searchInput.CursorEnd()
	m.searchInput.Focus()
}

func (m *Model) closeCommandPrompt() {
	m.commandActive = false
	m.commandDraft = ""
	m.commandHistoryIx = -1
	m.searchInput.Blur()
	m.searchInput.Prompt = "/ "
	m.searchInput.Placeholder = "subject, sender, body"
	m.applySearchInputStyles(false)
	m.searchInput.SetValue("")
}

func (m *Model) restoreListCursor(selectedID string) {
	if len(m.messages) == 0 {
		m.listCursor = 0
		return
	}
	if selectedID != "" {
		for index, msg := range m.messages {
			if msg != nil && msg.ID == selectedID {
				m.listCursor = index
				return
			}
		}
	}
	if m.listCursor >= len(m.messages) {
		m.listCursor = len(m.messages) - 1
	}
	if m.listCursor < 0 {
		m.listCursor = 0
	}
}

func (m *Model) clearSearch() tea.Cmd {
	m.searchActive = false
	m.commandActive = false
	m.searchQuery = ""
	m.searchDebouncing = false
	m.searchToken++
	m.searchInput.Prompt = "/ "
	m.searchInput.Placeholder = "subject, sender, body"
	m.applySearchInputStyles(false)
	m.searchInput.SetValue("")
	m.searchInput.Blur()
	m.prepareFreshMessageFetch()
	m.messages = nil
	m.allMessages = nil
	m.listCursor = 0
	m.syncContentViewport(true)
	return m.fetchMessages()
}

func (m Model) executeCommandPrompt() (tea.Model, tea.Cmd) {
	rawCommand := strings.TrimSpace(m.searchInput.Value())
	command, argument := parseCommandPrompt(rawCommand)
	m.recordCommandHistory(rawCommand)
	m.closeCommandPrompt()
	if m.state == stateCompose && m.hasUnsavedComposeChanges() && commandLeavesCompose(command) {
		m.setError("Unsaved draft. Save, send, or cancel before leaving compose")
		return m, nil
	}

	switch command {
	case "", "cancel":
		m.setStatus("Command cancelled")
		return m, nil
	case "q", "quit":
		return m, tea.Quit
	case "c", "compose":
		m.resetComposeState()
		m.enterComposeState(&models.Message{AccountID: m.defaultAcctID, From: m.defaultFrom, Subject: "", To: []string{}, Body: ""}, 0)
		m.setComposeContext("Composer", "Write now, save when ready.")
		m.composeEditing = false
		m.clearStatus()
		return m, nil
	case "compose-ai":
		options, err := parseAICommandOptions(argument)
		if err != nil {
			m.setError(err.Error())
			return m, nil
		}
		m.prepareAIGeneration("AI draft")
		m.setStatus("Generating AI draft...")
		return m, m.withAILoadingIndicator(m.generateComposeAIDraft(options))
	case "reply-ai":
		options, err := parseAICommandOptions(argument)
		if err != nil {
			m.setError(err.Error())
			return m, nil
		}
		m.prepareAIGeneration("AI reply")
		m.setStatus("Generating AI reply...")
		return m, m.withAILoadingIndicator(m.generateReplyAIDraft(options, false))
	case "reply-all-ai":
		options, err := parseAICommandOptions(argument)
		if err != nil {
			m.setError(err.Error())
			return m, nil
		}
		m.prepareAIGeneration("AI reply-all")
		m.setStatus("Generating AI reply-all...")
		return m, m.withAILoadingIndicator(m.generateReplyAIDraft(options, true))
	case "sync", "refresh":
		m.setStatus("Mailbox refreshed")
		return m, m.fetchMessages()
	case "inbox", "sent", "drafts", "archive", "trash", "spam":
		if m.selectMailboxCommand(command) {
			m.resetComposeState()
			m.state = stateList
			m.setStatus("Switched to " + titleCaseASCII(command))
			return m, m.fetchMessages()
		}
	case "help":
		m.setStatus("Commands: " + strings.Join(commandPromptCandidates(), " "))
		return m, nil
	}

	m.setError("Unknown command: " + command)
	return m, nil
}

func (m *Model) recordCommandHistory(command string) {
	if command == "" {
		return
	}
	if len(m.commandHistory) > 0 && m.commandHistory[len(m.commandHistory)-1] == command {
		return
	}
	m.commandHistory = append(m.commandHistory, command)
}

func (m *Model) commandHistoryPrev() {
	if len(m.commandHistory) == 0 {
		return
	}
	if m.commandHistoryIx == -1 {
		m.commandDraft = m.searchInput.Value()
		m.commandHistoryIx = len(m.commandHistory) - 1
	} else if m.commandHistoryIx > 0 {
		m.commandHistoryIx--
	}
	m.searchInput.SetValue(m.commandHistory[m.commandHistoryIx])
	m.searchInput.CursorEnd()
}

func (m *Model) commandHistoryNext() {
	if len(m.commandHistory) == 0 || m.commandHistoryIx == -1 {
		return
	}
	if m.commandHistoryIx < len(m.commandHistory)-1 {
		m.commandHistoryIx++
		m.searchInput.SetValue(m.commandHistory[m.commandHistoryIx])
		m.searchInput.CursorEnd()
		return
	}
	m.commandHistoryIx = -1
	m.searchInput.SetValue(m.commandDraft)
	m.searchInput.CursorEnd()
}

func (m *Model) completeCommandPrompt() {
	current := strings.ToLower(strings.TrimSpace(m.searchInput.Value()))
	candidates := commandPromptCandidates()
	if current == "" {
		m.searchInput.SetValue(candidates[0])
		m.commandDraft = m.searchInput.Value()
		m.searchInput.CursorEnd()
		return
	}
	matches := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if strings.HasPrefix(candidate, current) {
			matches = append(matches, candidate)
		}
	}
	if len(matches) == 0 {
		return
	}
	completion := matches[0]
	for index, match := range matches {
		if match == current {
			completion = matches[(index+1)%len(matches)]
			break
		}
	}
	m.searchInput.SetValue(completion)
	m.commandDraft = completion
	m.commandHistoryIx = -1
	m.searchInput.CursorEnd()
}

func (m *Model) selectMailboxCommand(command string) bool {
	for index, item := range m.sidebarItems {
		if strings.EqualFold(strings.TrimSpace(item), command) {
			m.sidebarCursor = index
			return true
		}
	}
	return false
}

func titleCaseASCII(value string) string {
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func commandLeavesCompose(command string) bool {
	switch command {
	case "q", "quit", "c", "compose", "reply-ai", "reply-all-ai", "inbox", "sent", "drafts", "archive", "trash", "spam":
		return true
	default:
		return false
	}
}

func (m Model) currentMailboxAllowsMessage(msg *models.Message) bool {
	if msg == nil {
		return false
	}
	if accountID := strings.TrimSpace(m.activeAccountID); accountID != "" && !strings.EqualFold(accountID, msg.AccountID) {
		return false
	}
	if tagID := strings.TrimSpace(m.activeTagID); tagID != "" && !messageHasLabel(msg, tagID) {
		return false
	}
	if query := strings.TrimSpace(m.searchQuery); query != "" && !messageMatchesSearch(msg, strings.ToLower(query)) {
		return false
	}

	switch strings.ToLower(strings.TrimSpace(currentMailboxTitle(m))) {
	case "inbox":
		return !msg.IsDraft && !msg.IsDeleted && !msg.IsSpam && messageHasLabel(msg, "inbox")
	case "sent":
		return !msg.IsDeleted && messageHasLabel(msg, "sent")
	case "drafts":
		return msg.IsDraft && !msg.IsDeleted
	case "archive":
		return !msg.IsDeleted && messageHasLabel(msg, "archive")
	case "trash":
		return msg.IsDeleted
	case "spam":
		return !msg.IsDeleted && msg.IsSpam
	default:
		return true
	}
}

func messageHasLabel(msg *models.Message, label string) bool {
	if msg == nil {
		return false
	}
	for _, candidate := range msg.Labels {
		if strings.EqualFold(strings.TrimSpace(candidate), strings.TrimSpace(label)) {
			return true
		}
	}
	return false
}

func (m *Model) applyRepeatableAction(action repeatableAction, count int) tea.Cmd {
	if count < 1 {
		count = 1
	}
	snapshots := make([]*models.Message, 0, count)
	for range count {
		snapshot, applied, failure := m.applyRepeatableActionOnce(action)
		if failure != "" {
			if len(snapshots) == 0 {
				m.setError(failure)
			}
			break
		}
		if !applied {
			break
		}
		snapshots = append(snapshots, snapshot)
	}
	if len(snapshots) == 0 {
		return nil
	}

	m.lastAction = action
	m.armUndoBatch(snapshots, string(action))
	m.setStatus(repeatableActionStatus(action, len(snapshots)))
	m.prepareFreshMessageFetch()
	if selectedID := m.currentMessageID(); selectedID != "" {
		return m.fetchMessagesForID(selectedID)
	}
	return m.fetchMessagesAtCursor(m.listCursor)
}

func (m *Model) applyRepeatableActionOnce(action repeatableAction) (*models.Message, bool, string) {
	selected, ok := m.selectedMessage()
	if !ok {
		return nil, false, ""
	}
	snapshot := cloneMessage(m.messages[m.listCursor])
	var (
		updated *models.Message
		err     error
	)

	switch action {
	case repeatableActionNone:
		return nil, false, ""
	case repeatableActionTrash:
		if m.isTrashSelection() {
			return nil, false, "Trash action is unavailable in Trash"
		}
		updated, err = m.service.ToggleDelete(context.Background(), selected.ID)
		if err != nil {
			return nil, false, "Delete failed"
		}
	case repeatableActionDelete:
		if err = m.service.DeleteMessage(context.Background(), selected.ID); err != nil {
			return nil, false, "Permanent delete failed"
		}
	case repeatableActionArchive:
		updated, err = m.service.ArchiveMessage(context.Background(), selected.ID)
		if err != nil {
			return nil, false, "Archive failed"
		}
	case repeatableActionSpam:
		updated, err = m.service.MarkAsSpam(context.Background(), selected.ID)
		if err != nil {
			return nil, false, "Spam update failed"
		}
	default:
		return nil, false, ""
	}

	if action == repeatableActionDelete || !m.currentMailboxAllowsMessage(updated) {
		m.removeMessageAtCursor()
	} else if updated != nil {
		m.messages[m.listCursor] = updated
		m.syncContentViewport(true)
	}

	return snapshot, true, ""
}

func repeatableActionStatus(action repeatableAction, count int) string {
	if count <= 1 {
		switch action {
		case repeatableActionNone:
			return ""
		case repeatableActionTrash:
			return "Message moved to trash. Press u to undo"
		case repeatableActionDelete:
			return "Message permanently deleted. Press u to undo"
		case repeatableActionArchive:
			return "Message archived. Press u to undo"
		case repeatableActionSpam:
			return "Message marked as spam. Press u to undo"
		}
	}
	switch action {
	case repeatableActionNone:
		return ""
	case repeatableActionTrash:
		return fmt.Sprintf("%d messages moved to trash. Press u to undo", count)
	case repeatableActionDelete:
		return fmt.Sprintf("%d messages permanently deleted. Press u to undo", count)
	case repeatableActionArchive:
		return fmt.Sprintf("%d messages archived. Press u to undo", count)
	case repeatableActionSpam:
		return fmt.Sprintf("%d messages marked as spam. Press u to undo", count)
	default:
		return ""
	}
}

func (m *Model) armUndoBatch(snapshots []*models.Message, action string) {
	if len(snapshots) == 0 {
		return
	}
	clones := make([]*models.Message, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if snapshot != nil {
			clones = append(clones, cloneMessage(snapshot))
		}
	}
	if len(clones) == 0 {
		return
	}
	m.undoToken++
	token := m.undoToken
	m.pendingUndo = &undoState{
		message:   clones[0],
		messages:  clones,
		action:    action,
		token:     token,
		expiresAt: time.Now().Add(8 * time.Second),
	}
}

func (m Model) undoCountdownCmd() tea.Cmd {
	if m.pendingUndo == nil {
		return nil
	}
	remaining := time.Until(m.pendingUndo.expiresAt)
	if remaining <= 0 {
		return nil
	}
	token := m.pendingUndo.token
	return tea.Tick(remaining, func(time.Time) tea.Msg {
		return undoExpiredMsg{token: token}
	})
}

func (m *Model) applyLoadedCursor(targetCursor int, targetID string) {
	if len(m.messages) == 0 {
		m.listCursor = 0
		return
	}
	if targetID != "" {
		for index, candidate := range m.messages {
			if candidate != nil && candidate.ID == targetID {
				m.listCursor = index
				return
			}
		}
	}
	if targetCursor >= 0 {
		if targetCursor >= len(m.messages) {
			m.listCursor = len(m.messages) - 1
			return
		}
		m.listCursor = targetCursor
		return
	}
	m.listCursor = 0
}

func cloneMessage(message *models.Message) *models.Message {
	if message == nil {
		return nil
	}
	return &models.Message{
		ID:        message.ID,
		AccountID: message.AccountID,
		Subject:   message.Subject,
		From:      message.From,
		To:        append([]string{}, message.To...),
		Cc:        append([]string{}, message.Cc...),
		Bcc:       append([]string{}, message.Bcc...),
		Body:      message.Body,
		HTML:      message.HTML,
		Date:      message.Date,
		IsRead:    message.IsRead,
		IsSpam:    message.IsSpam,
		IsDraft:   message.IsDraft,
		IsStarred: message.IsStarred,
		IsDeleted: message.IsDeleted,
		Labels:    append([]string{}, message.Labels...),
		ThreadID:  message.ThreadID,
		Size:      message.Size,
	}
}

func (m Model) searchStatusMessage() string {
	return fmt.Sprintf("Search: %d of %d messages • n/N next/prev", len(m.messages), len(m.allMessages))
}

func messageMatchesSearch(msg *models.Message, query string) bool {
	fields := []string{
		msg.Subject,
		msg.From,
		msg.Body,
		strings.Join(msg.To, " "),
		strings.Join(msg.Cc, " "),
		strings.Join(msg.Labels, " "),
	}
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), query) {
			return true
		}
	}
	return false
}
