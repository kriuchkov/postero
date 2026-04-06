package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTemplateVariables(t *testing.T) {
	variables, err := parseTemplateVariables([]string{"tone=warm", " language = en "})

	require.NoError(t, err)
	assert.Equal(t, map[string]string{"tone": "warm", "language": "en"}, variables)
}

func TestParseTemplateVariablesRejectsInvalidEntries(t *testing.T) {
	_, err := parseTemplateVariables([]string{"missing-separator"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected key=value")
}
