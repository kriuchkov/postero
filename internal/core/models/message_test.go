package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMessageCreation(t *testing.T) {
	msg := &Message{
		ID:      "1",
		Subject: "Test Email",
		From:    "sender@example.com",
		To:      []string{"recipient@example.com"},
		Date:    time.Now(),
		IsRead:  false,
	}

	assert.Equal(t, "1", msg.ID)
	assert.Equal(t, "Test Email", msg.Subject)
	assert.Equal(t, false, msg.IsRead)
}
