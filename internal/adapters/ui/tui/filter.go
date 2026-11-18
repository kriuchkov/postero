package tui

import (
	"bytes"
	"context"
	"os/exec"
	"strings"

	"github.com/kriuchkov/postero/internal/core/models"
	"github.com/kriuchkov/postero/pkg/htmlmd"
)

func (m Model) getFilteredBody(msg *models.Message) string {
	htmlBody, plainBody := msg.HTML, msg.Body
	// Some servers deliver HTML in the plain Body field, or the "plain" alternative
	// is itself HTML. Whenever the plain part looks like HTML, route it through the
	// converter so the reader never shows raw tags.
	if htmlmd.LooksLikeHTML(plainBody) {
		if strings.TrimSpace(htmlBody) == "" {
			htmlBody = plainBody
		}
		plainBody = ""
	}

	// Honour an explicit external filter first (e.g. w3m), if configured.
	if out, ok := m.applyExternalFilter("text/html", htmlBody); ok {
		return out
	}
	if out, ok := m.applyExternalFilter("text/plain", plainBody); ok {
		return out
	}

	// Prefer a plain-text part; otherwise convert HTML into readable Markdown so
	// the reader never shows raw tags.
	if strings.TrimSpace(plainBody) != "" {
		return plainBody
	}
	if strings.TrimSpace(htmlBody) != "" {
		return htmlmd.ToMarkdown(htmlBody)
	}
	return "No content."
}

// applyExternalFilter runs a configured filter command for the given MIME type
// over content, returning (output, true) on success.
func (m Model) applyExternalFilter(mimeType, content string) (string, bool) {
	if m.config == nil || strings.TrimSpace(content) == "" {
		return "", false
	}
	cmdStr, ok := m.config.Filters[mimeType]
	if !ok || cmdStr == "" {
		return "", false
	}
	out, err := applyFilterCmd(cmdStr, content)
	if err != nil {
		return "", false
	}
	return out, true
}

func applyFilterCmd(command string, input string) (string, error) {
	// Simple command split by spaces
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return input, nil
	}

	binary, err := exec.LookPath(parts[0])
	if err != nil {
		return "", err
	}

	cmd := exec.CommandContext(context.Background(), binary, parts[1:]...)
	cmd.Stdin = strings.NewReader(input)

	var out bytes.Buffer
	cmd.Stdout = &out

	err = cmd.Run()
	if err != nil {
		return "", err
	}

	return out.String(), nil
}
