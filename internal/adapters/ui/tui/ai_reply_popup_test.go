package tui

import (
	"strings"
	"testing"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kriuchkov/postero/internal/core/models"
)

func aiPopupModel(t *testing.T) (Model, *draftAssistantStub) {
	t.Helper()
	service := &messageServiceStub{inbox: sampleMessages()}
	assistant := &draftAssistantStub{response: &models.GeneratedDraft{Subject: "Re: Subject 1", Body: "Sure, that works."}}
	m := testModelWithService(service)
	m.assistant = assistant
	m.width, m.height = 120, 40
	m.state = stateList
	return m, assistant
}

func TestReplyAIKeyOpensPopup(t *testing.T) {
	m, _ := aiPopupModel(t)

	m = updateModel(t, m, keyRune('A'))

	require.True(t, m.aiPromptActive, "A opens the AI reply popup")
	assert.True(t, m.aiPromptInput.Focused())
}

func TestReplyAIKeyWithoutAssistantErrors(t *testing.T) {
	m, _ := aiPopupModel(t)
	m.assistant = nil

	m = updateModel(t, m, keyRune('A'))

	assert.False(t, m.aiPromptActive)
	assert.Contains(t, m.statusMessage, "not configured")
}

func TestReplyAIPopupSubmitGeneratesReply(t *testing.T) {
	m, assistant := aiPopupModel(t)

	m = updateModel(t, m, keyRune('A'))
	require.True(t, m.aiPromptActive)
	m.aiPromptInput.SetValue("Politely decline and suggest next week")

	updated, cmd := updateModelWithCmd(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	assert.False(t, updated.aiPromptActive, "submitting closes the popup")
	assert.Contains(t, updated.statusMessage, "Generating")

	updated = updateModel(t, updated, cmd())
	require.Len(t, assistant.requests, 1)
	assert.Equal(t, "reply", assistant.requests[0].Mode)
	assert.Equal(t, "Politely decline and suggest next week", assistant.requests[0].Instruction)
	require.NotNil(t, assistant.requests[0].Original)
	assert.Equal(t, stateCompose, updated.state)
	require.NotNil(t, updated.activeDraft)
	assert.Contains(t, updated.activeDraft.Body, "Sure, that works.")
}

func TestReplyAIPopupEscCancels(t *testing.T) {
	m, assistant := aiPopupModel(t)

	m = updateModel(t, m, keyRune('A'))
	require.True(t, m.aiPromptActive)

	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyEsc})

	assert.False(t, m.aiPromptActive)
	assert.Contains(t, m.statusMessage, "cancelled")
	assert.Empty(t, assistant.requests, "cancelling must not call the assistant")
}

// TestReplyAIPopupHasSolidBackground guards the "floating background" bug: on a
// translucent terminal any cell inside the modal without an explicit background
// shows the pane underneath. Walk every rendered cell and require an active
// Surface background — lipgloss's outer Background stops at inner ANSI resets,
// so this only holds when each row paints itself.
func TestReplyAIPopupHasSolidBackground(t *testing.T) {
	forceColorProfile(t)
	m, _ := aiPopupModel(t)
	m = updateModel(t, m, keyRune('A'))
	require.True(t, m.aiPromptActive)

	box := m.renderAIReplyPopup()
	surfaceBg := "48;5;" + string(m.styles.Palette.Surface)

	for lineNo, line := range strings.Split(box, "\n") {
		bgActive := false
		i := 0
		for i < len(line) {
			if strings.HasPrefix(line[i:], "\x1b[") {
				end := strings.Index(line[i:], "m")
				require.GreaterOrEqual(t, end, 0, "line %d: unterminated ANSI sequence", lineNo)
				params := line[i+2 : i+end]
				switch {
				case strings.Contains(params, surfaceBg):
					bgActive = true
				case params == "" || params == "0" || strings.HasPrefix(params, "0;"):
					bgActive = false
				}
				i += end + 1
				continue
			}
			r, size := utf8.DecodeRuneInString(line[i:])
			assert.True(t, bgActive,
				"line %d: cell %q at byte %d has no surface background — transparent gap", lineNo, string(r), i)
			i += size
		}
	}
}

func TestReplyAIPopupRendersOverAndKeepsContext(t *testing.T) {
	m, _ := aiPopupModel(t)
	m = updateModel(t, m, keyRune('A'))

	view := ansi.Strip(m.View())

	assert.Contains(t, view, "Reply with AI", "the modal title is shown")
	assert.Contains(t, view, "Enter", "the key hint is shown")
	// The base UI is still visible around the popup (composited, not replaced).
	assert.Contains(t, view, "Postero")
}
