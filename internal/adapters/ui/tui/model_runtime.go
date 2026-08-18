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

func (m Model) commandPromptCandidates() []string {
	if m.state == stateCompose {
		// Vim-style verbs: :w save, :wq/:x save+close, :q close, :q! discard.
		return []string{"w", "wq", "x", "q", "q!", "send", "compose-ai", "reply-ai", "reply-all-ai", "help"}
	}
	return []string{"compose", "compose-ai", "reply-ai", "reply-all-ai", "inbox", "sent", "drafts", "archive", "trash", "spam", "refresh", "setup", "demo", "help", "quit"}
}

func (m Model) commandPromptPlaceholder() string {
	return strings.Join(m.commandPromptCandidates(), " | ")
}

func commandPromptHelpCandidates() []string {
	return []string{
		"enter run • esc cancel • try compose compose-ai reply-ai reply-all-ai inbox sent drafts archive trash spam refresh help quit",
		"enter run • esc cancel • compose compose-ai reply-ai inbox archive help quit",
	}
}

type repeatableAction string

const (
	repeatableActionNone    repeatableAction = ""
	repeatableActionTrash   repeatableAction = "trash"
	repeatableActionDelete  repeatableAction = "delete"
	repeatableActionArchive repeatableAction = "archive"
	repeatableActionSpam    repeatableAction = "spam"
)

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
	messages  []*models.Message
	action    string
	token     int
	expiresAt time.Time
}

func (u *undoState) snapshots() []*models.Message {
	if u == nil {
		return nil
	}
	if len(u.messages) > 0 {
		return u.messages
	}
	if u.message != nil {
		return []*models.Message{u.message}
	}
	return nil
}

type undoExpiredMsg struct {
	token int
}

type loadingTickMsg struct {
	token int
	frame int
}

type aiLoadingTickMsg struct {
	token int
	frame int
}

type searchDebounceMsg struct {
	token int
	query string
}

type syncCompletedMsg struct {
	count int
	err   error
}

type demoSeededMsg struct {
	count int
	err   error
}

// mailboxCountsMsg carries store-side totals: per-folder counts for the
// sidebar plus the current view's total/unread for the header. scopeKey guards
// against a late reply landing after the user moved to another mailbox.
type mailboxCountsMsg struct {
	scopeKey   string
	folders    map[string]int
	total      int
	unread     int
	scopeTotal int // mailbox total ignoring the search query ("N of M")
	valid      bool
}

// trashPushedMsg reports the background IMAP move of one trashed message.
type trashPushedMsg struct {
	id  string
	err error
}

// pushTrashMoveCmd propagates a local trash to the IMAP server off the update
// loop, so the delete key stays instant while the network round-trip runs in
// the background. PushTrashMove no-ops if the user undoes the delete first.
func (m Model) pushTrashMoveCmd(id string) tea.Cmd {
	service := m.service
	if service == nil {
		return nil
	}
	return func() tea.Msg {
		_, err := service.PushTrashMove(context.Background(), id)
		return trashPushedMsg{id: id, err: err}
	}
}

// demoSeedCmd loads sample messages into the local store so the app can be
// explored without a real account.
func (m Model) demoSeedCmd() tea.Cmd {
	seeder := m.seeder
	return func() tea.Msg {
		if seeder == nil {
			return demoSeededMsg{}
		}
		messages, err := seeder.SeedDemo(context.Background())
		return demoSeededMsg{count: len(messages), err: err}
	}
}

// canSyncAccounts reports whether an IMAP sync is possible for this session.
func (m Model) canSyncAccounts() bool {
	return m.syncer != nil && m.config != nil && len(m.config.Accounts) > 0
}

// syncAccountsCmd pulls new mail from every configured account into the local
// store; the completion message triggers a normal list reload.
func (m Model) syncAccountsCmd() tea.Cmd {
	syncer := m.syncer
	return func() tea.Msg {
		if syncer == nil {
			return syncCompletedMsg{}
		}
		messages, err := syncer.SyncAll(context.Background())
		return syncCompletedMsg{count: len(messages), err: err}
	}
}

