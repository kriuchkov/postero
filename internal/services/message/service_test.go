package message

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
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
		return message != nil &&
			message.Subject == "Hello" &&
			message.From == "sender@example.com" &&
			message.IsDraft &&
			len(message.Attachments) == 1 &&
			message.Attachments[0].Filename == "note.txt"
	})).Return(nil)

	svc := NewService(repo)
	msg, err := svc.ComposeMessage(context.Background(), &models.CreateMessageRequest{
		AccountID:   "personal",
		From:        "sender@example.com",
		To:          []string{"recipient@example.com"},
		Subject:     "Hello",
		Body:        "Body",
		Attachments: []*models.Attachment{{Filename: "note.txt", Size: 4, MimeType: "text/plain", Data: []byte("test")}},
	})

	require.NoError(t, err)
	assert.NotNil(t, msg)
	assert.Equal(t, "Hello", msg.Subject)
	assert.Equal(t, "sender@example.com", msg.From)
	assert.True(t, msg.IsDraft)
	require.Len(t, msg.Attachments, 1)
	assert.Equal(t, "note.txt", msg.Attachments[0].Filename)
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

func TestGetMessageReturnsErrorFromRepository(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	repo.On("GetByID", context.Background(), "bad").Return((*models.Message)(nil), errors.New("db error"))

	svc := NewService(repo)
	_, err := svc.GetMessage(context.Background(), "bad")

	require.Error(t, err)
	assert.EqualError(t, err, "db error")
}

func TestListMessagesReturnsClonedMessages(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	repo.On("List", context.Background(), 10, 5).Return([]*models.Message{
		{ID: "m1", Subject: "First", Labels: []string{"inbox"}},
		{ID: "m2", Subject: "Second", Labels: []string{"sent"}},
	}, nil)

	svc := NewService(repo)
	msgs, err := svc.ListMessages(context.Background(), 10, 5)

	require.NoError(t, err)
	require.Len(t, msgs, 2)
	assert.Equal(t, "m1", msgs[0].ID)
	assert.Equal(t, "m2", msgs[1].ID)
}

func TestListMessagesReturnsErrorFromRepository(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	repo.On("List", context.Background(), 10, 0).Return(([]*models.Message)(nil), errors.New("list error"))

	svc := NewService(repo)
	_, err := svc.ListMessages(context.Background(), 10, 0)

	require.Error(t, err)
	assert.EqualError(t, err, "list error")
}

func TestSearchMessagesReturnsErrorFromRepository(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	repo.On("Search", context.Background(), mock.AnythingOfType("models.SearchCriteria")).
		Return(([]*models.Message)(nil), errors.New("search error"))

	svc := NewService(repo)
	_, err := svc.SearchMessages(context.Background(), models.SearchCriteria{Query: "hello"})

	require.Error(t, err)
	assert.EqualError(t, err, "search error")
}

func TestComposeMessageReturnsErrorWhenSaveFails(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	repo.On("Save", context.Background(), mock.Anything).Return(errors.New("save failed"))

	svc := NewService(repo)
	_, err := svc.ComposeMessage(context.Background(), &models.CreateMessageRequest{
		AccountID: "personal",
		From:      "me@example.com",
		To:        []string{"you@example.com"},
		Subject:   "Hello",
		Body:      "Body",
	})

	require.Error(t, err)
	assert.EqualError(t, err, "save failed")
}

func TestSendMessageWithoutSMTPFactoryOnlySaves(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	messageModel := &models.Message{
		ID:        "draft-2",
		AccountID: "personal",
		From:      "sender@example.com",
		To:        []string{"recipient@example.com"},
		Subject:   "Hello",
		Body:      "Body",
		IsDraft:   true,
		Labels:    []string{"draft"},
	}
	repo.On("GetByID", context.Background(), "draft-2").Return(messageModel, nil)
	repo.On("Save", context.Background(), mock.MatchedBy(func(m *models.Message) bool {
		return m.ID == "draft-2" && !m.IsDraft && containsLabel(m.Labels, "sent")
	})).Return(nil)

	svc := NewService(repo)
	err := svc.SendMessage(context.Background(), "draft-2")

	require.NoError(t, err)
}

