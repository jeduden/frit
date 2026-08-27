package claim

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCasPushReportsAnUnconfirmedPushWhenTheReconciliationReadFails:
// the push errors and the follow-up ls-remote read used to classify
// it also errors — the same stalled or dropped connection taking out
// both calls. This must not read as a confirmed-absent ref: casPush's
// retry mints a new marker commit, so a blind retry cannot land on
// top of a push that actually succeeded.
func TestCasPushReportsAnUnconfirmedPushWhenTheReconciliationReadFails(t *testing.T) {
	pushErr := errors.New("connection reset")
	readErr := errors.New("git: timed out after 1ms")
	run := func(dir string, args ...string) ([]byte, error) {
		switch args[0] {
		case "push":
			return nil, pushErr
		case "ls-remote":
			return nil, readErr
		}
		t.Fatalf("unexpected git call: %v", args)

		return nil, nil
	}

	lost, tip, err := casPush("/repo", "refs/heads/plan/7",
		LeaseOptions{PlanID: 7, Remote: "origin"}, "marker-sha", "", run)

	require.Error(t, err)
	var unconfirmed *UnconfirmedPushError
	require.ErrorAs(t, err, &unconfirmed)
	assert.ErrorIs(t, err, readErr)
	assert.False(t, lost)
	assert.Empty(t, tip)
}

// TestCasPushTreatsItsOwnLandedMarkerAsAWin: the push errors, but a
// clean ls-remote read shows the marker landed anyway — a connection
// dropped after the ref transaction committed. The transition is
// still ours.
func TestCasPushTreatsItsOwnLandedMarkerAsAWin(t *testing.T) {
	pushErr := errors.New("connection reset")
	run := func(dir string, args ...string) ([]byte, error) {
		switch args[0] {
		case "push":
			return nil, pushErr
		case "ls-remote":
			return []byte("marker-sha\trefs/heads/plan/7\n"), nil
		case "update-ref":
			return nil, nil
		}
		t.Fatalf("unexpected git call: %v", args)

		return nil, nil
	}

	lost, tip, err := casPush("/repo", "refs/heads/plan/7",
		LeaseOptions{PlanID: 7, Remote: "origin"}, "marker-sha", "", run)

	require.NoError(t, err)
	assert.False(t, lost)
	assert.Equal(t, "marker-sha", tip)
}

// TestCasPushReportsALostRaceWhenAnotherMarkerWon: the push errors,
// and a clean ls-remote read shows a different sha holding the ref —
// another machine won the race.
func TestCasPushReportsALostRaceWhenAnotherMarkerWon(t *testing.T) {
	pushErr := errors.New("stale info")
	run := func(dir string, args ...string) ([]byte, error) {
		switch args[0] {
		case "push":
			return nil, pushErr
		case "ls-remote":
			return []byte("winner-sha\trefs/heads/plan/7\n"), nil
		}
		t.Fatalf("unexpected git call: %v", args)

		return nil, nil
	}

	lost, tip, err := casPush("/repo", "refs/heads/plan/7",
		LeaseOptions{PlanID: 7, Remote: "origin"}, "marker-sha", "", run)

	require.NoError(t, err)
	assert.True(t, lost)
	assert.Equal(t, "winner-sha", tip)
}

// TestCasPushReportsARealFaultWhenTheRefIsGenuinelyAbsent: the push
// errors, and a clean ls-remote read confirms the ref carries nothing
// at all — a real fault, not a lost arbitration or an unconfirmed one.
func TestCasPushReportsARealFaultWhenTheRefIsGenuinelyAbsent(t *testing.T) {
	pushErr := errors.New("remote rejected")
	run := func(dir string, args ...string) ([]byte, error) {
		switch args[0] {
		case "push":
			return nil, pushErr
		case "ls-remote":
			return []byte(""), nil
		}
		t.Fatalf("unexpected git call: %v", args)

		return nil, nil
	}

	lost, tip, err := casPush("/repo", "refs/heads/plan/7",
		LeaseOptions{PlanID: 7, Remote: "origin"}, "marker-sha", "", run)

	require.Error(t, err)
	assert.ErrorIs(t, err, pushErr)
	assert.False(t, lost)
	assert.Empty(t, tip)
}
