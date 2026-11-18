package htmlmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToMarkdownKeepsImportantContentDropsChrome(t *testing.T) {
	t.Parallel()
	html := `<!doctype html><html><head><title>Yandex ID</title>
<style>body{margin:0}</style><script>alert(1)</script></head>
<body><h1>Passwords</h1><p>You created an <strong>app</strong> password.</p>
<p>Copy it: <b>abcd</b></p><img src="logo.png" alt="logo">
<a href="https://id.yandex.ru">Open</a>
<ul><li>One</li><li>Two</li></ul></body></html>`

	out := ToMarkdown(html)

	assert.Contains(t, out, "# Passwords")
	assert.Contains(t, out, "**app**")
	assert.Contains(t, out, "Copy it: **abcd**")
	assert.Contains(t, out, "[Open](https://id.yandex.ru)")
	assert.Contains(t, out, "- One")
	assert.Contains(t, out, "- Two")

	// Chrome is dropped entirely.
	assert.NotContains(t, out, "<")
	assert.NotContains(t, out, "alert(1)")
	assert.NotContains(t, out, "margin:0")
	assert.NotContains(t, out, "logo.png")
}

func TestToMarkdownSingleSpacesBlocks(t *testing.T) {
	t.Parallel()
	// Blocks are separated by a single newline, so a document that wraps every
	// line in its own <p>/<div> is not double-spaced with a blank line each time.
	out := ToMarkdown("<div><p>a</p></div><div><div><p>b</p></div></div>")
	assert.NotContains(t, out, "\n\n")
	assert.Equal(t, "a\nb", out)
}

func TestToMarkdownParagraphLinesAreNotDoubleSpaced(t *testing.T) {
	t.Parallel()
	out := ToMarkdown("<p>line one</p><p>line two</p><p>line three</p>")
	assert.Equal(t, "line one\nline two\nline three", out)
}

func TestToMarkdownIsUniformlySingleSpaced(t *testing.T) {
	t.Parallel()
	// Every line pitch is a single newline — no blank lines anywhere, including
	// around headings (the "#" prefix still marks them).
	out := ToMarkdown("<p>intro</p><h2>Title</h2><p>body</p><br><br><p>end</p>")
	assert.NotContains(t, out, "\n\n", "the body must never be double-spaced")
	assert.Equal(t, "intro\n## Title\nbody\nend", out)
}

func TestToMarkdownDropsUnsafeLinks(t *testing.T) {
	t.Parallel()
	out := ToMarkdown(`<a href="javascript:steal()">click</a> <a href="cid:x">img</a>`)
	assert.Contains(t, out, "click")
	assert.Contains(t, out, "img")
	assert.NotContains(t, out, "javascript:")
	assert.NotContains(t, out, "cid:")
}

func TestLooksLikeHTML(t *testing.T) {
	t.Parallel()
	assert.True(t, LooksLikeHTML("<!doctype html><html>"))
	assert.True(t, LooksLikeHTML("hello <br> world"))
	assert.True(t, LooksLikeHTML("<div>x</div>"))
	assert.False(t, LooksLikeHTML("just plain text, no tags"))
	assert.False(t, LooksLikeHTML(""))
}

func TestToMarkdownFallsBackOnPlainText(t *testing.T) {
	t.Parallel()
	out := ToMarkdown("no tags here")
	assert.Equal(t, "no tags here", strings.TrimSpace(out))
}
