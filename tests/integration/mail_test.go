package integration

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/kriuchkov/postero/internal/adapters/mail/imap"
	"github.com/kriuchkov/postero/internal/adapters/mail/smtp"
	"github.com/kriuchkov/postero/internal/core/models"
)

func setupGreenMail(t *testing.T) (testcontainers.Container, int, int) {
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "greenmail/standalone:2.1.8",
		ExposedPorts: []string{"3025/tcp", "3143/tcp", "8080/tcp"},
		Env: map[string]string{
			"GREENMAIL_OPTS": "-Dgreenmail.setup.test.smtp -Dgreenmail.setup.test.imap -Dgreenmail.hostname=0.0.0.0 -Dgreenmail.users=tester:secret@test.local -Dgreenmail.users.login=email",
		},
		WaitingFor: wait.ForHTTP("/api/service/readiness").WithPort("8080/tcp").WithStartupTimeout(2 * time.Minute),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)
	_ = host

	smtpPortObj, err := container.MappedPort(ctx, "3025")
	require.NoError(t, err)

	imapPortObj, err := container.MappedPort(ctx, "3143")
	require.NoError(t, err)

	smtpPort, _ := strconv.Atoi(smtpPortObj.Port())
	imapPort, _ := strconv.Atoi(imapPortObj.Port())

	return container, smtpPort, imapPort
}

func TestMailIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	container, smtpPort, imapPort := setupGreenMail(t)
	defer func() {
		require.NoError(t, container.Terminate(context.Background()))
	}()

	ctx := context.Background()

	// Test Negative Scenario: Bad Login
	t.Run("InvalidIMAPLogin", func(t *testing.T) {
		imapRepo := imap.NewRepository()
		err := imapRepo.Connect(ctx, "127.0.0.1", imapPort, "tester@test.local", "badpassword", "plain", false)
		require.Error(t, err, "Expected error on connect with bad password")
		assert.Contains(t, err.Error(), "LOGIN failed", "Error should indicate auth failure")
	})

	t.Run("InvalidSMTPSend", func(t *testing.T) {
		smtpRepo := smtp.NewRepository()
		err := smtpRepo.Connect(ctx, "127.0.0.1", smtpPort, "tester@test.local", "badpassword", "plain", false)
		require.NoError(t, err, "Connect may pass without auth in some implementations")

		msg := &models.Message{
			ID:        "bad-auth-msg",
			AccountID: "local",
			Subject:   "Bad Auth",
			From:      "tester@test.local",
			To:        []string{"tester@test.local"},
		}
		err = smtpRepo.Send(ctx, msg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "smtp auth")
	})

	// Test Negative Scenario: Missing TLS on Greenmail
	t.Run("TLSError", func(t *testing.T) {
		imapRepo := imap.NewRepository()
		// Try to connect with TLS enabled on a non-TLS port
		err := imapRepo.Connect(ctx, "127.0.0.1", imapPort, "tester@test.local", "secret", "plain", true)
		require.Error(t, err)
	})

	// Test Positive Scenario: Send Mail
	t.Run("SendAndSyncMail", func(t *testing.T) {
		smtpRepo := smtp.NewRepository()
		err := smtpRepo.Connect(ctx, "127.0.0.1", smtpPort, "tester@test.local", "secret", "plain", false)
		require.NoError(t, err)

		uid := fmt.Sprintf("test-%d", time.Now().UnixNano())
		msg := &models.Message{
			AccountID: "local",
			ID:        uid,
			Subject:   "Smoke Test",
			From:      "tester@test.local",
			To:        []string{"tester@test.local"},
			Date:      time.Now(),
		}

		err = smtpRepo.Send(ctx, msg)
		require.NoError(t, err)

		err = smtpRepo.Disconnect(ctx)
		require.NoError(t, err)

		// Give greenmail a second to process
		time.Sleep(1 * time.Second)

		// Sync mail
		imapRepo := imap.NewRepository()
		err = imapRepo.Connect(ctx, "127.0.0.1", imapPort, "tester@test.local", "secret", "plain", false)
		require.NoError(t, err)
		defer imapRepo.Disconnect(ctx)

		messages, err := imapRepo.Fetch(ctx, "INBOX", 10)
		require.NoError(t, err)

		assert.NotEmpty(t, messages)

		found := false
		for _, m := range messages {
			if m.Subject == "Smoke Test" {
				found = true
				break
			}
		}
		assert.True(t, found, "Sent message should be found via IMAP")
	})
}