func TestSendMessageReturnsErrorWhenSMTPFactoryFails(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	repo.On("GetByID", context.Background(), "draft-3").Return(&models.Message{
		ID: "draft-3", AccountID: "personal", IsDraft: true, Labels: []string{"draft"},
	}, nil)

	svc := NewServiceWithSMTP(repo, func(_ string) (ports.SMTPRepository, error) {
		return nil, errors.New("smtp factory error")
	})
	err := svc.SendMessage(context.Background(), "draft-3")

	require.Error(t, err)
	assert.EqualError(t, err, "smtp factory error")
}

func TestDeleteMessageCallsRepository(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	repo.On("Delete", context.Background(), "msg-del").Return(nil)

	svc := NewService(repo)
	err := svc.DeleteMessage(context.Background(), "msg-del")

	require.NoError(t, err)
}

func TestDeleteMessageReturnsErrorFromRepository(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	repo.On("Delete", context.Background(), "msg-bad").Return(errors.New("delete failed"))

	svc := NewService(repo)
	err := svc.DeleteMessage(context.Background(), "msg-bad")

	require.Error(t, err)
	assert.EqualError(t, err, "delete failed")
}

func TestReplyToMessageBuildsReplyDraft(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	original := &models.Message{
		ID:       "orig-1",
		Subject:  "Hello",
		From:     "sender@example.com",
		To:       []string{"me@example.com"},
		Body:     "Original body",
		ThreadID: "thread-1",
	}
	repo.On("GetByID", context.Background(), "orig-1").Return(original, nil)
	repo.On("Save", context.Background(), mock.MatchedBy(func(m *models.Message) bool {
		return m.IsDraft && strings.Contains(m.Subject, "Re:")
	})).Return(nil)

	svc := NewService(repo)
	reply, err := svc.ReplyToMessage(context.Background(), "orig-1", "My reply")

	require.NoError(t, err)
	require.NotNil(t, reply)
	assert.Contains(t, reply.Subject, "Re:")
	assert.True(t, reply.IsDraft)
	assert.Equal(t, "thread-1", reply.ThreadID)
}

func TestReplyToMessageReturnsNotFoundError(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	repo.On("GetByID", context.Background(), "missing").Return((*models.Message)(nil), nil)

	svc := NewService(repo)
	_, err := svc.ReplyToMessage(context.Background(), "missing", "reply")

	require.Error(t, err)
	require.ErrorIs(t, err, coreerrors.ErrMessageNotFound)
}

func TestReplyToMessageReturnsRepositoryError(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	repo.On("GetByID", context.Background(), "err-id").Return((*models.Message)(nil), errors.New("repo error"))

	svc := NewService(repo)
	_, err := svc.ReplyToMessage(context.Background(), "err-id", "reply")

	require.Error(t, err)
	assert.EqualError(t, err, "repo error")
}

func TestForwardMessageBuildsForwardDraft(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	original := &models.Message{
		ID:      "orig-2",
		Subject: "Meeting",
		From:    "boss@example.com",
		To:      []string{"me@example.com"},
		Body:    "Please attend.",
	}
	repo.On("GetByID", context.Background(), "orig-2").Return(original, nil)
	repo.On("Save", context.Background(), mock.MatchedBy(func(m *models.Message) bool {
		return m.IsDraft && strings.Contains(m.Subject, "Fwd:")
	})).Return(nil)

	svc := NewService(repo)
	fwd, err := svc.ForwardMessage(context.Background(), "orig-2", []string{"colleague@example.com"})

	require.NoError(t, err)
	require.NotNil(t, fwd)
	assert.Contains(t, fwd.Subject, "Fwd:")
	assert.True(t, fwd.IsDraft)
}

