package ports

import (
	"context"

	"github.com/kriuchkov/postero/internal/core/models"
)

// DraftAssistant defines AI-assisted draft generation.
type DraftAssistant interface {
	GenerateDraft(ctx context.Context, request models.GenerateDraftRequest) (*models.GeneratedDraft, error)
}

// PromptCompletionProvider defines a provider-neutral text completion boundary.
type PromptCompletionProvider interface {
	CompletePrompt(ctx context.Context, request models.PromptCompletionRequest) (string, error)
}

// AccountSyncer pulls remote mailboxes into the local message store.
type AccountSyncer interface {
	SyncAccounts(ctx context.Context, targets []models.SyncTarget) ([]*models.Message, error)
}

// MailboxSyncer synchronises every configured account into the local store and
// returns the resulting messages. It is the high-level capability the UI drives;
// the composition root binds it to the concrete accounts/config.
type MailboxSyncer interface {
	SyncAll(ctx context.Context) ([]*models.Message, error)
}

// MailboxSeeder loads sample messages into the local store so the app can be
// explored without a real account.
type MailboxSeeder interface {
	SeedDemo(ctx context.Context) ([]*models.Message, error)
}

// MessageRepository defines the interface for message persistence
type MessageRepository interface {
	// GetByID retrieves a message by its ID
	GetByID(ctx context.Context, id string) (*models.Message, error)

	// List retrieves messages with optional filtering
	List(ctx context.Context, limit, offset int) ([]*models.Message, error)

	// Search searches messages based on criteria
	Search(ctx context.Context, criteria models.SearchCriteria) ([]*models.Message, error)

	// Count returns how many messages match the criteria, ignoring its Limit
	// and Offset. Callers use it to report mailbox totals without paging the
	// whole mailbox into memory.
	Count(ctx context.Context, criteria models.SearchCriteria) (int, error)

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
	GetMessage(ctx context.Context, id string) (*models.Message, error)

	// ListMessages retrieves a list of messages
	ListMessages(ctx context.Context, limit, offset int) ([]*models.Message, error)

	// SearchMessages searches for messages
	SearchMessages(ctx context.Context, criteria models.SearchCriteria) ([]*models.Message, error)

	// CountMessages returns how many messages match the criteria, ignoring
	// pagination — the honest mailbox total behind a paged list.
	CountMessages(ctx context.Context, criteria models.SearchCriteria) (int, error)

	// ComposeMessage creates a new message draft
	ComposeMessage(ctx context.Context, request *models.CreateMessageRequest) (*models.Message, error)

	// SendMessage sends a message
	SendMessage(ctx context.Context, id string) error

	// DeleteMessage deletes a message
	DeleteMessage(ctx context.Context, id string) error

	// ReplyToMessage creates a reply to a message
	ReplyToMessage(ctx context.Context, messageID string, body string) (*models.Message, error)

	// ForwardMessage forwards a message
	ForwardMessage(ctx context.Context, messageID string, to []string) (*models.Message, error)

	ReplyAllToMessage(ctx context.Context, originalID string, body string) (*models.Message, error)
	UpdateDraft(ctx context.Context, id string, request *models.UpdateMessageRequest) (*models.Message, error)

	ToggleStar(ctx context.Context, id string) (*models.Message, error)
	MarkAsRead(ctx context.Context, id string) (*models.Message, error)
	ToggleDelete(ctx context.Context, id string) (*models.Message, error)
	// PushTrashMove propagates a locally trashed message to the IMAP server.
	// It is split from ToggleDelete so the (slow) network round-trip can run
	// outside the UI loop; it no-ops when the message is no longer trashed or
	// has no server copy.
	PushTrashMove(ctx context.Context, id string) (*models.Message, error)
	ArchiveMessage(ctx context.Context, id string) (*models.Message, error)
	MarkAsSpam(ctx context.Context, id string) (*models.Message, error)
	RestoreMessage(ctx context.Context, snapshot *models.Message) (*models.Message, error)
	AddLabel(ctx context.Context, id, label string) (*models.Message, error)
}

// IMAPRepository defines the interface for IMAP operations
type IMAPRepository interface {
	// Connect establishes a connection to the IMAP server
	Connect(ctx context.Context, host string, port int, username, password string, authType string, useTLS bool) error

	// Disconnect closes the IMAP connection
	Disconnect(ctx context.Context) error

	// Fetch retrieves messages from a mailbox
	Fetch(ctx context.Context, mailbox string, limit int) ([]*models.Message, error)

	// MoveToTrash moves a message (identified by its UID within mailbox) into the
	// server's trash mailbox and returns the trash mailbox name it resolved.
	MoveToTrash(ctx context.Context, mailbox string, uid uint32) (string, error)

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
