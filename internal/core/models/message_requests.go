package models

import "time"

// MessageDTO represents a message data transfer object for service and UI layers.
type MessageDTO struct {
	ID          string          `json:"id"`
	AccountID   string          `json:"account_id,omitempty"`
	Subject     string          `json:"subject"`
	From        string          `json:"from"`
	To          []string        `json:"to"`
	Cc          []string        `json:"cc,omitempty"`
	Bcc         []string        `json:"bcc,omitempty"`
	Body        string          `json:"body"`
	HTML        string          `json:"html,omitempty"`
	Date        time.Time       `json:"date"`
	IsRead      bool            `json:"is_read"`
	IsSpam      bool            `json:"is_spam"`
	IsDraft     bool            `json:"is_draft"`
	IsStarred   bool            `json:"is_starred"`
	IsDeleted   bool            `json:"is_deleted"`
	Labels      []string        `json:"labels,omitempty"`
	ThreadID    string          `json:"thread_id,omitempty"`
	Size        int64           `json:"size"`
	Attachments []AttachmentDTO `json:"attachments,omitempty"`
}

// AttachmentDTO represents a message attachment in service and UI layers.
type AttachmentDTO struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	MimeType string `json:"mime_type"`
	Data     []byte `json:"-"`
}

// CreateMessageRequest contains fields required to create a new draft.
type CreateMessageRequest struct {
	AccountID string   `json:"account_id,omitempty"`
	From      string   `json:"from,omitempty"`
	Subject   string   `json:"subject"`
	To        []string `json:"to"`
	Cc        []string `json:"cc,omitempty"`
	Bcc       []string `json:"bcc,omitempty"`
	Body      string   `json:"body"`
	HTML      string   `json:"html,omitempty"`
	Labels    []string `json:"labels,omitempty"`
}

// UpdateMessageRequest contains fields that may be updated on an existing draft.
type UpdateMessageRequest struct {
	AccountID *string   `json:"account_id,omitempty"`
	From      *string   `json:"from,omitempty"`
	Subject   *string   `json:"subject"`
	To        *[]string `json:"to"`
	Cc        *[]string `json:"cc"`
	Bcc       *[]string `json:"bcc"`
	Body      *string   `json:"body"`
}

// SearchCriteria represents message search parameters.
type SearchCriteria struct {
	Subject   string
	From      string
	To        string
	Body      string
	Since     *time.Time
	Before    *time.Time
	IsRead    *bool
	IsSpam    *bool
	IsDraft   *bool
	IsStarred *bool
	IsDeleted *bool
	AccountID string
	Labels    []string
	Limit     int
	Offset    int
}