func TestForwardMessageReturnsNotFoundError(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	repo.On("GetByID", context.Background(), "missing").Return((*models.Message)(nil), nil)

	svc := NewService(repo)
	_, err := svc.ForwardMessage(context.Background(), "missing", nil)

	require.Error(t, err)
	require.ErrorIs(t, err, coreerrors.ErrMessageNotFound)
}

func TestGetAllInboxesFiltersCorrectly(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	isDraft := false
	isSpam := false
	isDeleted := false
	repo.On("Search", context.Background(), models.SearchCriteria{
		IsDraft:   &isDraft,
		IsSpam:    &isSpam,
		IsDeleted: &isDeleted,
		Labels:    []string{"inbox"},
		Limit:     20,
		Offset:    0,
	}).Return([]*models.Message{{ID: "inbox-1", Labels: []string{"inbox"}}}, nil)

	svc := NewService(repo)
	msgs, err := svc.GetAllInboxes(context.Background(), 20, 0)

	require.NoError(t, err)
	require.Len(t, msgs, 1)
	assert.Equal(t, "inbox-1", msgs[0].ID)
}

func TestGetFlaggedFiltersStarred(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	isStarred := true
	repo.On("Search", context.Background(), models.SearchCriteria{
		IsStarred: &isStarred,
		Limit:     10,
		Offset:    0,
	}).Return([]*models.Message{{ID: "starred-1", IsStarred: true}}, nil)

	svc := NewService(repo)
	msgs, err := svc.GetFlagged(context.Background(), 10, 0)

	require.NoError(t, err)
	require.Len(t, msgs, 1)
	assert.Equal(t, "starred-1", msgs[0].ID)
	assert.True(t, msgs[0].IsStarred)
}

func TestUpdateDraftAppliesPartialFields(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	existing := &models.Message{
		ID:      "draft-upd",
		Subject: "Old subject",
		Body:    "Old body",
		From:    "me@example.com",
		To:      []string{"old@example.com"},
	}
	newSubject := "New subject"
	newBody := "New body"

	repo.On("GetByID", context.Background(), "draft-upd").Return(existing, nil)
	repo.On("Save", context.Background(), mock.MatchedBy(func(m *models.Message) bool {
		return m.ID == "draft-upd" && m.Subject == "New subject" && m.Body == "New body"
	})).Return(nil)

	svc := NewService(repo)
	updated, err := svc.UpdateDraft(context.Background(), "draft-upd", &models.UpdateMessageRequest{
		Subject: &newSubject,
		Body:    &newBody,
	})

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "New subject", updated.Subject)
	assert.Equal(t, "New body", updated.Body)
	assert.Equal(t, "me@example.com", updated.From)
}

func TestUpdateDraftReturnsNotFoundError(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	repo.On("GetByID", context.Background(), "missing").Return((*models.Message)(nil), nil)

	svc := NewService(repo)
	_, err := svc.UpdateDraft(context.Background(), "missing", &models.UpdateMessageRequest{})

	require.Error(t, err)
	require.ErrorIs(t, err, coreerrors.ErrMessageNotFound)
}

func TestReplyAllToMessageBuildsReplyAllDraft(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	original := &models.Message{
		ID:       "orig-3",
		Subject:  "Team update",
		From:     "boss@example.com",
		To:       []string{"me@example.com", "team@example.com"},
		Cc:       []string{"ceo@example.com"},
		Body:     "Please read.",
		ThreadID: "thread-3",
	}
	repo.On("GetByID", context.Background(), "orig-3").Return(original, nil)
	repo.On("Save", context.Background(), mock.MatchedBy(func(m *models.Message) bool {
		return m.IsDraft && strings.Contains(m.Subject, "Re:")
	})).Return(nil)

	svc := NewService(repo)
	reply, err := svc.ReplyAllToMessage(context.Background(), "orig-3", "Got it")

	require.NoError(t, err)
	require.NotNil(t, reply)
	assert.Contains(t, reply.Subject, "Re:")
	assert.True(t, reply.IsDraft)
	assert.Equal(t, "thread-3", reply.ThreadID)
}

