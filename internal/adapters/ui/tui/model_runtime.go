package tui

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kriuchkov/postero/internal/core/models"
)

const (
	defaultListFetchPageSize      = 30
	defaultListFetchNextThreshold = 5
	defaultLoadingTickIntervalMS  = 120
	defaultSearchDebounceMS       = 180
)

var loadingFrames = []string{"-", "\\", "|", "/"}

func (m *Model) applySearchInputStyles(commandMode bool) {
	if commandMode {
		m.searchInput.PromptStyle = lipgloss.NewStyle().Bold(true).Foreground(m.styles.Palette.Highlight).Background(m.styles.Palette.Primary).Padding(0, 1)
		m.searchInput.TextStyle = lipgloss.NewStyle().Bold(true).Foreground(m.styles.Palette.Highlight)
		m.searchInput.PlaceholderStyle = lipgloss.NewStyle().Foreground(m.styles.Palette.SubText)
		return
	}

	m.searchInput.PromptStyle = lipgloss.NewStyle().Bold(true).Foreground(m.styles.Palette.Primary)
	m.searchInput.TextStyle = lipgloss.NewStyle().Foreground(m.styles.Palette.Text)
	m.searchInput.PlaceholderStyle = lipgloss.NewStyle().Foreground(m.styles.Palette.SubText)
}

func commandPromptCandidates() []string {
	return []string{"compose", "inbox", "drafts", "refresh", "quit"}
}

type messagesLoadedMsg struct {
	messages        []*models.Message
	targetCursor    int
	targetID        string
	activeAccountID string
	activeTagID     string
	appendPage      bool
	hasMore         bool
	nextOffset      int
	scopeKey        string
}

type undoState struct {
	message   *models.Message
	action    string
	token     int
	expiresAt time.Time
}

type undoExpiredMsg struct {
	token int
}

type loadingTickMsg struct {
	token int
	frame int
}

type searchDebounceMsg struct {
	token int
	query string
}

func (m Model) fetchMessages() tea.Cmd {
	return m.withLoadingIndicator(m.fetchMessagesPage(-1, "", 0, false))
}

func (m Model) fetchMessagesAtCursor(targetCursor int) tea.Cmd {
	return m.withLoadingIndicator(m.fetchMessagesPage(targetCursor, "", 0, false))
}

func (m Model) fetchMessagesForID(targetID string) tea.Cmd {
	return m.withLoadingIndicator(m.fetchMessagesPage(-1, targetID, 0, false))
}

func (m Model) fetchNextMessages() tea.Cmd {
	if m.service == nil || !m.hasMoreMessages {
		return nil
	}
	return m.withLoadingIndicator(m.fetchMessagesPage(m.listCursor, m.currentMessageID(), m.fetchOffset, true))
}

func (m *Model) prepareFreshMessageFetch() {
	m.fetchOffset = 0
	m.hasMoreMessages = false
	m.messagesLoading = true
	m.loadingFrame = 0
	m.loadingToken++
}

func (m *Model) prepareNextMessageFetch() {
	if m.messagesLoading || !m.hasMoreMessages {
		return
	}
	m.messagesLoading = true
	m.loadingFrame = 0
	m.loadingToken++
}

func (m *Model) maybeFetchMoreMessages() tea.Cmd {
	if m.state != stateList {
		return nil
	}
	if m.messagesLoading || !m.hasMoreMessages {
		return nil
	}
	if len(m.messages) == 0 {
		if strings.TrimSpace(m.searchQuery) == "" {
			return nil
		}
		m.prepareNextMessageFetch()
		return m.fetchNextMessages()
	}
	if len(m.messages)-1-m.listCursor > m.listFetchNextThreshold() {
		return nil
	}
	m.prepareNextMessageFetch()
	return m.fetchNextMessages()
}

