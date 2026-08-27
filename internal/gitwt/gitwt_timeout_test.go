package gitwt

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWithTimeoutPassesAFastCallThrough: a runner that answers within
// the bound returns its output and error unchanged.
func TestWithTimeoutPassesAFastCallThrough(t *testing.T) {
	run := func(dir string, args ...string) ([]byte, error) {
		return []byte("ok"), nil
	}

	out, err := WithTimeout(run, time.Second)("/repo", "status")
	require.NoError(t, err)
	assert.Equal(t, "ok", string(out))
}

// TestWithTimeoutBoundsAStalledCall is the hang-fast rule: a runner
// that outlasts the bound returns a timeout error within roughly the
// bound, not after the fake eventually unblocks.
func TestWithTimeoutBoundsAStalledCall(t *testing.T) {
	started := make(chan struct{})
	run := func(dir string, args ...string) ([]byte, error) {
		close(started)
		time.Sleep(time.Second)

		return []byte("late"), nil
	}

	before := time.Now()
	out, err := WithTimeout(run, 10*time.Millisecond)("/repo", "fetch")
	elapsed := time.Since(before)

	<-started
	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "timed out")
	assert.Less(t, elapsed, 500*time.Millisecond)
}
