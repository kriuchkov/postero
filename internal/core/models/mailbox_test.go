package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kriuchkov/postero/internal/core/models"
)

func TestParseMailboxAcceptsAliasesAndCasing(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		expected models.Mailbox
	}{
		{"inbox", models.MailboxInbox},
		{"Inbox", models.MailboxInbox},
		{" TRASH ", models.MailboxTrash}, // padded and upper-cased, as typed
		{"draft", models.MailboxDrafts},
		{"drafts", models.MailboxDrafts},
		{"junk", models.MailboxSpam},
		{"starred", models.MailboxFlagged},
	} {
		name, expected := tc.name, tc.expected
		box, ok := models.ParseMailbox(name)
		require.Truef(t, ok, "%q must name a mailbox", name)
		assert.Equalf(t, expected, box, "%q", name)
	}

	_, ok := models.ParseMailbox("работа")
	assert.False(t, ok, "a user tag is not a mailbox")
}

// TestMailboxContainsMatchesItsCriteria is the guarantee the whole design rests
// on: the predicate every in-memory filter uses is derived from the criteria
// every query and counter uses, so the two cannot disagree about any message.
func TestMailboxContainsMatchesItsCriteria(t *testing.T) {
	t.Parallel()

	messages := []*models.Message{
		{ID: "inbox", Labels: []string{"inbox"}},
		{ID: "unread-inbox", Labels: []string{"inbox"}},
		{ID: "draft", IsDraft: true, Labels: []string{"draft"}},
		{ID: "sent", Labels: []string{"sent"}},
		{ID: "archived", Labels: []string{"archive"}},
		{ID: "spam", IsSpam: true, Labels: []string{"inbox"}},
		{ID: "trashed-inbox", IsDeleted: true, Labels: []string{"inbox"}},
		{ID: "trashed-spam", IsDeleted: true, IsSpam: true, Labels: []string{"inbox"}},
		{ID: "starred", IsStarred: true, Labels: []string{"inbox"}},
	}
	boxes := []models.Mailbox{
		models.MailboxInbox, models.MailboxSent, models.MailboxDrafts, models.MailboxArchive,
		models.MailboxTrash, models.MailboxSpam, models.MailboxFlagged, models.MailboxAll,
	}

	for _, box := range boxes {
		for _, msg := range messages {
			assert.Equalf(t, models.MatchesCriteria(msg, box.Criteria("")), box.Contains(msg),
				"mailbox %q disagrees with its own criteria for %q", box, msg.ID)
		}
	}
}

// TestMailboxMembershipRules pins the rules themselves — a trashed message
// belongs to Trash and nowhere else, spam leaves the inbox, drafts are not mail.
func TestMailboxMembershipRules(t *testing.T) {
	t.Parallel()

	trashedInbox := &models.Message{IsDeleted: true, Labels: []string{"inbox"}}
	assert.True(t, models.MailboxTrash.Contains(trashedInbox))
	assert.False(t, models.MailboxInbox.Contains(trashedInbox), "trashed mail leaves its old mailbox")

	trashedSpam := &models.Message{IsDeleted: true, IsSpam: true}
	assert.True(t, models.MailboxTrash.Contains(trashedSpam))
	assert.False(t, models.MailboxSpam.Contains(trashedSpam), "trashed spam belongs to Trash only")

	spam := &models.Message{IsSpam: true, Labels: []string{"inbox"}}
	assert.True(t, models.MailboxSpam.Contains(spam))
	assert.False(t, models.MailboxInbox.Contains(spam), "spam leaves the inbox")

	draft := &models.Message{IsDraft: true, Labels: []string{"inbox"}}
	assert.True(t, models.MailboxDrafts.Contains(draft))
	assert.False(t, models.MailboxInbox.Contains(draft), "an unsent draft is not delivered mail")

	assert.True(t, models.MailboxAll.Contains(trashedSpam), "All spans every mailbox")
}

func TestMailboxCriteriaScopesToAccount(t *testing.T) {
	t.Parallel()

	criteria := models.MailboxInbox.Criteria("personal")
	assert.Equal(t, "personal", criteria.AccountID)
	assert.Equal(t, []string{"inbox"}, criteria.Labels)
	assert.Zero(t, criteria.Limit, "mailbox criteria carry no pagination")

	assert.Empty(t, models.MailboxInbox.Criteria("").AccountID)
}

func TestIsSystemMailboxLabelSeparatesTagsFromFolders(t *testing.T) {
	t.Parallel()

	assert.True(t, models.IsSystemMailboxLabel("inbox"))
	assert.True(t, models.IsSystemMailboxLabel("Drafts"))
	assert.False(t, models.IsSystemMailboxLabel("работа"))
}
