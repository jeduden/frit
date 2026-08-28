package herdr

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWithTimeoutPassesAFastCallThrough: a runner that answers within
// the bound returns its output and error unchanged, so bounding a
// healthy herdr costs nothing.
func TestWithTimeoutPassesAFastCallThrough(t *testing.T) {
	boom := errors.New("no herdr socket")
	run := func(args ...string) ([]byte, error) {
		return []byte("panes"), boom
	}

	out, err := WithTimeout(run, time.Second)("agent", "list")
	assert.Equal(t, "panes", string(out))
	assert.ErrorIs(t, err, boom)
}

// TestWithTimeoutBoundsAStalledCall is the hang-fast rule: a herdr
// call that outlasts the bound returns a timeout error within roughly
// the bound, not after the wedged socket eventually unblocks. This is
// the release/board/who hang the seam wiring closes.
func TestWithTimeoutBoundsAStalledCall(t *testing.T) {
	started := make(chan struct{})
	run := func(args ...string) ([]byte, error) {
		close(started)
		time.Sleep(time.Second)

		return []byte("late"), nil
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
