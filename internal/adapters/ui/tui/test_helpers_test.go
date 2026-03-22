package tui

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kriuchkov/postero/internal/core/models"
	"github.com/stretchr/testify/require"
)

type messageServiceStub struct {
	inbox             []*models.MessageDTO
	composeCalls      []*models.CreateMessageRequest
	composeErr        error
	updateDraftCalls  []updateDraftCall
	updateDraftErr    error
	sendCalls         []string
	sendErr           error
	markReadCalls     []string
	markReadErr       error
	archiveCalls      []string
	archiveErr        error
	spamCalls         []string
	spamErr           error
	deleteCalls       []string
	deleteErr         error
	toggleDeleteCalls []string
	toggleDeleteErr   error
	lastLabelQuery    string
	lastSearch        models.SearchCriteria
	composedDraftID   string
	updatedDraftCall  *models.UpdateMessageRequest
}

type updateDraftCall struct {
	id  string
	dto *models.UpdateMessageRequest
}

func (s *messageServiceStub) GetMessage(ctx context.Context, id string) (*models.MessageDTO, error) {
	for _, msg := range s.inbox {
		if msg.ID == id {
			return msg, nil
		}
	}
	return nil, nil
}

func (s *messageServiceStub) ListMessages(ctx context.Context, limit, offset int) ([]*models.MessageDTO, error) {
	return s.filterMessages(models.SearchCriteria{}), nil
}

func (s *messageServiceStub) SearchMessages(ctx context.Context, criteria models.SearchCriteria) ([]*models.MessageDTO, error) {
	s.lastSearch = criteria
	return s.filterMessages(criteria), nil
}

func (s *messageServiceStub) ComposeMessage(ctx context.Context, createDTO *models.CreateMessageRequest) (*models.MessageDTO, error) {
	if s.composeErr != nil {
		return nil, s.composeErr
	}
	s.composeCalls = append(s.composeCalls, createDTO)
	id := s.composedDraftID
	if id == "" {
		id = "draft-test"
	}
	return &models.MessageDTO{ID: id, Subject: createDTO.Subject, To: createDTO.To, Body: createDTO.Body, IsDraft: true}, nil
}

func (s *messageServiceStub) SendMessage(ctx context.Context, id string) error {
	if s.sendErr != nil {
		return s.sendErr
	}
	s.sendCalls = append(s.sendCalls, id)
	return nil
}

func (s *messageServiceStub) DeleteMessage(ctx context.Context, id string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.deleteCalls = append(s.deleteCalls, id)
	for index, msg := range s.inbox {
		if msg != nil && msg.ID == id {
			s.inbox = append(s.inbox[:index], s.inbox[index+1:]...)
			break
		}
	}
	return nil
}

func (s *messageServiceStub) ReplyToMessage(ctx context.Context, messageID string, body string) (*models.MessageDTO, error) {
	return nil, nil
}

func (s *messageServiceStub) ForwardMessage(ctx context.Context, messageID string, to []string) (*models.MessageDTO, error) {
	return nil, nil
}

func (s *messageServiceStub) GetAllInboxes(ctx context.Context, limit, offset int) ([]*models.MessageDTO, error) {
	isDraft := false
	isSpam := false
	isDeleted := false
	return s.filterMessages(models.SearchCriteria{IsDraft: &isDraft, IsSpam: &isSpam, IsDeleted: &isDeleted, Labels: []string{"inbox"}}), nil
}

func (s *messageServiceStub) GetFlagged(ctx context.Context, limit, offset int) ([]*models.MessageDTO, error) {
	return s.filterMessages(models.SearchCriteria{}), nil
}

func (s *messageServiceStub) GetDrafts(ctx context.Context, limit, offset int) ([]*models.MessageDTO, error) {
	isDraft := true
	isDeleted := false
	return s.filterMessages(models.SearchCriteria{IsDraft: &isDraft, IsDeleted: &isDeleted}), nil
}

func (s *messageServiceStub) GetSent(ctx context.Context, limit, offset int) ([]*models.MessageDTO, error) {
	isDeleted := false
	return s.filterMessages(models.SearchCriteria{Labels: []string{"sent"}, IsDeleted: &isDeleted}), nil
}

