package tui

import (
	"context"
	"errors"
	"fmt"
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
				m.composeEditing = false
				m.applyComposeFocus()
				m.setStatus("Exited writing mode")
				return m, nil
			case key.Matches(msg, m.keys.Esc):
				m.state = stateList
				m.activeDraft = nil
				m.composeEditing = false
				m.setStatus("Compose cancelled")
				return m, m.fetchMessages()
			case !m.composeEditing && m.handleComposeMotion(msg):
				return m, nil
			case !m.composeEditing && isSingleRune(msg, 'o'):
				m.openComposeBody(false)
				return m, nil
			case !m.composeEditing && isSingleRune(msg, 'O'):
				m.openComposeBody(true)
				return m, nil
			case key.Matches(msg, m.keys.Up) && !m.composeEditing:
				if m.focusIndex == 0 && msg.Type == tea.KeyUp {
					m.cycleComposeAccount(-1)
					return m, nil
				}
				m.moveComposeFocus(-1)
				return m, nil
			case key.Matches(msg, m.keys.Down) && !m.composeEditing:
				if m.focusIndex == 0 && msg.Type == tea.KeyDown {
					m.cycleComposeAccount(1)
					return m, nil
				}
				m.moveComposeFocus(1)
				return m, nil
			case key.Matches(msg, m.keys.Left) && !m.composeEditing && m.focusIndex == 0:
				m.cycleComposeAccount(-1)
				return m, nil
			case key.Matches(msg, m.keys.Right) && !m.composeEditing && m.focusIndex == 0:
				m.cycleComposeAccount(1)
				return m, nil
			case key.Matches(msg, m.keys.Enter) && !m.composeEditing && m.focusIndex == 0:
				m.focusIndex++
				m.composeEditing = true
				m.applyComposeFocus()
				return m, nil
			case key.Matches(msg, m.keys.Edit) && !m.composeEditing && m.focusIndex > 0:
				m.composeEditing = true
				m.applyComposeFocus()
				m.clearStatus()
				return m, nil
			case key.Matches(msg, m.keys.Enter) && !m.composeEditing && m.focusIndex > 0:
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
				if _, err := m.persistActiveDraft(context.Background()); err != nil {
					m.setError(err.Error())
					return m, nil
				}
				m.state = stateList
				m.activeDraft = nil
				m.composeEditing = false
				m.setStatus("Draft saved")
				return m, m.fetchMessages()
			case key.Matches(msg, m.keys.Send) && m.activeDraft != nil:
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
				m.activeDraft = nil
				m.composeEditing = false
				m.setStatus("Message sent")
				return m, m.fetchMessages()
			}

			if msg.String() == "tab" && !m.composeEditing {
				m.focusIndex = (m.focusIndex + 1) % 4
				m.applyComposeFocus()
				return m, nil
			}
			if msg.String() == "shift+tab" && !m.composeEditing {
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
				m.clearSearch()
				m.setStatus("Search cleared")
				return m, nil
			case keyMatches(msg, m.keys.Enter):
				m.searchActive = false
				m.searchInput.Blur()
				if strings.TrimSpace(m.searchQuery) == "" {
					m.setStatus("Search cleared")
				} else {
					m.setStatus(m.searchStatusMessage())
				}
				return m, nil
			}
		}

		m.searchInput, cmd = m.searchInput.Update(msg)
		m.searchQuery = strings.TrimSpace(m.searchInput.Value())
		m.applySearchFilter()
		if m.searchQuery == "" {
			m.statusMessage = ""
			m.statusError = false
		}
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if handled, motionCmd := m.handleBrowseMotion(msg); handled {
			return m, motionCmd
		}

		switch {
		case keyMatches(msg, m.keys.Quit):
			return m, tea.Quit
		case isSingleRune(msg, ':'):
			m.openCommandPrompt()
			return m, nil
		case keyMatches(msg, m.keys.Refresh):
			m.setStatus("Mailbox refreshed")
			return m, m.fetchMessages()
		case keyMatches(msg, m.keys.Esc) && m.state == stateSidebar && strings.TrimSpace(m.activeAccountID) != "":
			if _, ok := m.selectedAccountID(); ok {
				m.sidebarCursor = 0
			}
			m.activeAccountID = ""
			m.setStatus("Account scope cleared")
			return m, m.fetchMessages()
		case keyMatches(msg, m.keys.Undo) && m.pendingUndo != nil:
			snapshot := cloneMessageDTO(m.pendingUndo.message)
			m.pendingUndo = nil
			if _, err := m.service.RestoreMessage(context.Background(), snapshot); err != nil {
				m.setError("Undo failed")
				return m, nil
			}
			m.setStatus("Undo applied")
			return m, m.fetchMessagesForID(snapshot.ID)
		case keyMatches(msg, m.keys.Search):
			m.openSearchPrompt()
			m.setStatus("Search the current mailbox")
			return m, nil
		case keyMatches(msg, m.keys.MarkRead):
			if len(m.messages) == 0 || m.listCursor < 0 || m.listCursor >= len(m.messages) {
				return m, nil
			}
			selected := m.messages[m.listCursor]
			if selected == nil {
				return m, nil
			}
			m.markSelectedMessageRead(context.Background())
			m.setStatus("Message marked as read")
			return m, m.fetchMessagesForID(selected.ID)
		case keyMatches(msg, m.keys.Esc) && strings.TrimSpace(m.searchQuery) != "":
			m.clearSearch()
			m.setStatus("Search cleared")
			return m, nil
		case keyMatches(msg, m.keys.Enter):
			if m.state == stateSidebar {
				oldState := m.state
				m.nextFocus()
				if oldState == stateSidebar && m.state == stateList {
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
		case keyMatches(msg, m.keys.Up):
			if m.state == stateSidebar {
				if m.sidebarCursor > 0 {
					m.sidebarCursor--
					return m, m.fetchMessages()
				}
				return m, nil
			}
			if m.state == stateList {
				if m.listCursor > 0 {
					m.listCursor--
					m.markSelectedMessageRead(context.Background())
					m.syncContentViewport(true)
				}
				return m, nil
			}
			m.contentViewport.SetYOffset(max(m.contentViewport.YOffset-1, 0))
		case keyMatches(msg, m.keys.Down):
			if m.state == stateSidebar {
				if m.sidebarCursor < len(m.sidebarItems)-1 {
					m.sidebarCursor++
					return m, m.fetchMessages()
				}
				return m, nil
			}
			if m.state == stateList {
				if m.listCursor < len(m.messages)-1 {
					m.listCursor++
					m.markSelectedMessageRead(context.Background())
					m.syncContentViewport(true)
				}
				return m, nil
			}
			m.contentViewport.SetYOffset(m.contentViewport.YOffset + 1)
		case keyMatches(msg, m.keys.Right):
			oldState := m.state
			m.nextFocus()
			if oldState == stateSidebar && m.state == stateList {
				return m, m.fetchMessages()
			}
			if oldState == stateList && m.state == stateContent {
				m.markSelectedMessageRead(context.Background())
				m.syncContentViewport(true)
			}
		case keyMatches(msg, m.keys.Left):
			m.prevFocus()
		case keyMatches(msg, m.keys.PageUp):
			if m.state == stateSidebar {
				m.sidebarCursor = max(0, m.sidebarCursor-10)
				return m, m.fetchMessages()
			}
			if m.state == stateList {
				if len(m.messages) == 0 {
					return m, nil
				}
				m.listCursor = max(0, m.listCursor-10)
				m.markSelectedMessageRead(context.Background())
				m.syncContentViewport(true)
				return m, nil
			}
			m.contentViewport.PageUp()
		case keyMatches(msg, m.keys.PageDown):
			if m.state == stateSidebar {
				m.sidebarCursor = min(len(m.sidebarItems)-1, m.sidebarCursor+10)
				return m, m.fetchMessages()
			}
			if m.state == stateList {
				if len(m.messages) == 0 {
					return m, nil
				}
				m.listCursor = min(len(m.messages)-1, m.listCursor+10)
				m.markSelectedMessageRead(context.Background())
				m.syncContentViewport(true)
				return m, nil
			}
			m.contentViewport.PageDown()
		case keyMatches(msg, m.keys.HalfUp):
			if m.state == stateSidebar {
				m.sidebarCursor = max(0, m.sidebarCursor-5)
				return m, m.fetchMessages()
			}
			if m.state == stateList {
				if len(m.messages) == 0 {
					return m, nil
				}
				m.listCursor = max(0, m.listCursor-5)
				m.markSelectedMessageRead(context.Background())
				m.syncContentViewport(true)
				return m, nil
			}
			m.contentViewport.HalfPageUp()
		case keyMatches(msg, m.keys.HalfDown):
			if m.state == stateSidebar {
				m.sidebarCursor = min(len(m.sidebarItems)-1, m.sidebarCursor+5)
				return m, m.fetchMessages()
			}
			if m.state == stateList {
				if len(m.messages) == 0 {
					return m, nil
				}
				m.listCursor = min(len(m.messages)-1, m.listCursor+5)
				m.markSelectedMessageRead(context.Background())
				m.syncContentViewport(true)
				return m, nil
			}
			m.contentViewport.HalfPageDown()
		case keyMatches(msg, m.keys.Top):
			if m.state == stateSidebar {
				m.sidebarCursor = 0
				return m, m.fetchMessages()
			}
			if m.state == stateList {
				m.listCursor = 0
				m.markSelectedMessageRead(context.Background())
				m.syncContentViewport(true)
				return m, nil
			}
			m.contentViewport.GotoTop()
		case keyMatches(msg, m.keys.Bottom):
			if m.state == stateSidebar {
				m.sidebarCursor = len(m.sidebarItems) - 1
				return m, m.fetchMessages()
			}
			if m.state == stateList {
				if len(m.messages) == 0 {
					return m, nil
				}
				m.listCursor = len(m.messages) - 1
				m.markSelectedMessageRead(context.Background())
				m.syncContentViewport(true)
				return m, nil
			}
			m.contentViewport.GotoBottom()
		case keyMatches(msg, m.keys.Delete):
			if selected, ok := m.selectedMessage(); ok {
				snapshot := cloneMessageDTO(m.messages[m.listCursor])
				if m.isTrashSelection() {
					if err := m.service.DeleteMessage(context.Background(), selected.ID); err == nil {
						m.removeMessageAtCursor()
						m.armUndo(snapshot, "delete")
						m.setStatus("Message permanently deleted. Press u to undo")
						return m, m.fetchMessagesAtCursor(m.listCursor)
					}
					m.setError("Permanent delete failed")
					return m, nil
				}
				if _, err := m.service.ToggleDelete(context.Background(), selected.ID); err == nil {
					m.removeMessageAtCursor()
					m.armUndo(snapshot, "trash")
					m.setStatus("Message moved to trash. Press u to undo")
					return m, m.fetchMessagesAtCursor(m.listCursor)
				}
				m.setError("Delete failed")
			}
		case keyMatches(msg, m.keys.Archive):
			if selected, ok := m.selectedMessage(); ok {
				snapshot := cloneMessageDTO(m.messages[m.listCursor])
				if _, err := m.service.ArchiveMessage(context.Background(), selected.ID); err == nil {
					if m.listCursor > 0 && m.listCursor >= len(m.messages)-1 {
						m.listCursor--
					}
					m.armUndo(snapshot, "archive")
					m.setStatus("Message archived. Press u to undo")
					return m, m.fetchMessagesAtCursor(m.listCursor)
				}
				m.setError("Archive failed")
			}
		case keyMatches(msg, m.keys.Spam):
			if selected, ok := m.selectedMessage(); ok {
				snapshot := cloneMessageDTO(m.messages[m.listCursor])
				if _, err := m.service.MarkAsSpam(context.Background(), selected.ID); err == nil {
					if m.listCursor > 0 && m.listCursor >= len(m.messages)-1 {
						m.listCursor--
					}
					m.armUndo(snapshot, "spam")
					m.setStatus("Message marked as spam. Press u to undo")
					return m, m.fetchMessagesAtCursor(m.listCursor)
				}
				m.setError("Spam update failed")
			}
		case keyMatches(msg, m.keys.Download):
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
		case keyMatches(msg, m.keys.Reply):
			if selected, ok := m.selectedMessage(); ok {
				draft := compose.BuildReply(&selected, compose.ReplyOptions{Self: []string{m.senderForAccount(selected.AccountID)}})
				m.enterComposeState(&models.MessageDTO{AccountID: selected.AccountID, From: m.senderForAccount(selected.AccountID), ThreadID: draft.ThreadID, Subject: draft.Subject, To: draft.To, Cc: draft.Cc, Body: draft.Body}, 3)
				m.setComposeContext("Reply", "Type above the quoted message.")
				m.composeEditing = true
				m.moveReplyCursorToStart()
				m.clearStatus()
			}
		case keyMatches(msg, m.keys.ReplyAll):
			if selected, ok := m.selectedMessage(); ok {
				draft := compose.BuildReply(&selected, compose.ReplyOptions{ReplyAll: true, Self: []string{m.senderForAccount(selected.AccountID)}})
				m.enterComposeState(&models.MessageDTO{AccountID: selected.AccountID, From: m.senderForAccount(selected.AccountID), ThreadID: draft.ThreadID, Subject: draft.Subject, To: draft.To, Cc: draft.Cc, Body: draft.Body}, 3)
				m.setComposeContext("Reply all", "Type above the quoted message.")
				m.composeEditing = true
				m.moveReplyCursorToStart()
				m.clearStatus()
			}
		case keyMatches(msg, m.keys.Forward):
			if selected, ok := m.selectedMessage(); ok {
				draft := compose.BuildForward(&selected, nil, "")
				m.enterComposeState(&models.MessageDTO{AccountID: selected.AccountID, From: m.senderForAccount(selected.AccountID), ThreadID: draft.ThreadID, Subject: draft.Subject, To: draft.To, Body: draft.Body}, 1)
				m.setComposeContext("Forward", "Add recipients, then edit the forwarded message below.")
				m.composeEditing = true
				m.applyComposeFocus()
				m.clearStatus()
			}
		case keyMatches(msg, m.keys.Compose):
			m.enterComposeState(&models.MessageDTO{AccountID: m.defaultAcctID, From: m.defaultFrom, Subject: "", To: []string{}, Body: ""}, 0)
			m.setComposeContext("Composer", "Write now, save when ready.")
			m.composeEditing = false
			m.clearStatus()
		}
	case messagesLoadedMsg:
		m.activeAccountID = strings.TrimSpace(msg.activeAccountID)
		m.allMessages = append([]*models.MessageDTO{}, msg.messages...)
		if strings.TrimSpace(m.searchQuery) == "" {
			m.messages = append([]*models.MessageDTO{}, msg.messages...)
			m.applyLoadedCursor(msg.targetCursor, msg.targetID)
		} else {
			m.applySearchFilter()
			m.applyLoadedCursor(msg.targetCursor, msg.targetID)
		}
		if len(m.messages) == 0 && !m.statusError {
			m.setStatus("No messages found")
		}
		m.syncContentViewport(true)
		return m, m.undoCountdownCmd()
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

func (m *Model) enterComposeState(draft *models.MessageDTO, focusIndex int) {
	m.state = stateCompose
	m.activeDraft = draft
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

func defaultComposeContext(draft *models.MessageDTO) (string, string) {
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

func (m *Model) handleComposeMotion(msg tea.KeyMsg) bool {
	if m.pendingMotion == "g" && !isSingleRune(msg, 'g') {
		m.pendingMotion = ""
	}

	switch {
	case isSingleRune(msg, 'g'):
		if m.pendingMotion == "g" {
			m.pendingMotion = ""
			m.focusIndex = 0
			m.applyComposeFocus()
			return true
		}
		m.pendingMotion = "g"
		return true
	case isSingleRune(msg, '0'):
		m.pendingMotion = ""
		m.focusIndex = 0
		m.applyComposeFocus()
		return true
	case isSingleRune(msg, '$'):
		m.pendingMotion = ""
		m.focusIndex = 3
		m.applyComposeFocus()
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
		m.repeatSearch(1)
		return true, nil
	case isSingleRune(msg, 'N'):
		m.repeatSearch(-1)
		return true, nil
	case isSingleRune(msg, 'H'):
		return m.jumpToListViewport(listViewportTop)
	case isSingleRune(msg, 'M'):
		return m.jumpToListViewport(listViewportMiddle)
	case isSingleRune(msg, 'L'):
		return m.jumpToListViewport(listViewportBottom)
	case isSingleRune(msg, 'g'):
		if m.pendingMotion == "g" {
			m.pendingMotion = ""
			return true, m.jumpToTop()
		}
		m.pendingMotion = "g"
		return true, nil
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

func (m *Model) repeatSearch(step int) {
	if strings.TrimSpace(m.searchQuery) == "" || len(m.messages) == 0 || step == 0 {
		return
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
		m.sidebarCursor = 0
		return m.fetchMessages()
	case stateList:
		if len(m.messages) == 0 {
			return nil
		}
		m.listCursor = 0
		m.markSelectedMessageRead(context.Background())
		m.syncContentViewport(true)
		return nil
	case stateContent, stateCompose:
		m.contentViewport.GotoTop()
		return nil
	}
	return nil
}

func (m *Model) jumpToBottom() tea.Cmd {
	switch m.state {
	case stateSidebar:
		if len(m.sidebarItems) == 0 {
			return nil
		}
		m.sidebarCursor = len(m.sidebarItems) - 1
		return m.fetchMessages()
	case stateList:
		if len(m.messages) == 0 {
			return nil
		}
		m.listCursor = len(m.messages) - 1
		m.markSelectedMessageRead(context.Background())
		m.syncContentViewport(true)
		return nil
	case stateContent, stateCompose:
		m.contentViewport.GotoBottom()
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

func (m Model) selectedDraft() (*models.MessageDTO, bool) {
	if m.state < stateList || len(m.messages) == 0 || m.listCursor >= len(m.messages) {
		return nil, false
	}
	selected := m.messages[m.listCursor]
	if selected == nil || !selected.IsDraft {
		return nil, false
	}

	clone := &models.MessageDTO{
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
func (m *Model) persistActiveDraft(ctx context.Context) (*models.MessageDTO, error) {
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
		m.messages = append([]*models.MessageDTO{}, m.allMessages...)
		m.restoreListCursor(selectedID)
		m.syncContentViewport(true)
		return
	}

	filtered := make([]*models.MessageDTO, 0, len(m.allMessages))
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
	m.pendingMotion = ""
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
	m.searchInput.Prompt = ": "
	m.searchInput.Placeholder = "compose | inbox | drafts | refresh | quit"
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

func (m *Model) clearSearch() {
	m.searchActive = false
	m.commandActive = false
	m.searchQuery = ""
	m.searchInput.Prompt = "/ "
	m.searchInput.Placeholder = "subject, sender, body"
	m.applySearchInputStyles(false)
	m.searchInput.SetValue("")
	m.searchInput.Blur()
	m.messages = append([]*models.MessageDTO{}, m.allMessages...)
	m.restoreListCursor("")
	m.syncContentViewport(true)
}

func (m Model) executeCommandPrompt() (tea.Model, tea.Cmd) {
	command := strings.ToLower(strings.TrimSpace(m.searchInput.Value()))
	m.recordCommandHistory(command)
	m.closeCommandPrompt()

	switch command {
	case "", "cancel":
		m.setStatus("Command cancelled")
		return m, nil
	case "q", "quit":
		return m, tea.Quit
	case "c", "compose":
		m.enterComposeState(&models.MessageDTO{AccountID: m.defaultAcctID, From: m.defaultFrom, Subject: "", To: []string{}, Body: ""}, 0)
		m.setComposeContext("Composer", "Write now, save when ready.")
		m.composeEditing = false
		m.clearStatus()
		return m, nil
	case "sync", "refresh":
		m.setStatus("Mailbox refreshed")
		return m, m.fetchMessages()
	case "inbox", "sent", "drafts", "archive", "trash", "spam":
		if m.selectMailboxCommand(command) {
			m.state = stateList
			m.setStatus("Switched to " + titleCaseASCII(command))
			return m, m.fetchMessages()
		}
	case "help":
		m.setStatus("Commands: compose inbox sent drafts archive trash spam refresh quit")
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

func (m *Model) armUndo(snapshot *models.MessageDTO, action string) {
	if snapshot == nil {
		return
	}
	m.undoToken++
	token := m.undoToken
	m.pendingUndo = &undoState{
		message:   cloneMessageDTO(snapshot),
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

func cloneMessageDTO(message *models.MessageDTO) *models.MessageDTO {
	if message == nil {
		return nil
	}
	return &models.MessageDTO{
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

func messageMatchesSearch(msg *models.MessageDTO, query string) bool {
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
