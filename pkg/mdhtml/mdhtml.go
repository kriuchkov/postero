// Package mdhtml renders a Markdown message body to HTML for the text/html
// alternative part of an outgoing email. It is the inverse of pkg/htmlmd.
package mdhtml

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

// markdownMarker matches the common Markdown constructs Postero renders: ATX
// headings, list items, ordered lists, blockquotes, code fences, bold, and
// links. A body with none of these is sent as plain text only (no HTML part).
var markdownMarker = regexp.MustCompile(
	"(?m)^\\s{0,3}#{1,6}\\s" + // # heading
		"|^\\s{0,3}[-*+]\\s" + // - list item
		"|^\\s{0,3}\\d+\\.\\s" + // 1. ordered list
		"|^\\s{0,3}>\\s" + // > blockquote
		"|```" + // ``` code fence
		"|\\*\\*[^*\\n]+\\*\\*" + // **bold**
		"|\\[[^\\]\\n]+\\]\\([^)\\n]+\\)", // [text](url)
)

// LooksLikeMarkdown reports whether body contains Markdown worth rendering to
// HTML for recipients.
func LooksLikeMarkdown(body string) bool {
	return markdownMarker.MatchString(body)
}

// ToHTML renders Markdown into a self-contained HTML document. Newlines become
// hard breaks so a plain body keeps its line layout. Returns "" for empty input
// or on a render error (the caller then falls back to a plain-text-only message).
func ToHTML(markdown string) string {
	if strings.TrimSpace(markdown) == "" {
		return ""
	}

	converter := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithRendererOptions(html.WithHardWraps()),
	)

	var body bytes.Buffer
	if err := converter.Convert([]byte(markdown), &body); err != nil {
		return ""
	}
	return "<!doctype html>\n<html>\n<body>\n" + body.String() + "</body>\n</html>\n"
}
