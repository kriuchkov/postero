package imap

import (
	"context"
	"crypto/tls"
	stderrors "errors"
	"fmt"
	"io"
	"math"
	"net"
	"slices"
	"strconv"
	"strings"

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

func (a *xoauth2Client) Start() (string, []byte, error) {
	str := fmt.Sprintf("user=%s\x01auth=Bearer %s\x01\x01", a.username, a.token)
	return "XOAUTH2", []byte(str), nil
}

func (a *xoauth2Client) Next(_ []byte) ([]byte, error) {
	return nil, nil
}

// Connect establishes a connection to the IMAP server
//
//nolint:nestif // auth flow is clearer as a linear branch on transport and auth type.
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
		// Without TLS the LOGIN/XOAUTH2 credentials and all fetched mail travel in
		// cleartext. Only permit that for loopback hosts (local testing); refuse
		// for any real server rather than silently exposing the password.
		if !isLoopbackHost(host) {
			return errors.Errorf(
				"refusing to connect to IMAP %q without TLS: credentials would be sent in cleartext — enable tls for this account",
				host,
			)
		}
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
	if fetchCountU32, ok := intToUint32(fetchCount); ok && fetchCountU32 > 0 && fetchCountU32 < to {
		from = to - fetchCountU32 + 1
	}

	seqset := new(goimap.SeqSet)
	seqset.AddRange(from, to)

	section := &goimap.BodySectionName{}
	items := []goimap.FetchItem{
		goimap.FetchEnvelope,
		goimap.FetchFlags,
		goimap.FetchInternalDate,
		goimap.FetchRFC822Size,
		goimap.FetchUid,
		section.FetchItem(),
	}
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

		message, convErr := toModelMessage(fetched, section, mailbox, mbox.UidValidity)
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

// MoveToTrash moves one message (by UID within mailbox) into the server's trash
// mailbox and returns the trash mailbox name. go-imap's UidMove uses the MOVE
// extension when the server advertises it and falls back to
// COPY + STORE \Deleted + EXPUNGE otherwise.
func (r *Repository) MoveToTrash(ctx context.Context, mailbox string, uid uint32) (string, error) {
	_ = ctx
	if !r.connected || r.client == nil {
		return "", ErrNotConnected
	}
	if uid == 0 {
		return "", errors.New("message has no imap uid")
	}

	trash, err := r.findTrashMailbox(mailbox)
	if err != nil {
		return "", err
	}

	if _, err := r.client.Select(mailbox, false); err != nil {
		return "", errors.Wrapf(err, "select mailbox %s", mailbox)
	}
	seqset := new(goimap.SeqSet)
	seqset.AddNum(uid)
	if err := r.client.UidMove(seqset, trash); err != nil {
		return "", errors.Wrapf(err, "move uid %d from %s to %s", uid, mailbox, trash)
	}
	return trash, nil
}

// findTrashMailbox resolves the server's trash mailbox: the \Trash special-use
// mailbox if advertised (RFC 6154), else a well-known name that already exists,
// else it creates "Trash". The source mailbox is never a candidate.
func (r *Repository) findTrashMailbox(source string) (string, error) {
	commonTrashNames := []string{
		"Trash", "INBOX.Trash", "INBOX/Trash", "[Gmail]/Trash",
		"Deleted Items", "Deleted Messages",
	}
	ch := make(chan *goimap.MailboxInfo, 32)
	done := make(chan error, 1)
	go func() {
		done <- r.client.List("", "*", ch)
	}()
	var mailboxes []*goimap.MailboxInfo
	for info := range ch {
		mailboxes = append(mailboxes, info)
	}
	if err := <-done; err != nil {
		return "", errors.Wrap(err, "list mailboxes")
	}

	for _, info := range mailboxes {
		if strings.EqualFold(info.Name, source) {
			continue
		}
		if slices.ContainsFunc(info.Attributes, func(attr string) bool {
			return strings.EqualFold(attr, goimap.TrashAttr)
		}) {
			return info.Name, nil
		}
	}
	for _, candidate := range commonTrashNames {
		if strings.EqualFold(candidate, source) {
			continue
		}
		for _, info := range mailboxes {
			if strings.EqualFold(info.Name, candidate) {
				return info.Name, nil
			}
		}
	}

	if err := r.client.Create("Trash"); err != nil {
		return "", errors.Wrap(err, "create Trash mailbox")
	}
	return "Trash", nil
}

func toModelMessage(message *goimap.Message, section *goimap.BodySectionName, mailbox string, uidValidity uint32) (*models.Message, error) {
	if message == nil || message.Envelope == nil {
		return nil, errors.New("imap message envelope is empty")
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
		ID:          envelopeMessageID(message.Envelope, uidValidity, message.Uid, message.SeqNum),
		UID:         message.Uid,
		Mailbox:     mailbox,
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
		data, readErr := io.ReadAll(reader)
		if readErr != nil {
			return "", "", nil, errors.Wrap(readErr, "read raw message body")
		}
		return string(data), "", nil, errors.Wrap(err, "parse message body")
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

func intToUint32(value int) (uint32, bool) {
	if value < 0 || value > math.MaxUint32 {
		return 0, false
	}
	return uint32(value), true
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

// envelopeMessageID prefers the RFC Message-ID; otherwise it derives a stable ID
// from UIDVALIDITY and UID so repeated syncs upsert instead of duplicating.
func envelopeMessageID(envelope *goimap.Envelope, uidValidity, uid, seqNum uint32) string {
	if envelope != nil && strings.TrimSpace(envelope.MessageId) != "" {
		return strings.TrimSpace(envelope.MessageId)
	}
	if uid > 0 {
		return fmt.Sprintf("imap-%d-%d", uidValidity, uid)
	}
	return fmt.Sprintf("imap-%d-seq-%d", uidValidity, seqNum)
}

func hasFlag(flags []string, flag string) bool {
	return slices.Contains(flags, flag)
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
