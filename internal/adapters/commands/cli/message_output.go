package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/kriuchkov/postero/internal/core/models"
)

const (
	outputFormatText = "text"
	outputFormatJSON = "json"
)

func writeJSON(out io.Writer, value any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func renderMessageSummary(message *models.Message) string {
	status := make([]string, 0, 5)
	if message == nil {
		return "[] read | from=- | subject=(no subject)"
	}
	if !message.IsRead {
		status = append(status, "unread")
	}
	if message.IsStarred {
		status = append(status, "starred")
	}
	if message.IsDraft {
		status = append(status, "draft")
	}
	if message.IsSpam {
		status = append(status, "spam")
	}
	if message.IsDeleted {
		status = append(status, "trash")
	}

	statusText := "read"
	if len(status) > 0 {
		statusText = strings.Join(status, ",")
	}

	from := "-"
	if strings.TrimSpace(message.From) != "" {
		from = message.From
	}
	subject := "(no subject)"
	if strings.TrimSpace(message.Subject) != "" {
		subject = message.Subject
	}

	return fmt.Sprintf("[%s] %s | from=%s | subject=%s", message.ID, statusText, from, subject)
}

func renderMessageDetail(message *models.Message) string {
	if message == nil {
		return ""
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "ID: %s\n", message.ID)
	if strings.TrimSpace(message.AccountID) != "" {
		fmt.Fprintf(&builder, "Account: %s\n", message.AccountID)
	}
	fmt.Fprintf(&builder, "Subject: %s\n", fallbackText(message.Subject, "(no subject)"))
	fmt.Fprintf(&builder, "From: %s\n", fallbackText(message.From, "-"))
	if len(message.To) > 0 {
		fmt.Fprintf(&builder, "To: %s\n", strings.Join(message.To, ", "))
	}
	if len(message.Cc) > 0 {
		fmt.Fprintf(&builder, "Cc: %s\n", strings.Join(message.Cc, ", "))
	}
	if len(message.Bcc) > 0 {
		fmt.Fprintf(&builder, "Bcc: %s\n", strings.Join(message.Bcc, ", "))
	}
	if !message.Date.IsZero() {
		fmt.Fprintf(&builder, "Date: %s\n", message.Date.Format(time.RFC3339))
	}
	fmt.Fprintf(&builder, "Flags: %s\n", renderMessageFlags(message))
	if len(message.Labels) > 0 {
		fmt.Fprintf(&builder, "Labels: %s\n", strings.Join(message.Labels, ", "))
	}
	if len(message.Attachments) > 0 {
		builder.WriteString("Attachments:\n")
		for _, attachment := range message.Attachments {
			if attachment == nil {
				continue
			}
			fmt.Fprintf(&builder, "- %s (%d bytes, %s)\n", attachment.Filename, attachment.Size, attachment.MimeType)
		}
	}

	body := strings.TrimSpace(message.Body)
	if body == "" {
		body = strings.TrimSpace(message.HTML)
	}
	if body != "" {
		builder.WriteString("\n")
		builder.WriteString(body)
		if !strings.HasSuffix(body, "\n") {
			builder.WriteString("\n")
		}
	}

	return builder.String()
}

func renderMessageFlags(message *models.Message) string {
	flags := make([]string, 0, 6)
	if !message.IsRead {
		flags = append(flags, "unread")
	}
	if message.IsStarred {
		flags = append(flags, "starred")
	}
	if message.IsDraft {
		flags = append(flags, "draft")
	}
	if message.IsSpam {
		flags = append(flags, "spam")
	}
	if message.IsDeleted {
		flags = append(flags, "trash")
	}
	if len(flags) == 0 {
		return "read"
	}
	return strings.Join(flags, ", ")
}

func fallbackText(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
