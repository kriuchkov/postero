package errors

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageNotFoundMatchesSentinel(t *testing.T) {
	err := MessageNotFound("msg-42")

	require.ErrorIs(t, err, ErrMessageNotFound)
	assert.EqualError(t, err, "message msg-42 not found: message not found")
}

func TestAccountNotFoundMatchesSentinel(t *testing.T) {
	err := AccountNotFound("gmail")

	require.ErrorIs(t, err, ErrAccountNotFound)
	assert.EqualError(t, err, "account \"gmail\" not found: account not found")
}

func TestPasswordNotConfiguredMatchesSentinel(t *testing.T) {
	err := PasswordNotConfigured("personal")

	require.ErrorIs(t, err, ErrPasswordNotConfigured)
	assert.EqualError(t, err, "account personal password is not configured: password is not configured")
}

func TestSnapshotNilMatchesSentinel(t *testing.T) {
	err := SnapshotNil()

	require.ErrorIs(t, err, ErrSnapshotNil)
	assert.EqualError(t, err, "snapshot is nil: snapshot is nil")
}

func TestNewDomainErrorSupportsErrorsAs(t *testing.T) {
	err := MessageNotFound("msg-1")

	var domainErr DomainError
	require.ErrorAs(t, err, &domainErr)
	assert.Equal(t, "message_not_found", domainErr.Code)
	assert.Equal(t, ErrMessageNotFound, domainErr.Unwrap())
}