// refreshMailboxCmd syncs accounts when possible and falls back to a plain
// local reload otherwise.
func (m *Model) refreshMailboxCmd() tea.Cmd {
	if !m.canSyncAccounts() {
		m.setStatus("Mailbox refreshed")
		m.prepareFreshMessageFetch()
		return m.fetchMessages()
	}
	m.setStatus("Syncing accounts...")
	m.prepareFreshMessageFetch()
	return tea.Batch(m.loadingTickCmd(), m.syncAccountsCmd())
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

func (m *Model) prepareAIGeneration(label string) {
	m.aiGenerating = true
	m.aiLoadingFrame = 0
	m.aiLoadingLabel = strings.TrimSpace(label)
	m.aiLoadingToken++
}

func (m *Model) finishAIGeneration() {
	m.aiGenerating = false
	m.aiLoadingFrame = 0
	m.aiLoadingLabel = ""
}

func (m Model) withAILoadingIndicator(cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	if !m.aiGenerating {
		return cmd
	}
	return tea.Batch(m.aiLoadingTickCmd(), cmd)
}

func (m Model) aiLoadingTickCmd() tea.Cmd {
	if !m.aiGenerating {
		return nil
	}
	nextFrame := (m.aiLoadingFrame + 1) % len(loadingFrames)
	token := m.aiLoadingToken
	return tea.Tick(m.loadingTickInterval(), func(time.Time) tea.Msg {
		return aiLoadingTickMsg{token: token, frame: nextFrame}
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

// fetchMessagesPage keeps sidebar-driven loading in one place so browse state
// and refresh flow share the same selection rules. Every view — mailbox, tag,
// search, account scope — resolves through scopeCriteria, the same builder the
// mailbox counters use.
func (m Model) fetchMessagesPage(targetCursor int, targetID string, offset int, appendPage bool) tea.Cmd {
	return func() tea.Msg {
		if m.service == nil {
			return nil
		}

		accountID, selectedItem, activeTagID, searchQuery := m.currentScope()
		criteria, ok := m.scopeCriteria(accountID, selectedItem, activeTagID, searchQuery)
		if !ok {
			return nil
		}
		criteria.Limit = m.listFetchPageSize()
		criteria.Offset = offset

		msgs, err := m.service.SearchMessages(context.Background(), criteria)
		if err != nil {
			return nil
		}

		return messagesLoadedMsg{
			messages:        msgs,
			targetCursor:    targetCursor,
			targetID:        targetID,
			activeAccountID: accountID,
			activeTagID:     activeTagID,
			appendPage:      appendPage,
			hasMore:         len(msgs) == m.listFetchPageSize(),
			nextOffset:      offset + len(msgs),
			scopeKey:        m.currentMessageScopeKey(),
		}
	}
}

// scopeCriteria maps the whole current view — mailbox or tag, account scope and
// search query — onto one unpaginated criteria. Both the list fetch and the
// mailbox counters go through it.
func (m Model) scopeCriteria(accountID, selectedItem, tagID, query string) (models.SearchCriteria, bool) {
	accountID = strings.TrimSpace(accountID)
	selectedItem = strings.TrimSpace(selectedItem)
	query = strings.TrimSpace(query)

	if criteria, ok := tagCriteria(accountID, tagID); ok {
		criteria.Query = query
		return criteria, true
	}
	if selectedItem == "" || strings.HasPrefix(selectedItem, "Accounts:") {
		return models.SearchCriteria{}, false
	}
	// An account row in the sidebar scopes the inbox to that account.
	if m.sidebarCursor >= 0 && m.sidebarCursor < len(m.sidebarItems) &&
		strings.HasPrefix(m.sidebarItems[m.sidebarCursor], "  ") {
		selectedItem = "Inbox"
	}

	criteria, ok := mailboxCriteria(accountID, selectedItem)
	if !ok {
		// A custom sidebar row (a tag section entry) filters by that label.
		notDeleted := false
		criteria = models.SearchCriteria{
			AccountID: accountID,
			Labels:    []string{selectedItem},
			IsDeleted: &notDeleted,
		}
	}
	criteria.Query = query
	return criteria, true
}

// currentScope resolves the view the user is looking at right now: the account
// scope (sidebar selection wins over the pinned scope), the selected sidebar
// row, the active tag, and the search query.
func (m Model) currentScope() (string, string, string, string) {
	selectedItem := ""
	if m.sidebarCursor >= 0 && m.sidebarCursor < len(m.sidebarItems) {
		selectedItem = m.sidebarItems[m.sidebarCursor]
	}
	accountID := strings.TrimSpace(m.activeAccountID)
	if selected, ok := m.selectedAccountID(); ok {
		accountID = selected
	}
	return accountID, selectedItem, strings.TrimSpace(m.activeTagID), strings.TrimSpace(m.searchQuery)
}

// fetchMailboxCountsCmd loads the true mailbox totals from the store. The list
// itself is paged, so counting loaded messages would report "30 messages" for a
// 46-message inbox and silently change to 46 once scrolling pulled the next
// page in — these counts come from the store and never depend on paging.
func (m Model) fetchMailboxCountsCmd() tea.Cmd {
	service := m.service
	if service == nil {
		return nil
	}
	accountID, selectedItem, tagID, query := m.currentScope()
	scopeKey := m.currentMessageScopeKey()

	return func() tea.Msg {
		ctx := context.Background()
		counts := mailboxCountsMsg{scopeKey: scopeKey, folders: make(map[string]int, len(sidebarMailboxes))}

		for _, mailbox := range sidebarMailboxes {
			criteria, ok := mailboxCriteria(accountID, mailbox)
			if !ok {
				continue
			}
			total, err := service.CountMessages(ctx, criteria)
			if err != nil {
				return nil
			}
			counts.folders[mailbox] = total
		}

		criteria, ok := m.scopeCriteria(accountID, selectedItem, tagID, query)
		if !ok {
			return counts
		}
		total, err := service.CountMessages(ctx, criteria)
		if err != nil {
			return nil
		}
		counts.total = total

		unreadCriteria := criteria
		notRead := false
		unreadCriteria.IsRead = &notRead
		if unread, err := service.CountMessages(ctx, unreadCriteria); err == nil {
			counts.unread = unread
		}

		// With a search active the header reads "N of M": M is the mailbox
		// total the query filters down, so count it without the query.
		if query != "" {
			unfiltered := criteria
			unfiltered.Query = ""
			if scopeTotal, err := service.CountMessages(ctx, unfiltered); err == nil {
				counts.scopeTotal = scopeTotal
			}
		}
		counts.valid = true
		return counts
	}
}

// mailboxCriteria maps a sidebar row onto the domain's mailbox definition. The
// TUI only knows the row's display name; what belongs in that mailbox is the
// domain's business, shared with the CLI, the counters and the storage query.
func mailboxCriteria(accountID, mailbox string) (models.SearchCriteria, bool) {
	box, ok := models.ParseMailbox(mailbox)
	if !ok || box == models.MailboxAll || box == models.MailboxFlagged {
		// The sidebar has no All/Flagged rows; treat them as unknown here so a
		// tag row named "all" keeps falling through to the label filter.
		return models.SearchCriteria{}, false
	}
	return box.Criteria(accountID), true
}

// tagCriteria describes a sidebar tag selection as an unpaginated criteria.
func tagCriteria(accountID, tag string) (models.SearchCriteria, bool) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return models.SearchCriteria{}, false
	}
	notDeleted := false
	return models.SearchCriteria{
		AccountID: strings.TrimSpace(accountID),
		Labels:    []string{tag},
		IsDeleted: &notDeleted,
	}, true
}

// sidebarMailboxes are the folder rows whose totals the sidebar reports.
var sidebarMailboxes = []string{"Inbox", "Sent", "Drafts", "Archive", "Trash", "Spam"}

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
