package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// openAIReplyPrompt shows the modal that collects an instruction for the AI reply.
func (m *Model) openAIReplyPrompt(replyAll bool) {
	m.aiPromptActive = true
	m.aiPromptReplyAll = replyAll
	m.aiPromptInput.SetValue("")
	m.aiPromptInput.CursorEnd()
	m.aiPromptInput.Focus()
}

func (m *Model) closeAIReplyPrompt() {
	m.aiPromptActive = false
	m.aiPromptInput.Blur()
	m.aiPromptInput.SetValue("")
}

// submitAIReplyPrompt closes the popup and kicks off AI reply generation with the
// typed instruction, reusing the same loading indicator as the `:reply-ai` command.
func (m Model) submitAIReplyPrompt() (tea.Model, tea.Cmd) {
	instruction := strings.TrimSpace(m.aiPromptInput.Value())
	replyAll := m.aiPromptReplyAll
	m.closeAIReplyPrompt()

	label := "AI reply"
	if replyAll {
		label = "AI reply-all"
	}
	m.prepareAIGeneration(label)
	m.setStatus("Generating " + strings.ToLower(label) + "…")
	return m, m.withAILoadingIndicator(m.generateReplyAIDraft(aiCommandOptions{instruction: instruction}, replyAll))
}

// renderAIReplyPopup draws the centered modal box (title, the message it replies
// to, the instruction input, and the key hints).
//
// Every cell between the borders must carry an explicit Surface background:
// lipgloss's outer Background stops at the first inner ANSI reset, so a box
// composed from separately styled substrings leaves transparent gaps that show
// the underlying pane through a translucent terminal. Each row is therefore a
// single style over plain text at the full content width (margins included),
// and the one row that must contain inner ANSI — the text input — gets the
// background via the input's own styles instead.
func (m Model) renderAIReplyPopup() string {
	p := m.styles.Palette
	boxWidth := clampInt(m.width-8, 44, 76)
	contentWidth := max(boxWidth-2, 24) // minus the border columns
	inner := contentWidth - 4           // minus the baked-in 2-cell side margins

	surface := lipgloss.NewStyle().Background(p.Surface)
	row := surface.Width(contentWidth)
	margin := "  "
	blank := row.Render("")

	rows := []string{
		blank,
		row.Foreground(p.Highlight).Bold(true).Render(margin + "Reply with AI"),
		blank,
	}
	if sel, ok := m.selectedMessage(); ok {
		meta := row.Foreground(p.SubText)
		rows = append(rows, meta.Render(margin+"To  "+truncateText(strings.TrimSpace(sel.From), inner-4)))
		if subject := strings.TrimSpace(sel.Subject); subject != "" {
			rows = append(rows, meta.Render(margin+"Re  "+truncateText(subject, inner-4)))
		}
		rows = append(rows, blank)
	}

	input := m.aiPromptInput
	input.Width = max(inner-lipgloss.Width(input.Prompt)-1, 8)
	input.PromptStyle = surface.Foreground(p.Primary)
	input.TextStyle = surface.Foreground(p.Text)
	input.PlaceholderStyle = surface.Foreground(p.Faint)
	input.CompletionStyle = surface.Foreground(p.Faint)
	input.Cursor.Style = surface.Foreground(p.Highlight)
	input.Cursor.TextStyle = surface
	inputView := input.View()
	if pad := contentWidth - len(margin) - lipgloss.Width(inputView); pad > 0 {
		inputView += surface.Render(strings.Repeat(" ", pad))
	}
	rows = append(rows,
		surface.Render(margin)+inputView,
		blank,
		row.Foreground(p.SubText).Render(margin+"Enter  generate   ·   Esc  cancel"),
		blank,
	)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.Primary).
		BorderBackground(p.Surface).
		Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

// overlayCenter composites box centered on top of base, keeping the base visible
// around it. base is expected to be width×height cells.
func overlayCenter(base, box string, width, height int) string {
	if width <= 0 || height <= 0 {
		return base
	}
	lines := strings.Split(base, "\n")
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	boxLines := strings.Split(box, "\n")
	boxWidth := lipgloss.Width(box)
	top := max((height-len(boxLines))/2, 0)
	left := max((width-boxWidth)/2, 0)
	for i, boxLine := range boxLines {
		row := top + i
		if row < 0 || row >= len(lines) {
			continue
		}
		lines[row] = spliceLine(lines[row], boxLine, left, boxWidth)
	}
	return strings.Join(lines, "\n")
}

// spliceLine replaces insW cells starting at column col of base with ins,
// preserving the ANSI styling on either side.
func spliceLine(base, ins string, col, insW int) string {
	leftPart := ansi.Truncate(base, col, "")
	if pad := col - lipgloss.Width(leftPart); pad > 0 {
		leftPart += strings.Repeat(" ", pad)
	}
	rightPart := ansi.TruncateLeft(base, col+insW, "")
	return leftPart + "\x1b[0m" + ins + "\x1b[0m" + rightPart
}