func (s *messageServiceStub) GetByLabel(ctx context.Context, label string, limit, offset int) ([]*models.MessageDTO, error) {
	s.lastLabelQuery = label
	isDeleted := false
	return s.filterMessages(models.SearchCriteria{Labels: []string{label}, IsDeleted: &isDeleted}), nil
}

func (s *messageServiceStub) ReplyAllToMessage(ctx context.Context, originalID string, body string) (*models.MessageDTO, error) {
	return nil, nil
}

func (s *messageServiceStub) UpdateDraft(ctx context.Context, id string, updateDTO *models.UpdateMessageRequest) (*models.MessageDTO, error) {
	if s.updateDraftErr != nil {
		return nil, s.updateDraftErr
	}
	s.updateDraftCalls = append(s.updateDraftCalls, updateDraftCall{id: id, dto: updateDTO})
	s.updatedDraftCall = updateDTO
	return &models.MessageDTO{ID: id, AccountID: derefString(updateDTO.AccountID), From: derefString(updateDTO.From), Subject: derefString(updateDTO.Subject), To: derefStrings(updateDTO.To), Cc: derefStrings(updateDTO.Cc), Bcc: derefStrings(updateDTO.Bcc), Body: derefString(updateDTO.Body), IsDraft: true}, nil
}

func (s *messageServiceStub) ToggleStar(ctx context.Context, id string) (*models.MessageDTO, error) {
	return nil, nil
}

func (s *messageServiceStub) MarkAsRead(ctx context.Context, id string) (*models.MessageDTO, error) {
	if s.markReadErr != nil {
		return nil, s.markReadErr
	}
	s.markReadCalls = append(s.markReadCalls, id)
	for _, msg := range s.inbox {
		if msg != nil && msg.ID == id {
			msg.IsRead = true
			return msg, nil
		}
	}
	return &models.MessageDTO{ID: id, IsRead: true}, nil
}

func (s *messageServiceStub) ToggleDelete(ctx context.Context, id string) (*models.MessageDTO, error) {
	if s.toggleDeleteErr != nil {
		return nil, s.toggleDeleteErr
	}
	s.toggleDeleteCalls = append(s.toggleDeleteCalls, id)
	for _, msg := range s.inbox {
		if msg != nil && msg.ID == id {
			msg.IsDeleted = !msg.IsDeleted
			return msg, nil
		}
	}
	return &models.MessageDTO{ID: id, IsDeleted: true}, nil
}

func (s *messageServiceStub) ArchiveMessage(ctx context.Context, id string) (*models.MessageDTO, error) {
	if s.archiveErr != nil {
		return nil, s.archiveErr
	}
	s.archiveCalls = append(s.archiveCalls, id)
	for _, msg := range s.inbox {
		if msg != nil && msg.ID == id {
			msg.Labels = filterTestLabels(msg.Labels, "inbox")
			if !slices.Contains(msg.Labels, "archive") {
				msg.Labels = append(msg.Labels, "archive")
			}
			msg.IsRead = true
			return msg, nil
		}
	}
	return &models.MessageDTO{ID: id, Labels: []string{"archive"}}, nil
}

func (s *messageServiceStub) MarkAsSpam(ctx context.Context, id string) (*models.MessageDTO, error) {
	if s.spamErr != nil {
		return nil, s.spamErr
	}
	s.spamCalls = append(s.spamCalls, id)
	for _, msg := range s.inbox {
		if msg != nil && msg.ID == id {
			msg.IsSpam = true
			msg.Labels = filterTestLabels(msg.Labels, "inbox")
			if !slices.Contains(msg.Labels, "spam") {
				msg.Labels = append(msg.Labels, "spam")
			}
			return msg, nil
		}
	}
	return &models.MessageDTO{ID: id, IsSpam: true}, nil
}

func (s *messageServiceStub) RestoreMessage(ctx context.Context, snapshot *models.MessageDTO) (*models.MessageDTO, error) {
	if snapshot == nil {
		return nil, nil
	}
	for index, msg := range s.inbox {
		if msg != nil && msg.ID == snapshot.ID {
			s.inbox[index] = cloneMessageDTO(snapshot)
			return cloneMessageDTO(snapshot), nil
		}
	}
	restored := cloneMessageDTO(snapshot)
	s.inbox = append(s.inbox, restored)
	return cloneMessageDTO(snapshot), nil
}

