package command

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kriuchkov/postero/internal/adapters/ai/aiutil"
	"github.com/kriuchkov/postero/internal/core/models"
)

func TestCompletePromptPipesPromptToStdinAndReturnsStdout(t *testing.T) {
	// `cat` echoes stdin to stdout — a stand-in for an agent CLI.
	provider := NewProvider(aiutil.Options{Command: []string{"cat"}})

	out, err := provider.CompletePrompt(context.Background(), models.PromptCompletionRequest{
		SystemPrompt: "You write replies.",
		Prompt:       "Reply to this email.",
	})

	require.NoError(t, err)
	// System prompt is prepended, then the user prompt.
	assert.Contains(t, out, "You write replies.")
	assert.Contains(t, out, "Reply to this email.")
}

func TestCompletePromptRequiresCommand(t *testing.T) {
	_, err := NewProvider(aiutil.Options{}).
		CompletePrompt(context.Background(), models.PromptCompletionRequest{Prompt: "hi"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires a command")
}

func TestCompletePromptSurfacesCommandFailure(t *testing.T) {
	// `false` exits non-zero with no output.
	_, err := NewProvider(aiutil.Options{Command: []string{"false"}}).
		CompletePrompt(context.Background(), models.PromptCompletionRequest{Prompt: "hi"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "run ai command")
}

func TestCompletePromptReportsMissingBinary(t *testing.T) {
	_, err := NewProvider(aiutil.Options{Command: []string{"pstr-no-such-agent-binary"}}).
		CompletePrompt(context.Background(), models.PromptCompletionRequest{Prompt: "hi"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
