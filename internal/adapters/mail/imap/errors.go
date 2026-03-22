package imap

import "errors"

var (
	ErrNotConnected = errors.New("not connected to IMAP server")
	ErrFetchFailed  = errors.New("failed to fetch messages")
)
