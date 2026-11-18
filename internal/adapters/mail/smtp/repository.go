package smtp

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	gomail "github.com/emersion/go-message/mail"
	"github.com/go-faster/errors"

	"github.com/kriuchkov/postero/internal/core/models"
	"github.com/kriuchkov/postero/internal/core/ports"
	"github.com/kriuchkov/postero/pkg/mdhtml"
)

type Repository struct {
	host      string
	port      int
	username  string
	password  string
	authType  string
	useTLS    bool
	connected bool
}

func NewRepository() ports.SMTPRepository {
	return &Repository{}
}

func (r *Repository) Connect(ctx context.Context, host string, port int, username, password string, authType string, useTLS bool) error {
	_ = ctx
	r.host = host
	r.port = port
	r.username = username
	r.password = password
	r.authType = authType
	r.useTLS = useTLS
	r.connected = true
	return nil
}

func (r *Repository) Disconnect(ctx context.Context) error {
	_ = ctx
	r.connected = false
	return nil
}

func (r *Repository) IsConnected() bool {
	return r.connected
}

func (r *Repository) Send(ctx context.Context, message *models.Message) error {
	if !r.connected {
		return errors.New("smtp repository is not connected")
	}
	if strings.TrimSpace(r.host) == "" || r.port == 0 {
		return errors.New("smtp host or port is not configured")
	}
	if message == nil {
		return errors.New("message is nil")
	}

	from := strings.TrimSpace(message.From)
	if from == "" {
		from = strings.TrimSpace(r.username)
	}
	if from == "" {
		return errors.New("message sender is empty")
	}

	recipients := collectRecipients(message)
	if len(recipients) == 0 {
		return errors.New("message has no recipients")
	}

	payload, err := buildMessagePayload(from, message)
	if err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if r.useTLS && r.port == 465 {
		return r.sendDirectTLS(from, recipients, payload)
	}
	return r.sendSMTP(from, recipients, payload)
}

func (r *Repository) sendDirectTLS(from string, recipients []string, payload []byte) error {
	address := net.JoinHostPort(r.host, strconv.Itoa(r.port))
	conn, err := (&tls.Dialer{Config: &tls.Config{ServerName: r.host, MinVersion: tls.VersionTLS12}}).DialContext(
		context.Background(),
		"tcp",
		address,
	)
	if err != nil {
		return errors.Wrap(err, "dial smtp tls")
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, r.host)
	if err != nil {
		return errors.Wrap(err, "create smtp client")
	}
	defer func() {
		_ = client.Quit()
	}()

	if err := r.authenticate(client); err != nil {
		return err
	}
	return sendEnvelope(client, from, recipients, payload)
}

func (r *Repository) sendSMTP(from string, recipients []string, payload []byte) error {
	address := net.JoinHostPort(r.host, strconv.Itoa(r.port))
	client, err := smtp.Dial(address)
	if err != nil {
		return errors.Wrap(err, "dial smtp")
	}
	defer func() {
		_ = client.Quit()
	}()

	if r.useTLS {
		// Require STARTTLS; if the server does not offer it (or a MITM stripped it),
		// abort rather than fall through to cleartext AUTH + message body.
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return errors.Errorf(
				"smtp server %q does not offer STARTTLS: refusing to send credentials and email in cleartext",
				r.host,
			)
		}
		if err := client.StartTLS(&tls.Config{ServerName: r.host, MinVersion: tls.VersionTLS12}); err != nil {
			return errors.Wrap(err, "starttls")
		}
	} else if !isLoopbackHost(r.host) {
		// No TLS at all: credentials and the message would be sent in cleartext.
		// Permit only loopback hosts (local testing).
		return errors.Errorf(
			"refusing to send to SMTP %q without TLS: credentials and email would be sent in cleartext — enable tls for this account",
			r.host,
		)
	}

	if err := r.authenticate(client); err != nil {
		return err
	}
	return sendEnvelope(client, from, recipients, payload)
}