func TestReplyAllToMessageReturnsNotFoundError(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	repo.On("GetByID", context.Background(), "missing").Return((*models.Message)(nil), nil)

	svc := NewService(repo)
	_, err := svc.ReplyAllToMessage(context.Background(), "missing", "reply")

	require.Error(t, err)
	require.ErrorIs(t, err, coreerrors.ErrMessageNotFound)
}

func TestToggleStarTogglesOnAndOff(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	msg := &models.Message{ID: "msg-star", IsStarred: false}
	repo.On("GetByID", context.Background(), "msg-star").Return(msg, nil).Once()
	repo.On("Save", context.Background(), mock.MatchedBy(func(m *models.Message) bool {
		return m.ID == "msg-star" && m.IsStarred
	})).Return(nil).Once()

	svc := NewService(repo)
	starred, err := svc.ToggleStar(context.Background(), "msg-star")

	require.NoError(t, err)
	assert.True(t, starred.IsStarred)

	msg.IsStarred = true
	repo.On("GetByID", context.Background(), "msg-star").Return(msg, nil).Once()
	repo.On("Save", context.Background(), mock.MatchedBy(func(m *models.Message) bool {
		return m.ID == "msg-star" && !m.IsStarred
	})).Return(nil).Once()

	unstarred, err := svc.ToggleStar(context.Background(), "msg-star")
	require.NoError(t, err)
	assert.False(t, unstarred.IsStarred)
}

func TestToggleStarReturnsNotFoundError(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	repo.On("GetByID", context.Background(), "missing").Return((*models.Message)(nil), nil)

	svc := NewService(repo)
	_, err := svc.ToggleStar(context.Background(), "missing")

	require.Error(t, err)
	require.ErrorIs(t, err, coreerrors.ErrMessageNotFound)
}

func TestMarkAsReadSkipsSaveForAlreadyReadMessage(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	repo.On("GetByID", context.Background(), "msg-read").Return(&models.Message{
		ID:     "msg-read",
		IsRead: true,
	}, nil)

	svc := NewService(repo)
	msg, err := svc.MarkAsRead(context.Background(), "msg-read")

	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.True(t, msg.IsRead)
	repo.AssertNotCalled(t, "Save")
}

func TestMarkAsReadReturnsNotFoundError(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	repo.On("GetByID", context.Background(), "missing").Return((*models.Message)(nil), nil)

	svc := NewService(repo)
	_, err := svc.MarkAsRead(context.Background(), "missing")

	require.Error(t, err)
	require.ErrorIs(t, err, coreerrors.ErrMessageNotFound)
}

func TestToggleDeleteTogglesOnAndOff(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	msg := &models.Message{ID: "msg-del-toggle", IsDeleted: false}
	repo.On("GetByID", context.Background(), "msg-del-toggle").Return(msg, nil).Once()
	repo.On("Save", context.Background(), mock.MatchedBy(func(m *models.Message) bool {
		return m.ID == "msg-del-toggle" && m.IsDeleted
	})).Return(nil).Once()

	svc := NewService(repo)
	deleted, err := svc.ToggleDelete(context.Background(), "msg-del-toggle")
	require.NoError(t, err)
	assert.True(t, deleted.IsDeleted)

	msg.IsDeleted = true
	repo.On("GetByID", context.Background(), "msg-del-toggle").Return(msg, nil).Once()
	repo.On("Save", context.Background(), mock.MatchedBy(func(m *models.Message) bool {
		return m.ID == "msg-del-toggle" && !m.IsDeleted
	})).Return(nil).Once()

	restored, err := svc.ToggleDelete(context.Background(), "msg-del-toggle")
	require.NoError(t, err)
	assert.False(t, restored.IsDeleted)
}

type imapStub struct {
	moved        []string // "<mailbox>/<uid>" per MoveToTrash call
	moveErr      error
	trashName    string
	disconnected bool
}

