package herdr

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunContextKillsTheChildWhenTheContextExpires is the proof a
// timed-out herdr call is killed rather than abandoned, mirroring
// gitwt's own proof: exec.CommandContext kills sleep the moment ctx
// fires, so Run cannot return until it does.
func TestRunContextKillsTheChildWhenTheContextExpires(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	before := time.Now()
	_, err := runContext(ctx, "sleep", "5")
	elapsed := time.Since(before)

	require.Error(t, err)
	assert.Less(t, elapsed, time.Second)
}

// TestWithTimeoutPassesAFastCallThrough: a runner that answers within
// the bound returns its output and error unchanged, so bounding a
// healthy herdr costs nothing.
func TestWithTimeoutPassesAFastCallThrough(t *testing.T) {
	boom := errors.New("no herdr socket")
	run := func(ctx context.Context, args ...string) ([]byte, error) {
		return []byte("panes"), boom
	}

	out, err := WithTimeout(run, time.Second)("agent", "list")
	assert.Equal(t, "panes", string(out))
	assert.ErrorIs(t, err, boom)
}

// TestWithTimeoutBoundsAStalledCall is the hang-fast rule: a herdr
// call that outlasts the bound returns a timeout error within roughly
// the bound, not after the wedged socket eventually unblocks. This is
// the release/board/who hang the seam wiring closes. The fake obeys
// ctx the way a real killed subprocess does.
func TestWithTimeoutBoundsAStalledCall(t *testing.T) {
	started := make(chan struct{})
	run := func(ctx context.Context, args ...string) ([]byte, error) {
		close(started)
		select {
		case <-time.After(time.Second):
			return []byte("late"), nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	before := time.Now()
	out, err := WithTimeout(run, 10*time.Millisecond)("agent", "list")
	elapsed := time.Since(before)

	<-started
	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "timed out")
	assert.Less(t, elapsed, 500*time.Millisecond)
}