func (m Model) withLoadingIndicator(fetchCmd tea.Cmd) tea.Cmd {
	if fetchCmd == nil {
		return nil
	}
	if !m.messagesLoading {
		return fetchCmd
	}
	return tea.Batch(m.loadingTickCmd(), fetchCmd)
}

func (m Model) loadingTickCmd() tea.Cmd {
	if !m.messagesLoading {
		return nil
	}
	nextFrame := (m.loadingFrame + 1) % len(loadingFrames)
	token := m.loadingToken
	return tea.Tick(m.loadingTickInterval(), func(time.Time) tea.Msg {
		return loadingTickMsg{token: token, frame: nextFrame}
	})
}

func (m Model) listFetchPageSize() int {
	if m.config == nil || m.config.TUI.ListPageSize < 1 {
		return defaultListFetchPageSize
	}
	return m.config.TUI.ListPageSize
}

func (m Model) listFetchNextThreshold() int {
	if m.config == nil || m.config.TUI.ListPrefetchAhead < 1 {
		return defaultListFetchNextThreshold
	}
	return m.config.TUI.ListPrefetchAhead
}

func (m Model) loadingTickInterval() time.Duration {
	if m.config == nil || m.config.TUI.LoadingTickMS < 10 {
		return defaultLoadingTickIntervalMS * time.Millisecond
	}
	return time.Duration(m.config.TUI.LoadingTickMS) * time.Millisecond
}

func (m Model) searchDebounceInterval() time.Duration {
	return defaultSearchDebounceMS * time.Millisecond
}

func (m Model) searchDebounceCmd() tea.Cmd {
	if strings.TrimSpace(m.searchQuery) == "" {
		return nil
	}
	token := m.searchToken
	query := strings.TrimSpace(m.searchQuery)
	return tea.Tick(m.searchDebounceInterval(), func(time.Time) tea.Msg {
		return searchDebounceMsg{token: token, query: query}
	})
}

// fetchMessagesSelection keeps sidebar-driven loading in one place so browse state and refresh flow share the same selection rules.
func (m Model) fetchMessagesPage(targetCursor int, targetID string, offset int, appendPage bool) tea.Cmd {
	return func() tea.Msg {
		if m.service == nil {
			return nil
		}

		ctx := context.Background()
		var msgs []*models.Message
		var err error

		if m.sidebarCursor >= len(m.sidebarItems) {
			return nil
		}
		selectedItem := m.sidebarItems[m.sidebarCursor]
		scopeAccountID := strings.TrimSpace(m.activeAccountID)
		activeTagID := strings.TrimSpace(m.activeTagID)
		searchQuery := strings.TrimSpace(m.searchQuery)
		scopeKey := m.currentMessageScopeKey()
		if accountID, ok := m.selectedAccountID(); ok {
			scopeAccountID = accountID
		}
		if searchQuery != "" {
			msgs, err = m.fetchScopedSearch(ctx, scopeAccountID, selectedItem, activeTagID, searchQuery, offset)
			if err != nil {
				return nil
			}
			return messagesLoadedMsg{
				messages:        msgs,
				targetCursor:    targetCursor,
				targetID:        targetID,
				activeAccountID: scopeAccountID,
				activeTagID:     activeTagID,
				appendPage:      appendPage,
				hasMore:         len(msgs) == m.listFetchPageSize(),
				nextOffset:      offset + len(msgs),
				scopeKey:        scopeKey,
			}
		}
		if activeTagID != "" {
			msgs, err = m.fetchScopedTag(ctx, scopeAccountID, activeTagID, offset)
			if err != nil {
				return nil
			}
			return messagesLoadedMsg{
				messages:        msgs,
				targetCursor:    targetCursor,
				targetID:        targetID,
				activeAccountID: scopeAccountID,
				activeTagID:     activeTagID,
				appendPage:      appendPage,
				hasMore:         len(msgs) == m.listFetchPageSize(),
				nextOffset:      offset + len(msgs),
				scopeKey:        scopeKey,
			}
		}

		switch selectedItem {
		case "Inbox":
			msgs, err = m.fetchScopedMailbox(ctx, scopeAccountID, "Inbox", offset)
		case "Sent":
			msgs, err = m.fetchScopedMailbox(ctx, scopeAccountID, "Sent", offset)
		case "Drafts":
			msgs, err = m.fetchScopedMailbox(ctx, scopeAccountID, "Drafts", offset)
		case "Archive":
			msgs, err = m.fetchScopedMailbox(ctx, scopeAccountID, "Archive", offset)
		case "Trash":
			msgs, err = m.fetchScopedMailbox(ctx, scopeAccountID, "Trash", offset)
		case "Spam":
			msgs, err = m.fetchScopedMailbox(ctx, scopeAccountID, "Spam", offset)
		default:
			if selectedItem == "" || selectedItem == "Accounts:" {
				return nil
			}
			if accountID, ok := m.selectedAccountID(); ok {
				msgs, err = m.fetchScopedMailbox(ctx, accountID, "Inbox", offset)
			} else {
				msgs, err = m.service.GetByLabel(ctx, strings.TrimSpace(selectedItem), m.listFetchPageSize(), offset)
			}
		}

		if err != nil {
			return nil
		}

		return messagesLoadedMsg{
			messages:        msgs,
			targetCursor:    targetCursor,
			targetID:        targetID,
			activeAccountID: scopeAccountID,
			activeTagID:     activeTagID,
			appendPage:      appendPage,
			hasMore:         len(msgs) == m.listFetchPageSize(),
			nextOffset:      offset + len(msgs),
			scopeKey:        scopeKey,
		}
	}
}

