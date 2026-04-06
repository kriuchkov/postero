package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-faster/errors"

	"github.com/kriuchkov/postero/internal/config"
	"github.com/kriuchkov/postero/internal/core/models"
)

type Provider struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func NewProvider(cfg config.AIProviderConfig, client *http.Client) *Provider {
	if client == nil {
		client = http.DefaultClient
	}
	return &Provider{
		baseURL: strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"),
		apiKey:  strings.TrimSpace(cfg.APIKey),
		client:  client,
	}
}

func (p *Provider) CompletePrompt(ctx context.Context, request models.PromptCompletionRequest) (string, error) {
	if p.apiKey == "" {
		return "", errors.New("openai api key is not configured")
	}
	if strings.TrimSpace(request.Model) == "" {
		return "", errors.New("openai model is not configured")
	}
	body, err := json.Marshal(map[string]any{
		"model":       request.Model,
		"messages":    buildMessages(request),
		"temperature": request.Temperature,
	})
	if err != nil {
		return "", errors.Wrap(err, "marshal openai request")
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", errors.Wrap(err, "create openai request")
	}
	httpRequest.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")

	response, err := p.client.Do(httpRequest)
	if err != nil {
		return "", errors.Wrap(err, "call openai")
	}
	defer response.Body.Close()

	var payload struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return "", errors.Wrap(err, "decode openai response")
	}
	if response.StatusCode >= http.StatusBadRequest {
		if payload.Error != nil && payload.Error.Message != "" {
			return "", errors.Errorf("openai request failed: %s", payload.Error.Message)
		}
		return "", errors.Errorf("openai request failed with status %s", response.Status)
	}
	if len(payload.Choices) == 0 {
		return "", errors.New("openai response did not include choices")
	}
	return strings.TrimSpace(payload.Choices[0].Message.Content), nil
}

func buildMessages(request models.PromptCompletionRequest) []map[string]string {
	messages := make([]map[string]string, 0, 2)
	if systemPrompt := strings.TrimSpace(request.SystemPrompt); systemPrompt != "" {
		messages = append(messages, map[string]string{"role": "system", "content": systemPrompt})
	}
	messages = append(messages, map[string]string{"role": "user", "content": strings.TrimSpace(request.Prompt)})
	return messages
}
