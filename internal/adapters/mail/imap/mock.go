package imap

import (
	"context"
	"time"

	"github.com/kriuchkov/postero/internal/core/models"
	"github.com/kriuchkov/postero/internal/core/ports"
)

// MockRepository implements the IMAPRepository interface with test data
type MockRepository struct {
	connected bool
	messages  []*models.Message
}

// NewMockRepository creates a new mock IMAP repository with test data
func NewMockRepository() ports.IMAPRepository {
	return &MockRepository{
		connected: true,
		messages:  generateTestMessages(),
	}
}

// Connect establishes a mock connection
func (m *MockRepository) Connect(ctx context.Context, host string, port int, username, password string, authType string, useTLS bool) error {
	m.connected = true
	return nil
}

// Disconnect closes the mock connection
func (m *MockRepository) Disconnect(ctx context.Context) error {
	m.connected = false
	return nil
}

// Fetch retrieves mock test messages
func (m *MockRepository) Fetch(ctx context.Context, mailbox string, limit int) ([]*models.Message, error) {
	if !m.connected {
		return nil, ErrNotConnected
	}

	if limit == 0 || limit > len(m.messages) {
		return m.messages, nil
	}

	return m.messages[:limit], nil
}

// IsConnected returns whether the mock connection is active
func (m *MockRepository) IsConnected() bool {
	return m.connected
}

// generateTestMessages creates sample email messages for testing
func generateTestMessages() []*models.Message {
	now := time.Now()

	return []*models.Message{
		{
			ID:        "msg-001",
			AccountID: "personal",
			Subject:   "Welcome to Postero",
			From:      "team@postero.dev",
			To:        []string{"user@example.com"},
			Body:      "Thank you for trying Postero! This is a test email to verify your setup.",
			HTML:      "<p>Thank you for trying Postero! This is a test email to verify your setup.</p>",
			Date:      now.Add(-2 * time.Hour),
			Flags: models.MessageFlags{
				Seen:     true,
				Answered: false,
				Flagged:  false,
				Draft:    false,
				Deleted:  false,
				Junk:     false,
			},
			Labels:    []string{"important"},
			IsRead:    true,
			IsSpam:    false,
			IsDraft:   false,
			IsStarred: true,
			Size:      1024,
		},
		{
			ID:        "msg-002",
			AccountID: "personal",
			Subject:   "Quick Tips for Email Management",
			From:      "tips@example.com",
			To:        []string{"user@example.com"},
			Body:      "Here are some tips for effective email management:\n1. Use labels\n2. Archive old emails\n3. Set up filters",
			HTML:      "<ol><li>Use labels</li><li>Archive old emails</li><li>Set up filters</li></ol>",
			Date:      now.Add(-6 * time.Hour),
			Flags: models.MessageFlags{
				Seen:     true,
				Answered: false,
				Flagged:  false,
				Draft:    false,
				Deleted:  false,
				Junk:     false,
			},
			Labels:    []string{"tips"},
			IsRead:    true,
			IsSpam:    false,
			IsDraft:   false,
			IsStarred: false,
			Size:      2048,
		},
		{
			ID:        "msg-003",
			AccountID: "personal",
			Subject:   "Meeting Tomorrow at 2 PM",
			From:      "john@company.com",
			To:        []string{"user@example.com"},
			Cc:        []string{"jane@company.com"},
			Body:      "Don't forget about our meeting tomorrow at 2 PM in the conference room.",
			HTML:      "<p>Don't forget about our meeting tomorrow at 2 PM in the conference room.</p>",
			Date:      now.Add(-12 * time.Hour),
			Flags: models.MessageFlags{
				Seen:     false,
				Answered: false,
				Flagged:  true,
				Draft:    false,
				Deleted:  false,
				Junk:     false,
			},
			Labels:    []string{"work"},
			IsRead:    false,
			IsSpam:    false,
			IsDraft:   false,
			IsStarred: false,
			Size:      512,
		},
		{
			ID:        "msg-004",
			AccountID: "personal",
			Subject:   "Project Status Update",
			From:      "manager@company.com",
			To:        []string{"user@example.com"},
			Body:      "Can you provide a status update on the current project?",
			HTML:      "<p>Can you provide a status update on the current project?</p>",
			Date:      now.Add(-24 * time.Hour),
			Flags: models.MessageFlags{
				Seen:     false,
				Answered: false,
				Flagged:  false,
				Draft:    false,
				Deleted:  false,
				Junk:     false,
			},
			Labels:    []string{"work", "projects"},
			IsRead:    false,
			IsSpam:    false,
			IsDraft:   false,
			IsStarred: false,
			Size:      768,
		},
		{
			ID:        "msg-005",
			AccountID: "personal",
			Subject:   "Newsletter: January 2026",
			From:      "newsletter@opensource.org",
			To:        []string{"user@example.com"},
			Body:      "Check out the latest updates from the open source community...",
			HTML:      "<p>Check out the latest updates from the open source community...</p>",
			Date:      now.Add(-48 * time.Hour),
			Flags: models.MessageFlags{
				Seen:     true,
				Answered: false,
				Flagged:  false,
				Draft:    false,
				Deleted:  false,
				Junk:     false,
			},
			Labels:    []string{"newsletters"},
			IsRead:    true,
			IsSpam:    false,
			IsDraft:   false,
			IsStarred: false,
			Size:      4096,
		},
		{
			ID:        "msg-006",
			AccountID: "personal",
			Subject:   "Draft: Response to Client Feedback",
			From:      "user@example.com",
			To:        []string{"client@example.com"},
			Body:      "Thank you for your feedback on our proposal...",
			HTML:      "<p>Thank you for your feedback on our proposal...</p>",
			Date:      now.Add(-1 * time.Hour),
			Flags: models.MessageFlags{
				Seen:     true,
				Answered: false,
				Flagged:  false,
				Draft:    true,
				Deleted:  false,
				Junk:     false,
			},
			Labels:    []string{},
			IsRead:    true,
			IsSpam:    false,
			IsDraft:   true,
			IsStarred: false,
			Size:      1536,
		},
	}
}
