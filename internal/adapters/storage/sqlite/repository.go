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
	_ "github.com/mattn/go-sqlite3" // required for database/sql SQLite driver registration.

	"github.com/kriuchkov/postero/internal/core/models"
	"github.com/kriuchkov/postero/internal/core/ports"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(dbPath string) (ports.MessageRepository, error) {
	dsn := dbPath
	if !strings.Contains(dsn, "?") {
		dsn += "?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000"
	} else {
		dsn += "&_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000"
	}
	db, err := sql.Open("sqlite3", dsn)
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

	// The database holds message bodies (personal data); keep it owner-only.
	// The driver creates the file with the process umask (often world-readable).
	if dbPath != "" && dbPath != ":memory:" {
		if err := os.Chmod(dbPath, 0o600); err != nil && !os.IsNotExist(err) {
			if closeErr := db.Close(); closeErr != nil {
				return nil, errors.Wrap(closeErr, "failed to close database after chmod error")
			}
			return nil, errors.Wrap(err, "failed to restrict database file permissions")
		}
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
		flags_junk BOOLEAN,
		attachments TEXT,
		uid INTEGER DEFAULT 0,
		mailbox TEXT DEFAULT ''
	);
	`
	if _, err := r.db.ExecContext(context.Background(), query); err != nil {
		return err
	}

	// Best-effort migrations for databases created before these columns existed.
	migrations := []string{
		"ALTER TABLE messages ADD COLUMN attachments TEXT",
		"ALTER TABLE messages ADD COLUMN uid INTEGER DEFAULT 0",
		"ALTER TABLE messages ADD COLUMN mailbox TEXT DEFAULT ''",
	}
	for _, stmt := range migrations {
		if _, err := r.db.ExecContext(context.Background(), stmt); err != nil &&
			!strings.Contains(err.Error(), "duplicate column name") {
			return err
		}
	}

	// List and Search always order by date and commonly filter by account; these
	// indexes keep those queries off a full table scan as the mailbox grows.
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_messages_date ON messages(date DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_acct_date ON messages(account_id, date DESC)`,
	}
	for _, stmt := range indexes {
		if _, err := r.db.ExecContext(context.Background(), stmt); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) GetByID(ctx context.Context, id string) (*models.Message, error) {
	query := `
		SELECT id, account_id, subject, from_addr, to_addrs, cc_addrs, bcc_addrs, body, html, date, labels, thread_id,
		       is_read, is_spam, is_draft, is_starred, size,
		       flags_seen, flags_answered, flags_flagged, flags_draft, flags_deleted, flags_junk, uid, mailbox, attachments
		FROM messages WHERE id = ?`

	row := r.db.QueryRowContext(ctx, query, id)
	return r.scanMessage(row)
}

