package imap

import (
	"context"
	"crypto/tls"
	stderrors "errors"
	"fmt"
	"io"
	"math"
	"net"
	"strconv"
	"strings"
	"time"

	imail "github.com/emersion/go-message/mail"

	goimap "github.com/emersion/go-imap"
	imapclient "github.com/emersion/go-imap/client"
	"github.com/go-faster/errors"
	"github.com/kriuchkov/postero/internal/core/models"
	"github.com/kriuchkov/postero/internal/core/ports"
)

// Repository implements the IMAPRepository interface
type Repository struct {
	client    *imapclient.Client
	connected bool
}

// NewRepository creates a new IMAP repository
func NewRepository() ports.IMAPRepository {
	return &Repository{}
}

type xoauth2Client struct {
	username string
	token    string
}

func (a *xoauth2Client) Start() (mech string, ir []byte, err error) {
	str := fmt.Sprintf("user=%s\x01auth=Bearer %s\x01\x01", a.username, a.token)
	return "XOAUTH2", []byte(str), nil
}

func (a *xoauth2Client) Next(challenge []byte) ([]byte, error) {
	return nil, nil
}

// Connect establishes a connection to the IMAP server
func (r *Repository) Connect(ctx context.Context, host string, port int, username, password string, authType string, useTLS bool) error {
	_ = ctx
	address := net.JoinHostPort(host, strconv.Itoa(port))

	var (
		client *imapclient.Client
		err    error
	)
	if useTLS {
		client, err = imapclient.DialTLS(address, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
	} else {
		client, err = imapclient.Dial(address)
	}
	if err != nil {
		return errors.Wrap(err, "dial imap")
	}

	if authType == "oauth2" {
		if err := client.Authenticate(&xoauth2Client{username: username, token: password}); err != nil {
			if logoutErr := client.Logout(); logoutErr != nil {
				return errors.Wrapf(err, "oauth2 authenticate imap (logout failed: %v)", logoutErr)
			}
			return errors.Wrap(err, "oauth2 authenticate imap")
		}
	} else {
		if err := client.Login(username, password); err != nil {
			if logoutErr := client.Logout(); logoutErr != nil {
				return errors.Wrapf(err, "login imap (logout failed: %v)", logoutErr)
			}
			return errors.Wrap(err, "login imap")
		}
	}

	r.client = client
	r.connected = true
	return nil
}

// Disconnect closes the IMAP connection
func (r *Repository) Disconnect(ctx context.Context) error {
	_ = ctx
	if r.client != nil {
		if err := r.client.Logout(); err != nil {
			return err
		}
		r.client = nil
	}
	r.connected = false
	return nil
}

// Fetch retrieves messages from a mailbox
func (r *Repository) Fetch(ctx context.Context, mailbox string, limit int) ([]*models.Message, error) {
	if !r.connected || r.client == nil {
		return nil, ErrNotConnected
	}

	mbox, err := r.client.Select(mailbox, true)
	if err != nil {
		return nil, errors.Wrapf(err, "select mailbox %s", mailbox)
	}
	if mbox.Messages == 0 {
		return []*models.Message{}, nil
	}

	from := uint32(1)
	to := mbox.Messages
	fetchCount := limitOrAll(limit, int(to))
	if fetchCount > 0 && fetchCount < int(to) {
		fetchCountU64 := uint64(fetchCount)
		from = to - uint32(fetchCountU64) + 1
	}

	seqset := new(goimap.SeqSet)
	seqset.AddRange(from, to)

	section := &goimap.BodySectionName{}
	items := []goimap.FetchItem{goimap.FetchEnvelope, goimap.FetchFlags, goimap.FetchInternalDate, goimap.FetchRFC822Size, section.FetchItem()}
	messagesCh := make(chan *goimap.Message, min(limitOrAll(limit, int(to-from+1)), 64))
	errCh := make(chan error, 1)

	go func() {
		errCh <- r.client.Fetch(seqset, items, messagesCh)
	}()

	results := make([]*models.Message, 0)
	for fetched := range messagesCh {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		message, convErr := toModelMessage(fetched, section)
		if convErr != nil {
			return nil, convErr
		}
		results = append(results, message)
	}

	if err := <-errCh; err != nil {
		return nil, errors.Wrap(err, "fetch messages")
	}

	return results, nil
}

// IsConnected returns whether the connection is active
func (r *Repository) IsConnected() bool {
	return r.connected
}

func toModelMessage(message *goimap.Message, section *goimap.BodySectionName) (*models.Message, error) {
	if message == nil || message.Envelope == nil {
		return nil, fmt.Errorf("imap message envelope is empty")
	}

	body := ""
	html := ""
	var atts []*models.Attachment
	if reader := message.GetBody(section); reader != nil {
		parsedBody, parsedHTML, parsedAtts, err := readMessageBody(reader)
		if err != nil {
			return nil, err
		}
		body = parsedBody
		html = parsedHTML
		atts = parsedAtts
	}

	result := &models.Message{
		ID:          envelopeMessageID(message.Envelope, message.SeqNum),
		Subject:     message.Envelope.Subject,
		From:        formatAddresses(message.Envelope.From),
		To:          convertAddresses(message.Envelope.To),
		Cc:          convertAddresses(message.Envelope.Cc),
		Bcc:         convertAddresses(message.Envelope.Bcc),
		Body:        body,
		HTML:        html,
		Attachments: atts,
		Date:        message.InternalDate,
		Size:        int64(message.Size),
		ThreadID:    strings.TrimSpace(message.Envelope.MessageId),
		IsRead:      hasFlag(message.Flags, goimap.SeenFlag),
		IsDraft:     hasFlag(message.Flags, goimap.DraftFlag),
		IsStarred:   hasFlag(message.Flags, goimap.FlaggedFlag),
		IsDeleted:   hasFlag(message.Flags, goimap.DeletedFlag),
	}
	if result.ThreadID == "" {
		result.ThreadID = result.ID
	}
	result.Flags = models.MessageFlags{
		Seen:     result.IsRead,
		Answered: hasFlag(message.Flags, goimap.AnsweredFlag),
		Flagged:  result.IsStarred,
		Draft:    result.IsDraft,
		Deleted:  result.IsDeleted,
		Junk:     false,
	}
	return result, nil
}

func readMessageBody(reader io.Reader) (string, string, []*models.Attachment, error) {
	mr, err := imail.CreateReader(reader)
	if err != nil {
		// Fallback to raw readout
		data, _ := io.ReadAll(reader)
		return string(data), "", nil, nil
	}

	var plainBody, htmlBody string
	var attachments []*models.Attachment

	for {
		p, err := mr.NextPart()
		if stderrors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			continue
		}

		switch h := p.Header.(type) {
		case *imail.InlineHeader:
			contentType, _, _ := h.ContentType()
			b, _ := io.ReadAll(p.Body)

			if strings.HasPrefix(contentType, "text/plain") {
				if plainBody == "" {
					plainBody = string(b)
				}
			} else if strings.HasPrefix(contentType, "text/html") {
				if htmlBody == "" {
					htmlBody = string(b)
				}
			}
		case *imail.AttachmentHeader:
			filename, _ := h.Filename()
			contentType, _, _ := h.ContentType()
			b, _ := io.ReadAll(p.Body)

			if filename == "" {
				filename = "unnamed_attachment"
			}

			attachments = append(attachments, &models.Attachment{
				Filename: filename,
				MimeType: contentType,
				Size:     int64(len(b)),
				Data:     b,
			})
		}
	}

	// If no structured parts but a raw body exists, might be needed,
	// but go-message parses simple emails as single inline part.
	return plainBody, htmlBody, attachments, nil
}

