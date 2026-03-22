package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-faster/errors"
	"github.com/kriuchkov/postero/internal/core/models"
	"github.com/kriuchkov/postero/internal/core/ports"
	_ "github.com/mattn/go-sqlite3"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(dbPath string) (ports.MessageRepository, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open database")
	}

	repo := &Repository{db: db}
	if err := repo.initSchema(); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, errors.Wrap(closeErr, "failed to close database after schema init error")
		}
		return nil, errors.Wrap(err, "failed to init schema")
	}

	if err := repo.seedTestData(); err != nil {
		// Just log error to stdout for now as we don't have logger passed in
		_, _ = fmt.Fprintf(os.Stderr, "Failed to seed test data: %v\n", err)
	}

	return repo, nil
}

func (r *Repository) initSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS messages (
		id TEXT PRIMARY KEY,
		account_id TEXT,
		subject TEXT,
		from_addr TEXT,
		to_addrs TEXT,
		cc_addrs TEXT,
		bcc_addrs TEXT,
		body TEXT,
		html TEXT,
		date DATETIME,
		labels TEXT,
		thread_id TEXT,
		is_read BOOLEAN,
		is_spam BOOLEAN,
		is_draft BOOLEAN,
		is_starred BOOLEAN,
		size INTEGER,
		flags_seen BOOLEAN,
		flags_answered BOOLEAN,
		flags_flagged BOOLEAN,
		flags_draft BOOLEAN,
		flags_deleted BOOLEAN,
		flags_junk BOOLEAN
	);
	`
	_, err := r.db.Exec(query)
	return err
}

func (r *Repository) seedTestData() error {
	var count int
	if err := r.db.QueryRow("SELECT COUNT(*) FROM messages").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	testMessages := []*models.Message{
		{
			ID:        "msg-001",
			AccountID: "Outlook",
			Subject:   "Christmas Special Offer | 50.8% OFF Unlimited Traffic Plan",
			From:      "noreply@911proxy.com",
			To:        []string{"nkriuchkov@outlook.com"},
			Body:      "🎄 Christmas Limited-Time Promotion offers 50.8% off Unlimited Traffic Proxy...",
			HTML:      "<p>Christmas Limited-Time Promotion...</p>",
			Date:      time.Date(2025, 12, 24, 10, 0, 0, 0, time.UTC),
			Flags:     models.MessageFlags{Seen: true},
			Labels:    []string{"inbox", "promotion"},
			IsRead:    true,
			IsStarred: false,
			Size:      2048,
		},
		{
			ID:        "msg-002",
			AccountID: "Outlook",
			Subject:   "Материалы с Golang-митапа от Lamoda Tech 🍕",
			From:      "Lamoda Tech",
			To:        []string{"nkriuchkov@outlook.com"},
			Body:      "Привет! Спасибо за интерес к митапу по Golang от Lamoda Tech, который...",
			HTML:      "<p>Привет! Спасибо...</p>",
			Date:      time.Date(2025, 05, 06, 14, 30, 0, 0, time.UTC),
			Flags:     models.MessageFlags{Flagged: true},
			Labels:    []string{"inbox", "tech", "meetup"},
			IsRead:    true,
			IsStarred: true,
			Size:      5024,
		},
		{
			ID:        "msg-003",
			AccountID: "Outlook",
			Subject:   "Собрали самые полезные материалы года от Lamoda Tech 🎄",
			From:      "Lamoda Tech",
			To:        []string{"nkriuchkov@outlook.com"},
			Body:      "Привет, на связи Lamoda Tech! Спасибо, что были с нами весь этот ярк...",
			HTML:      "<p>Привет...</p>",
			Date:      time.Date(2024, 12, 27, 9, 15, 0, 0, time.UTC),
			Flags:     models.MessageFlags{Flagged: true},
			Labels:    []string{"inbox", "tech", "digest"},
			IsRead:    true,
			IsStarred: true,
			Size:      4096,
		},
		{
			ID:        "msg-004",
			AccountID: "Outlook",
			Subject:   "Thanks for signing up, Nikita",
			From:      "italki.com",
			To:        []string{"nkriuchkov@outlook.com"},
			Body:      "Hi Nikita, Thanks for signing up! Your italki language learning journey has j...",
			HTML:      "<p>Hi Nikita...</p>",
			Date:      time.Date(2024, 12, 19, 11, 45, 0, 0, time.UTC),
			Flags:     models.MessageFlags{Flagged: true},
			Labels:    []string{"inbox", "learning"},
			IsRead:    true,
			IsStarred: true,
			Size:      1024,
		},
		{
			ID:        "msg-005",
			AccountID: "Gmail",
			Subject:   "Project Sync Meeting Notes",
			From:      "alex@postero.dev",
			To:        []string{"n2kriuchkov@gmail.com"},
			Cc:        []string{"manager@postero.dev"},
			Body:      "Here are the notes from today's sync:\n- Setup CI/CD pipeline\n- Refactor TUI components\n- Database migration plan",
			HTML:      "<ul><li>Setup CI/CD pipeline</li><li>Refactor TUI components</li><li>Database migration plan</li></ul>",
			Date:      time.Now().Add(-2 * time.Hour),
			Flags:     models.MessageFlags{Seen: false},
			Labels:    []string{"inbox", "work", "important"},
			IsRead:    false,
			IsStarred: false,
			Size:      1500,
		},
		{
			ID:        "msg-006",
			AccountID: "Outlook",
			Subject:   "Your GitHub Security Alert",
			From:      "support@github.com",
			To:        []string{"nkriuchkov@outlook.com"},
			Body:      "We found a potential security vulnerability in one of your dependencies. Please upgrade immediately.",
			HTML:      "<p>Security alert...</p>",
			Date:      time.Now().Add(-24 * time.Hour),
			Flags:     models.MessageFlags{Seen: false},
			Labels:    []string{"inbox", "github", "security"},
			IsRead:    false,
			IsStarred: false,
			Size:      800,
		},
		{
			ID:        "msg-007",
			AccountID: "Outlook",
			Subject:   "Invoice #10234 from AWS",
			From:      "no-reply@aws.amazon.com",
			To:        []string{"nkriuchkov@outlook.com"},
			Body:      "Your invoice for January 2026 is ready. Total: $0.52",
			HTML:      "<p>Invoice...</p>",
			Date:      time.Now().Add(-48 * time.Hour),
			Flags:     models.MessageFlags{Seen: true},
			Labels:    []string{"inbox", "finance", "aws"},
			IsRead:    true,
			IsStarred: false,
			Size:      3000,
		},
	}

	for _, msg := range testMessages {
		if err := r.Save(context.Background(), msg); err != nil {
			return err
		}
	}

	return nil
}

func (r *Repository) GetByID(ctx context.Context, id string) (*models.Message, error) {
	query := `
		SELECT id, account_id, subject, from_addr, to_addrs, cc_addrs, bcc_addrs, body, html, date, labels, thread_id, 
		       is_read, is_spam, is_draft, is_starred, size,
		       flags_seen, flags_answered, flags_flagged, flags_draft, flags_deleted, flags_junk
		FROM messages WHERE id = ?`

	row := r.db.QueryRowContext(ctx, query, id)
	return r.scanMessage(row)
}

func (r *Repository) List(ctx context.Context, limit, offset int) ([]*models.Message, error) {
	query := `
		SELECT id, account_id, subject, from_addr, to_addrs, cc_addrs, bcc_addrs, body, html, date, labels, thread_id, 
		       is_read, is_spam, is_draft, is_starred, size,
		       flags_seen, flags_answered, flags_flagged, flags_draft, flags_deleted, flags_junk
		FROM messages
		ORDER BY date DESC
		LIMIT ? OFFSET ?`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*models.Message
	for rows.Next() {
		msg, err := r.scanMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

func (r *Repository) Search(ctx context.Context, criteria models.SearchCriteria) ([]*models.Message, error) {
	query := `
		SELECT id, account_id, subject, from_addr, to_addrs, cc_addrs, bcc_addrs, body, html, date, labels, thread_id, 
		       is_read, is_spam, is_draft, is_starred, size,
		       flags_seen, flags_answered, flags_flagged, flags_draft, flags_deleted, flags_junk
		FROM messages
		WHERE 1=1
	`
	var args []interface{}

	if criteria.Subject != "" {
		query += " AND subject LIKE ?"
		args = append(args, "%"+criteria.Subject+"%")
	}
	if criteria.From != "" {
		query += " AND from_addr LIKE ?"
		args = append(args, "%"+criteria.From+"%")
	}
	if criteria.To != "" {
		query += " AND to_addrs LIKE ?"
		args = append(args, "%"+criteria.To+"%")
	}
	if criteria.Body != "" {
		query += " AND body LIKE ?"
		args = append(args, "%"+criteria.Body+"%")
	}
	if criteria.Since != nil {
		query += " AND date >= ?"
		args = append(args, *criteria.Since)
	}
	if criteria.Before != nil {
		query += " AND date <= ?"
		args = append(args, *criteria.Before)
	}

	if criteria.IsDraft != nil {
		query += " AND is_draft = ?"
		args = append(args, *criteria.IsDraft)
	}
	if criteria.IsStarred != nil {
		query += " AND is_starred = ?"
		args = append(args, *criteria.IsStarred)
	}
	if criteria.IsRead != nil {
		query += " AND is_read = ?"
		args = append(args, *criteria.IsRead)
	}
	if criteria.IsSpam != nil {
		query += " AND is_spam = ?"
		args = append(args, *criteria.IsSpam)
	}
	// logical deletion (flag)
	if criteria.IsDeleted != nil {
		query += " AND flags_deleted = ?"
		args = append(args, *criteria.IsDeleted)
	}
	if criteria.AccountID != "" {
		query += " AND account_id = ?"
		args = append(args, criteria.AccountID)
	}

	// Handle Labels (Naive JSON string match for now)
	// Stored as ["label1","label2"].
	for _, label := range criteria.Labels {
		query += " AND labels LIKE ?"
		args = append(args, "%\""+label+"\"%")
	}

	query += " ORDER BY date DESC"

	if criteria.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, criteria.Limit)
	}
	if criteria.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, criteria.Offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*models.Message
	for rows.Next() {
		msg, err := r.scanMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

func (r *Repository) Save(ctx context.Context, msg *models.Message) error {
	if strings.TrimSpace(msg.ID) == "" {
		msg.ID = fmt.Sprintf("msg-%d", time.Now().UnixNano())
	}
	if msg.Date.IsZero() {
		msg.Date = time.Now()
	}
	if msg.ThreadID == "" {
		msg.ThreadID = msg.ID
	}

	query := `
		INSERT INTO messages (
			id, account_id, subject, from_addr, to_addrs, cc_addrs, bcc_addrs, body, html, date, labels, thread_id, 
			is_read, is_spam, is_draft, is_starred, size,
			flags_seen, flags_answered, flags_flagged, flags_draft, flags_deleted, flags_junk
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			subject=excluded.subject,
			from_addr=excluded.from_addr,
			to_addrs=excluded.to_addrs,
			cc_addrs=excluded.cc_addrs,
			bcc_addrs=excluded.bcc_addrs,
			body=excluded.body,
			html=excluded.html,
			date=excluded.date,
			labels=excluded.labels,
			thread_id=excluded.thread_id,
			is_read=excluded.is_read,
			is_spam=excluded.is_spam,
			is_draft=excluded.is_draft,
			is_starred=excluded.is_starred,
			size=excluded.size,
			flags_seen=excluded.flags_seen,
			flags_answered=excluded.flags_answered,
			flags_flagged=excluded.flags_flagged,
			flags_draft=excluded.flags_draft,
			flags_deleted=excluded.flags_deleted,
			flags_junk=excluded.flags_junk
	`

	toAddrs, _ := json.Marshal(msg.To)
	ccAddrs, _ := json.Marshal(msg.Cc)
	bccAddrs, _ := json.Marshal(msg.Bcc)
	labels, _ := json.Marshal(msg.Labels)

	// Sync flags before save
	msg.Flags.Deleted = msg.IsDeleted

	_, err := r.db.ExecContext(ctx, query,
		msg.ID, msg.AccountID, msg.Subject, msg.From, string(toAddrs), string(ccAddrs), string(bccAddrs), msg.Body, msg.HTML, msg.Date, string(labels), msg.ThreadID,
		msg.IsRead, msg.IsSpam, msg.IsDraft, msg.IsStarred, msg.Size,
		msg.Flags.Seen, msg.Flags.Answered, msg.Flags.Flagged, msg.Flags.Draft, msg.Flags.Deleted, msg.Flags.Junk,
	)
	return err
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	query := "DELETE FROM messages WHERE id = ?"
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *Repository) MarkAsRead(ctx context.Context, id string) error {
	query := "UPDATE messages SET is_read = TRUE, flags_seen = TRUE WHERE id = ?"
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *Repository) MarkAsSpam(ctx context.Context, id string) error {
	query := "UPDATE messages SET is_spam = TRUE WHERE id = ?"
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

type scanner interface {
	Scan(dest ...interface{}) error
}

func (r *Repository) scanMessage(row scanner) (*models.Message, error) {
	var msg models.Message
	var toAddrs, ccAddrs, bccAddrs, labels string

	err := row.Scan(
		&msg.ID, &msg.AccountID, &msg.Subject, &msg.From, &toAddrs, &ccAddrs, &bccAddrs, &msg.Body, &msg.HTML, &msg.Date, &labels, &msg.ThreadID,
		&msg.IsRead, &msg.IsSpam, &msg.IsDraft, &msg.IsStarred, &msg.Size,
		&msg.Flags.Seen, &msg.Flags.Answered, &msg.Flags.Flagged, &msg.Flags.Draft, &msg.Flags.Deleted, &msg.Flags.Junk,
	)
	if err != nil {
		return nil, err
	}

	_ = json.Unmarshal([]byte(toAddrs), &msg.To)
	_ = json.Unmarshal([]byte(ccAddrs), &msg.Cc)
	_ = json.Unmarshal([]byte(bccAddrs), &msg.Bcc)
	_ = json.Unmarshal([]byte(labels), &msg.Labels)

	// Sync generic fields with flags
	msg.IsDeleted = msg.Flags.Deleted

	return &msg, nil
}
