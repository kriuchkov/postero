package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kriuchkov/postero/internal/core/models"
)

// bigHTMLBody returns a large HTML document whose readable text ("Invoice total")
// sits after a heavy <head>/<style> block — the shape that made the old preview
// path show raw tags and run in O(n²).
func bigHTMLBody() string {
	var b strings.Builder
	b.WriteString(`<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 4.0 Transitional//EN">`)
	b.WriteString("<html><head><title></title><style>td,p,span{color:red;}</style></head><body>")
	b.WriteString("<table><tbody><tr><td>Invoice total 1234 rub</td></tr></tbody></table>")
	b.WriteString(strings.Repeat("<div>filler filler filler</div>", 4000)) // ~120 KB
	b.WriteString("</body></html>")
	return b.String()
}

func TestPreviewLineStripsHTMLAndIsBounded(t *testing.T) {
	preview := previewLine(bigHTMLBody(), 40)

	assert.NotContains(t, preview, "<", "preview must not contain raw HTML tags")
	assert.NotContains(t, preview, "color:red", "preview must not leak CSS from <style>")
	assert.LessOrEqual(t, len([]rune(preview)), 41, "preview must be bounded to the card width")
	assert.Contains(t, preview, "Invoice", "preview should surface the readable text")
}

// TestRenderListCardFastOnLargeBody guards the O(n²) truncation regression: a card
// built from a ~120 KB HTML body must render in well under a second.
func TestRenderListCardFastOnLargeBody(t *testing.T) {
	m := testModel()
	msg := &models.Message{
		ID:      "big-1",
		From:    "Sender <s@example.com>",
		Subject: "Big HTML",
		Body:    bigHTMLBody(),
		Date:    time.Now(),
	}

	start := time.Now()
	card, _ := renderListCard(m, msg, 44, listCursorActive)
	elapsed := time.Since(start)

	require.NotEmpty(t, card)
	assert.NotContains(t, ansi.Strip(card), "<!DOCTYPE", "card must not render raw HTML")
	assert.Less(t, elapsed, 500*time.Millisecond, "rendering a large card must stay cheap (no O(n²) truncation)")
}

// TestGetFilteredBodyConvertsHTMLInPlainField guards the reader showing raw HTML
// when a server delivers HTML inside the plain Body field.
func TestGetFilteredBodyConvertsHTMLInPlainField(t *testing.T) {
	m := testModel()
	msg := &models.Message{
		ID:   "html-plain",
		Body: "<h1>Электронный счёт</h1><p>за июль</p>",
	}

	out := m.getFilteredBody(msg)

	assert.NotContains(t, out, "<h1>", "HTML in the plain field must be converted, not shown raw")
	assert.Contains(t, out, "Электронный счёт", "the readable text must survive conversion")
}