func (s *messageServiceStub) AddLabel(ctx context.Context, id, label string) (*models.MessageDTO, error) {
	return nil, nil
}

func testModel() Model {
	return testModelWithService(&messageServiceStub{inbox: sampleMessages()})
}

func testModelWithService(service *messageServiceStub) Model {
	messages := append([]*models.MessageDTO{}, service.inbox...)
	m := Model{
		state:         stateSidebar,
		keys:          defaultKeyMap(),
		styles:        DefaultStyles(),
		sidebarItems:  []string{"Inbox", "Sent", "Drafts", "Archive", "Trash", "Spam"},
		sidebarCursor: 0,
		service:       service,
		allMessages:   messages,
		messages:      messages,
		listCursor:    0,
		accountNames:  []string{"personal", "work"},
		accountEmails: map[string]string{"personal": "me@example.com"},
		defaultFrom:   "me@example.com",
		defaultAcctID: "personal",
		searchInput: func() textinput.Model {
			input := textinput.New()
			input.Prompt = "Search: "
			input.Placeholder = "subject, sender, body"
			return input
		}(),
		contentViewport: viewport.New(0, 0),
		help:            help.New(),
		toInput:         textinput.New(),
		subjectInput:    textinput.New(),
		bodyInput: func() textarea.Model {
			input := textarea.New()
			input.ShowLineNumbers = false
			return input
		}(),
	}
	m.accountEmails["work"] = "work@example.com"
	return m
}

func sampleMessages() []*models.MessageDTO {
	return []*models.MessageDTO{
		{ID: "msg-1", AccountID: "personal", ThreadID: "thread-1", Subject: "Subject 1", From: "sender1@example.com", To: []string{"me@example.com"}, Cc: []string{"copy@example.com"}, Body: "Body 1", Labels: []string{"inbox"}, Date: time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)},
		{ID: "msg-2", AccountID: "personal", ThreadID: "thread-2", Subject: "Subject 2", From: "sender2@example.com", To: []string{"me@example.com"}, Body: "Body 2", Labels: []string{"inbox"}, Date: time.Date(2026, 3, 19, 12, 0, 0, 0, time.UTC)},
	}
}

func sampleAccountMessages() []*models.MessageDTO {
	return []*models.MessageDTO{
		{ID: "outlook-1", AccountID: "Outlook", ThreadID: "thread-o1", Subject: "Outlook inbox", From: "outlook@example.com", To: []string{"me@example.com"}, Body: "Outlook body", Labels: []string{"inbox"}, Date: time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)},
		{ID: "gmail-1", AccountID: "Gmail", ThreadID: "thread-g1", Subject: "Gmail inbox", From: "gmail@example.com", To: []string{"me@example.com"}, Body: "Gmail body", Labels: []string{"inbox"}, Date: time.Date(2026, 3, 20, 11, 0, 0, 0, time.UTC)},
		{ID: "gmail-2", AccountID: "Gmail", ThreadID: "thread-g2", Subject: "Gmail sent", From: "me@gmail.com", To: []string{"you@example.com"}, Body: "Sent body", Labels: []string{"sent"}, Date: time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)},
		{ID: "gmail-3", AccountID: "Gmail", ThreadID: "thread-g3", Subject: "Gmail spam", From: "spam@example.com", To: []string{"me@example.com"}, Body: "Spam body", Labels: []string{"spam"}, IsSpam: true, Date: time.Date(2026, 3, 20, 9, 0, 0, 0, time.UTC)},
	}
}

func sampleDraftMessages() []*models.MessageDTO {
	return []*models.MessageDTO{
		{ID: "draft-1", AccountID: "personal", ThreadID: "thread-d1", Subject: "Draft subject", From: "me@example.com", To: []string{"user@example.com"}, Body: "Draft body", IsDraft: true, Date: time.Date(2026, 3, 20, 13, 0, 0, 0, time.UTC)},
	}
}

