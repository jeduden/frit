package gitwt

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunContextKillsTheChildWhenTheContextExpires is the proof a
// timed-out call is killed rather than abandoned: exec.CommandContext
// kills sleep the moment ctx fires, so Run cannot return until it
// does. A fast return here is only possible because the child died,
// not because the wait merely gave up on it.
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
// bound, not after the fake eventually unblocks. The error names the
// subcommand that stalled, the way Exec's own error does, so a
// multi-repo run doesn't leave the reader guessing which git call hung.
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
	assert.Contains(t, err.Error(), "fetch")
	assert.Less(t, elapsed, 500*time.Millisecond)
}

// TestWithTimeoutPipePassesAFastCallThrough: a pipe runner that
// answers within the bound returns its output and error unchanged.
func TestWithTimeoutPipePassesAFastCallThrough(t *testing.T) {
	run := func(dir string, stdin []byte, args ...string) ([]byte, error) {
		return []byte("ok"), nil
	}

	out, err := WithTimeoutPipe(run, time.Second)(
		"/repo", []byte("request"), "cat-file", "--batch")
	require.NoError(t, err)
	assert.Equal(t, "ok", string(out))
}

// TestWithTimeoutPipeBoundsAStalledCall: the batch cat-file path gets
// the same hang-fast bound as an ordinary Runner call — a partial
// clone's promisor remote can pull a missing object over the network
// on demand, so this path is not purely local either.
func TestWithTimeoutPipeBoundsAStalledCall(t *testing.T) {
	started := make(chan struct{})
	run := func(dir string, stdin []byte, args ...string) ([]byte, error) {
		close(started)
		time.Sleep(time.Second)

		return []byte("late"), nil
	}

	before := time.Now()
	out, err := WithTimeoutPipe(run, 10*time.Millisecond)(
		"/repo", []byte("request"), "cat-file", "--batch")
	elapsed := time.Since(before)

	<-started
	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "timed out")
	assert.Less(t, elapsed, 500*time.Millisecond)
}

// TestWithDeadlinePassesAFastCallThrough: a call well inside the
// deadline returns its output and error unchanged.
func TestWithDeadlinePassesAFastCallThrough(t *testing.T) {
	run := func(dir string, args ...string) ([]byte, error) {
		return []byte("ok"), nil
	}

	out, err := WithDeadline(run, time.Now().Add(time.Second))(
		"/repo", "status")
	require.NoError(t, err)
	assert.Equal(t, "ok", string(out))
}

// TestWithDeadlineSharesOneBudgetAcrossSequentialCalls is the
// compounding-latency fix: unlike WithTimeout, which re-arms a fixed
// duration on every call, WithDeadline's calls share one clock. A
// call that spends most of the budget leaves the next call only what
// remains, not a fresh full duration.
func TestWithDeadlineSharesOneBudgetAcrossSequentialCalls(t *testing.T) {
	wrapped := WithDeadline(
		func(dir string, args ...string) ([]byte, error) {
			time.Sleep(70 * time.Millisecond)

			return []byte("ok"), nil
		},
		time.Now().Add(100*time.Millisecond))

	_, err := wrapped("/repo", "fetch")
	require.NoError(t, err, "the first call fits inside the deadline")

	before := time.Now()
	_, err = wrapped("/repo", "push")
	elapsed := time.Since(before)

	require.Error(t, err,
		"the second call gets only the ~30ms left of the shared "+
			"deadline, not a fresh 100ms, so its own 70ms call times out")
	assert.Less(t, elapsed, 50*time.Millisecond)
}

// TestWithDeadlineFailsImmediatelyOnceTheBudgetIsExhausted: a call made
// after the deadline has already passed returns at once, without
// starting the wrapped runner at all — the fourth of four sequential
// calls against an already-spent budget must not wait out another
// full --git-timeout before failing.
func TestWithDeadlineFailsImmediatelyOnceTheBudgetIsExhausted(t *testing.T) {
	called := false
	run := func(dir string, args ...string) ([]byte, error) {
		called = true

		return []byte("ok"), nil
	}

	before := time.Now()
	out, err := WithDeadline(run, time.Now().Add(-time.Millisecond))(
		"/repo", "status")
	elapsed := time.Since(before)

	require.Error(t, err)
	assert.Nil(t, out)
	assert.False(t, called,
		"a call made after the budget is spent must not start")
	assert.Less(t, elapsed, 20*time.Millisecond)
}
