package herdr

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFocusTargetsThePane sends the pane straight to `agent focus` and
// nothing else — no text, no agent start. It is the read-only handoff.
func TestFocusTargetsThePane(t *testing.T) {
	var got []string
	runner := func(args ...string) ([]byte, error) {
		got = args

		return nil, nil
	}

	require.NoError(t, Focus(runner, "wC:p1"))
	assert.Equal(t, []string{"agent", "focus", "wC:p1"}, got)
}

// TestFocusReturnsTheRunnerError hands a failed focus straight up, so
// the command can report it rather than pretending the pane was raised.
func TestFocusReturnsTheRunnerError(t *testing.T) {
	want := errors.New("no such pane")
	err := Focus(func(...string) ([]byte, error) { return nil, want }, "gone")
	assert.ErrorIs(t, err, want)
}
