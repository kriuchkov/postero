package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
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
		return "", errors.New("gemini api key is not configured")
	}
	if strings.TrimSpace(request.Model) == "" {
		return "", errors.New("gemini model is not configured")
	}
	bodyPayload := map[string]any{
		"contents": []map[string]any{{
			"role":  "user",
			"parts": []map[string]string{{"text": strings.TrimSpace(request.Prompt)}},
		}},
	}
	if strings.TrimSpace(request.SystemPrompt) != "" {
		bodyPayload["systemInstruction"] = map[string]any{
			"parts": []map[string]string{{"text": strings.TrimSpace(request.SystemPrompt)}},
		}
	}
	if request.Temperature != 0 {
		bodyPayload["generationConfig"] = map[string]any{"temperature": request.Temperature}
	}
	body, err := json.Marshal(bodyPayload)
	if err != nil {
		return "", errors.Wrap(err, "marshal gemini request")
	}

	endpoint := p.baseURL + "/models/" + url.PathEscape(
		strings.TrimSpace(request.Model),
	) + ":generateContent?key=" + url.QueryEscape(
		p.apiKey,
	)
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", errors.Wrap(err, "create gemini request")
	}
	httpRequest.Header.Set("Content-Type", "application/json")

	response, err := p.client.Do(httpRequest)
	if err != nil {
		return "", errors.Wrap(err, "call gemini")
	}
	defer response.Body.Close()

	var payload struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return "", errors.Wrap(err, "decode gemini response")
	}
	if response.StatusCode >= http.StatusBadRequest {
		if payload.Error != nil && payload.Error.Message != "" {
			return "", errors.Errorf("gemini request failed: %s", payload.Error.Message)
		}
		return "", errors.Errorf("gemini request failed with status %s", response.Status)
	}
	if len(payload.Candidates) == 0 {
		return "", errors.New("gemini response did not include candidates")
	}
	parts := payload.Candidates[0].Content.Parts
	texts := make([]string, 0, len(parts))
	for _, part := range parts {
		if text := strings.TrimSpace(part.Text); text != "" {
			texts = append(texts, text)
		}
	}
	if len(texts) == 0 {
		return "", errors.New("gemini response did not include text content")
	}
	return strings.Join(texts, "\n"), nil
}
