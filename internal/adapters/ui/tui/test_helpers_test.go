package tui

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"

	"github.com/kriuchkov/postero/internal/core/models"
)

type messageServiceStub struct {
	inbox             []*models.Message
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

type draftAssistantStub struct {
	response *models.GeneratedDraft
	err      error
	requests []models.GenerateDraftRequest
}

func (s *draftAssistantStub) GenerateDraft(_ context.Context, request models.GenerateDraftRequest) (*models.GeneratedDraft, error) {
	s.requests = append(s.requests, request)
	if s.err != nil {
		return nil, s.err
	}
	if s.response == nil {
		return &models.GeneratedDraft{}, nil
	}
	return &models.GeneratedDraft{Subject: s.response.Subject, Body: s.response.Body}, nil
}

type updateDraftCall struct {
	id  string
	dto *models.UpdateMessageRequest
}

func (s *messageServiceStub) GetMessage(_ context.Context, id string) (*models.Message, error) {
	for _, msg := range s.inbox {
		if msg.ID == id {
			return msg, nil
		}
	}
	return &models.Message{}, nil
}

func (s *messageServiceStub) ListMessages(_ context.Context, limit, offset int) ([]*models.Message, error) {
	return s.filterMessages(models.SearchCriteria{Limit: limit, Offset: offset}), nil
}

func (s *messageServiceStub) SearchMessages(_ context.Context, criteria models.SearchCriteria) ([]*models.Message, error) {
	s.lastSearch = criteria
	return s.filterMessages(criteria), nil
}

func (s *messageServiceStub) ComposeMessage(_ context.Context, createDTO *models.CreateMessageRequest) (*models.Message, error) {
	if s.composeErr != nil {
		return nil, s.composeErr
	}
	s.composeCalls = append(s.composeCalls, createDTO)
	id := s.composedDraftID
	if id == "" {
		id = "draft-test"
	}
	return &models.Message{ID: id, Subject: createDTO.Subject, To: createDTO.To, Body: createDTO.Body, IsDraft: true}, nil
}

func (s *messageServiceStub) SendMessage(_ context.Context, id string) error {
	if s.sendErr != nil {
		return s.sendErr
	}
	s.sendCalls = append(s.sendCalls, id)
	return nil
}

func (s *messageServiceStub) DeleteMessage(_ context.Context, id string) error {
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

func (s *messageServiceStub) ReplyToMessage(_ context.Context, _ string, _ string) (*models.Message, error) {
	return &models.Message{}, nil
}

func (s *messageServiceStub) ForwardMessage(_ context.Context, _ string, _ []string) (*models.Message, error) {
	return &models.Message{}, nil
}

func (s *messageServiceStub) GetAllInboxes(_ context.Context, limit, offset int) ([]*models.Message, error) {
	isDraft := false
	isSpam := false
	isDeleted := false
	return s.filterMessages(
		models.SearchCriteria{IsDraft: &isDraft, IsSpam: &isSpam, IsDeleted: &isDeleted, Labels: []string{"inbox"}, Limit: limit, Offset: offset},
	), nil
}

func (s *messageServiceStub) GetFlagged(_ context.Context, limit, offset int) ([]*models.Message, error) {
	return s.filterMessages(models.SearchCriteria{Limit: limit, Offset: offset}), nil
}

func (s *messageServiceStub) GetDrafts(_ context.Context, limit, offset int) ([]*models.Message, error) {
	isDraft := true
	isDeleted := false
	return s.filterMessages(models.SearchCriteria{IsDraft: &isDraft, IsDeleted: &isDeleted, Limit: limit, Offset: offset}), nil
}

func (s *messageServiceStub) GetSent(_ context.Context, limit, offset int) ([]*models.Message, error) {
	isDeleted := false
	return s.filterMessages(models.SearchCriteria{Labels: []string{"sent"}, IsDeleted: &isDeleted, Limit: limit, Offset: offset}), nil
}

func (s *messageServiceStub) GetByLabel(_ context.Context, label string, limit, offset int) ([]*models.Message, error) {
	s.lastLabelQuery = label
	isDeleted := false
	return s.filterMessages(models.SearchCriteria{Labels: []string{label}, IsDeleted: &isDeleted, Limit: limit, Offset: offset}), nil
}

func (s *messageServiceStub) ReplyAllToMessage(_ context.Context, _ string, _ string) (*models.Message, error) {
	return &models.Message{}, nil
}

func (s *messageServiceStub) UpdateDraft(
	_ context.Context,
	id string,
	updateDTO *models.UpdateMessageRequest,
) (*models.Message, error) {
	if s.updateDraftErr != nil {
		return nil, s.updateDraftErr
	}
	s.updateDraftCalls = append(s.updateDraftCalls, updateDraftCall{id: id, dto: updateDTO})
	s.updatedDraftCall = updateDTO
	return &models.Message{
		ID:        id,
		AccountID: derefString(updateDTO.AccountID),
		From:      derefString(updateDTO.From),
		Subject:   derefString(updateDTO.Subject),
		To:        derefStrings(updateDTO.To),
		Cc:        derefStrings(updateDTO.Cc),
		Bcc:       derefStrings(updateDTO.Bcc),
		Body:      derefString(updateDTO.Body),
		IsDraft:   true,
	}, nil
}

func (s *messageServiceStub) ToggleStar(_ context.Context, id string) (*models.Message, error) {
	return &models.Message{ID: id}, nil
}

func (s *messageServiceStub) MarkAsRead(_ context.Context, id string) (*models.Message, error) {
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
	return &models.Message{ID: id, IsRead: true}, nil
}

func (s *messageServiceStub) ToggleDelete(_ context.Context, id string) (*models.Message, error) {
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
	return &models.Message{ID: id, IsDeleted: true}, nil
}

func (s *messageServiceStub) ArchiveMessage(_ context.Context, id string) (*models.Message, error) {
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
	return &models.Message{ID: id, Labels: []string{"archive"}}, nil
}

func (s *messageServiceStub) MarkAsSpam(_ context.Context, id string) (*models.Message, error) {
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
	return &models.Message{ID: id, IsSpam: true}, nil
}

func (s *messageServiceStub) RestoreMessage(_ context.Context, snapshot *models.Message) (*models.Message, error) {
	if snapshot == nil {
		return nil, errors.New("snapshot is nil")
	}
	for index, msg := range s.inbox {
		if msg != nil && msg.ID == snapshot.ID {
			s.inbox[index] = cloneMessage(snapshot)
			return cloneMessage(snapshot), nil
		}
	}
	restored := cloneMessage(snapshot)
	s.inbox = append(s.inbox, restored)
	return cloneMessage(snapshot), nil
}

func (s *messageServiceStub) AddLabel(_ context.Context, id, label string) (*models.Message, error) {
	return &models.Message{ID: id, Labels: []string{label}}, nil
}

func testModel() Model {
	return testModelWithService(&messageServiceStub{inbox: sampleMessages()})
}

func testModelWithService(service *messageServiceStub) Model {
	messages := append([]*models.Message{}, service.inbox...)
	m := Model{
		state:            stateSidebar,
		keys:             defaultKeyMap(),
		styles:           DefaultStyles(),
		sidebarItems:     []string{"Inbox", "Sent", "Drafts", "Archive", "Trash", "Spam"},
		sidebarCursor:    0,
		service:          service,
		assistant:        nil,
		allMessages:      messages,
		sidebarTagSource: append([]*models.Message{}, messages...),
		messages:         messages,
		listCursor:       0,
		accountNames:     []string{"personal", "work"},
		accountEmails:    map[string]string{"personal": "me@example.com"},
		defaultFrom:      "me@example.com",
		defaultAcctID:    "personal",
		activeTagID:      "",
		commandActive:    false,
		searchInput: func() textinput.Model {
			input := textinput.New()
			input.Prompt = "/ "
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
	m.applySearchInputStyles(false)
	return m
}

func sampleMessages() []*models.Message {
	return []*models.Message{
		{
			ID:        "msg-1",
			AccountID: "personal",
			ThreadID:  "thread-1",
			Subject:   "Subject 1",
			From:      "sender1@example.com",
			To:        []string{"me@example.com"},
			Cc:        []string{"copy@example.com"},
			Body:      "Body 1",
			Labels:    []string{"inbox"},
			Date:      time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC),
		},
		{
			ID:        "msg-2",
			AccountID: "personal",
			ThreadID:  "thread-2",
			Subject:   "Subject 2",
			From:      "sender2@example.com",
			To:        []string{"me@example.com"},
			Body:      "Body 2",
			Labels:    []string{"inbox"},
			Date:      time.Date(2026, 3, 19, 12, 0, 0, 0, time.UTC),
		},
	}
}

func sampleAccountMessages() []*models.Message {
	return []*models.Message{
		{
			ID:        "outlook-1",
			AccountID: "Outlook",
			ThreadID:  "thread-o1",
			Subject:   "Outlook inbox",
			From:      "outlook@example.com",
			To:        []string{"me@example.com"},
			Body:      "Outlook body",
			Labels:    []string{"inbox"},
			Date:      time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC),
		},
		{
			ID:        "gmail-1",
			AccountID: "Gmail",
			ThreadID:  "thread-g1",
			Subject:   "Gmail inbox",
			From:      "gmail@example.com",
			To:        []string{"me@example.com"},
			Body:      "Gmail body",
			Labels:    []string{"inbox"},
			Date:      time.Date(2026, 3, 20, 11, 0, 0, 0, time.UTC),
		},
		{
			ID:        "gmail-2",
			AccountID: "Gmail",
			ThreadID:  "thread-g2",
			Subject:   "Gmail sent",
			From:      "me@gmail.com",
			To:        []string{"you@example.com"},
			Body:      "Sent body",
			Labels:    []string{"sent"},
			Date:      time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
		},
		{
			ID:        "gmail-3",
			AccountID: "Gmail",
			ThreadID:  "thread-g3",
			Subject:   "Gmail spam",
			From:      "spam@example.com",
			To:        []string{"me@example.com"},
			Body:      "Spam body",
			Labels:    []string{"spam"},
			IsSpam:    true,
			Date:      time.Date(2026, 3, 20, 9, 0, 0, 0, time.UTC),
		},
	}
}

func sampleDraftMessages() []*models.Message {
	return []*models.Message{
		{
			ID:        "draft-1",
			AccountID: "personal",
			ThreadID:  "thread-d1",
			Subject:   "Draft subject",
			From:      "me@example.com",
			To:        []string{"user@example.com"},
			Body:      "Draft body",
			IsDraft:   true,
			Date:      time.Date(2026, 3, 20, 13, 0, 0, 0, time.UTC),
		},
	}
}

func sampleSentMessages() []*models.Message {
	return []*models.Message{
		{
			ID:        "sent-1",
			AccountID: "personal",
			ThreadID:  "thread-s1",
			Subject:   "Sent subject",
			From:      "me@example.com",
			To:        []string{"user@example.com"},
			Body:      "Sent body",
			Labels:    []string{"sent"},
			IsRead:    true,
			Date:      time.Date(2026, 3, 20, 14, 0, 0, 0, time.UTC),
		},
	}
}

func sampleSpamMessages() []*models.Message {
	return []*models.Message{
		{
			ID:        "spam-1",
			AccountID: "personal",
			ThreadID:  "thread-x1",
			Subject:   "Spam subject",
			From:      "spam@example.com",
			To:        []string{"me@example.com"},
			Body:      "Spam body",
			Labels:    []string{"spam"},
			IsSpam:    true,
			Date:      time.Date(2026, 3, 20, 15, 0, 0, 0, time.UTC),
		},
	}
}

func manyDraftMessages(count int) []*models.Message {
	result := make([]*models.Message, 0, count)
	for i := range count {
		result = append(result, &models.Message{
			ID: "draft-bulk-" + time.Date(2026, 3, 20, 13, 0, 0, 0, time.UTC).
				Add(time.Duration(i)*time.Minute).
				Format("150405") +
				"-" + string(
				rune('a'+(i%26)),
			),
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

func pagedInboxMessages(count int) []*models.Message {
	result := make([]*models.Message, 0, count)
	baseDate := time.Date(2026, 3, 20, 18, 0, 0, 0, time.UTC)
	for i := range count {
		result = append(result, &models.Message{
			ID:        fmt.Sprintf("msg-%03d", i+1),
			AccountID: "personal",
			ThreadID:  fmt.Sprintf("thread-%03d", i+1),
			Subject:   fmt.Sprintf("Subject %03d", i+1),
			From:      fmt.Sprintf("sender%03d@example.com", i+1),
			To:        []string{"me@example.com"},
			Body:      fmt.Sprintf("Body %03d", i+1),
			Labels:    []string{"inbox"},
			Date:      baseDate.Add(-time.Duration(i) * time.Minute),
		})
	}
	return result
}

func sampleLongMessage() []*models.Message {
	bodyLines := make([]string, 0, 40)
	for i := 1; i <= 40; i++ {
		bodyLines = append(
			bodyLines,
			"Line "+time.Date(2026, 3, 20, 13, 0, 0, 0, time.UTC).
				Add(time.Duration(i)*time.Minute).
				Format("15:04")+
				" of a long message body",
		)
	}
	return []*models.Message{{
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

func (s *messageServiceStub) filterMessages(criteria models.SearchCriteria) []*models.Message {
	filtered := make([]*models.Message, 0, len(s.inbox))
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
		if criteria.Query != "" && !messageMatchesSearch(msg, strings.ToLower(criteria.Query)) {
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
	if criteria.Offset >= len(filtered) {
		return []*models.Message{}
	}
	start := max(criteria.Offset, 0)
	end := len(filtered)
	if criteria.Limit > 0 && start+criteria.Limit < end {
		end = start + criteria.Limit
	}
	return append([]*models.Message{}, filtered[start:end]...)
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
	if batch, ok := msg.(tea.BatchMsg); ok {
		msg = resolveBatchMsgForTests(batch)
		if msg == nil {
			return model
		}
	}
	updated, _ := model.Update(msg)
	casted, ok := updated.(Model)
	require.True(t, ok)
	return casted
}

func updateModelWithCmd(t *testing.T, model Model, msg tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	updated, cmd := model.Update(msg)
	casted, ok := updated.(Model)
	require.True(t, ok)
	return casted, resolveCmdForTests(cmd)
}

func resolveCmdForTests(cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		msg := cmd()
		if batch, ok := msg.(tea.BatchMsg); ok {
			return resolveBatchMsgForTests(batch)
		}
		return msg
	}
}

func resolveBatchMsgForTests(batch tea.BatchMsg) tea.Msg {
	for index := len(batch) - 1; index >= 0; index-- {
		if batch[index] == nil {
			continue
		}
		msg := batch[index]()
		if nested, ok := msg.(tea.BatchMsg); ok {
			msg = resolveBatchMsgForTests(nested)
		}
		if msg != nil {
			return msg
		}
	}
	return nil
}

func keyRune(value rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{value}}
}