// isLoopbackHost reports whether host is localhost or a loopback IP, where a
// plaintext connection cannot be observed by a network attacker.
func isLoopbackHost(host string) bool {
	h := strings.TrimSpace(strings.ToLower(host))
	if h == "localhost" {
		return true
	}
	if ip := net.ParseIP(h); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

func (r *Repository) authenticate(client *smtp.Client) error {
	if strings.TrimSpace(r.username) == "" || strings.TrimSpace(r.password) == "" {
		return nil
	}
	if ok, _ := client.Extension("AUTH"); !ok {
		return nil
	}

	var auth smtp.Auth
	if r.authType == "oauth2" {
		// Use local implementation of XOAUTH2 since standard go library doesn't have it
		// For simplicity, we can pass our own Auth or go-sasl's sasl.NewXoauth2Client but go's native smtp expects smtp.Auth.
		// wait, actually go-sasl has sasl.Client which is not smtp.Auth.
		// Let me just write an inline smtp.Auth wrapper for xoauth2.
		auth = &xoauth2Auth{username: r.username, token: r.password}
	} else {
		auth = smtp.PlainAuth("", r.username, r.password, r.host)
	}

	if err := client.Auth(auth); err != nil {
		return errors.Wrap(err, "smtp auth")
	}
	return nil
}

type xoauth2Auth struct {
	username string
	token    string
}

func (a *xoauth2Auth) Start(_ *smtp.ServerInfo) (string, []byte, error) {
	resp := fmt.Appendf(nil, "user=%s\x01auth=Bearer %s\x01\x01", a.username, a.token)
	return "XOAUTH2", resp, nil
}

func (a *xoauth2Auth) Next(_ []byte, more bool) ([]byte, error) {
	if more {
		// We shouldn't need a second step for XOAUTH2 on success, but if failure, server may send error JSON.
		return []byte{}, nil
	}
	return nil, nil
}

func sendEnvelope(client *smtp.Client, from string, recipients []string, payload []byte) error {
	if err := client.Mail(from); err != nil {
		return errors.Wrap(err, "smtp mail from")
	}
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return errors.Wrapf(err, "smtp rcpt %s", recipient)
		}
	}

	writer, err := client.Data()
	if err != nil {
		return errors.Wrap(err, "smtp data")
	}
	if _, err := writer.Write(payload); err != nil {
		_ = writer.Close()
		return errors.Wrap(err, "write payload")
	}
	if err := writer.Close(); err != nil {
		return errors.Wrap(err, "close data writer")
	}
	return nil
}

func buildMessagePayload(from string, message *models.Message) ([]byte, error) {
	// When the body is Markdown, render an HTML alternative so recipients see
	// formatted text (e.g. "## Hello" as a heading) instead of raw Markdown. The
	// plain body is kept as well via multipart/alternative. Plain bodies stay
	// plain (single text/plain part).
	if strings.TrimSpace(message.HTML) == "" && mdhtml.LooksLikeMarkdown(message.Body) {
		if rendered := mdhtml.ToHTML(message.Body); rendered != "" {
			clone := *message
			clone.HTML = rendered
			message = &clone
		}
	}

	if len(message.Attachments) == 0 && strings.TrimSpace(message.HTML) == "" {
		return buildPlainPayload(from, message)
	}
	return buildMIMEPayload(from, message)
}

func buildPlainPayload(from string, message *models.Message) ([]byte, error) {
	var buffer bytes.Buffer
	headers := map[string]string{
		"Date":         time.Now().Format(time.RFC1123Z),
		"From":         from,
		"To":           strings.Join(message.To, ", "),
		"Cc":           strings.Join(message.Cc, ", "),
		"Subject":      message.Subject,
		"Message-Id":   "<" + messageID(from, message) + ">",
		"MIME-Version": "1.0",
		"Content-Type": "text/plain; charset=UTF-8",
	}

	for _, key := range []string{"Date", "From", "To", "Cc", "Subject", "Message-Id", "MIME-Version", "Content-Type"} {
		value := sanitizeHeaderValue(headers[key])
		if value == "" {
			continue
		}
		if key == "From" || key == "To" || key == "Cc" {
			if _, err := mail.ParseAddressList(value); err != nil && key != "From" {
				return nil, errors.Wrapf(err, "invalid %s header", strings.ToLower(key))
			}
		}
		buffer.WriteString(key + ": " + value + "\r\n")
	}

	buffer.WriteString("\r\n")
	buffer.WriteString(message.Body)
	if !strings.HasSuffix(message.Body, "\n") {
		buffer.WriteString("\r\n")
	}

	return buffer.Bytes(), nil
}

// buildMIMEPayload renders a multipart message: text (plus optional HTML
// alternative) followed by base64-encoded attachments.
func buildMIMEPayload(from string, message *models.Message) ([]byte, error) {
	var buffer bytes.Buffer
	var header gomail.Header
	header.SetDate(time.Now())
	header.Set("From", from)
	if value := strings.TrimSpace(strings.Join(message.To, ", ")); value != "" {
		if _, err := mail.ParseAddressList(value); err != nil {
			return nil, errors.Wrap(err, "invalid to header")
		}
		header.Set("To", value)
	}
	if value := strings.TrimSpace(strings.Join(message.Cc, ", ")); value != "" {
		if _, err := mail.ParseAddressList(value); err != nil {
			return nil, errors.Wrap(err, "invalid cc header")
		}
		header.Set("Cc", value)
	}
	header.SetSubject(message.Subject)
	header.SetMessageID(messageID(from, message))

	writer, err := gomail.CreateWriter(&buffer, header)
	if err != nil {
		return nil, errors.Wrap(err, "create mime writer")
	}
	if err := writeInlineParts(writer, message); err != nil {
		return nil, err
	}
	if err := writeAttachments(writer, message.Attachments); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, errors.Wrap(err, "close mime writer")
	}
	return buffer.Bytes(), nil
}

