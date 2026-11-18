package models

import "strings"

// Mailbox is a logical folder a message can belong to.
//
// Membership is defined exactly once — as a SearchCriteria — and every consumer
// derives from that definition: the storage query that lists a mailbox, the
// counter that reports its size, and the in-memory filter that decides whether
// a message still belongs after the user acts on it. Hand-writing "what counts
// as spam" a second time is how those three drift apart.
type Mailbox string

const (
	MailboxInbox   Mailbox = "inbox"
	MailboxSent    Mailbox = "sent"
	MailboxDrafts  Mailbox = "drafts"
	MailboxArchive Mailbox = "archive"
	MailboxTrash   Mailbox = "trash"
	MailboxSpam    Mailbox = "spam"
	MailboxFlagged Mailbox = "flagged"
	MailboxAll     Mailbox = "all"
)

// ParseMailbox resolves a mailbox name — a sidebar row, a CLI argument or a
// config value — to a Mailbox, accepting the usual aliases and any casing.
func ParseMailbox(name string) (Mailbox, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case string(MailboxInbox):
		return MailboxInbox, true
	case string(MailboxSent):
		return MailboxSent, true
	case "draft", string(MailboxDrafts):
		return MailboxDrafts, true
	case string(MailboxArchive):
		return MailboxArchive, true
	case string(MailboxTrash):
		return MailboxTrash, true
	case string(MailboxSpam), "junk":
		return MailboxSpam, true
	case string(MailboxFlagged), "starred":
		return MailboxFlagged, true
	case string(MailboxAll):
		return MailboxAll, true
	default:
		return "", false
	}
}

// Criteria describes which messages belong to this mailbox. An empty accountID
// spans every account. The result carries no pagination — callers add their own
// Limit/Offset, and counters use it as is.
func (m Mailbox) Criteria(accountID string) SearchCriteria {
	criteria := SearchCriteria{AccountID: strings.TrimSpace(accountID)}
	no, yes := false, true

	switch m {
	case MailboxInbox:
		// The inbox holds delivered mail only: drafts, spam and trashed
		// messages have their own mailboxes.
		criteria.IsDraft = &no
		criteria.IsSpam = &no
		criteria.IsDeleted = &no
		criteria.Labels = []string{string(MailboxInbox)}
	case MailboxSent:
		criteria.IsDeleted = &no
		criteria.Labels = []string{string(MailboxSent)}
	case MailboxDrafts:
		criteria.IsDraft = &yes
		criteria.IsDeleted = &no
	case MailboxArchive:
		criteria.IsDeleted = &no
		criteria.Labels = []string{string(MailboxArchive)}
	case MailboxTrash:
		// Trash is defined by the deleted flag alone: a trashed message keeps
		// the labels it had, so it must not also appear in its old mailbox.
		criteria.IsDeleted = &yes
	case MailboxSpam:
		criteria.IsSpam = &yes
		criteria.IsDeleted = &no
	case MailboxFlagged:
		criteria.IsStarred = &yes
		criteria.IsDeleted = &no
	case MailboxAll:
		// No filter beyond the account scope.
	}
	return criteria
}

// Contains reports whether the message belongs to this mailbox, ignoring which
// account it came from. It is derived from Criteria, so a mailbox listing and
// this predicate can never disagree.
func (m Mailbox) Contains(message *Message) bool {
	return MatchesCriteria(message, m.Criteria(""))
}

// MatchesCriteria reports whether a message satisfies the criteria. Storage
// backends that cannot push the filter down to a query evaluate it here, so
// every backend answers the same question the same way.
func MatchesCriteria(message *Message, criteria SearchCriteria) bool {
	if message == nil {
		return false
	}
	if criteria.Query != "" && !matchesFreeTextQuery(message, criteria.Query) {
		return false
	}
	if criteria.Subject != "" && !containsFold(message.Subject, criteria.Subject) {
		return false
	}
	if criteria.From != "" && !containsFold(message.From, criteria.From) {
		return false
	}
	if criteria.To != "" && !containsFold(strings.Join(message.To, ","), criteria.To) {
		return false
	}
	if criteria.Body != "" && !containsFold(message.Body, criteria.Body) {
		return false
	}
	if criteria.Since != nil && message.Date.Before(*criteria.Since) {
		return false
	}
	if criteria.Before != nil && message.Date.After(*criteria.Before) {
		return false
	}
	if criteria.IsRead != nil && message.IsRead != *criteria.IsRead {
		return false
	}
	if criteria.IsSpam != nil && message.IsSpam != *criteria.IsSpam {
		return false
	}
	if criteria.IsDraft != nil && message.IsDraft != *criteria.IsDraft {
		return false
	}
	if criteria.IsStarred != nil && message.IsStarred != *criteria.IsStarred {
		return false
	}
	if criteria.IsDeleted != nil && message.IsDeleted != *criteria.IsDeleted {
		return false
	}
	if criteria.AccountID != "" && !strings.EqualFold(message.AccountID, criteria.AccountID) {
		return false
	}
	for _, label := range criteria.Labels {
		if !HasLabel(message.Labels, label) {
			return false
		}
	}
	return true
}

// HasLabel reports whether the label set contains the expected label, ignoring
// case and surrounding space.
func HasLabel(labels []string, expected string) bool {
	for _, label := range labels {
		if strings.EqualFold(strings.TrimSpace(label), strings.TrimSpace(expected)) {
			return true
		}
	}
	return false
}

// IsSystemMailboxLabel reports whether a label names a built-in mailbox rather
// than a user tag.
func IsSystemMailboxLabel(label string) bool {
	_, ok := ParseMailbox(label)
	return ok
}

func matchesFreeTextQuery(message *Message, query string) bool {
	fields := []string{
		message.Subject,
		message.From,
		strings.Join(message.To, " "),
		strings.Join(message.Cc, " "),
		message.Body,
	}
	for _, field := range fields {
		if containsFold(field, query) {
			return true
		}
	}
	return false
}

func containsFold(value, query string) bool {
	return strings.Contains(strings.ToLower(value), strings.ToLower(query))
}
