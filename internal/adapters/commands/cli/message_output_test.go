package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kriuchkov/postero/internal/core/models"
)

func TestRenderMessageSummaryIncludesStableFields(t *testing.T) {
	message := &models.Message{
		ID:        "msg-1",
		From:      "sender@example.com",
		Subject:   "Hello",
		IsRead:    false,
		IsStarred: true,
	}

	summary := renderMessageSummary(message)

	assert.Contains(t, summary, "[msg-1]")
	assert.Contains(t, summary, "unread,starred")
	assert.Contains(t, summary, "sender@example.com")
	assert.Contains(t, summary, "Hello")
}

func TestRenderMessageDetailIncludesHeadersBodyAndAttachments(t *testing.T) {
	message := &models.Message{
		ID:        "msg-1",
		AccountID: "personal",
		Subject:   "Hello",
		From:      "sender@example.com",
		To:        []string{"one@example.com", "two@example.com"},
		Labels:    []string{"inbox", "work"},
		Date:      time.Date(2026, time.March, 26, 10, 11, 12, 0, time.UTC),
		Body:      "Message body",
		Attachments: []*models.Attachment{{
			Filename: "notes.txt",
			Size:     42,
			MimeType: "text/plain",
		}},
	}

	detail := renderMessageDetail(message)

	assert.Contains(t, detail, "ID: msg-1")
	assert.Contains(t, detail, "Account: personal")
	assert.Contains(t, detail, "To: one@example.com, two@example.com")
	assert.Contains(t, detail, "Labels: inbox, work")
	assert.Contains(t, detail, "notes.txt (42 bytes, text/plain)")
	assert.True(t, strings.HasSuffix(detail, "Message body\n"))
}

func TestWriteJSONProducesIndentedPayload(t *testing.T) {
	var builder strings.Builder

	err := writeJSON(&builder, map[string]string{"id": "msg-1"})

	require.NoError(t, err)
	assert.Contains(t, builder.String(), "\n  \"id\": \"msg-1\"\n")
}

func TestRenderMessageFlagsDefaultsToRead(t *testing.T) {
	assert.Equal(t, "read", renderMessageFlags(&models.Message{IsRead: true}))
}

func TestRenderMessageSummaryHandlesNilMessage(t *testing.T) {
	assert.Equal(t, "[] read | from=- | subject=(no subject)", renderMessageSummary(nil))
}

func TestFallbackTextUsesFallbackForWhitespace(t *testing.T) {
	assert.Equal(t, "fallback", fallbackText("   ", "fallback"))
}