func (s *imapStub) Connect(_ context.Context, _ string, _ int, _, _ string, _ string, _ bool) error {
	return nil
}
func (s *imapStub) Disconnect(_ context.Context) error { s.disconnected = true; return nil }
func (s *imapStub) Fetch(_ context.Context, _ string, _ int) ([]*models.Message, error) {
	return nil, nil
}
func (s *imapStub) IsConnected() bool { return true }
func (s *imapStub) MoveToTrash(_ context.Context, mailbox string, uid uint32) (string, error) {
	if s.moveErr != nil {
		return "", s.moveErr
	}
	s.moved = append(s.moved, fmt.Sprintf("%s/%d", mailbox, uid))
	if s.trashName == "" {
		return "Trash", nil
	}
	return s.trashName, nil
}

func imapFactoryFor(stub *imapStub) func(string) (ports.IMAPRepository, error) {
	return func(string) (ports.IMAPRepository, error) { return stub, nil }
}

// TestToggleDeleteNeverTouchesTheServer: the toggle is the instant, local half
// of a delete — the network round-trip lives in PushTrashMove.
func TestToggleDeleteNeverTouchesTheServer(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	repo.On("GetByID", context.Background(), "m1").
		Return(&models.Message{ID: "m1", AccountID: "work", UID: 42, Mailbox: "INBOX"}, nil).Once()
	repo.On("Save", context.Background(), mock.Anything).Return(nil).Once()

	imap := &imapStub{}
	svc := NewServiceWithTransports(repo, nil, imapFactoryFor(imap))

	deleted, err := svc.ToggleDelete(context.Background(), "m1")
	require.NoError(t, err)
	assert.True(t, deleted.IsDeleted)
	assert.Empty(t, imap.moved)
}

// TestPushTrashMoveMovesOnServer: pushing a locally trashed message must move
// it on the IMAP server and record the new server location (trash mailbox, UID
// unknown → 0).
func TestPushTrashMoveMovesOnServer(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	msg := &models.Message{ID: "m1", AccountID: "work", UID: 42, Mailbox: "INBOX", IsDeleted: true}
	repo.On("GetByID", context.Background(), "m1").Return(msg, nil).Once()
	repo.On("Save", context.Background(), mock.MatchedBy(func(m *models.Message) bool {
		return m.IsDeleted && m.Mailbox == "Deleted Items" && m.UID == 0
	})).Return(nil).Once()

	imap := &imapStub{trashName: "Deleted Items"}
	svc := NewServiceWithTransports(repo, nil, imapFactoryFor(imap))

	pushed, err := svc.PushTrashMove(context.Background(), "m1")
	require.NoError(t, err)
	assert.Equal(t, "Deleted Items", pushed.Mailbox)
	assert.Zero(t, pushed.UID)
	assert.Equal(t, []string{"INBOX/42"}, imap.moved)
	assert.True(t, imap.disconnected, "the per-action IMAP connection must be closed")
}

// TestPushTrashMoveSurfacesServerFailure: a failed server move must surface as
// an error with no local Save, so the caller can report it honestly.
func TestPushTrashMoveSurfacesServerFailure(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	msg := &models.Message{ID: "m1", AccountID: "work", UID: 42, Mailbox: "INBOX", IsDeleted: true}
	repo.On("GetByID", context.Background(), "m1").Return(msg, nil).Once()

	imap := &imapStub{moveErr: errors.New("server unavailable")}
	svc := NewServiceWithTransports(repo, nil, imapFactoryFor(imap))

	_, err := svc.PushTrashMove(context.Background(), "m1")
	require.ErrorContains(t, err, "server unavailable")
	repo.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
}

