package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kriuchkov/postero/internal/config"
	"github.com/kriuchkov/postero/internal/core/models"
)

// pagedMailbox builds an inbox larger than one page so the loaded list and the
// mailbox total genuinely differ.
func pagedMailbox(total, unread int) []*models.Message {
	msgs := make([]*models.Message, 0, total)
	for i := range total {
		msgs = append(msgs, &models.Message{
			ID:        fmt.Sprintf("paged-%d", i),
			AccountID: "personal",
			Subject:   fmt.Sprintf("Message %d", i),
			From:      "sender@example.com",
			Body:      "Body",
			Labels:    []string{"inbox"},
			Date:      sampleMessages()[0].Date,
			IsRead:    i >= unread,
		})
	}
	return msgs
}

// loadFirstPage runs the initial fetch and the counts command that follows it,
// mirroring what happens when the user opens a mailbox.
func loadFirstPage(t *testing.T, m Model) Model {
	t.Helper()
	m.prepareFreshMessageFetch()
	cmd := m.fetchMessages()
	require.NotNil(t, cmd)

	// The fetch is batched with the loading spinner; unwrap it to the load result.
	batch, ok := cmd().(tea.BatchMsg)
	require.True(t, ok, "fetching a page batches the spinner with the load")
	m, next := updateModelWithCmd(t, m, resolveBatchMsgForTests(batch))
	require.NotNil(t, next, "loading a page must schedule the mailbox counts")

	counts, isCounts := next().(mailboxCountsMsg)
	require.True(t, isCounts, "the follow-up command must load the mailbox counts")
	return updateModel(t, m, counts)
}

// TestMailboxCountsReportStoreTotalsNotLoadedPage guards the "30 messages that
// silently became 46" bug: the list is paged, so the header and sidebar must
// report what the store holds, not how much of it has been paged in.
func TestMailboxCountsReportStoreTotalsNotLoadedPage(t *testing.T) {
	const (
		total     = 46
		unread    = 3
		pageSize  = 30
		sidebarUp = 24
	)
	service := &messageServiceStub{inbox: pagedMailbox(total, unread)}
	m := testModelWithService(service)
	m.config = &config.Config{TUI: config.TUIConfig{ListPageSize: pageSize}}
	m.state = stateList

	m = loadFirstPage(t, m)

	require.Len(t, m.messages, pageSize, "only the first page is loaded")
	require.True(t, m.mailboxCounts.valid, "counts must arrive with the page")

	assert.Equal(t, fmt.Sprintf("%d messages, %d unread", total, unread), mailboxSubtitle(m),
		"the header reports the mailbox total, not the loaded page")
	assert.Equal(t, total, sidebarMailboxCount(m, "Inbox"))
	assert.Contains(t, ansi.Strip(renderSidebar(m, sidebarUp, 18)), fmt.Sprintf("Inbox (%d)", total))
}

// TestMailboxCountsSurviveScrollingToTheNextPage: pulling in a second page must
// not change the reported totals — that jump from 30 to 46 was the bug.
func TestMailboxCountsSurviveScrollingToTheNextPage(t *testing.T) {
	service := &messageServiceStub{inbox: pagedMailbox(46, 0)}
	m := testModelWithService(service)
	m.config = &config.Config{TUI: config.TUIConfig{ListPageSize: 30}}
	m.state = stateList
	m = loadFirstPage(t, m)

	before := mailboxSubtitle(m)

	// Walk to the end of the loaded page so the next one is fetched.
	for range 40 {
		updated, cmd := m.Update(keyRune('j'))
		m = updated.(Model)
		if cmd == nil {
			continue
		}
		if msg := cmd(); msg != nil {
			m = updateModel(t, m, msg)
		}
	}

	require.Greater(t, len(m.messages), 30, "scrolling must have pulled the next page in")
	assert.Equal(t, before, mailboxSubtitle(m), "the mailbox total must not change while paging")
	assert.Equal(t, 46, sidebarMailboxCount(m, "Inbox"))
}

