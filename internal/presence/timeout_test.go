package presence

import (
	"context"
	"testing"
	"time"

	"github.com/jeduden/frit/internal/herdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWithTimeoutKillsAGenuinelySlowProbe is the proof a timed-out
// probe is killed rather than abandoned, mirroring gitwt's and
// herdr's own proof: wrapping the real context-aware herdr core
// around a genuinely slow child (sleep) under a 20ms bound returns in
// under a second, which is only possible because exec.CommandContext
// killed it.
func TestWithTimeoutKillsAGenuinelySlowProbe(t *testing.T) {
	before := time.Now()
	_, err := WithTimeout(herdr.RunContext, 20*time.Millisecond)("sleep", "5")
	elapsed := time.Since(before)

	require.Error(t, err)
	assert.Less(t, elapsed, time.Second)
}

// TestWithTimeoutPassesAPromptReadThrough: a runner that answers within
// the bound returns its output unchanged, timeout or not.
func TestWithTimeoutPassesAPromptReadThrough(t *testing.T) {
	exec := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte("ok"), nil
	}

	out, err := WithTimeout(exec, time.Second)("herdr", "agent", "list")
	require.NoError(t, err)
	assert.Equal(t, "ok", string(out))
}

// TestWithTimeoutBoundsASlowHost is the "merely slow is treated the
// same as dead" rule: a runner that outlasts the bound returns a
// timeout error instead of hanging, so reconciliation renders it stale
// rather than blocking the board on it. The fake obeys ctx the way a
// real killed subprocess does.
func TestWithTimeoutBoundsASlowHost(t *testing.T) {
	started := make(chan struct{})
	exec := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		close(started)
		select {
		case <-time.After(time.Second):
			return []byte("late"), nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	out, err := WithTimeout(exec, time.Millisecond)("ssh", "box", "herdr")
	<-started
	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "timed out")
}
