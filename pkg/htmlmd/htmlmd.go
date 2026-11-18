// Package htmlmd converts email HTML into readable Markdown-ish plain text.
//
// It is intentionally lossy: it keeps the important content (text, headings,
// links, list items) and drops presentation (images, styles, scripts, tables as
// layout). The goal is a legible message in the terminal, not a faithful render.
package htmlmd

import (
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

var (
	inlineWhitespace = regexp.MustCompile(`[ \t\r\f\v]+`)
	// blankLines collapses every run of line breaks to a single newline, so email
	// HTML that separates lines with double <br>, empty paragraphs, or table cells
	// does not render double-spaced. The whole body reads at a single line pitch.
	blankLines = regexp.MustCompile(`\n{2,}`)
)

// LooksLikeHTML reports whether s appears to contain an HTML document/fragment.
func LooksLikeHTML(s string) bool {
	l := strings.ToLower(s)
	for _, marker := range []string{"<!doctype html", "<html", "<body", "<div", "<table", "<p>", "<p ", "<br"} {
		if strings.Contains(l, marker) {
			return true
		}
	}
	return false
}

// ToMarkdown converts an HTML string into Markdown-ish plain text. On a parse
// error it falls back to the trimmed input.
func ToMarkdown(input string) string {
	doc, err := html.Parse(strings.NewReader(input))
	if err != nil {
		return strings.TrimSpace(input)
	}
	var sb strings.Builder
	renderNode(&sb, doc)
	return normalize(sb.String())
}

func renderNode(sb *strings.Builder, node *html.Node) {
	switch node.Type {
	case html.CommentNode, html.DoctypeNode:
		return
	case html.TextNode:
		sb.WriteString(inlineWhitespace.ReplaceAllString(node.Data, " "))
	case html.ElementNode:
		renderElement(sb, node)
	case html.ErrorNode, html.DocumentNode, html.RawNode:
		renderChildren(sb, node)
	}
}

func renderElement(sb *strings.Builder, node *html.Node) {
	switch node.Data {
	case "script", "style", "head", "title", "noscript", "iframe", "object", "template":
		return // drop non-content subtrees entirely
	case "img":
		return // images are dropped
	case "br":
		sb.WriteString("\n")
	case "hr":
		ensureNewline(sb)
		sb.WriteString("---")
		ensureNewline(sb)
	case "a":
		renderLink(sb, node)
	case "b", "strong":
		renderInlineMark(sb, node, "**")
	case "i", "em":
		renderInlineMark(sb, node, "*")
	case "h1", "h2", "h3", "h4", "h5", "h6":
		ensureNewline(sb)
		sb.WriteString(headingPrefix(node.Data) + " ")
		renderChildren(sb, node)
		ensureNewline(sb)
	case "li":
		ensureNewline(sb)
		sb.WriteString("- ")
		renderChildren(sb, node)
	case "td", "th":
		renderChildren(sb, node)
		sb.WriteString(" ")
	case "tr", "p", "div", "ul", "ol", "blockquote", "section", "article", "header", "footer", "table":
		// Block elements are separated by a single newline (idempotent), so a
		// document that wraps every line in its own block does not become
		// double-spaced with a blank line after every line.
		ensureNewline(sb)
		renderChildren(sb, node)
		ensureNewline(sb)
	default:
		renderChildren(sb, node)
	}
}

// ensureNewline appends a newline only when the buffer does not already end with
// one, so adjacent blocks are separated by exactly one line break, never two.
func ensureNewline(sb *strings.Builder) {
	s := sb.String()
	if len(s) > 0 && s[len(s)-1] != '\n' {
		sb.WriteByte('\n')
	}
}

func renderChildren(sb *strings.Builder, node *html.Node) {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		renderNode(sb, child)
	}
}

func renderInlineMark(sb *strings.Builder, node *html.Node, mark string) {
	var inner strings.Builder
	renderChildren(&inner, node)
	text := strings.TrimSpace(inner.String())
	if text == "" {
		return
	}
	sb.WriteString(mark + text + mark)
}

func renderLink(sb *strings.Builder, node *html.Node) {
	var inner strings.Builder
	renderChildren(&inner, node)
	text := strings.TrimSpace(inner.String())

	href := strings.TrimSpace(attr(node, "href"))
	lower := strings.ToLower(href)
	if href == "" || strings.HasPrefix(lower, "javascript:") || strings.HasPrefix(lower, "cid:") || strings.HasPrefix(lower, "data:") {
		sb.WriteString(text)
		return
	}
	switch text {
	case "":
		sb.WriteString(href)
	case href:
		sb.WriteString(href)
	default:
		sb.WriteString("[" + text + "](" + href + ")")
	}
}

func headingPrefix(tag string) string {
	switch tag {
	case "h1":
		return "#"
	case "h2":
		return "##"
	case "h3":
		return "###"
	case "h4":
		return "####"
	case "h5":
		return "#####"
	default:
		return "######"
	}
}

func attr(node *html.Node, name string) string {
	for _, a := range node.Attr {
		if strings.EqualFold(a.Key, name) {
			return a.Val
		}
	}
	return ""
}

func normalize(text string) string {
	lines := strings.Split(text, "\n")
	for index := range lines {
		lines[index] = strings.TrimRight(inlineWhitespace.ReplaceAllString(lines[index], " "), " ")
		lines[index] = strings.TrimLeft(lines[index], " ")
	}
	joined := strings.Join(lines, "\n")
	joined = blankLines.ReplaceAllString(joined, "\n")
	return strings.TrimSpace(joined)
}