// TestMailboxCountsIgnoreStaleScope: a counts reply for a mailbox the user has
// already left must not overwrite the counters of the current one.
func TestMailboxCountsIgnoreStaleScope(t *testing.T) {
	service := &messageServiceStub{inbox: pagedMailbox(46, 0)}
	m := testModelWithService(service)
	m.state = stateList
	m = loadFirstPage(t, m)

	stale := mailboxCountsMsg{
		scopeKey: "Spam|||",
		folders:  map[string]int{"Inbox": 999},
		total:    999,
		valid:    true,
	}
	updated := updateModel(t, m, stale)

	assert.NotEqual(t, 999, sidebarMailboxCount(updated, "Inbox"), "a stale reply must be dropped")
	assert.NotContains(t, mailboxSubtitle(updated), "999")
}

// TestMailboxCriteriaMatchesSidebarSemantics pins the shared criteria builder:
// list fetches and counters both go through it, so a mailbox and its counter
// can never disagree about which messages belong in it.
func TestMailboxCriteriaMatchesSidebarSemantics(t *testing.T) {
	t.Parallel()

	inbox, ok := mailboxCriteria("personal", "Inbox")
	require.True(t, ok)
	assert.Equal(t, "personal", inbox.AccountID)
	assert.Equal(t, []string{"inbox"}, inbox.Labels)
	require.NotNil(t, inbox.IsDeleted)
	assert.False(t, *inbox.IsDeleted)
	require.NotNil(t, inbox.IsDraft)
	assert.False(t, *inbox.IsDraft)
	require.NotNil(t, inbox.IsSpam)
	assert.False(t, *inbox.IsSpam)

	trash, ok := mailboxCriteria("", "Trash")
	require.True(t, ok)
	require.NotNil(t, trash.IsDeleted)
	assert.True(t, *trash.IsDeleted)
	assert.Empty(t, trash.AccountID)

	_, ok = mailboxCriteria("", "Nope")
	assert.False(t, ok, "an unknown row is not a mailbox")

	tag, ok := tagCriteria("personal", "работа")
	require.True(t, ok)
	assert.Equal(t, []string{"работа"}, tag.Labels)
	_, ok = tagCriteria("personal", "  ")
	assert.False(t, ok)
}

// TestScopeCriteriaAppliesSearchOnTopOfMailbox: a search filters the current
// mailbox rather than the whole store.
func TestScopeCriteriaAppliesSearchOnTopOfMailbox(t *testing.T) {
	m := testModel()
	m.sidebarItems = []string{"Inbox", "Sent"}
	m.sidebarCursor = 1

	criteria, ok := m.scopeCriteria("personal", "Sent", "", "invoice")
	require.True(t, ok)
	assert.Equal(t, "invoice", criteria.Query)
	assert.Equal(t, []string{"sent"}, criteria.Labels)
	assert.Equal(t, "personal", criteria.AccountID)

	_, ok = m.scopeCriteria("", strings.TrimSpace("Accounts:"), "", "")
	assert.False(t, ok, "the accounts header is not a view")
}

// TestSidebarFallbackAgreesWithMailboxCriteria: before the store-side counts
// arrive the sidebar counts loaded messages itself, so that fallback must sort
// messages into mailboxes exactly like the criteria the list and the counts
// use — deleted spam belongs to Trash, not Spam.
func TestSidebarFallbackAgreesWithMailboxCriteria(t *testing.T) {
	t.Parallel()

	deletedSpam := &models.Message{ID: "s1", IsSpam: true, IsDeleted: true, Labels: []string{"inbox"}}
	liveSpam := &models.Message{ID: "s2", IsSpam: true, Labels: []string{"inbox"}}

	assert.False(t, sidebarMessageInMailbox(deletedSpam, "Spam"), "trashed spam is not in Spam")
	assert.True(t, sidebarMessageInMailbox(deletedSpam, "Trash"), "trashed spam is in Trash")
	assert.True(t, sidebarMessageInMailbox(liveSpam, "Spam"))

	spam, ok := mailboxCriteria("", "Spam")
	require.True(t, ok)
	require.NotNil(t, spam.IsDeleted, "the Spam criteria excludes trashed messages")
	assert.False(t, *spam.IsDeleted)
}
