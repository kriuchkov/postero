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
func (m Model) renderAIReplyPopup() string {
	p := m.styles.Palette
	boxWidth := clampInt(m.width-8, 44, 76)
	inner := max(boxWidth-6, 20) // minus border (2) + padding (4)

	surface := lipgloss.NewStyle().Background(p.Surface)
	title := surface.Foreground(p.Highlight).Bold(true).Render("Reply with AI")

	rows := []string{title, ""}
	if sel, ok := m.selectedMessage(); ok {
		rows = append(rows, surface.Foreground(p.SubText).Render("To  "+truncateText(strings.TrimSpace(sel.From), inner-4)))
		if subject := strings.TrimSpace(sel.Subject); subject != "" {
			rows = append(rows, surface.Foreground(p.SubText).Render("Re  "+truncateText(subject, inner-4)))
		}
		rows = append(rows, "")
	}

	input := m.aiPromptInput
	input.Width = max(inner-2, 8)
	rows = append(rows,
		surface.Render(input.View()),
		"",
		surface.Foreground(p.SubText).Render("Enter  generate   ·   Esc  cancel"),
	)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.Primary).
		BorderBackground(p.Surface).
		Background(p.Surface).
		Padding(1, 2).
		Width(boxWidth).
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
