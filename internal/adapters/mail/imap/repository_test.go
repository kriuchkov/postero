package imap

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	goimap "github.com/emersion/go-imap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvelopeMessageID(t *testing.T) {
	t.Parallel()

	t.Run("prefers rfc message id", func(t *testing.T) {
		t.Parallel()
		envelope := &goimap.Envelope{MessageId: " <abc@example.com> "}
		assert.Equal(t, "<abc@example.com>", envelopeMessageID(envelope, 100, 7, 1))
	})

	t.Run("falls back to stable uid id", func(t *testing.T) {
		t.Parallel()
		envelope := &goimap.Envelope{}
		first := envelopeMessageID(envelope, 100, 7, 1)
		second := envelopeMessageID(envelope, 100, 7, 2)
		assert.Equal(t, "imap-100-7", first)
		assert.Equal(t, first, second, "same uidvalidity+uid must map to the same id across syncs")
	})

	t.Run("uses sequence number without uid", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "imap-100-seq-3", envelopeMessageID(nil, 100, 0, 3))
	})
}

const crlf = "\r\n"

func mimeMessage(lines ...string) string {
	return strings.Join(lines, crlf) + crlf
}

func TestReadMessageBodyPlainText(t *testing.T) {
	t.Parallel()

	raw := mimeMessage(
		"From: alice@example.com",
		"To: bob@example.com",
		"Subject: Plain",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"Hello, plain world.",
	)

	body, html, atts, err := readMessageBody(strings.NewReader(raw))
	require.NoError(t, err)
	assert.Equal(t, "Hello, plain world."+crlf, body)
	assert.Empty(t, html)
	assert.Empty(t, atts)
}

func TestReadMessageBodyMultipartAlternative(t *testing.T) {
	t.Parallel()

	raw := mimeMessage(
		"From: alice@example.com",
		"Subject: Alt",
		"MIME-Version: 1.0",
		`Content-Type: multipart/alternative; boundary="alt"`,
		"",
		"--alt",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"plain variant",
		"--alt",
		"Content-Type: text/html; charset=utf-8",
		"",
		"<p>rich variant</p>",
		"--alt--",
	)

	body, html, atts, err := readMessageBody(strings.NewReader(raw))
	require.NoError(t, err)
	assert.Contains(t, body, "plain variant")
	assert.Contains(t, html, "<p>rich variant</p>")
	assert.Empty(t, atts)
}

func TestReadMessageBodyFirstTextPartWins(t *testing.T) {
	t.Parallel()

	raw := mimeMessage(
		"Subject: Two texts",
		"MIME-Version: 1.0",
		`Content-Type: multipart/mixed; boundary="mix"`,
		"",
		"--mix",
		"Content-Type: text/plain",
		"",
		"first",
		"--mix",
		"Content-Type: text/plain",
		"",
		"second",
		"--mix--",
	)

	body, _, _, err := readMessageBody(strings.NewReader(raw))
	require.NoError(t, err)
	assert.Contains(t, body, "first")
	assert.NotContains(t, body, "second")
}

func TestReadMessageBodyBase64Attachment(t *testing.T) {
	t.Parallel()

	payload := base64.StdEncoding.EncodeToString([]byte("attachment payload"))
	raw := mimeMessage(
		"Subject: With attachment",
		"MIME-Version: 1.0",
		`Content-Type: multipart/mixed; boundary="mix"`,
		"",
		"--mix",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"See attached.",
		"--mix",
		"Content-Type: application/octet-stream",
		"Content-Transfer-Encoding: base64",
		`Content-Disposition: attachment; filename="report.bin"`,
		"",
		payload,
		"--mix--",
	)

	body, _, atts, err := readMessageBody(strings.NewReader(raw))
	require.NoError(t, err)
	assert.Contains(t, body, "See attached.")
	require.Len(t, atts, 1)
	assert.Equal(t, "report.bin", atts[0].Filename)
	assert.Equal(t, "application/octet-stream", atts[0].MimeType)
	assert.Equal(t, []byte("attachment payload"), atts[0].Data)
	assert.Equal(t, int64(len("attachment payload")), atts[0].Size)
}

func TestReadMessageBodyAttachmentWithoutFilename(t *testing.T) {
	t.Parallel()

	raw := mimeMessage(
		"Subject: Nameless",
		"MIME-Version: 1.0",
		`Content-Type: multipart/mixed; boundary="mix"`,
		"",
		"--mix",
		"Content-Type: text/plain",
		"",
		"body",
		"--mix",
		"Content-Type: application/pdf",
		"Content-Disposition: attachment",
		"",
		"%PDF-fake",
		"--mix--",
	)

	_, _, atts, err := readMessageBody(strings.NewReader(raw))
	require.NoError(t, err)
	require.Len(t, atts, 1)
	assert.Equal(t, "unnamed_attachment", atts[0].Filename)
}

