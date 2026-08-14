package models

// GenerateDraftRequest describes the context passed to the AI draft assistant.
type GenerateDraftRequest struct {
	Mode        string            `json:"mode,omitempty"`
	Template    string            `json:"template,omitempty"`
	AccountID   string            `json:"account_id,omitempty"`
	From        string            `json:"from,omitempty"`
	To          []string          `json:"to,omitempty"`
	Cc          []string          `json:"cc,omitempty"`
	Bcc         []string          `json:"bcc,omitempty"`
	Subject     string            `json:"subject,omitempty"`
	Body        string            `json:"body,omitempty"`
	Instruction string            `json:"instruction,omitempty"`
	ReplyAll    bool              `json:"reply_all,omitempty"`
	Original    *Message          `json:"original,omitempty"`
	Variables   map[string]string `json:"variables,omitempty"`
}

// GeneratedDraft is the structured output expected from AI providers.
type GeneratedDraft struct {
	Subject string `json:"subject,omitempty"`
	Body    string `json:"body,omitempty"`
}

// PromptCompletionRequest is the provider-neutral prompt payload.
type PromptCompletionRequest struct {
	Model        string  `json:"model,omitempty"`
	SystemPrompt string  `json:"system_prompt,omitempty"`
	Prompt       string  `json:"prompt,omitempty"`
	Temperature  float64 `json:"temperature,omitempty"`
}

// AISettings is the provider-neutral configuration the draft assistant needs. It
// is a domain type so the assistant service depends only on core, never on the
// infrastructure config package; the composition root maps config into it.
type AISettings struct {
	DefaultComposeTemplate string
	DefaultReplyTemplate   string
	Providers              map[string]AIProviderSettings
	Templates              map[string]AITemplate
}

// AIProviderSettings holds the per-provider values the assistant uses when
// building a prompt. Secrets and transport (api keys, base URLs) belong to the
// provider adapters, not here.
type AIProviderSettings struct {
	Model string
}

// AITemplate describes one prompt template the assistant can render.
type AITemplate struct {
	Mode         string
	Provider     string
	SystemPrompt string
	Prompt       string
	Temperature  float64
}
