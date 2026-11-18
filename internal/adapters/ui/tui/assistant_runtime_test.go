package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kriuchkov/postero/internal/core/models"
)

func TestParseAICommandOptionsTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		raw         string
		template    string
		instruction string
		wantErr     bool
	}{
		{name: "empty", raw: "   "},
		{name: "instruction only", raw: "write a short reply", instruction: "write a short reply"},
		{name: "template equals", raw: "--template=formal be polite", template: "formal", instruction: "be polite"},
		{name: "short equals", raw: "-t=casual hey there", template: "casual", instruction: "hey there"},
		{name: "bare template equals", raw: "template=direct", template: "direct"},
		{name: "template flag with value", raw: "--template formal be polite", template: "formal", instruction: "be polite"},
		{name: "short flag with value", raw: "-t casual", template: "casual"},
		{name: "template equals empty", raw: "--template=", wantErr: true},
		{name: "short equals empty", raw: "-t=", wantErr: true},
		{name: "bare equals empty", raw: "template=", wantErr: true},
		{name: "flag without value", raw: "--template", wantErr: true},
		{name: "short flag without value", raw: "-t", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			options, err := parseAICommandOptions(tc.raw)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.template, options.template)
			assert.Equal(t, tc.instruction, options.instruction)
		})
	}
}

func TestComposeDraftHasContent(t *testing.T) {
	t.Parallel()

	assert.False(t, composeDraftHasContent(nil))
	assert.False(t, composeDraftHasContent(&models.Message{}))
	assert.False(t, composeDraftHasContent(&models.Message{To: []string{"  "}}), "blank recipients do not count")
	assert.True(t, composeDraftHasContent(&models.Message{Subject: "s"}))
	assert.True(t, composeDraftHasContent(&models.Message{Body: "b"}))
	assert.True(t, composeDraftHasContent(&models.Message{To: []string{"a@b.c"}}))
	assert.True(t, composeDraftHasContent(&models.Message{Bcc: []string{"a@b.c"}}))
}
