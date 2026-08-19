package presence

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWithTimeoutPassesAPromptReadThrough: a runner that answers within
// the bound returns its output unchanged, timeout or not.
func TestWithTimeoutPassesAPromptReadThrough(t *testing.T) {
	exec := func(name string, args ...string) ([]byte, error) {
		return []byte("ok"), nil
	}

	out, err := WithTimeout(exec, time.Second)("herdr", "agent", "list")
	require.NoError(t, err)
	assert.Equal(t, "ok", string(out))
}

// TestWithTimeoutBoundsASlowHost is the "merely slow is treated the
// same as dead" rule: a runner that outlasts the bound returns a
// timeout error instead of hanging, so reconciliation renders it stale
// rather than blocking the board on it.
func TestWithTimeoutBoundsASlowHost(t *testing.T) {
	started := make(chan struct{})
	exec := func(name string, args ...string) ([]byte, error) {
		close(started)
		time.Sleep(time.Second)

		return []byte("late"), nil
	}

	out, err := WithTimeout(exec, time.Millisecond)("ssh", "box", "herdr")
	<-started
	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "timed out")
}
