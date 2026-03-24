package message

import (
	"context"
	"strings"
	"time"

	coreerrors "github.com/kriuchkov/postero/internal/core/errors"
	"github.com/kriuchkov/postero/internal/core/models"
	"github.com/kriuchkov/postero/internal/core/ports"
	"github.com/kriuchkov/postero/pkg/compose"
)

type Service struct {
	repository  ports.MessageRepository
	smtpFactory func(accountID string) (ports.SMTPRepository, error)
}

func NewService(repository ports.MessageRepository) *Service {
	return &Service{repository: repository}
}

func NewServiceWithSMTP(repository ports.MessageRepository, smtpFactory func(accountID string) (ports.SMTPRepository, error)) *Service {
	return &Service{repository: repository, smtpFactory: smtpFactory}
}

func (s *Service) GetMessage(ctx context.Context, id string) (*models.MessageDTO, error) {
	msg, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toMessageDTO(msg), nil
}

func (s *Service) ListMessages(ctx context.Context, limit, offset int) ([]*models.MessageDTO, error) {
	messages, err := s.repository.List(ctx, limit, offset)
	if err != nil {
		return nil, err
	}

	dtos := make([]*models.MessageDTO, len(messages))
	for i, msg := range messages {
		dtos[i] = toMessageDTO(msg)
	}
	return dtos, nil
}

func (s *Service) SearchMessages(ctx context.Context, criteria models.SearchCriteria) ([]*models.MessageDTO, error) {
	messages, err := s.repository.Search(ctx, criteria)
	if err != nil {
		return nil, err
	}

	dtos := make([]*models.MessageDTO, len(messages))
	for i, msg := range messages {
		dtos[i] = toMessageDTO(msg)
	}
	return dtos, nil
}

func (s *Service) ComposeMessage(ctx context.Context, request *models.CreateMessageRequest) (*models.MessageDTO, error) {
	msg := &models.Message{
		AccountID: request.AccountID,
		Subject:   request.Subject,
		From:      request.From,
		To:        request.To,
		Cc:        request.Cc,
		Bcc:       request.Bcc,
		Body:      request.Body,
		HTML:      request.HTML,
		Date:      time.Now(),
		Labels:    addUniqueLabels(request.Labels, "draft"),
		IsDraft:   true,
	}
	msg.Flags.Draft = true

	if err := s.repository.Save(ctx, msg); err != nil {
		return nil, err
	}
	return toMessageDTO(msg), nil
}

func (s *Service) SendMessage(ctx context.Context, id string) error {
	msg, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if msg == nil {
		return coreerrors.MessageNotFound(id)
	}
	if s.smtpFactory != nil {
		smtpRepo, err := s.smtpFactory(msg.AccountID)
		if err != nil {
			return err
		}
		if smtpRepo != nil {
			defer smtpRepo.Disconnect(ctx) //nolint:errcheck // best-effort disconnect after send.
			if err := smtpRepo.Send(ctx, msg); err != nil {
				return err
			}
		}
	}

	msg.IsDraft = false
	msg.Flags.Draft = false
	msg.IsRead = true
	msg.Date = time.Now()
	msg.Labels = filterLabels(msg.Labels, "draft")
	msg.Labels = addUniqueLabels(msg.Labels, "sent")
	return s.repository.Save(ctx, msg)
}

// DeleteMessage deletes a message.
func (s *Service) DeleteMessage(ctx context.Context, id string) error {
	return s.repository.Delete(ctx, id)
}

