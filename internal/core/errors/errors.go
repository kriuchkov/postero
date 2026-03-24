package errors

import (
	stderrors "errors"
	"fmt"
)

// DomainError represents a domain-specific error
type DomainError struct {
	Code    string
	Message string
	Err     error
}

// NewDomainError creates a new domain error
func NewDomainError(code, message string, err error) DomainError {
	return DomainError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

func (e DomainError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e DomainError) Unwrap() error {
	return e.Err
}

// Common domain errors
var (
	ErrMessageNotFound       = stderrors.New("message not found")
	ErrAccountNotFound       = stderrors.New("account not found")
	ErrInvalidEmailFormat    = stderrors.New("invalid email format")
	ErrConnectionFailed      = stderrors.New("connection failed")
	ErrAuthenticationFailed  = stderrors.New("authentication failed")
	ErrEmptySubject          = stderrors.New("subject cannot be empty")
	ErrNoRecipients          = stderrors.New("no recipients specified")
	ErrPasswordNotConfigured = stderrors.New("password is not configured")
	ErrSnapshotNil           = stderrors.New("snapshot is nil")
)

func MessageNotFound(id string) error {
	return NewDomainError("message_not_found", fmt.Sprintf("message %s not found", id), ErrMessageNotFound)
}

func AccountNotFound(account string) error {
	return NewDomainError("account_not_found", fmt.Sprintf("account %q not found", account), ErrAccountNotFound)
}

func PasswordNotConfigured(account string) error {
	return NewDomainError(
		"password_not_configured",
		fmt.Sprintf("account %s password is not configured", account),
		ErrPasswordNotConfigured,
	)
}

func SnapshotNil() error {
	return NewDomainError("snapshot_nil", "snapshot is nil", ErrSnapshotNil)
}
