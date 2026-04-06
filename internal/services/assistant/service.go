package assistant

import (
	"context"
	"encoding/json"
	"maps"
	"strings"
	"text/template"

	"github.com/go-faster/errors"

	"github.com/kriuchkov/postero/internal/config"
	"github.com/kriuchkov/postero/internal/core/models"
	"github.com/kriuchkov/postero/internal/core/ports"
	"github.com/kriuchkov/postero/pkg/compose"
)

const (
	ModeCompose = "compose"
	ModeReply   = "reply"
)

type Service struct {
	aiConfig  config.AIConfig
	providers map[string]ports.PromptCompletionProvider
}

type templateData struct {
	Mode        string
	AccountID   string
	From        string
	To          []string
	Cc          []string
	Bcc         []string
	Subject     string
	Body        string
	Instruction string
	ReplyAll    bool
	Original    *models.Message
	Vars        map[string]string
}

func NewService(aiConfig config.AIConfig, providers map[string]ports.PromptCompletionProvider) *Service {
	return &Service{aiConfig: aiConfig, providers: providers}
}

func (s *Service) GenerateDraft(ctx context.Context, request models.GenerateDraftRequest) (*models.GeneratedDraft, error) {
	templateName, templateConfig, err := s.resolveTemplate(request)
	if err != nil {
		return nil, err
	}

	providerConfig, ok := s.aiConfig.Providers[templateConfig.Provider]
	if !ok {
		return nil, errors.Errorf("ai provider %q referenced by template %q is not configured", templateConfig.Provider, templateName)
	}
	provider, ok := s.providers[templateConfig.Provider]
	if !ok {
		return nil, errors.Errorf("ai provider %q is not available", templateConfig.Provider)
	}

	data := templateData{
		Mode:        strings.TrimSpace(request.Mode),
		AccountID:   strings.TrimSpace(request.AccountID),
		From:        strings.TrimSpace(request.From),
		To:          append([]string(nil), request.To...),
		Cc:          append([]string(nil), request.Cc...),
		Bcc:         append([]string(nil), request.Bcc...),
		Subject:     strings.TrimSpace(request.Subject),
		Body:        strings.TrimSpace(request.Body),
		Instruction: strings.TrimSpace(request.Instruction),
		ReplyAll:    request.ReplyAll,
		Original:    request.Original,
		Vars:        copyVars(request.Variables),
	}

	systemPrompt, err := renderPromptTemplate("system", templateConfig.SystemPrompt, data)
	if err != nil {
		return nil, errors.Wrap(err, "render system prompt")
	}
	prompt, err := renderPromptTemplate("prompt", templateConfig.Prompt, data)
	if err != nil {
		return nil, errors.Wrap(err, "render prompt")
	}
	raw, err := provider.CompletePrompt(ctx, models.PromptCompletionRequest{
		Model:        strings.TrimSpace(providerConfig.Model),
		SystemPrompt: systemPrompt,
		Prompt:       prompt,
		Temperature:  templateConfig.Temperature,
	})
	if err != nil {
		return nil, errors.Wrap(err, "generate ai draft")
	}

	draft, err := parseGeneratedDraft(raw)
	if err != nil {
		return nil, err
	}
	if draft.Subject == "" {
		draft.Subject = fallbackSubject(request)
	}
	return draft, nil
}

func (s *Service) resolveTemplate(request models.GenerateDraftRequest) (string, config.AITemplateConfig, error) {
	mode := strings.ToLower(strings.TrimSpace(request.Mode))
	name := strings.TrimSpace(request.Template)
	if name == "" {
		switch mode {
		case ModeCompose:
			name = strings.TrimSpace(s.aiConfig.DefaultComposeTemplate)
		case ModeReply:
			name = strings.TrimSpace(s.aiConfig.DefaultReplyTemplate)
		}
	}
	if name == "" {
		matches := make([]string, 0, len(s.aiConfig.Templates))
		for candidate, cfg := range s.aiConfig.Templates {
			candidateMode := strings.ToLower(strings.TrimSpace(cfg.Mode))
			if candidateMode == "" || candidateMode == mode {
				matches = append(matches, candidate)
			}
		}
		if len(matches) == 1 {
			name = matches[0]
		}
	}
	if name == "" {
		return "", config.AITemplateConfig{}, errors.Errorf("no ai template configured for %s mode", mode)
	}

	templateConfig, ok := s.aiConfig.Templates[name]
	if !ok {
		return "", config.AITemplateConfig{}, errors.Errorf("ai template %q is not configured", name)
	}
	if templateConfig.Provider == "" {
		return "", config.AITemplateConfig{}, errors.Errorf("ai template %q does not declare a provider", name)
	}
	templateMode := strings.ToLower(strings.TrimSpace(templateConfig.Mode))
	if templateMode != "" && templateMode != mode {
		return "", config.AITemplateConfig{}, errors.Errorf("ai template %q only supports %s mode", name, templateMode)
	}
	if strings.TrimSpace(templateConfig.Prompt) == "" {
		return "", config.AITemplateConfig{}, errors.Errorf("ai template %q does not define a prompt", name)
	}

	return name, templateConfig, nil
}

func renderPromptTemplate(name, source string, data templateData) (string, error) {
	if strings.TrimSpace(source) == "" {
		return "", nil
	}
	tmpl, err := template.New(name).Funcs(template.FuncMap{
		"join": strings.Join,
	}).Option("missingkey=error").Parse(source)
	if err != nil {
		return "", err
	}
	var builder strings.Builder
	if err := tmpl.Execute(&builder, data); err != nil {
		return "", err
	}
	return strings.TrimSpace(builder.String()), nil
}

func parseGeneratedDraft(raw string) (*models.GeneratedDraft, error) {
	payload := extractJSONObject(raw)
	var draft models.GeneratedDraft
	if err := json.Unmarshal([]byte(payload), &draft); err != nil {
		return nil, errors.Wrap(err, "parse ai response as draft json")
	}
	draft.Subject = strings.TrimSpace(draft.Subject)
	draft.Body = strings.TrimSpace(draft.Body)
	if draft.Body == "" {
		return nil, errors.New("ai response did not include a body")
	}
	return &draft, nil
}

func extractJSONObject(raw string) string {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end > start {
		return trimmed[start : end+1]
	}
	return trimmed
}

func fallbackSubject(request models.GenerateDraftRequest) string {
	if strings.TrimSpace(request.Mode) == ModeReply && request.Original != nil {
		return compose.BuildReply(request.Original, compose.ReplyOptions{ReplyAll: request.ReplyAll}).Subject
	}
	return strings.TrimSpace(request.Subject)
}

func copyVars(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	return maps.Clone(values)
}