func (r *Repository) List(ctx context.Context, limit, offset int) ([]*models.Message, error) {
	// Attachment blobs are intentionally excluded from list views; use GetByID
	// to load a full message.
	query := `
		SELECT id, account_id, subject, from_addr, to_addrs, cc_addrs, bcc_addrs, body, html, date, labels, thread_id,
		       is_read, is_spam, is_draft, is_starred, size,
		       flags_seen, flags_answered, flags_flagged, flags_draft, flags_deleted, flags_junk, uid, mailbox
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
		msg, err := r.scanMessageSummary(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return messages, nil
}

func (r *Repository) Search(ctx context.Context, criteria models.SearchCriteria) ([]*models.Message, error) {
	// Attachment blobs are intentionally excluded from search results; use
	// GetByID to load a full message.
	query := `
		SELECT id, account_id, subject, from_addr, to_addrs, cc_addrs, bcc_addrs, body, html, date, labels, thread_id,
		       is_read, is_spam, is_draft, is_starred, size,
		       flags_seen, flags_answered, flags_flagged, flags_draft, flags_deleted, flags_junk, uid, mailbox
		FROM messages
		WHERE 1=1
	`
	var args []any

	if criteria.Query != "" {
		query += " AND (subject LIKE ? OR from_addr LIKE ? OR to_addrs LIKE ? OR body LIKE ?)"
		for range 4 {
			args = append(args, "%"+criteria.Query+"%")
		}
	}

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
	var querySb303 strings.Builder
	for _, label := range criteria.Labels {
		querySb303.WriteString(" AND labels LIKE ?")
		args = append(args, "%\""+label+"\"%")
	}
	// Only constant fragments with ? placeholders are concatenated here; the label
	// values are bound via args, so there is no SQL injection surface.
	query += querySb303.String() //nolint:gosec // G202: parameterized, constant fragments only

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
		msg, err := r.scanMessageSummary(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return messages, nil
}

func (r *Repository) Save(ctx context.Context, msg *models.Message) error {
	return saveMessage(ctx, r.db, msg)
}

// SaveBatch persists many messages in a single transaction, so a full sync pays
// one fsync instead of one per message.
func (r *Repository) SaveBatch(ctx context.Context, messages []*models.Message) error {
	if len(messages) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "begin batch save transaction")
	}
	for _, msg := range messages {
		if err := saveMessage(ctx, tx, msg); err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				return errors.Wrapf(err, "save message in batch (rollback failed: %v)", rbErr)
			}
			return errors.Wrap(err, "save message in batch")
		}
	}
	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "commit batch save")
	}
	return nil
}

// execContext is satisfied by both *sql.DB and *sql.Tx, letting saveMessage run
// standalone or inside a transaction.
type execContext interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func saveMessage(ctx context.Context, exec execContext, msg *models.Message) error {
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
			flags_seen, flags_answered, flags_flagged, flags_draft, flags_deleted, flags_junk, attachments,
			uid, mailbox
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
			flags_junk=excluded.flags_junk,
			attachments=COALESCE(excluded.attachments, messages.attachments),
			uid=excluded.uid,
			mailbox=excluded.mailbox
	`

	toAddrs, _ := json.Marshal(msg.To)
	ccAddrs, _ := json.Marshal(msg.Cc)
	bccAddrs, _ := json.Marshal(msg.Bcc)
	labels, _ := json.Marshal(msg.Labels)
	// nil means "attachments unknown" (e.g. a summary loaded without blobs) and
	// must not wipe stored data; an empty non-nil slice explicitly clears it.
	var attachments any
	if msg.Attachments != nil {
		encoded, _ := json.Marshal(msg.Attachments)
		attachments = string(encoded)
	}

	// Sync flags before save
	msg.Flags.Deleted = msg.IsDeleted

	_, err := exec.ExecContext(
		ctx,
		query,
		msg.ID,
		msg.AccountID,
		msg.Subject,
		msg.From,
		string(toAddrs),
		string(ccAddrs),
		string(bccAddrs),
		msg.Body,
		msg.HTML,
		msg.Date,
		string(labels),
		msg.ThreadID,
		msg.IsRead,
		msg.IsSpam,
		msg.IsDraft,
		msg.IsStarred,
		msg.Size,
		msg.Flags.Seen,
		msg.Flags.Answered,
		msg.Flags.Flagged,
		msg.Flags.Draft,
		msg.Flags.Deleted,
		msg.Flags.Junk,
		attachments,
		msg.UID,
		msg.Mailbox,
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
	Scan(dest ...any) error
}

// scanMessage reads a full row, including attachment blobs.
func (r *Repository) scanMessage(row scanner) (*models.Message, error) {
	var attachments sql.NullString
	msg, err := r.scanMessageInto(row, &attachments)
	if err != nil {
		return nil, err
	}
	if attachments.Valid && attachments.String != "" {
		_ = json.Unmarshal([]byte(attachments.String), &msg.Attachments)
	}
	return msg, nil
}

// scanMessageSummary reads a row selected without the attachments column.
func (r *Repository) scanMessageSummary(row scanner) (*models.Message, error) {
	return r.scanMessageInto(row)
}

func (r *Repository) scanMessageInto(row scanner, extra ...any) (*models.Message, error) {
	var msg models.Message
	var toAddrs, ccAddrs, bccAddrs, labels []byte

	dest := []any{
		&msg.ID,
		&msg.AccountID,
		&msg.Subject,
		&msg.From,
		&toAddrs,
		&ccAddrs,
		&bccAddrs,
		&msg.Body,
		&msg.HTML,
		&msg.Date,
		&labels,
		&msg.ThreadID,
		&msg.IsRead,
		&msg.IsSpam,
		&msg.IsDraft,
		&msg.IsStarred,
		&msg.Size,
		&msg.Flags.Seen,
		&msg.Flags.Answered,
		&msg.Flags.Flagged,
		&msg.Flags.Draft,
		&msg.Flags.Deleted,
		&msg.Flags.Junk,
		&msg.UID,
		&msg.Mailbox,
	}
	dest = append(dest, extra...)

	if err := row.Scan(dest...); err != nil {
		return nil, err
	}

	_ = json.Unmarshal(toAddrs, &msg.To)
	_ = json.Unmarshal(ccAddrs, &msg.Cc)
	_ = json.Unmarshal(bccAddrs, &msg.Bcc)
	_ = json.Unmarshal(labels, &msg.Labels)

	// Sync generic fields with flags
	msg.IsDeleted = msg.Flags.Deleted

	return &msg, nil
}
