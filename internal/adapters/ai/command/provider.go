// Package command implements the PromptCompletionProvider port by shelling out to
// a local agent CLI (e.g. OpenClaw, Claude Code, opencode). The prompt is written
// to the process stdin and the completion is read from stdout — no HTTP, no API
// key, and no shell interpretation of the configured argv.
package command

import (
	"bytes"
	"context"
	"os/exec"
	"strings"

	"github.com/go-faster/errors"

	"github.com/kriuchkov/postero/internal/adapters/ai/aiutil"
	"github.com/kriuchkov/postero/internal/core/models"
)

type Provider struct {
	command []string
}

// NewProvider builds a command provider from the configured argv.
func NewProvider(opts aiutil.Options) *Provider {
	return &Provider{command: append([]string(nil), opts.Command...)}
}

func (p *Provider) CompletePrompt(ctx context.Context, request models.PromptCompletionRequest) (string, error) {
	if len(p.command) == 0 {
		return "", errors.New(`command ai provider requires a command, e.g. command: ["openclaw","agent","exec","--message-file","-"]`)
	}

	// Agent CLIs like OpenClaw's `agent exec` take no system-prompt flag, so the
	// system prompt is prepended to the user prompt as a single stdin payload.
	input := strings.TrimSpace(request.Prompt)
	if systemPrompt := strings.TrimSpace(request.SystemPrompt); systemPrompt != "" {
		input = systemPrompt + "\n\n" + input
	}

	// Resolve the binary explicitly and run it via argv — never through a shell —
	// so nothing in the prompt or config is interpreted as a shell command.
	binary, err := exec.LookPath(p.command[0])
	if err != nil {
		return "", errors.Wrapf(err, "ai command %q not found", p.command[0])
	}
	cmd := exec.CommandContext(ctx, binary, p.command[1:]...)
	cmd.Stdin = strings.NewReader(input)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if message := strings.TrimSpace(stderr.String()); message != "" {
			return "", errors.Wrapf(err, "run ai command %q: %s", p.command[0], message)
		}
		return "", errors.Wrapf(err, "run ai command %q", p.command[0])
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return "", errors.New("ai command produced no output")
	}
	return output, nil
}