// ReplyToMessage creates a reply draft.
func (s *Service) ReplyToMessage(ctx context.Context, messageID string, body string) (*models.MessageDTO, error) {
	original, err := s.repository.GetByID(ctx, messageID)
	if err != nil {
		return nil, err
	}
	if original == nil {
		return nil, coreerrors.MessageNotFound(messageID)
	}

	draft := compose.BuildReply(original, compose.ReplyOptions{Body: body})
	reply := &models.Message{
		AccountID: original.AccountID,
		Subject:   draft.Subject,
		To:        draft.To,
		Cc:        draft.Cc,
		Body:      draft.Body,
		ThreadID:  draft.ThreadID,
		Date:      time.Now(),
		Labels:    []string{"draft"},
		IsDraft:   true,
	}
	reply.Flags.Draft = true

	if err := s.repository.Save(ctx, reply); err != nil {
		return nil, err
	}
	return toMessageDTO(reply), nil
}

// ForwardMessage creates a forward draft.
func (s *Service) ForwardMessage(ctx context.Context, messageID string, to []string) (*models.MessageDTO, error) {
	original, err := s.repository.GetByID(ctx, messageID)
	if err != nil {
		return nil, err
	}
	if original == nil {
		return nil, coreerrors.MessageNotFound(messageID)
	}

	draft := compose.BuildForward(original, to, "")
	forward := &models.Message{
		AccountID: original.AccountID,
		Subject:   draft.Subject,
		To:        draft.To,
		Body:      draft.Body,
		HTML:      original.HTML,
		ThreadID:  draft.ThreadID,
		Date:      time.Now(),
		Labels:    []string{"draft"},
		IsDraft:   true,
	}
	forward.Flags.Draft = true

	if err := s.repository.Save(ctx, forward); err != nil {
		return nil, err
	}
	return toMessageDTO(forward), nil
}

// GetAllInboxes retrieves inbox messages.
func (s *Service) GetAllInboxes(ctx context.Context, limit, offset int) ([]*models.MessageDTO, error) {
	isDraft := false
	isSpam := false
	isDeleted := false
	return s.SearchMessages(ctx, models.SearchCriteria{
		IsDraft:   &isDraft,
		IsSpam:    &isSpam,
		IsDeleted: &isDeleted,
		Labels:    []string{"inbox"},
		Limit:     limit,
		Offset:    offset,
	})
}

// GetFlagged retrieves starred messages.
func (s *Service) GetFlagged(ctx context.Context, limit, offset int) ([]*models.MessageDTO, error) {
	isStarred := true
	return s.SearchMessages(ctx, models.SearchCriteria{
		IsStarred: &isStarred,
		Limit:     limit,
		Offset:    offset,
	})
}

// GetDrafts retrieves draft messages.
func (s *Service) GetDrafts(ctx context.Context, limit, offset int) ([]*models.MessageDTO, error) {
	isDraft := true
	isDeleted := false
	return s.SearchMessages(ctx, models.SearchCriteria{
		IsDraft:   &isDraft,
		IsDeleted: &isDeleted,
		Limit:     limit,
		Offset:    offset,
	})
}

// GetSent retrieves sent messages.
func (s *Service) GetSent(ctx context.Context, limit, offset int) ([]*models.MessageDTO, error) {
	isDeleted := false
	return s.SearchMessages(ctx, models.SearchCriteria{
		Labels:    []string{"sent"},
		IsDeleted: &isDeleted,
		Limit:     limit,
		Offset:    offset,
	})
}

// GetByLabel retrieves messages by label.
func (s *Service) GetByLabel(ctx context.Context, label string, limit, offset int) ([]*models.MessageDTO, error) {
	isDeleted := false
	return s.SearchMessages(ctx, models.SearchCriteria{
		Labels:    []string{label},
		IsDeleted: &isDeleted,
		Limit:     limit,
		Offset:    offset,
	})
}

