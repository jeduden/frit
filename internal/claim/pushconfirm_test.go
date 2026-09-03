package claim

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPushThenConfirmSkipsTheReadWhenThePushSucceeds: nothing to
// classify, so the confirming read must not run at all — a success
// costs exactly one git call, the way each of the three call sites
// costs it today.
func TestPushThenConfirmSkipsTheReadWhenThePushSucceeds(t *testing.T) {
	var calls [][]string
	run := func(dir string, args ...string) ([]byte, error) {
		calls = append(calls, args)

		return nil, nil
	}

	pushErr, now, readErr := pushThenConfirm("/repo",
		LeaseOptions{PlanID: 7, Remote: "origin"},
		"refs/heads/plan/7", "old-sha", "marker-sha", run)

	require.NoError(t, pushErr)
	require.NoError(t, readErr)
	assert.Empty(t, now)
	require.Len(t, calls, 1)
	assert.Equal(t, []string{"push",
		"--force-with-lease=refs/heads/plan/7:old-sha",
		"origin", "marker-sha:refs/heads/plan/7"}, calls[0])
}

// TestPushThenConfirmDeletesWhenTheSourceIsEmpty: an empty source is
// the delete refspec ":<ref>", which is how Scavenge's confirmation
// arrives here.
func TestPushThenConfirmDeletesWhenTheSourceIsEmpty(t *testing.T) {
	var calls [][]string
	run := func(dir string, args ...string) ([]byte, error) {
		calls = append(calls, args)

		return nil, nil
	}

	_, _, _ = pushThenConfirm("/repo",
		LeaseOptions{PlanID: 7, Remote: "origin"},
		"refs/heads/plan/7", "tip-sha", "", run)

	require.Len(t, calls, 1)
	assert.Equal(t, []string{"push",
		"--force-with-lease=refs/heads/plan/7:tip-sha",
		"origin", ":refs/heads/plan/7"}, calls[0])
}

// TestPushThenConfirmReadsTheRefWhenThePushFails: the push's own
// error comes back alongside what the remote says holds the ref now,
// so each caller can run its own failure-shape switch on it.
func TestPushThenConfirmReadsTheRefWhenThePushFails(t *testing.T) {
	wanted := errors.New("stale info")
	run := func(dir string, args ...string) ([]byte, error) {
		switch args[0] {
		case "push":
			return nil, wanted
		case "ls-remote":
			return []byte("other-sha\trefs/heads/plan/7\n"), nil
		}
		t.Fatalf("unexpected git call: %v", args)

		return nil, nil
	}

	pushErr, now, readErr := pushThenConfirm("/repo",
		LeaseOptions{PlanID: 7, Remote: "origin"},
		"refs/heads/plan/7", "old-sha", "marker-sha", run)

	assert.ErrorIs(t, pushErr, wanted)
	require.NoError(t, readErr)
	assert.Equal(t, "other-sha", now)
}

// TestPushThenConfirmKeepsAnUnreadableRemoteApartFromAnAbsentRef: the
// read fault stays its own answer and never folds into "", which is
// the one step all three call sites must not get wrong independently.
func TestPushThenConfirmKeepsAnUnreadableRemoteApartFromAnAbsentRef(t *testing.T) {
	pushErr := errors.New("connection reset")
	wanted := errors.New("git: timed out after 1ms")
	run := func(dir string, args ...string) ([]byte, error) {
		switch args[0] {
		case "push":
			return nil, pushErr
		case "ls-remote":
			return nil, wanted
		}
		t.Fatalf("unexpected git call: %v", args)

		return nil, nil
	}

	gotPush, now, readErr := pushThenConfirm("/repo",
		LeaseOptions{PlanID: 7, Remote: "origin"},
		"refs/heads/plan/7", "old-sha", "marker-sha", run)

	assert.ErrorIs(t, gotPush, pushErr)
	assert.ErrorIs(t, readErr, wanted)
	assert.Empty(t, now)
}
