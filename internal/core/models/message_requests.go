package models

import "time"

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
	Query     string
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
