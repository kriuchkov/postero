package compose

import (
	"testing"
	"time"

	"github.com/kriuchkov/postero/internal/core/models"
	"github.com/stretchr/testify/assert"
)

func TestBuildReplyReplyAllDedupesSelf(t *testing.T) {
	message := &models.Message{
		ID:      "msg-1",
		Subject: "Status Update",
		From:    "Alice <alice@example.com>",
		To:      []string{"me@example.com", "Bob <bob@example.com>"},
		Cc:      []string{"carol@example.com", "me@example.com"},
		Body:    "hello\nworld",
		Date:    time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
	}

	draft := BuildReply(message, ReplyOptions{ReplyAll: true, Self: []string{"me@example.com"}})

	assert.Equal(t, []string{"Alice <alice@example.com>", "Bob <bob@example.com>"}, draft.To)
	assert.Equal(t, []string{"<carol@example.com>"}, draft.Cc)
	assert.Equal(t, "Re: Status Update", draft.Subject)
	assert.Contains(t, draft.Body, "On Fri, 20 Mar 2026 10:00:00 +0000, Alice <alice@example.com> wrote:")
	assert.Contains(t, draft.Body, "> hello")
	assert.Contains(t, draft.Body, "> world")
}

func TestBuildForwardPrefixesSubjectOnce(t *testing.T) {
	message := &models.Message{Subject: "Fwd: Existing", Body: "payload"}

	draft := BuildForward(message, []string{"user@example.com"}, "")

	assert.Equal(t, "Fwd: Existing", draft.Subject)
	assert.Equal(t, []string{"<user@example.com>"}, draft.To)
	assert.Contains(t, draft.Body, "Forwarded message")
	assert.Contains(t, draft.Body, "payload")
}
