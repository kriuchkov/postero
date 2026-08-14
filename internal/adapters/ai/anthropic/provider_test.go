package anthropic

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kriuchkov/postero/internal/adapters/ai/aiutil"
	"github.com/kriuchkov/postero/internal/core/models"
)

func TestCompletePromptSendsMessagesRequestAndParsesText(t *testing.T) {
	var gotBody map[string]any
	var gotHeaders http.Header

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		assert.Equal(t, "/v1/messages", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"thinking","text":""},{"type":"text","text":"Draft reply"}],"stop_reason":"end_turn"}`))
	}))
	defer server.Close()

	provider := NewProvider(aiutil.Options{BaseURL: server.URL + "/v1", APIKey: "sk-test"}, server.Client())

	out, err := provider.CompletePrompt(context.Background(), models.PromptCompletionRequest{
		Model:        "claude-opus-5",
		SystemPrompt: "You write replies.",
		Prompt:       "Reply to this email.",
		Temperature:  0.7, // must NOT be forwarded — current Claude models reject it
	})

	require.NoError(t, err)
	assert.Equal(t, "Draft reply", out)

	// Correct auth + version headers.
	assert.Equal(t, "sk-test", gotHeaders.Get("X-Api-Key"))
	assert.Equal(t, anthropicVersion, gotHeaders.Get("Anthropic-Version"))

	// Request shape: model, max_tokens, system, user message; no temperature.
	assert.Equal(t, "claude-opus-5", gotBody["model"])
	assert.Equal(t, "You write replies.", gotBody["system"])
	assert.NotContains(t, gotBody, "temperature")
	assert.Contains(t, gotBody, "max_tokens")
	msgs, ok := gotBody["messages"].([]any)
	require.True(t, ok)
	require.Len(t, msgs, 1)
}

func TestCompletePromptSurfacesRefusal(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"content":[],"stop_reason":"refusal"}`))
	}))
	defer server.Close()

	provider := NewProvider(aiutil.Options{BaseURL: server.URL + "/v1", APIKey: "sk-test"}, server.Client())
	_, err := provider.CompletePrompt(context.Background(), models.PromptCompletionRequest{Model: "claude-opus-5", Prompt: "hi"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "declined")
}

func TestCompletePromptRejectsInsecureBaseURL(t *testing.T) {
	provider := NewProvider(aiutil.Options{BaseURL: "http://api.anthropic.com/v1", APIKey: "sk-test"}, http.DefaultClient)
	_, err := provider.CompletePrompt(context.Background(), models.PromptCompletionRequest{Model: "claude-opus-5", Prompt: "hi"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "https")
}

func TestCompletePromptRequiresConfig(t *testing.T) {
	_, err := NewProvider(aiutil.Options{BaseURL: "https://api.anthropic.com/v1"}, http.DefaultClient).
		CompletePrompt(context.Background(), models.PromptCompletionRequest{Model: "claude-opus-5", Prompt: "hi"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "api key")
}
