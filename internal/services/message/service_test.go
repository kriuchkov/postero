package message

import (
	"context"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	coreerrors "github.com/kriuchkov/postero/internal/core/errors"
	"github.com/kriuchkov/postero/internal/core/models"
	"github.com/kriuchkov/postero/internal/core/ports"
	"github.com/kriuchkov/postero/internal/core/ports/mocks"
)

type smtpStub struct {
	sent []*models.Message
}

func (s *smtpStub) Connect(_ context.Context, _ string, _ int, _ string, _ string, _ string, _ bool) error {
	return nil
}

func (s *smtpStub) Disconnect(_ context.Context) error {
	return nil
}

func (s *smtpStub) Send(_ context.Context, message *models.Message) error {
	s.sent = append(s.sent, message)
	return nil
}

func (s *smtpStub) IsConnected() bool {
	return true
}

func TestComposeMessage(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)

	repo.On("Save", context.Background(), mock.MatchedBy(func(message *models.Message) bool {
		return message != nil && message.Subject == "Hello" && message.From == "sender@example.com" && message.IsDraft
	})).Return(nil)

	svc := NewService(repo)
	msg, err := svc.ComposeMessage(context.Background(), &models.CreateMessageRequest{
		AccountID: "personal",
		From:      "sender@example.com",
		To:        []string{"recipient@example.com"},
		Subject:   "Hello",
		Body:      "Body",
	})

	require.NoError(t, err)
	assert.NotNil(t, msg)
	assert.Equal(t, "Hello", msg.Subject)
	assert.Equal(t, "sender@example.com", msg.From)
	assert.True(t, msg.IsDraft)
}

func TestGetMessage(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)

	expectedMessage := &models.Message{
		ID:      "1",
		Subject: "Test",
		From:    "test@example.com",
		To:      []string{"recipient@example.com"},
	}

	repo.On("GetByID", context.Background(), "1").Return(expectedMessage, nil)

	svc := NewService(repo)
	msg, err := svc.GetMessage(context.Background(), "1")

	require.NoError(t, err)
	assert.Equal(t, "Test", msg.Subject)
}

func TestGetMessagePreservesCoreFields(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)

	expectedMessage := &models.Message{
		ID:        "message-1",
		AccountID: "personal",
		Subject:   "Test",
		From:      "test@example.com",
		To:        []string{"recipient@example.com"},
		Labels:    []string{"inbox"},
		IsDeleted: true,
	}

	repo.On("GetByID", context.Background(), "message-1").Return(expectedMessage, nil)

	svc := NewService(repo)
	msg, err := svc.GetMessage(context.Background(), "message-1")

	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.Equal(t, "message-1", msg.ID)
	assert.Equal(t, "personal", msg.AccountID)
	assert.Equal(t, "test@example.com", msg.From)
	assert.Equal(t, []string{"recipient@example.com"}, msg.To)
	assert.Equal(t, []string{"inbox"}, msg.Labels)
	assert.True(t, msg.IsDeleted)
}

func TestSendMessageUsesSMTPTransport(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	messageModel := &models.Message{
		ID:        "draft-1",
		AccountID: "personal",
		From:      "sender@example.com",
		To:        []string{"recipient@example.com"},
		Subject:   "Hello",
		Body:      "Body",
		IsDraft:   true,
		Labels:    []string{"draft"},
	}
	repo.On("GetByID", context.Background(), "draft-1").Return(messageModel, nil)
	repo.On("Save", context.Background(), mock.MatchedBy(func(message *models.Message) bool {
		return message != nil && message.ID == "draft-1" && !message.IsDraft && containsLabel(message.Labels, "sent")
	})).Return(nil)

	smtpRepo := &smtpStub{}
	svc := NewServiceWithSMTP(repo, func(accountID string) (ports.SMTPRepository, error) {
		assert.Equal(t, "personal", accountID)
		return smtpRepo, nil
	})

	err := svc.SendMessage(context.Background(), "draft-1")

	require.NoError(t, err)
	assert.Len(t, smtpRepo.sent, 1)
	assert.Equal(t, "draft-1", smtpRepo.sent[0].ID)
}

func TestSendMessageReturnsMessageNotFoundDomainError(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	repo.On("GetByID", context.Background(), "missing").Return((*models.Message)(nil), nil)

	svc := NewService(repo)
	err := svc.SendMessage(context.Background(), "missing")

	require.Error(t, err)
	require.ErrorIs(t, err, coreerrors.ErrMessageNotFound)
	assert.EqualError(t, err, "message missing not found: message not found")
}