// UpdateDraft updates a draft message.
func (s *Service) UpdateDraft(ctx context.Context, id string, request *models.UpdateMessageRequest) (*models.MessageDTO, error) {
	msg, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if msg == nil {
		return nil, coreerrors.MessageNotFound(id)
	}

	if request.Subject != nil {
		msg.Subject = *request.Subject
	}
	if request.AccountID != nil {
		msg.AccountID = *request.AccountID
	}
	if request.From != nil {
		msg.From = *request.From
	}
	if request.Body != nil {
		msg.Body = *request.Body
	}
	if request.To != nil {
		msg.To = *request.To
	}
	if request.Cc != nil {
		msg.Cc = *request.Cc
	}
	if request.Bcc != nil {
		msg.Bcc = *request.Bcc
	}
	msg.Date = time.Now()

	if err := s.repository.Save(ctx, msg); err != nil {
		return nil, err
	}
	return toMessageDTO(msg), nil
}

// ReplyAllToMessage creates a reply-all draft.
func (s *Service) ReplyAllToMessage(ctx context.Context, originalID string, body string) (*models.MessageDTO, error) {
	original, err := s.repository.GetByID(ctx, originalID)
	if err != nil {
		return nil, err
	}
	if original == nil {
		return nil, coreerrors.MessageNotFound(originalID)
	}

	draft := compose.BuildReply(original, compose.ReplyOptions{ReplyAll: true, Body: body})
	reply := &models.Message{
		AccountID: original.AccountID,
		Subject:   draft.Subject,
		To:        draft.To,
		Cc:        draft.Cc,
		Body:      draft.Body,
		ThreadID:  draft.ThreadID,
		Date:      time.Now(),
		Labels:    []string{"draft"},
		IsDraft:   true,
	}
	reply.Flags.Draft = true

	if err := s.repository.Save(ctx, reply); err != nil {
		return nil, err
	}
	return toMessageDTO(reply), nil
}

// ToggleStar toggles the starred status.
func (s *Service) ToggleStar(ctx context.Context, id string) (*models.MessageDTO, error) {
	msg, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if msg == nil {
		return nil, coreerrors.MessageNotFound(id)
	}
	msg.IsStarred = !msg.IsStarred
	if err := s.repository.Save(ctx, msg); err != nil {
		return nil, err
	}
	return toMessageDTO(msg), nil
}

// MarkAsRead marks a message as read.
func (s *Service) MarkAsRead(ctx context.Context, id string) (*models.MessageDTO, error) {
	msg, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if msg == nil {
		return nil, coreerrors.MessageNotFound(id)
	}
	if msg.IsRead {
		return toMessageDTO(msg), nil
	}
	msg.IsRead = true
	msg.Flags.Seen = true
	if err := s.repository.Save(ctx, msg); err != nil {
		return nil, err
	}
	return toMessageDTO(msg), nil
}

// ToggleDelete toggles the deleted status.
func (s *Service) ToggleDelete(ctx context.Context, id string) (*models.MessageDTO, error) {
	msg, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if msg == nil {
		return nil, coreerrors.MessageNotFound(id)
	}
	msg.IsDeleted = !msg.IsDeleted
	if err := s.repository.Save(ctx, msg); err != nil {
		return nil, err
	}
	return toMessageDTO(msg), nil
}

// ArchiveMessage removes a message from inbox and marks it as archived.
func (s *Service) ArchiveMessage(ctx context.Context, id string) (*models.MessageDTO, error) {
	msg, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if msg == nil {
		return nil, coreerrors.MessageNotFound(id)
	}
	msg.Labels = filterLabels(msg.Labels, "inbox")
	msg.Labels = addUniqueLabels(msg.Labels, "archive")
	msg.IsRead = true
	msg.Flags.Seen = true
	if err := s.repository.Save(ctx, msg); err != nil {
		return nil, err
	}
	return toMessageDTO(msg), nil
}

// MarkAsSpam marks a message as spam and removes it from inbox.
func (s *Service) MarkAsSpam(ctx context.Context, id string) (*models.MessageDTO, error) {
	msg, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if msg == nil {
		return nil, coreerrors.MessageNotFound(id)
	}
	msg.IsSpam = true
	msg.Flags.Junk = true
	msg.Labels = filterLabels(msg.Labels, "inbox")
	msg.Labels = addUniqueLabels(msg.Labels, "spam")
	if err := s.repository.Save(ctx, msg); err != nil {
		return nil, err
	}
	return toMessageDTO(msg), nil
}

