package file

import (
	"context"
	"time"

	coreerrors "github.com/kriuchkov/postero/internal/core/errors"
	"github.com/kriuchkov/postero/internal/core/models"
	"github.com/kriuchkov/postero/internal/core/ports"
)

// MockRepository implements the MessageRepository interface with in-memory test data
type MockRepository struct {
	messages map[string]*models.Message
}

// NewMockRepository creates a new mock file repository with test data
func NewMockRepository() ports.MessageRepository {
	repo := &MockRepository{
		messages: make(map[string]*models.Message),
	}
	repo.loadTestData()
	return repo
}

// loadTestData populates the repository with sample test messages
func (m *MockRepository) loadTestData() {
	now := time.Now()

	testMessages := []*models.Message{
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
	}

	for _, msg := range testMessages {
		m.messages[msg.ID] = msg
	}
}

// GetByID retrieves a message by its ID
func (m *MockRepository) GetByID(_ context.Context, id string) (*models.Message, error) {
	if msg, exists := m.messages[id]; exists {
		return msg, nil
	}
	return nil, coreerrors.MessageNotFound(id)
}

// List retrieves messages with optional filtering
func (m *MockRepository) List(_ context.Context, limit, offset int) ([]*models.Message, error) {
	var messages []*models.Message
	for _, msg := range m.messages {
		messages = append(messages, msg)
	}

	if offset >= len(messages) {
		return []*models.Message{}, nil
	}

	end := offset + limit
	if limit == 0 || end > len(messages) {
		end = len(messages)
	}

	return messages[offset:end], nil
}

// Search searches messages based on criteria
func (m *MockRepository) Search(_ context.Context, criteria models.SearchCriteria) ([]*models.Message, error) {
	var results []*models.Message

	for _, msg := range m.messages {
		match := true

		if criteria.Subject != "" && !contains(msg.Subject, criteria.Subject) {
			match = false
		}
		if criteria.From != "" && !contains(msg.From, criteria.From) {
			match = false
		}
		if criteria.Body != "" && !contains(msg.Body, criteria.Body) {
			match = false
		}

		if match {
			results = append(results, msg)
		}
	}

	return results, nil
}

// contains checks if haystack contains needle (case-sensitive substring search)
func contains(haystack, needle string) bool {
	return len(needle) == 0 || (len(haystack) > 0 && len(needle) <= len(haystack) &&
		(haystack == needle || len(needle) > 0 && indexOf(haystack, needle) >= 0))
}

// indexOf finds the index of substring in string
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := range len(substr) {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// Save persists a message
func (m *MockRepository) Save(_ context.Context, message *models.Message) error {
	if message.ID == "" {
		message.ID = "msg-" + time.Now().Format("20060102150405")
	}
	m.messages[message.ID] = message
	return nil
}

// Delete removes a message
func (m *MockRepository) Delete(_ context.Context, id string) error {
	delete(m.messages, id)
	return nil
}

// MarkAsRead marks a message as read
func (m *MockRepository) MarkAsRead(_ context.Context, id string) error {
	if msg, exists := m.messages[id]; exists {
		msg.IsRead = true
		msg.Flags.Seen = true
	}
	return nil
}

// MarkAsSpam marks a message as spam
func (m *MockRepository) MarkAsSpam(_ context.Context, id string) error {
	if msg, exists := m.messages[id]; exists {
		msg.IsSpam = true
		msg.Flags.Junk = true
	}
	return nil
}