func sampleSentMessages() []*models.MessageDTO {
	return []*models.MessageDTO{
		{ID: "sent-1", AccountID: "personal", ThreadID: "thread-s1", Subject: "Sent subject", From: "me@example.com", To: []string{"user@example.com"}, Body: "Sent body", Labels: []string{"sent"}, IsRead: true, Date: time.Date(2026, 3, 20, 14, 0, 0, 0, time.UTC)},
	}
}

func sampleSpamMessages() []*models.MessageDTO {
	return []*models.MessageDTO{
		{ID: "spam-1", AccountID: "personal", ThreadID: "thread-x1", Subject: "Spam subject", From: "spam@example.com", To: []string{"me@example.com"}, Body: "Spam body", Labels: []string{"spam"}, IsSpam: true, Date: time.Date(2026, 3, 20, 15, 0, 0, 0, time.UTC)},
	}
}

func manyDraftMessages(count int) []*models.MessageDTO {
	result := make([]*models.MessageDTO, 0, count)
	for i := 0; i < count; i++ {
		result = append(result, &models.MessageDTO{
			ID:        "draft-bulk-" + time.Date(2026, 3, 20, 13, 0, 0, 0, time.UTC).Add(time.Duration(i)*time.Minute).Format("150405") + "-" + string(rune('a'+(i%26))),
			AccountID: "personal",
			ThreadID:  "thread-bulk",
			Subject:   "Draft subject",
			From:      "me@example.com",
			To:        []string{"user@example.com"},
			Body:      "Draft body",
			IsDraft:   true,
			Date:      time.Date(2026, 3, 20, 13, 0, 0, 0, time.UTC).Add(time.Duration(i) * time.Minute),
		})
	}
	return result
}

func sampleLongMessage() []*models.MessageDTO {
	bodyLines := make([]string, 0, 40)
	for i := 1; i <= 40; i++ {
		bodyLines = append(bodyLines, "Line "+time.Date(2026, 3, 20, 13, 0, 0, 0, time.UTC).Add(time.Duration(i)*time.Minute).Format("15:04")+" of a long message body")
	}
	return []*models.MessageDTO{{
		ID:        "long-1",
		AccountID: "personal",
		ThreadID:  "thread-long",
		Subject:   "Long body message",
		From:      "sender@example.com",
		To:        []string{"me@example.com"},
		Body:      strings.Join(bodyLines, "\n"),
		Labels:    []string{"inbox"},
		Date:      time.Date(2026, 3, 20, 16, 0, 0, 0, time.UTC),
	}}
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func derefStrings(value *[]string) []string {
	if value == nil {
		return nil
	}
	return append([]string{}, (*value)...)
}

func (s *messageServiceStub) filterMessages(criteria models.SearchCriteria) []*models.MessageDTO {
	filtered := make([]*models.MessageDTO, 0, len(s.inbox))
	for _, msg := range s.inbox {
		if msg == nil {
			continue
		}
		if criteria.IsDeleted != nil && msg.IsDeleted != *criteria.IsDeleted {
			continue
		}
		if criteria.IsDraft != nil && msg.IsDraft != *criteria.IsDraft {
			continue
		}
		if criteria.IsSpam != nil && msg.IsSpam != *criteria.IsSpam {
			continue
		}
		if criteria.IsRead != nil && msg.IsRead != *criteria.IsRead {
			continue
		}
		if criteria.AccountID != "" && !strings.EqualFold(msg.AccountID, criteria.AccountID) {
			continue
		}
		if len(criteria.Labels) > 0 && !containsAllLabels(msg.Labels, criteria.Labels) {
			continue
		}
		filtered = append(filtered, msg)
	}
	return filtered
}

func containsAllLabels(labels, expected []string) bool {
	for _, label := range expected {
		if !slices.Contains(labels, label) {
			return false
		}
	}
	return true
}

func filterTestLabels(labels []string, excluded string) []string {
	filtered := labels[:0]
	for _, label := range labels {
		if label != excluded {
			filtered = append(filtered, label)
		}
	}
	return filtered
}

func updateModel(t *testing.T, model Model, msg tea.Msg) Model {
	t.Helper()
	updated, _ := model.Update(msg)
	casted, ok := updated.(Model)
	require.True(t, ok)
	return casted
}

func keyRune(value rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{value}}
}