func formatAddresses(addresses []*goimap.Address) string {
	converted := convertAddresses(addresses)
	if len(converted) == 0 {
		return ""
	}
	return converted[0]
}

func convertAddresses(addresses []*goimap.Address) []string {
	result := make([]string, 0, len(addresses))
	for _, address := range addresses {
		if address == nil {
			continue
		}
		email := strings.TrimSpace(address.MailboxName + "@" + address.HostName)
		if email == "@" || email == "" {
			continue
		}
		name := strings.TrimSpace(address.PersonalName)
		if name != "" {
			result = append(result, fmt.Sprintf("%s <%s>", name, email))
			continue
		}
		result = append(result, email)
	}
	return result
}

func envelopeMessageID(envelope *goimap.Envelope, seqNum uint32) string {
	if envelope != nil && strings.TrimSpace(envelope.MessageId) != "" {
		return strings.TrimSpace(envelope.MessageId)
	}
	return fmt.Sprintf("imap-%d-%d", time.Now().UnixNano(), seqNum)
}

func hasFlag(flags []string, flag string) bool {
	for _, candidate := range flags {
		if candidate == flag {
			return true
		}
	}
	return false
}

func limitOrAll(limit, total int) int {
	if total > math.MaxInt32 {
		total = math.MaxInt32
	}
	if limit <= 0 || limit > total {
		return total
	}
	return limit
}

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}
