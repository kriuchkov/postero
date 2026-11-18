package mdhtml

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToHTMLRendersMarkdown(t *testing.T) {
	t.Parallel()
	out := ToHTML("## Hello\n\nThis is **bold** and a [link](https://example.com).\n\n- one\n- two")

	assert.Contains(t, out, "<h2>Hello</h2>")
	assert.Contains(t, out, "<strong>bold</strong>")
	assert.Contains(t, out, `<a href="https://example.com">link</a>`)
	assert.Contains(t, out, "<li>one</li>")
	assert.Contains(t, out, "<html>")
}

func TestToHTMLEmptyInput(t *testing.T) {
	t.Parallel()
	assert.Empty(t, ToHTML(""))
	assert.Empty(t, ToHTML("   \n  "))
}

func TestLooksLikeMarkdown(t *testing.T) {
	t.Parallel()
	assert.True(t, LooksLikeMarkdown("## Hello"))
	assert.True(t, LooksLikeMarkdown("# Title"))
	assert.True(t, LooksLikeMarkdown("- item\n- item"))
	assert.True(t, LooksLikeMarkdown("1. first"))
	assert.True(t, LooksLikeMarkdown("> quote"))
	assert.True(t, LooksLikeMarkdown("some **bold** text"))
	assert.True(t, LooksLikeMarkdown("see [here](https://x.com)"))
	assert.True(t, LooksLikeMarkdown("```\ncode\n```"))

	assert.False(t, LooksLikeMarkdown("Just a plain sentence."))
	assert.False(t, LooksLikeMarkdown("Hello,\n\nThanks for your email.\n\nRegards"))
	assert.False(t, LooksLikeMarkdown(""))
	assert.False(t, LooksLikeMarkdown("a * b * c"), "lone asterisks are not bold")
}
