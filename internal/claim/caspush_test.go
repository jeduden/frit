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
		LeaseOptions{PlanID: 7, Remote: "origin"}, markerBeat, "marker-sha", "", run)

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
		case "rev-parse", "update-ref":
			return nil, nil
		}
		t.Fatalf("unexpected git call: %v", args)

		return nil, nil
	}

	lost, tip, err := casPush("/repo", "refs/heads/plan/7",
		LeaseOptions{PlanID: 7, Remote: "origin"}, markerBeat, "marker-sha", "", run)

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
		LeaseOptions{PlanID: 7, Remote: "origin"}, markerBeat, "marker-sha", "", run)

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
		LeaseOptions{PlanID: 7, Remote: "origin"}, markerBeat, "marker-sha", "", run)

	require.Error(t, err)
	assert.ErrorIs(t, err, pushErr)
	assert.False(t, lost)
	assert.Empty(t, tip)
}

// TestSyncLocalRefPassesThePriorValueAndAReason: the move carries the
// ref's value just before it as update-ref's expected old value and
// the transition as its reflog message, so `git reflog` alone recovers
// the tip it replaced (#189).
func TestSyncLocalRefPassesThePriorValueAndAReason(t *testing.T) {
	var got []string
	run := func(dir string, args ...string) ([]byte, error) {
		switch args[0] {
		case "rev-parse":
			return []byte("old-sha\n"), nil
		case "update-ref":
			got = args
			return nil, nil
		}
		t.Fatalf("unexpected git call: %v", args)

		return nil, nil
	}

	syncLocalRef("/repo", "refs/heads/plan/7", "new-sha", "frit: plan 7: beat", run)

	assert.Equal(t, []string{"update-ref", "-m", "frit: plan 7: beat",
		"refs/heads/plan/7", "new-sha", "old-sha"}, got)
}

// TestSyncLocalRefCreatesAnAbsentRefWithTheTwoArgumentForm: a ref with
// no prior value has no old value to pass, so the move falls back to
// creating it, still carrying the reason.
func TestSyncLocalRefCreatesAnAbsentRefWithTheTwoArgumentForm(t *testing.T) {
	var got []string
	run := func(dir string, args ...string) ([]byte, error) {
		switch args[0] {
		case "rev-parse":
			return nil, errors.New("exit status 1")
		case "update-ref":
			got = args
			return nil, nil
		}
		t.Fatalf("unexpected git call: %v", args)

		return nil, nil
	}

	syncLocalRef("/repo", "refs/heads/plan/7", "new-sha", "frit: plan 7: claim", run)

	assert.Equal(t, []string{"update-ref", "-m", "frit: plan 7: claim",
		"refs/heads/plan/7", "new-sha"}, got)
}

// TestRelayBaseChoosesTheParentByTheLocalTipsAncestry: each shape of
// the local work ref against the tip a renewal is handed — absent,
// equal, ahead, behind, diverged — picks its own parent or refuses.
func TestRelayBaseChoosesTheParentByTheLocalTipsAncestry(t *testing.T) {
	cases := []struct {
		name      string
		local     string // "" means no local ref
		ancestors map[string]bool
		want      string
	}{
		{name: "absent", want: "from-sha"},
		{name: "equal", local: "from-sha", want: "from-sha"},
		{name: "ahead", local: "local-sha",
			ancestors: map[string]bool{"from-sha local-sha": true}, want: "local-sha"},
		{name: "behind", local: "local-sha",
			ancestors: map[string]bool{"local-sha from-sha": true}, want: "from-sha"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := relayBase("/repo", LeaseOptions{PlanID: 7},
				"refs/heads/plan/7", "from-sha", relayRunner(t, tc.local, tc.ancestors))
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestRelayBaseRefusesADivergedLocalTip: a local tip on neither side of
// the handed tip is refused, naming the branch and both tips.
func TestRelayBaseRefusesADivergedLocalTip(t *testing.T) {
	_, err := relayBase("/repo", LeaseOptions{PlanID: 7},
		"refs/heads/plan/7", "from-sha", relayRunner(t, "local-sha", nil))

	var diverges *LeaseDivergesError
	require.ErrorAs(t, err, &diverges)
	assert.Equal(t, LeaseDivergesError{PlanID: 7, Branch: "plan/7",
		LocalTip: "local-sha", From: "from-sha"}, *diverges)
}

// relayRunner fakes the two reads relayBase makes: the local ref's
// value ("" for absent) and merge-base --is-ancestor, answered yes only
// for the "ancestor descendant" pairs listed.
func relayRunner(t *testing.T, local string, ancestors map[string]bool) func(string, ...string) ([]byte, error) {
	return func(dir string, args ...string) ([]byte, error) {
		switch args[0] {
		case "rev-parse":
			if local == "" {
				return nil, errors.New("exit status 1")
			}

			return []byte(local + "\n"), nil
		case "merge-base":
			if ancestors[args[2]+" "+args[3]] {
				return nil, nil
			}

			return nil, errors.New("exit status 1")
		}
		t.Fatalf("unexpected git call: %v", args)

		return nil, nil
	}
}
