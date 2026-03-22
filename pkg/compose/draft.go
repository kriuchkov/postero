package compose

import (
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/kriuchkov/postero/internal/core/models"
	pformat "github.com/kriuchkov/postero/pkg/format"
)

type Draft struct {
	To       []string
	Cc       []string
	Subject  string
	Body     string
	ThreadID string
}

type ReplyOptions struct {
	ReplyAll bool
	Self     []string
	Body     string
}

func BuildReply(message *models.Message, options ReplyOptions) Draft {
	to := dedupeAddresses([]string{message.From}, options.Self)
	cc := []string{}
	if options.ReplyAll {
		to = dedupeAddresses(append([]string{message.From}, message.To...), options.Self)
		cc = dedupeAddresses(message.Cc, options.Self)
	}

	body := strings.TrimSpace(options.Body)
	if body == "" {
		body = buildQuotedReplyBody(message)
	}

	return Draft{
		To:       to,
		Cc:       cc,
		Subject:  prefixSubject(message.Subject, "Re:"),
		Body:     body,
		ThreadID: threadID(message),
	}
}

func BuildForward(message *models.Message, to []string, body string) Draft {
	forwardBody := strings.TrimSpace(body)
	if forwardBody == "" {
		forwardBody = buildForwardBody(message)
	}

	return Draft{
		To:       dedupeAddresses(to, nil),
		Subject:  prefixSubject(message.Subject, "Fwd:"),
		Body:     forwardBody,
		ThreadID: threadID(message),
	}
}

func prefixSubject(subject, prefix string) string {
	trimmed := strings.TrimSpace(subject)
	if trimmed == "" {
		return prefix
	}

	lowerPrefix := strings.ToLower(prefix)
	if strings.HasPrefix(strings.ToLower(trimmed), lowerPrefix) {
		return trimmed
	}

	return prefix + " " + trimmed
}

func buildQuotedReplyBody(message *models.Message) string {
	timestamp := "an unknown date"
	if !message.Date.IsZero() {
		timestamp = message.Date.Format(time.RFC1123Z)
	}

	from := message.From
	if from == "" {
		from = "unknown sender"
	}

	quoted := quoteText(message.Body)
	if quoted == "" {
		quoted = ">"
	}

	return fmt.Sprintf("\n\nOn %s, %s wrote:\n%s", timestamp, from, quoted)
}

func buildForwardBody(message *models.Message) string {
	dateLine := ""
	if !message.Date.IsZero() {
		dateLine = message.Date.Format(time.RFC1123Z)
	}

	return strings.TrimSpace(fmt.Sprintf(`

---------- Forwarded message ----------
From: %s
Date: %s
Subject: %s
To: %s
Cc: %s

%s`, message.From, dateLine, message.Subject, strings.Join(message.To, ", "), strings.Join(message.Cc, ", "), message.Body))
}

func quoteText(body string) string {
	if strings.TrimSpace(body) == "" {
		return ""
	}

	lines := strings.Split(body, "\n")
	quoted := make([]string, 0, len(lines))
	for _, line := range lines {
		quoted = append(quoted, "> "+line)
	}
	return strings.Join(quoted, "\n")
}

func threadID(message *models.Message) string {
	if message.ThreadID != "" {
		return message.ThreadID
	}
	if message.ID != "" {
		return message.ID
	}
	return fmt.Sprintf("thread-%d", time.Now().UnixNano())
}

func dedupeAddresses(values []string, self []string) []string {
	seen := make(map[string]struct{})
	for _, value := range self {
		for _, address := range expandAddresses(value) {
			seen[addressKey(address)] = struct{}{}
		}
	}

	result := make([]string, 0, len(values))
	for _, value := range values {
		for _, address := range expandAddresses(value) {
			key := addressKey(address)
			if key == "" {
				continue
			}
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, pformat.AddressForHumans(address))
		}
	}

	return result
}

func expandAddresses(value string) []*mail.Address {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}

	list, err := mail.ParseAddressList(trimmed)
	if err == nil {
		return list
	}

	address, err := mail.ParseAddress(trimmed)
	if err == nil {
		return []*mail.Address{address}
	}

	return []*mail.Address{{Address: trimmed}}
}

func addressKey(address *mail.Address) string {
	if address == nil {
		return ""
	}
	if address.Address != "" {
		return strings.ToLower(strings.TrimSpace(address.Address))
	}
	return strings.ToLower(strings.TrimSpace(address.Name))
}