func (m Model) fetchScopedSearch(ctx context.Context, accountID, selectedItem, activeTagID, query string, offset int) ([]*models.Message, error) {
	criteria := models.SearchCriteria{Query: query, Limit: m.listFetchPageSize(), Offset: offset}
	accountID = strings.TrimSpace(accountID)
	selectedItem = strings.TrimSpace(selectedItem)
	activeTagID = strings.TrimSpace(activeTagID)
	if activeTagID != "" {
		criteria.AccountID = accountID
		criteria.Labels = []string{activeTagID}
		isDeleted := false
		criteria.IsDeleted = &isDeleted
		return m.service.SearchMessages(ctx, criteria)
	}
	if strings.HasPrefix(selectedItem, "Accounts:") || selectedItem == "" {
		return nil, nil
	}
	if strings.HasPrefix(m.sidebarItems[m.sidebarCursor], "  ") {
		selectedItem = "Inbox"
	}
	if accountID != "" {
		criteria.AccountID = accountID
	}

	switch selectedItem {
	case "Inbox":
		isDraft := false
		isSpam := false
		isDeleted := false
		criteria.IsDraft = &isDraft
		criteria.IsSpam = &isSpam
		criteria.IsDeleted = &isDeleted
		criteria.Labels = []string{"inbox"}
	case "Sent":
		isDeleted := false
		criteria.IsDeleted = &isDeleted
		criteria.Labels = []string{"sent"}
	case "Drafts":
		isDraft := true
		isDeleted := false
		criteria.IsDraft = &isDraft
		criteria.IsDeleted = &isDeleted
	case "Archive":
		isDeleted := false
		criteria.IsDeleted = &isDeleted
		criteria.Labels = []string{"archive"}
	case "Trash":
		isDeleted := true
		criteria.IsDeleted = &isDeleted
	case "Spam":
		isDeleted := false
		isSpam := true
		criteria.IsDeleted = &isDeleted
		criteria.IsSpam = &isSpam
	default:
		isDeleted := false
		criteria.IsDeleted = &isDeleted
		criteria.Labels = []string{selectedItem}
	}

	return m.service.SearchMessages(ctx, criteria)
}

