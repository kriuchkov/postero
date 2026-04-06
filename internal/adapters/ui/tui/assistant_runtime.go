package tui

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-faster/errors"

	"github.com/kriuchkov/postero/internal/core/models"
	"github.com/kriuchkov/postero/pkg/compose"
)

type aiDraftGeneratedMsg struct {
	draft          *models.Message
	title          string
	hint           string
	status         string
	focusIndex     int
	composeEditing bool
	moveCursorTop  bool
}

type aiDraftFailedMsg struct {
	err error
}

type aiCommandOptions struct {
	template    string
	instruction string
}

func parseCommandPrompt(raw string) (string, string) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", ""
	}
	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return "", ""
	}
	command := strings.ToLower(parts[0])
	argument := strings.TrimSpace(strings.TrimPrefix(trimmed, parts[0]))
	return command, argument
}

func parseAICommandOptions(raw string) (aiCommandOptions, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return aiCommandOptions{}, nil
	}

	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return aiCommandOptions{}, nil
	}

	first := parts[0]
	switch {
	case strings.HasPrefix(first, "--template="):
		value := strings.TrimSpace(strings.TrimPrefix(first, "--template="))
		if value == "" {
			return aiCommandOptions{}, errors.New("template name is required after --template")
		}
		return aiCommandOptions{template: value, instruction: strings.TrimSpace(strings.Join(parts[1:], " "))}, nil
	case strings.HasPrefix(first, "-t="):
		value := strings.TrimSpace(strings.TrimPrefix(first, "-t="))
		if value == "" {
			return aiCommandOptions{}, errors.New("template name is required after -t")
		}
		return aiCommandOptions{template: value, instruction: strings.TrimSpace(strings.Join(parts[1:], " "))}, nil
	case strings.HasPrefix(first, "template="):
		value := strings.TrimSpace(strings.TrimPrefix(first, "template="))
		if value == "" {
			return aiCommandOptions{}, errors.New("template name is required after template=")
		}
		return aiCommandOptions{template: value, instruction: strings.TrimSpace(strings.Join(parts[1:], " "))}, nil
	case first == "--template" || first == "-t":
		if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
			return aiCommandOptions{}, errors.Errorf("template name is required after %s", first)
		}
		return aiCommandOptions{
			template:    strings.TrimSpace(parts[1]),
			instruction: strings.TrimSpace(strings.Join(parts[2:], " ")),
		}, nil
	default:
		return aiCommandOptions{instruction: trimmed}, nil
	}
}

func (m Model) generateComposeAIDraft(options aiCommandOptions) tea.Cmd {
	if m.assistant == nil {
		return func() tea.Msg {
			return aiDraftFailedMsg{err: errors.New("AI drafting is not configured")}
		}
	}

	accountID := strings.TrimSpace(m.defaultAcctID)
	from := strings.TrimSpace(m.defaultFrom)
	var draft *models.Message
	if m.activeDraft != nil {
		draft = cloneMessage(m.activeDraft)
		accountID = firstNonEmptyTrimmed(draft.AccountID, accountID)
		from = firstNonEmptyTrimmed(draft.From, from, m.senderForAccount(accountID))
	} else {
		draft = &models.Message{
			AccountID: accountID,
			From:      firstNonEmptyTrimmed(from, m.senderForAccount(accountID)),
			To:        []string{},
			Cc:        []string{},
			Bcc:       []string{},
		}
	}

	request := models.GenerateDraftRequest{
		Mode:        "compose",
		Template:    strings.TrimSpace(options.template),
		AccountID:   accountID,
		From:        draft.From,
		To:          append([]string(nil), draft.To...),
		Cc:          append([]string(nil), draft.Cc...),
		Bcc:         append([]string(nil), draft.Bcc...),
		Subject:     strings.TrimSpace(draft.Subject),
		Body:        strings.TrimSpace(draft.Body),
		Instruction: strings.TrimSpace(options.instruction),
	}

	return func() tea.Msg {
		generated, err := m.assistant.GenerateDraft(context.Background(), request)
		if err != nil {
			return aiDraftFailedMsg{err: err}
		}
		draft.Subject = firstNonEmptyTrimmed(generated.Subject, draft.Subject)
		draft.Body = generated.Body
		draft.AccountID = firstNonEmptyTrimmed(draft.AccountID, accountID)
		draft.From = firstNonEmptyTrimmed(draft.From, from, m.senderForAccount(draft.AccountID))
		return aiDraftGeneratedMsg{
			draft:          draft,
			title:          "AI Compose",
			hint:           "Review the generated draft, then edit or send it.",
			status:         "AI draft ready",
			focusIndex:     3,
			composeEditing: true,
		}
	}
}

func (m Model) generateReplyAIDraft(options aiCommandOptions, replyAll bool) tea.Cmd {
	if m.assistant == nil {
		return func() tea.Msg {
			return aiDraftFailedMsg{err: errors.New("AI drafting is not configured")}
		}
	}

	selected, ok := m.selectedMessage()
	if !ok {
		return func() tea.Msg {
			return aiDraftFailedMsg{err: errors.New("Select a message before generating an AI reply")}
		}
	}

	accountID := firstNonEmptyTrimmed(selected.AccountID, m.defaultAcctID)
	from := firstNonEmptyTrimmed(m.senderForAccount(accountID), m.defaultFrom)
	request := models.GenerateDraftRequest{
		Mode:        "reply",
		Template:    strings.TrimSpace(options.template),
		AccountID:   accountID,
		From:        from,
		Subject:     strings.TrimSpace(selected.Subject),
		Body:        strings.TrimSpace(selected.Body),
		Instruction: strings.TrimSpace(options.instruction),
		ReplyAll:    replyAll,
		Original:    &selected,
	}

	return func() tea.Msg {
		generated, err := m.assistant.GenerateDraft(context.Background(), request)
		if err != nil {
			return aiDraftFailedMsg{err: err}
		}
		replyDraft := compose.BuildReply(&selected, compose.ReplyOptions{
			ReplyAll: replyAll,
			Self:     []string{from},
			Body:     generated.Body,
		})
		title := "AI Reply"
		if replyAll {
			title = "AI Reply all"
		}
		return aiDraftGeneratedMsg{
			draft: &models.Message{
				AccountID: accountID,
				From:      from,
				ThreadID:  replyDraft.ThreadID,
				Subject:   firstNonEmptyTrimmed(generated.Subject, replyDraft.Subject),
				To:        replyDraft.To,
				Cc:        replyDraft.Cc,
				Body:      replyDraft.Body,
			},
			title:          title,
			hint:           "Review the generated reply above the quoted message.",
			status:         "AI reply ready",
			focusIndex:     3,
			composeEditing: true,
			moveCursorTop:  true,
		}
	}
}

func firstNonEmptyTrimmed(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
