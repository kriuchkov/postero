package models

import "time"

// Message represents an email message in the system
type Message struct {
	ID          string
	AccountID   string
	Subject     string
	From        string
	To          []string
	Cc          []string
	Bcc         []string
	Body        string
	HTML        string
	Date        time.Time
	Flags       MessageFlags
	Labels      []string
	ThreadID    string
	IsRead      bool
	IsSpam      bool
	IsDraft     bool
	IsStarred   bool
	IsDeleted   bool
	Size        int64
	Attachments []*Attachment
}

// MessageFlags represents IMAP message flags
type MessageFlags struct {
	Seen     bool
	Answered bool
	Flagged  bool
	Draft    bool
	Deleted  bool
	Junk     bool
}

// Attachment represents an email attachment
type Attachment struct {
	Filename string
	Size     int64
	MimeType string
	Data     []byte
}

// Account represents an email account configuration
type Account struct {
	ID       string
	Name     string
	Email    string
	IMAPHost string
	IMAPPort int
	SMTPHost string
	SMTPPort int
	Username string
	Password string
	UseTLS   bool
}