func (m Model) fetchScopedTag(ctx context.Context, accountID, tag string, offset int) ([]*models.Message, error) {
	accountID = strings.TrimSpace(accountID)
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return nil, nil
	}
	if accountID == "" {
		return m.service.GetByLabel(ctx, tag, m.listFetchPageSize(), offset)
	}

	isDeleted := false
	return m.service.SearchMessages(ctx, models.SearchCriteria{
		AccountID: accountID,
		Labels:    []string{tag},
		IsDeleted: &isDeleted,
		Limit:     m.listFetchPageSize(),
		Offset:    offset,
	})
}

func (m Model) fetchScopedMailbox(ctx context.Context, accountID, mailbox string, offset int) ([]*models.Message, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		switch mailbox {
		case "Inbox":
			return m.service.GetAllInboxes(ctx, m.listFetchPageSize(), offset)
		case "Sent":
			return m.service.GetSent(ctx, m.listFetchPageSize(), offset)
		case "Drafts":
			return m.service.GetDrafts(ctx, m.listFetchPageSize(), offset)
		case "Archive":
			return m.service.GetByLabel(ctx, "archive", m.listFetchPageSize(), offset)
		case "Trash":
			isDeleted := true
			return m.service.SearchMessages(ctx, models.SearchCriteria{IsDeleted: &isDeleted, Limit: m.listFetchPageSize(), Offset: offset})
		case "Spam":
			isSpam := true
			isDeleted := false
			return m.service.SearchMessages(ctx, models.SearchCriteria{IsSpam: &isSpam, IsDeleted: &isDeleted, Limit: m.listFetchPageSize(), Offset: offset})
		default:
			return nil, nil
		}
	}

	isDeleted := false
	criteria := models.SearchCriteria{AccountID: accountID, IsDeleted: &isDeleted, Limit: m.listFetchPageSize(), Offset: offset}
	switch mailbox {
	case "Inbox":
		isDraft := false
		isSpam := false
		criteria.IsDraft = &isDraft
		criteria.IsSpam = &isSpam
		criteria.Labels = []string{"inbox"}
	case "Sent":
		criteria.Labels = []string{"sent"}
	case "Drafts":
		isDraft := true
		criteria.IsDraft = &isDraft
	case "Archive":
		criteria.Labels = []string{"archive"}
	case "Trash":
		isDeleted = true
		criteria.IsDeleted = &isDeleted
	case "Spam":
		isSpam := true
		criteria.IsSpam = &isSpam
	default:
		return nil, nil
	}

	return m.service.SearchMessages(ctx, criteria)
}

func (m Model) currentMessageScopeKey() string {
	selectedItem := ""
	if m.sidebarCursor >= 0 && m.sidebarCursor < len(m.sidebarItems) {
		selectedItem = strings.TrimSpace(m.sidebarItems[m.sidebarCursor])
	}
	return strings.Join([]string{
		selectedItem,
		strings.TrimSpace(m.activeAccountID),
		strings.TrimSpace(m.activeTagID),
		strings.TrimSpace(m.searchQuery),
	}, "|")
}

func mergeMessages(existing, incoming []*models.Message) []*models.Message {
	if len(existing) == 0 {
		return append([]*models.Message{}, incoming...)
	}
	result := append([]*models.Message{}, existing...)
	seenIDs := make([]string, 0, len(existing))
	for _, message := range existing {
		if message != nil && strings.TrimSpace(message.ID) != "" {
			seenIDs = append(seenIDs, message.ID)
		}
	}
	for _, message := range incoming {
		if message == nil {
			continue
		}
		if message.ID != "" && slices.Contains(seenIDs, message.ID) {
			continue
		}
		result = append(result, message)
		if message.ID != "" {
			seenIDs = append(seenIDs, message.ID)
		}
	}
	return result
}

func keyMatches(msg tea.KeyMsg, k key.Binding) bool {
	return key.Matches(msg, k)
}