// TestPushTrashMoveSkipsWhenNothingToMove: the push no-ops for messages that
// were un-trashed before it ran (the undo race) and for messages without a
// server copy (uid 0: drafts, demo mail, pre-uid rows).
func TestPushTrashMoveSkipsWhenNothingToMove(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	repo.On("GetByID", context.Background(), "undone").
		Return(&models.Message{ID: "undone", AccountID: "work", UID: 42, Mailbox: "INBOX"}, nil).Once()
	repo.On("GetByID", context.Background(), "local").
		Return(&models.Message{ID: "local", AccountID: "work", IsDeleted: true}, nil).Once()

	imap := &imapStub{}
	svc := NewServiceWithTransports(repo, nil, imapFactoryFor(imap))

	undone, err := svc.PushTrashMove(context.Background(), "undone")
	require.NoError(t, err)
	assert.False(t, undone.IsDeleted)

	local, err := svc.PushTrashMove(context.Background(), "local")
	require.NoError(t, err)
	assert.True(t, local.IsDeleted)

	assert.Empty(t, imap.moved, "no server call without a trashed message and a server copy")
	repo.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
}

func TestToggleDeleteReturnsNotFoundError(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	repo.On("GetByID", context.Background(), "missing").Return((*models.Message)(nil), nil)

	svc := NewService(repo)
	_, err := svc.ToggleDelete(context.Background(), "missing")

	require.Error(t, err)
	require.ErrorIs(t, err, coreerrors.ErrMessageNotFound)
}

func TestArchiveMessageReturnsNotFoundError(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	repo.On("GetByID", context.Background(), "missing").Return((*models.Message)(nil), nil)

	svc := NewService(repo)
	_, err := svc.ArchiveMessage(context.Background(), "missing")

	require.Error(t, err)
	require.ErrorIs(t, err, coreerrors.ErrMessageNotFound)
}

func TestMarkAsSpamReturnsNotFoundError(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	repo.On("GetByID", context.Background(), "missing").Return((*models.Message)(nil), nil)

	svc := NewService(repo)
	_, err := svc.MarkAsSpam(context.Background(), "missing")

	require.Error(t, err)
	require.ErrorIs(t, err, coreerrors.ErrMessageNotFound)
}

func TestRestoreMessageReturnsErrorForNilSnapshot(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	svc := NewService(repo)

	_, err := svc.RestoreMessage(context.Background(), nil)

	require.Error(t, err)
	require.ErrorIs(t, err, coreerrors.ErrSnapshotNil)
}

func TestAddLabelAddsNewLabelToMessage(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	repo.On("GetByID", context.Background(), "msg-lbl").Return(&models.Message{
		ID:     "msg-lbl",
		Labels: []string{"inbox"},
	}, nil)
	repo.On("Save", context.Background(), mock.MatchedBy(func(m *models.Message) bool {
		return m.ID == "msg-lbl" && containsLabel(m.Labels, "important") && containsLabel(m.Labels, "inbox")
	})).Return(nil)

	svc := NewService(repo)
	msg, err := svc.AddLabel(context.Background(), "msg-lbl", "important")

	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.Contains(t, msg.Labels, "important")
	assert.Contains(t, msg.Labels, "inbox")
}

func TestAddLabelReturnsNotFoundError(t *testing.T) {
	repo := mocks.NewMockMessageRepository(t)
	repo.On("GetByID", context.Background(), "missing").Return((*models.Message)(nil), nil)

	svc := NewService(repo)
	_, err := svc.AddLabel(context.Background(), "missing", "important")

	require.Error(t, err)
	require.ErrorIs(t, err, coreerrors.ErrMessageNotFound)
}

func TestAddUniqueLabelsDeduplicatesCaseInsensitive(t *testing.T) {
	result := addUniqueLabels([]string{"Inbox", "sent"}, "inbox", "SENT", "archive")

	assert.Equal(t, []string{"Inbox", "sent", "archive"}, result)
}

func TestFilterLabelsRemovesMatchingLabel(t *testing.T) {
	result := filterLabels([]string{"inbox", "important", "INBOX"}, "inbox")

	assert.Equal(t, []string{"important"}, result)
}