// RestoreMessage restores a message snapshot after an undo operation.
func (s *Service) RestoreMessage(ctx context.Context, snapshot *models.MessageDTO) (*models.MessageDTO, error) {
	if snapshot == nil {
		return nil, coreerrors.SnapshotNil()
	}

	message := &models.Message{
		ID:        snapshot.ID,
		AccountID: snapshot.AccountID,
		Subject:   snapshot.Subject,
		From:      snapshot.From,
		To:        append([]string{}, snapshot.To...),
		Cc:        append([]string{}, snapshot.Cc...),
		Bcc:       append([]string{}, snapshot.Bcc...),
		Body:      snapshot.Body,
		HTML:      snapshot.HTML,
		Date:      snapshot.Date,
		Labels:    append([]string{}, snapshot.Labels...),
		ThreadID:  snapshot.ThreadID,
		IsRead:    snapshot.IsRead,
		IsSpam:    snapshot.IsSpam,
		IsDraft:   snapshot.IsDraft,
		IsStarred: snapshot.IsStarred,
		IsDeleted: snapshot.IsDeleted,
		Size:      snapshot.Size,
		Flags: models.MessageFlags{
			Seen:    snapshot.IsRead,
			Flagged: snapshot.IsStarred,
			Draft:   snapshot.IsDraft,
			Deleted: snapshot.IsDeleted,
			Junk:    snapshot.IsSpam,
		},
	}

	if err := s.repository.Save(ctx, message); err != nil {
		return nil, err
	}
	return toMessageDTO(message), nil
}

// AddLabel adds a label to a message.
func (s *Service) AddLabel(ctx context.Context, id, label string) (*models.MessageDTO, error) {
	msg, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if msg == nil {
		return nil, coreerrors.MessageNotFound(id)
	}
	msg.Labels = addUniqueLabels(msg.Labels, label)
	if err := s.repository.Save(ctx, msg); err != nil {
		return nil, err
	}
	return toMessageDTO(msg), nil
}

func toMessageDTO(msg *models.Message) *models.MessageDTO {
	if msg == nil {
		return nil
	}

	result := &models.MessageDTO{
		ID:        msg.ID,
		AccountID: msg.AccountID,
		Subject:   msg.Subject,
		From:      msg.From,
		To:        append([]string{}, msg.To...),
		Cc:        msg.Cc,
		Bcc:       msg.Bcc,
		Body:      msg.Body,
		HTML:      msg.HTML,
		Date:      msg.Date,
		IsRead:    msg.IsRead,
		IsSpam:    msg.IsSpam,
		IsDraft:   msg.IsDraft,
		IsStarred: msg.IsStarred,
		IsDeleted: msg.IsDeleted,
		Labels:    msg.Labels,
		ThreadID:  msg.ThreadID,
		Size:      msg.Size,
	}

	if len(msg.Attachments) > 0 {
		for _, att := range msg.Attachments {
			dtoAtt := models.AttachmentDTO{
				Filename: att.Filename,
				Size:     att.Size,
				MimeType: att.MimeType,
				Data:     att.Data,
			}
			result.Attachments = append(result.Attachments, dtoAtt)
		}
	}

	return result
}

func addUniqueLabels(existing []string, labels ...string) []string {
	seen := make(map[string]struct{}, len(existing))
	result := append([]string{}, existing...)
	for _, label := range existing {
		seen[strings.ToLower(label)] = struct{}{}
	}
	for _, label := range labels {
		key := strings.ToLower(strings.TrimSpace(label))
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, label)
	}
	return result
}

func filterLabels(labels []string, remove string) []string {
	filtered := make([]string, 0, len(labels))
	for _, label := range labels {
		if strings.EqualFold(label, remove) {
			continue
		}
		filtered = append(filtered, label)
	}
	return filtered
}
