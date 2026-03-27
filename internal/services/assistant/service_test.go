package assistant

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kriuchkov/postero/internal/config"
	"github.com/kriuchkov/postero/internal/core/models"
	"github.com/kriuchkov/postero/internal/core/ports"
)

type stubProvider struct {
	response string
	request  models.PromptCompletionRequest
	called   bool
}

func (s *stubProvider) CompletePrompt(_ context.Context, request models.PromptCompletionRequest) (string, error) {
	s.called = true
	s.request = request
	return s.response, nil
}

func TestGenerateDraftRendersTemplateAndParsesJSON(t *testing.T) {
	provider := &stubProvider{response: `{"subject":"Follow-up","body":"Thanks for the context."}`}
	service := NewService(config.AIConfig{
		DefaultReplyTemplate: "reply-default",
		Providers: map[string]config.AIProviderConfig{
			"openai": {Model: "gpt-4.1-mini"},
		},
		Templates: map[string]config.AITemplateConfig{
			"reply-default": {
				Mode:         ModeReply,
				Provider:     "openai",
				SystemPrompt: "You write careful replies.",
				Prompt:       `Instruction: {{ .Instruction }}\nSubject: {{ .Original.Subject }}\nVars: {{ index .Vars "tone" }}`,
				Temperature:  0.2,
			},
		},
	}, stubPromptProviders{"openai": provider}.toPorts())

	draft, err := service.GenerateDraft(context.Background(), models.GenerateDraftRequest{
		Mode:        ModeReply,
		Instruction: "Reply warmly",
		Original: &models.Message{
			Subject: "Status update",
			Body:    "Original body",
			Date:    time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
		},
		Variables: map[string]string{"tone": "warm"},
	})

	require.NoError(t, err)
	require.True(t, provider.called)
	assert.Equal(t, "gpt-4.1-mini", provider.request.Model)
	assert.Equal(t, "You write careful replies.", provider.request.SystemPrompt)
	assert.Contains(t, provider.request.Prompt, "Reply warmly")
	assert.Contains(t, provider.request.Prompt, "Status update")
	assert.Contains(t, provider.request.Prompt, "warm")
	assert.InDelta(t, 0.2, provider.request.Temperature, 0.0001)
	assert.Equal(t, "Follow-up", draft.Subject)
	assert.Equal(t, "Thanks for the context.", draft.Body)
}

func TestGenerateDraftFallsBackToReplySubject(t *testing.T) {
	provider := &stubProvider{response: "```json\n{\"body\":\"Sure, happy to help.\"}\n```"}
	service := NewService(config.AIConfig{
		DefaultReplyTemplate: "reply-default",
		Providers: map[string]config.AIProviderConfig{
			"openai": {Model: "gpt-4.1-mini"},
		},
		Templates: map[string]config.AITemplateConfig{
			"reply-default": {
				Mode:     ModeReply,
				Provider: "openai",
				Prompt:   "Write a reply",
			},
		},
	}, stubPromptProviders{"openai": provider}.toPorts())

	draft, err := service.GenerateDraft(context.Background(), models.GenerateDraftRequest{
		Mode:     ModeReply,
		ReplyAll: true,
		Original: &models.Message{
			Subject: "Status update",
			From:    "Alice <alice@example.com>",
			To:      []string{"me@example.com", "Bob <bob@example.com>"},
			Body:    "Original body",
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "Re: Status update", draft.Subject)
	assert.Equal(t, "Sure, happy to help.", draft.Body)
}

func TestGenerateDraftRejectsWrongTemplateMode(t *testing.T) {
	service := NewService(config.AIConfig{
		Templates: map[string]config.AITemplateConfig{
			"compose-default": {
				Mode:     ModeCompose,
				Provider: "openai",
				Prompt:   "Write a draft",
			},
		},
	}, nil)

	_, err := service.GenerateDraft(context.Background(), models.GenerateDraftRequest{Mode: ModeReply, Template: "compose-default"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "only supports compose mode")
}

type stubPromptProviders map[string]*stubProvider

func (s stubPromptProviders) toPorts() map[string]ports.PromptCompletionProvider {
	providers := make(map[string]ports.PromptCompletionProvider, len(s))
	for name, provider := range s {
		providers[name] = provider
	}
	return providers
}
