// Package aiutil holds small helpers shared by the AI provider adapters.
package aiutil

import (
	"net"
	"net/url"
	"strings"

	"github.com/go-faster/errors"
)

// Options is the transport configuration an AI provider adapter needs. It lives
// here rather than in the config package so provider adapters depend only on
// core+pkg; the composition root maps the infrastructure config into it.
type Options struct {
	BaseURL string
	APIKey  string
	// Command is the argv of a local agent CLI for the command provider
	// (e.g. OpenClaw or Claude Code). Empty for HTTP-based providers.
	Command []string
}

// EnsureHTTPS rejects a non-HTTPS AI base URL (unless it targets a loopback host,
// e.g. a local proxy) so email content and API keys are never sent in cleartext.
func EnsureHTTPS(baseURL string) error {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return errors.Wrap(err, "parse ai base_url")
	}
	switch {
	case parsed.Scheme == "https":
		return nil
	case parsed.Scheme == "http" && isLoopbackHost(parsed.Hostname()):
		return nil
	default:
		return errors.Errorf("refusing to use insecure AI base_url %q: only https is allowed (http permitted on loopback only)", baseURL)
	}
}

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