func TestReadMessageBodyQuotedPrintable(t *testing.T) {
	t.Parallel()

	raw := mimeMessage(
		"Subject: QP",
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
		"Content-Transfer-Encoding: quoted-printable",
		"",
		"Caf=C3=A9 =E2=80=94 encoded",
	)

	body, _, _, err := readMessageBody(strings.NewReader(raw))
	require.NoError(t, err)
	assert.Contains(t, body, "Café — encoded")
}

func TestReadMessageBodyMalformedFallsBackToRaw(t *testing.T) {
	t.Parallel()

	raw := "this is not a mime header" + crlf + "just text"

	body, html, atts, err := readMessageBody(strings.NewReader(raw))
	require.Error(t, err, "malformed input must surface a parse error")
	assert.Empty(t, html)
	assert.Empty(t, atts)
	_ = body // raw fallback content depends on how far the reader got
}

func TestToModelMessageMapsEnvelopeFlagsAndBody(t *testing.T) {
	t.Parallel()

	raw := mimeMessage(
		"Content-Type: text/plain; charset=utf-8",
		"",
		"parsed body",
	)
	section := &goimap.BodySectionName{}
	date := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	imapMsg := &goimap.Message{
		SeqNum:       4,
		Uid:          7,
		Size:         321,
		InternalDate: date,
		Flags:        []string{goimap.SeenFlag, goimap.FlaggedFlag, goimap.AnsweredFlag},
		Envelope: &goimap.Envelope{
			Subject:   "Mapped",
			MessageId: "<mapped@example.com>",
			From: []*goimap.Address{
				{PersonalName: "Alice", MailboxName: "alice", HostName: "example.com"},
			},
			To: []*goimap.Address{
				{MailboxName: "bob", HostName: "example.com"},
				nil,
			},
			Cc: []*goimap.Address{
				{MailboxName: "", HostName: ""},
			},
		},
		Body: map[*goimap.BodySectionName]goimap.Literal{
			section: bytes.NewBufferString(raw),
		},
	}

	result, err := toModelMessage(imapMsg, section, "INBOX", 100)
	require.NoError(t, err)

	assert.Equal(t, "<mapped@example.com>", result.ID)
	assert.Equal(t, uint32(7), result.UID, "the server UID must be kept for write-backs")
	assert.Equal(t, "INBOX", result.Mailbox)
	assert.Equal(t, "<mapped@example.com>", result.ThreadID)
	assert.Equal(t, "Mapped", result.Subject)
	assert.Equal(t, "Alice <alice@example.com>", result.From)
	assert.Equal(t, []string{"bob@example.com"}, result.To)
	assert.Empty(t, result.Cc, "empty addresses must be dropped")
	assert.Contains(t, result.Body, "parsed body")
	assert.Equal(t, date, result.Date)
	assert.Equal(t, int64(321), result.Size)
	assert.True(t, result.IsRead)
	assert.True(t, result.IsStarred)
	assert.False(t, result.IsDraft)
	assert.False(t, result.IsDeleted)
	assert.True(t, result.Flags.Answered)
	assert.True(t, result.Flags.Seen)
}

func TestToModelMessageWithoutEnvelopeFails(t *testing.T) {
	t.Parallel()

	_, err := toModelMessage(nil, &goimap.BodySectionName{}, "INBOX", 1)
	require.Error(t, err)

	_, err = toModelMessage(&goimap.Message{}, &goimap.BodySectionName{}, "INBOX", 1)
	require.Error(t, err)
}

func TestToModelMessageFallbackIDAndThread(t *testing.T) {
	t.Parallel()

	imapMsg := &goimap.Message{
		SeqNum:   2,
		Uid:      9,
		Envelope: &goimap.Envelope{Subject: "No message id"},
	}

	result, err := toModelMessage(imapMsg, &goimap.BodySectionName{}, "INBOX", 55)
	require.NoError(t, err)
	assert.Equal(t, "imap-55-9", result.ID)
	assert.Equal(t, "imap-55-9", result.ThreadID, "thread id must fall back to the stable id")
}

func TestConnectRefusesPlaintextForNonLoopback(t *testing.T) {
	t.Parallel()
	repo := &Repository{}
	err := repo.Connect(t.Context(), "imap.example.com", 143, "user", "pass", "plain", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cleartext")
	assert.False(t, repo.IsConnected())
}

func TestIsLoopbackHost(t *testing.T) {
	t.Parallel()
	assert.True(t, isLoopbackHost("localhost"))
	assert.True(t, isLoopbackHost("127.0.0.1"))
	assert.True(t, isLoopbackHost("::1"))
	assert.False(t, isLoopbackHost("imap.gmail.com"))
	assert.False(t, isLoopbackHost("8.8.8.8"))
}
