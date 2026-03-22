package ports

import (
	"context"

	"github.com/kriuchkov/postero/internal/core/models"
)

// MessageRepository defines the interface for message persistence
type MessageRepository interface {
	// GetByID retrieves a message by its ID
	GetByID(ctx context.Context, id string) (*models.Message, error)

	// List retrieves messages with optional filtering
	List(ctx context.Context, limit, offset int) ([]*models.Message, error)

	// Search searches messages based on criteria
	Search(ctx context.Context, criteria models.SearchCriteria) ([]*models.Message, error)

	// Save persists a message
	Save(ctx context.Context, message *models.Message) error

	// Delete removes a message
	Delete(ctx context.Context, id string) error

	// MarkAsRead marks a message as read
	MarkAsRead(ctx context.Context, id string) error

	// MarkAsSpam marks a message as spam
	MarkAsSpam(ctx context.Context, id string) error
}

// MessageService defines the interface for message business logic
type MessageService interface {
	// GetMessage retrieves a message by ID
	GetMessage(ctx context.Context, id string) (*models.MessageDTO, error)

	// ListMessages retrieves a list of messages
	ListMessages(ctx context.Context, limit, offset int) ([]*models.MessageDTO, error)

	// SearchMessages searches for messages
	SearchMessages(ctx context.Context, criteria models.SearchCriteria) ([]*models.MessageDTO, error)

	// ComposeMessage creates a new message draft
	ComposeMessage(ctx context.Context, request *models.CreateMessageRequest) (*models.MessageDTO, error)

	// SendMessage sends a message
	SendMessage(ctx context.Context, id string) error

	// DeleteMessage deletes a message
	DeleteMessage(ctx context.Context, id string) error

	// ReplyToMessage creates a reply to a message
	ReplyToMessage(ctx context.Context, messageID string, body string) (*models.MessageDTO, error)

	// ForwardMessage forwards a message
	ForwardMessage(ctx context.Context, messageID string, to []string) (*models.MessageDTO, error)

	// New required methods
	GetAllInboxes(ctx context.Context, limit, offset int) ([]*models.MessageDTO, error)
	GetFlagged(ctx context.Context, limit, offset int) ([]*models.MessageDTO, error)
	GetDrafts(ctx context.Context, limit, offset int) ([]*models.MessageDTO, error)
	GetSent(ctx context.Context, limit, offset int) ([]*models.MessageDTO, error)
	GetByLabel(ctx context.Context, label string, limit, offset int) ([]*models.MessageDTO, error)

	ReplyAllToMessage(ctx context.Context, originalID string, body string) (*models.MessageDTO, error)
	UpdateDraft(ctx context.Context, id string, request *models.UpdateMessageRequest) (*models.MessageDTO, error)

	ToggleStar(ctx context.Context, id string) (*models.MessageDTO, error)
	MarkAsRead(ctx context.Context, id string) (*models.MessageDTO, error)
	ToggleDelete(ctx context.Context, id string) (*models.MessageDTO, error)
	ArchiveMessage(ctx context.Context, id string) (*models.MessageDTO, error)
	MarkAsSpam(ctx context.Context, id string) (*models.MessageDTO, error)
	RestoreMessage(ctx context.Context, snapshot *models.MessageDTO) (*models.MessageDTO, error)
	AddLabel(ctx context.Context, id, label string) (*models.MessageDTO, error)
}

// IMAPRepository defines the interface for IMAP operations
type IMAPRepository interface {
	// Connect establishes a connection to the IMAP server
	Connect(ctx context.Context, host string, port int, username, password string, authType string, useTLS bool) error

	// Disconnect closes the IMAP connection
	Disconnect(ctx context.Context) error

	// Fetch retrieves messages from a mailbox
	Fetch(ctx context.Context, mailbox string, limit int) ([]*models.Message, error)

	// IsConnected returns whether the connection is active
	IsConnected() bool
}

// SMTPRepository defines the interface for SMTP operations
type SMTPRepository interface {
	// Connect establishes a connection to the SMTP server
	Connect(ctx context.Context, host string, port int, username, password string, authType string, useTLS bool) error

	// Disconnect closes the SMTP connection
	Disconnect(ctx context.Context) error

	// Send sends an email message
	Send(ctx context.Context, message *models.Message) error

	// IsConnected returns whether the connection is active
	IsConnected() bool
}
