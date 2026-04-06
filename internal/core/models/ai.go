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
