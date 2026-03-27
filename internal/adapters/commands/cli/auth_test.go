package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFirstNonEmptyReturnsFirstTrimmedCandidate(t *testing.T) {
	assert.Equal(t, "value", firstNonEmpty("   ", "value", "fallback"))
}

func TestFirstNonEmptyReturnsEmptyWhenAllBlank(t *testing.T) {
	assert.Empty(t, firstNonEmpty("", "   "))
}
