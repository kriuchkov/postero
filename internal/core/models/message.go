package models

import "time"

// Message represents an email message in the system
type Message struct {
	ID          string        `json:"id,omitempty"`
	AccountID   string        `json:"account_id,omitempty"`
	Subject     string        `json:"subject,omitempty"`
	From        string        `json:"from,omitempty"`
	To          []string      `json:"to,omitempty"`
	Cc          []string      `json:"cc,omitempty"`
	Bcc         []string      `json:"bcc,omitempty"`
	Body        string        `json:"body,omitempty"`
	HTML        string        `json:"html,omitempty"`
	Date        time.Time     `json:"date,omitempty"`
	Flags       MessageFlags  `json:"flags,omitempty"`
	Labels      []string      `json:"labels,omitempty"`
	ThreadID    string        `json:"thread_id,omitempty"`
	IsRead      bool          `json:"is_read,omitempty"`
	IsSpam      bool          `json:"is_spam,omitempty"`
	IsDraft     bool          `json:"is_draft,omitempty"`
	IsStarred   bool          `json:"is_starred,omitempty"`
	IsDeleted   bool          `json:"is_deleted,omitempty"`
	Size        int64         `json:"size,omitempty"`
	Attachments []*Attachment `json:"attachments,omitempty"`
}

// MessageFlags represents IMAP message flags
type MessageFlags struct {
	Seen     bool `json:"seen,omitempty"`
	Answered bool `json:"answered,omitempty"`
	Flagged  bool `json:"flagged,omitempty"`
	Draft    bool `json:"draft,omitempty"`
	Deleted  bool `json:"deleted,omitempty"`
	Junk     bool `json:"junk,omitempty"`
}

// Attachment represents an email attachment
type Attachment struct {
	Filename string `json:"filename,omitempty"`
	Size     int64  `json:"size,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
	Data     []byte `json:"data,omitempty"`
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
