package tui

import (
	"bytes"
	"context"
	"os/exec"
	"strings"

	"github.com/kriuchkov/postero/internal/core/models"
)

func (m Model) getFilteredBody(msg *models.MessageDTO) string {
	if m.config == nil {
		return defaultFallback(msg)
	}

	// First, check if HTML exists and we have a text/html filter
	if msg.HTML != "" {
		if cmdStr, ok := m.config.Filters["text/html"]; ok && cmdStr != "" {
			out, err := applyFilterCmd(cmdStr, msg.HTML)
			if err == nil { // On success, return filtered output
				return out
			}
			// If external command failed, we could log it, but for now we fallback
		}
	}

	// Next, check plain text filter
	if msg.Body != "" {
		if cmdStr, ok := m.config.Filters["text/plain"]; ok && cmdStr != "" {
			out, err := applyFilterCmd(cmdStr, msg.Body)
			if err == nil {
				return out
			}
		}
	}

	return defaultFallback(msg)
}

func defaultFallback(msg *models.MessageDTO) string {
	if msg.Body != "" {
		return msg.Body
	}
	if msg.HTML != "" {
		return msg.HTML
	}
	return "No content."
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