func TestArchiveMessage(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	messageModel := &models.Message{
		ID:      "msg-1",
		Labels:  []string{"inbox", "important"},
		IsRead:  false,
		Flags:   models.MessageFlags{},
		Subject: "Hello",
	}
	repo.On("GetByID", context.Background(), "msg-1").Return(messageModel, nil)
	repo.On("Save", context.Background(), mock.MatchedBy(func(message *models.Message) bool {
		return message != nil && containsLabel(message.Labels, "archive") && !containsLabel(message.Labels, "inbox") && message.IsRead &&
			message.Flags.Seen
	})).Return(nil)

	svc := NewService(repo)
	msg, err := svc.ArchiveMessage(context.Background(), "msg-1")

	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.Contains(t, msg.Labels, "archive")
	assert.NotContains(t, msg.Labels, "inbox")
}

func TestMarkAsRead(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	messageModel := &models.Message{
		ID:      "msg-1",
		IsRead:  false,
		Flags:   models.MessageFlags{},
		Subject: "Hello",
	}
	repo.On("GetByID", context.Background(), "msg-1").Return(messageModel, nil)
	repo.On("Save", context.Background(), mock.MatchedBy(func(message *models.Message) bool {
		return message != nil && message.IsRead && message.Flags.Seen
	})).Return(nil)

	svc := NewService(repo)
	msg, err := svc.MarkAsRead(context.Background(), "msg-1")

	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.True(t, msg.IsRead)
}

func TestMarkAsSpam(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	messageModel := &models.Message{
		ID:      "msg-1",
		Labels:  []string{"inbox", "important"},
		Subject: "Hello",
	}
	repo.On("GetByID", context.Background(), "msg-1").Return(messageModel, nil)
	repo.On("Save", context.Background(), mock.MatchedBy(func(message *models.Message) bool {
		return message != nil && message.IsSpam && message.Flags.Junk && containsLabel(message.Labels, "spam") &&
			!containsLabel(message.Labels, "inbox")
	})).Return(nil)

	svc := NewService(repo)
	msg, err := svc.MarkAsSpam(context.Background(), "msg-1")

	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.True(t, msg.IsSpam)
	assert.Contains(t, msg.Labels, "spam")
	assert.NotContains(t, msg.Labels, "inbox")
}

func TestRestoreMessagePersistsSnapshotState(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	repo.On("Save", context.Background(), mock.MatchedBy(func(message *models.Message) bool {
		return message != nil &&
			message.ID == "msg-1" &&
			!message.IsDeleted &&
			!message.IsSpam &&
			containsLabel(message.Labels, "inbox") &&
			!containsLabel(message.Labels, "archive")
	})).Return(nil)

	svc := NewService(repo)
	msg, err := svc.RestoreMessage(context.Background(), &models.Message{
		ID:        "msg-1",
		AccountID: "personal",
		Subject:   "Hello",
		From:      "sender@example.com",
		To:        []string{"me@example.com"},
		Body:      "Body",
		Labels:    []string{"inbox"},
		IsRead:    true,
		IsSpam:    false,
		IsDeleted: false,
	})

	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.False(t, msg.IsDeleted)
	assert.False(t, msg.IsSpam)
	assert.Contains(t, msg.Labels, "inbox")
}

func TestGetDraftsExcludesDeletedMessages(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	isDraft := true
	isDeleted := false
	repo.On("Search", context.Background(), models.SearchCriteria{
		IsDraft:   &isDraft,
		IsDeleted: &isDeleted,
		Limit:     50,
		Offset:    10,
	}).Return([]*models.Message{}, nil)

	svc := NewService(repo)
	_, err := svc.GetDrafts(context.Background(), 50, 10)

	require.NoError(t, err)
}

func TestGetSentExcludesDeletedMessages(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	isDeleted := false
	repo.On("Search", context.Background(), models.SearchCriteria{
		Labels:    []string{"sent"},
		IsDeleted: &isDeleted,
		Limit:     50,
		Offset:    10,
	}).Return([]*models.Message{}, nil)

	svc := NewService(repo)
	_, err := svc.GetSent(context.Background(), 50, 10)

	require.NoError(t, err)
}

func TestGetByLabelExcludesDeletedMessages(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	isDeleted := false
	repo.On("Search", context.Background(), models.SearchCriteria{
		Labels:    []string{"archive"},
		IsDeleted: &isDeleted,
		Limit:     50,
		Offset:    10,
	}).Return([]*models.Message{}, nil)

	svc := NewService(repo)
	_, err := svc.GetByLabel(context.Background(), "archive", 50, 10)

	require.NoError(t, err)
}

func containsLabel(labels []string, expected string) bool {
	return slices.Contains(labels, expected)
}
