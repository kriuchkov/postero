package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kriuchkov/postero/internal/config"
)

func TestResolveAccountIDReturnsCanonicalAccountName(t *testing.T) {
	cfg := &config.Config{Accounts: []config.AccountConfig{{Name: "Personal", Email: "me@example.com"}}}

	accountID, err := resolveAccountID(cfg, "me@example.com")

	require.NoError(t, err)
	assert.Equal(t, "Personal", accountID)
}

func TestBuildListCriteriaInboxDefaults(t *testing.T) {
	criteria, err := buildListCriteria("inbox", nil, "personal", 25, 10)

	require.NoError(t, err)
	assert.Equal(t, "personal", criteria.AccountID)
	assert.Equal(t, []string{"inbox"}, criteria.Labels)
	require.NotNil(t, criteria.IsDraft)
	require.NotNil(t, criteria.IsSpam)
	require.NotNil(t, criteria.IsDeleted)
	assert.False(t, *criteria.IsDraft)
	assert.False(t, *criteria.IsSpam)
	assert.False(t, *criteria.IsDeleted)
	assert.Equal(t, 25, criteria.Limit)
	assert.Equal(t, 10, criteria.Offset)
}

func TestBuildListCriteriaRejectsUnknownMailbox(t *testing.T) {
	_, err := buildListCriteria("weird", nil, "", 25, 0)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported mailbox")
}

func TestBuildListCriteriaFlaggedMailbox(t *testing.T) {
	criteria, err := buildListCriteria("flagged", []string{"work"}, "acct", 5, 1)

	require.NoError(t, err)
	require.NotNil(t, criteria.IsStarred)
	require.NotNil(t, criteria.IsDeleted)
	assert.True(t, *criteria.IsStarred)
	assert.False(t, *criteria.IsDeleted)
	assert.Equal(t, []string{"work"}, criteria.Labels)
}

func TestBuildListCriteriaTrashMailbox(t *testing.T) {
	criteria, err := buildListCriteria("trash", nil, "", 5, 0)

	require.NoError(t, err)
	require.NotNil(t, criteria.IsDeleted)
	assert.True(t, *criteria.IsDeleted)
}

func TestBuildListCriteriaAllMailboxLeavesFlagsUnset(t *testing.T) {
	criteria, err := buildListCriteria("all", nil, "acct", 9, 2)

	require.NoError(t, err)
	assert.Nil(t, criteria.IsDraft)
	assert.Nil(t, criteria.IsSpam)
	assert.Nil(t, criteria.IsDeleted)
	assert.Empty(t, criteria.Labels)
	assert.Equal(t, "acct", criteria.AccountID)
}
