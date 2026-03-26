package file

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-faster/errors"
	coreerrors "github.com/kriuchkov/postero/internal/core/errors"
	"github.com/kriuchkov/postero/internal/core/models"
	"github.com/kriuchkov/postero/internal/core/ports"
)

type Repository struct {
	basePath string
}

func NewRepository(basePath string) (ports.MessageRepository, error) {
	if err := os.MkdirAll(basePath, 0o700); err != nil {
		return nil, err
	}

	return &Repository{
		basePath: basePath,
	}, nil
}

func (r *Repository) GetByID(ctx context.Context, id string) (*models.Message, error) {
	path := r.messagePath(id)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, coreerrors.MessageNotFound(id)
		}
		return nil, errors.Wrap(err, "read message file")
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	var message models.Message
	if err := json.Unmarshal(data, &message); err != nil {
		return nil, errors.Wrap(err, "decode message file")
	}
	return &message, nil
}

func (r *Repository) List(ctx context.Context, limit, offset int) ([]*models.Message, error) {
	messages, err := r.loadAllMessages(ctx)
	if err != nil {
		return nil, err
	}
	return paginateMessages(messages, limit, offset), nil
}

func (r *Repository) Search(ctx context.Context, criteria models.SearchCriteria) ([]*models.Message, error) {
	messages, err := r.loadAllMessages(ctx)
	if err != nil {
		return nil, err
	}

	filtered := make([]*models.Message, 0, len(messages))
	for _, message := range messages {
		if matchesCriteria(message, criteria) {
			filtered = append(filtered, message)
		}
	}

	return paginateMessages(filtered, criteria.Limit, criteria.Offset), nil
}

func (r *Repository) Save(ctx context.Context, message *models.Message) error {
	if message == nil {
		return errors.New("message is nil")
	}
	if strings.TrimSpace(message.ID) == "" {
		message.ID = "msg-" + time.Now().Format("20060102150405.000000000")
	}
	if message.Date.IsZero() {
		message.Date = time.Now()
	}
	if strings.TrimSpace(message.ThreadID) == "" {
		message.ThreadID = message.ID
	}
	message.Flags.Seen = message.IsRead
	message.Flags.Junk = message.IsSpam
	message.Flags.Draft = message.IsDraft
	message.Flags.Flagged = message.IsStarred
	message.Flags.Deleted = message.IsDeleted

	data, err := json.MarshalIndent(message, "", "  ")
	if err != nil {
		return errors.Wrap(err, "marshal message")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	path := r.messagePath(message.ID)
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0o600); err != nil {
		return errors.Wrap(err, "write temp message file")
	}
	if err := os.Rename(tempPath, path); err != nil {
		return errors.Wrap(err, "replace message file")
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if err := os.Remove(r.messagePath(id)); err != nil {
		if os.IsNotExist(err) {
			return coreerrors.MessageNotFound(id)
		}
		return errors.Wrap(err, "delete message file")
	}
	return nil
}

func (r *Repository) MarkAsRead(ctx context.Context, id string) error {
	message, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}
	message.IsRead = true
	message.Flags.Seen = true
	return r.Save(ctx, message)
}

func (r *Repository) MarkAsSpam(ctx context.Context, id string) error {
	message, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}
	message.IsSpam = true
	message.Flags.Junk = true
	return r.Save(ctx, message)
}

func (r *Repository) loadAllMessages(ctx context.Context) ([]*models.Message, error) {
	entries, err := os.ReadDir(r.basePath)
	if err != nil {
		return nil, errors.Wrap(err, "read message directory")
	}

	messages := make([]*models.Message, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		data, readErr := os.ReadFile(filepath.Join(r.basePath, entry.Name()))
		if readErr != nil {
			return nil, errors.Wrap(readErr, "read message entry")
		}
		var message models.Message
		if err := json.Unmarshal(data, &message); err != nil {
			return nil, errors.Wrap(err, "decode message entry")
		}
		messages = append(messages, &message)
	}

	sort.Slice(messages, func(left, right int) bool {
		if messages[left].Date.Equal(messages[right].Date) {
			return messages[left].ID < messages[right].ID
		}
		return messages[left].Date.After(messages[right].Date)
	})

	return messages, nil
}

func (r *Repository) messagePath(id string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(id)))
	return filepath.Join(r.basePath, hex.EncodeToString(sum[:])+".json")
}

func paginateMessages(messages []*models.Message, limit, offset int) []*models.Message {
	if offset >= len(messages) {
		return []*models.Message{}
	}
	if offset < 0 {
		offset = 0
	}
	end := len(messages)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	return append([]*models.Message{}, messages[offset:end]...)
}

func matchesCriteria(message *models.Message, criteria models.SearchCriteria) bool {
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
		if !hasLabel(message.Labels, label) {
			return false
		}
	}
	return true
}

func matchesFreeTextQuery(message *models.Message, query string) bool {
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

func hasLabel(labels []string, expected string) bool {
	for _, label := range labels {
		if strings.EqualFold(strings.TrimSpace(label), strings.TrimSpace(expected)) {
			return true
		}
	}
	return false
}
