package smtp

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kriuchkov/postero/internal/core/models"
)

func TestBuildMessagePayloadPlain(t *testing.T) {
	t.Parallel()

	message := &models.Message{
		ID:      "msg-42",
		To:      []string{"rcpt@example.com"},
		Subject: "Hello",
		Body:    "Plain body",
	}

	payload, err := buildMessagePayload("sender@example.com", message)
	require.NoError(t, err)

	text := string(payload)
	assert.Contains(t, text, "Content-Type: text/plain; charset=UTF-8")
	assert.Contains(t, text, "Message-Id: <msg-42@example.com>")
	assert.Contains(t, text, "Plain body")
	assert.NotContains(t, text, "multipart")
}

func TestBuildMessagePayloadWithAttachment(t *testing.T) {
	t.Parallel()

	message := &models.Message{
		ID:      "msg-43",
		To:      []string{"rcpt@example.com"},
		Subject: "With attachment",
		Body:    "See attached",
		Attachments: []*models.Attachment{
			{Filename: "report.txt", MimeType: "text/plain", Data: []byte("attachment payload")},
		},
	}

	payload, err := buildMessagePayload("sender@example.com", message)
	require.NoError(t, err)

	text := string(payload)
	assert.Contains(t, text, "multipart/mixed")
	assert.Contains(t, text, "report.txt")
	assert.Contains(t, text, "Content-Disposition: attachment")
	assert.Contains(t, text, "See attached")
	assert.Contains(t, strings.ToLower(text), "message-id: <msg-43@example.com>")
}

func TestBuildMessagePayloadWithHTML(t *testing.T) {
	t.Parallel()

	message := &models.Message{
		ID:      "msg-44",
		To:      []string{"rcpt@example.com"},
		Subject: "Rich",
		Body:    "plain variant",
		HTML:    "<p>rich variant</p>",
	}

	payload, err := buildMessagePayload("sender@example.com", message)
	require.NoError(t, err)

	text := string(payload)
	assert.Contains(t, text, "multipart/alternative")
	assert.Contains(t, text, "plain variant")
	assert.Contains(t, text, "rich variant")
}

func TestMessageIDFallsBackWithoutDraftID(t *testing.T) {
	t.Parallel()

	id := messageID("Sender <sender@example.com>", &models.Message{})
	assert.True(t, strings.HasSuffix(id, "@example.com"), id)
	assert.True(t, strings.HasPrefix(id, "msg-"), id)
}

func TestBuildPlainPayloadStripsHeaderInjection(t *testing.T) {
	t.Parallel()

	message := &models.Message{
		ID:      "msg-45",
		To:      []string{"rcpt@example.com"},
		Subject: "Hi\r\nBcc: victim@example.com",
		Body:    "body",
	}

	payload, err := buildMessagePayload("sender@example.com", message)
	require.NoError(t, err)

	text := string(payload)
	// The CRLF must not survive, so no separate Bcc header line can appear.
	assert.NotContains(t, text, "\r\nBcc:")
	assert.Contains(t, text, "Subject: Hi  Bcc: victim@example.com")
}

func TestMessageIDReusesSyncedDraftID(t *testing.T) {
	t.Parallel()

	id := messageID("sender@example.com", &models.Message{ID: "<orig-123@other.host>"})
	assert.Equal(t, "orig-123@other.host", id)
}

func TestBuildMessagePayloadRendersMarkdownToHTML(t *testing.T) {
	t.Parallel()

	message := &models.Message{
		ID:      "msg-md",
		To:      []string{"rcpt@example.com"},
		Subject: "Markdown",
		Body:    "## Hello\n\nThis is **bold**.",
	}

	payload, err := buildMessagePayload("sender@example.com", message)
	require.NoError(t, err)

	text := string(payload)
	// Sent as multipart/alternative with both the raw Markdown and rendered HTML.
	assert.Contains(t, text, "multipart/alternative")
	assert.Contains(t, text, "## Hello", "plain Markdown part is preserved")
	assert.Contains(t, text, "<h2>Hello</h2>", "HTML alternative is rendered from Markdown")
	assert.Contains(t, text, "<strong>bold</strong>")
}