func writeInlineParts(writer *gomail.Writer, message *models.Message) error {
	inline, err := writer.CreateInline()
	if err != nil {
		return errors.Wrap(err, "create inline part")
	}
	if err := writeInlinePart(inline, "text/plain", message.Body); err != nil {
		return err
	}
	if strings.TrimSpace(message.HTML) != "" {
		if err := writeInlinePart(inline, "text/html", message.HTML); err != nil {
			return err
		}
	}
	if err := inline.Close(); err != nil {
		return errors.Wrap(err, "close inline part")
	}
	return nil
}

func writeInlinePart(inline *gomail.InlineWriter, contentType, body string) error {
	var header gomail.InlineHeader
	header.SetContentType(contentType, map[string]string{"charset": "utf-8"})
	part, err := inline.CreatePart(header)
	if err != nil {
		return errors.Wrapf(err, "create %s part", contentType)
	}
	if _, err := part.Write([]byte(body)); err != nil {
		return errors.Wrapf(err, "write %s part", contentType)
	}
	if err := part.Close(); err != nil {
		return errors.Wrapf(err, "close %s part", contentType)
	}
	return nil
}

func writeAttachments(writer *gomail.Writer, attachments []*models.Attachment) error {
	for _, attachment := range attachments {
		if attachment == nil || len(attachment.Data) == 0 {
			continue
		}
		mimeType := strings.TrimSpace(attachment.MimeType)
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		filename := strings.TrimSpace(attachment.Filename)
		if filename == "" {
			filename = "attachment"
		}
		var attachmentHeader gomail.AttachmentHeader
		attachmentHeader.SetContentType(mimeType, nil)
		attachmentHeader.SetFilename(filename)
		attachmentWriter, err := writer.CreateAttachment(attachmentHeader)
		if err != nil {
			return errors.Wrapf(err, "create attachment %s", filename)
		}
		if _, err := attachmentWriter.Write(attachment.Data); err != nil {
			return errors.Wrapf(err, "write attachment %s", filename)
		}
		if err := attachmentWriter.Close(); err != nil {
			return errors.Wrapf(err, "close attachment %s", filename)
		}
	}
	return nil
}

// sanitizeHeaderValue strips CR/LF so message-supplied values (e.g. Subject)
// cannot inject additional SMTP headers.
func sanitizeHeaderValue(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.TrimSpace(value)
}

// messageID derives a stable RFC Message-ID (without angle brackets) from the
// draft ID and the sender domain so synced copies keep the same identity.
func messageID(from string, message *models.Message) string {
	domain := "postero.local"
	if address, err := mail.ParseAddress(from); err == nil {
		if at := strings.LastIndex(address.Address, "@"); at >= 0 && at < len(address.Address)-1 {
			domain = address.Address[at+1:]
		}
	}
	local := strings.TrimSpace(message.ID)
	// Synced drafts can carry an RFC Message-ID as their store ID; strip the
	// delimiters so the generated header stays well-formed.
	local = strings.Trim(local, "<>")
	local = sanitizeHeaderValue(strings.ReplaceAll(local, " ", "-"))
	if local == "" {
		local = fmt.Sprintf("msg-%d", time.Now().UnixNano())
	}
	if strings.Contains(local, "@") {
		// Already a full msg-id (e.g. inherited from a synced draft).
		return local
	}
	return local + "@" + domain
}

func collectRecipients(message *models.Message) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(message.To)+len(message.Cc)+len(message.Bcc))
	for _, raw := range append(append([]string{}, message.To...), append(message.Cc, message.Bcc...)...) {
		addresses, err := mail.ParseAddressList(raw)
		if err != nil {
			address, addrErr := mail.ParseAddress(strings.TrimSpace(raw))
			if addrErr == nil {
				addresses = []*mail.Address{address}
			}
		}
		for _, address := range addresses {
			key := strings.ToLower(strings.TrimSpace(address.Address))
			if key == "" {
				continue
			}
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, address.Address)
		}
	}
	return result
}
