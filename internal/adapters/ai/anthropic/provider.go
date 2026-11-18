// Package anthropic implements the PromptCompletionProvider port against
// Anthropic's Claude Messages API, so Claude can draft and reply to email.
package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-faster/errors"

	"github.com/kriuchkov/postero/internal/adapters/ai/aiutil"
	"github.com/kriuchkov/postero/internal/core/models"
)

// anthropicVersion is the required API version header value.
const anthropicVersion = "2023-06-01"

// defaultMaxTokens caps a single completion. The Messages API requires
// max_tokens; an email draft never needs more than this.
const defaultMaxTokens = 4096

type Provider struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// NewProvider builds a Claude provider. Options carry the resolved API key and
// base URL; the composition root maps config into them.
func NewProvider(opts aiutil.Options, client *http.Client) *Provider {
	if client == nil {
		client = http.DefaultClient
	}
	return &Provider{
		baseURL: strings.TrimRight(strings.TrimSpace(opts.BaseURL), "/"),
		apiKey:  strings.TrimSpace(opts.APIKey),
		client:  client,
	}
}

func (p *Provider) CompletePrompt(ctx context.Context, request models.PromptCompletionRequest) (string, error) {
	if p.apiKey == "" {
		return "", errors.New("anthropic api key is not configured")
	}
	if err := aiutil.EnsureHTTPS(p.baseURL); err != nil {
		return "", err
	}
	if strings.TrimSpace(request.Model) == "" {
		return "", errors.New("anthropic model is not configured")
	}

	// The current Claude models reject temperature/top_p, so the prompt is the
	// only steering lever — we deliberately do not send sampling parameters.
	payload := map[string]any{
		"model":      strings.TrimSpace(request.Model),
		"max_tokens": defaultMaxTokens,
		"messages": []map[string]string{
			{"role": "user", "content": strings.TrimSpace(request.Prompt)},
		},
	}
	if systemPrompt := strings.TrimSpace(request.SystemPrompt); systemPrompt != "" {
		payload["system"] = systemPrompt
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", errors.Wrap(err, "marshal anthropic request")
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return "", errors.Wrap(err, "create anthropic request")
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("X-Api-Key", p.apiKey)
	httpRequest.Header.Set("Anthropic-Version", anthropicVersion)

	response, err := p.client.Do(httpRequest)
	if err != nil {
		return "", errors.Wrap(err, "call anthropic")
	}
	defer response.Body.Close()

	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
		Error      *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&parsed); err != nil {
		return "", errors.Wrap(err, "decode anthropic response")
	}
	if response.StatusCode >= http.StatusBadRequest {
		if parsed.Error != nil && parsed.Error.Message != "" {
			return "", errors.Errorf("anthropic request failed: %s", parsed.Error.Message)
		}
		return "", errors.Errorf("anthropic request failed with status %s", response.Status)
	}
	if parsed.StopReason == "refusal" {
		return "", errors.New("anthropic declined to generate a response for this request")
	}

	texts := make([]string, 0, len(parsed.Content))
	for _, block := range parsed.Content {
		if block.Type != "text" {
			continue // skip thinking / tool blocks
		}
		if text := strings.TrimSpace(block.Text); text != "" {
			texts = append(texts, text)
		}
	}
	if len(texts) == 0 {
		return "", errors.New("anthropic response did not include text content")
	}
	return strings.Join(texts, "\n"), nil
}
