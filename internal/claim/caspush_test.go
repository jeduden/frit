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
		LeaseOptions{PlanID: 7, Remote: "origin"}, markerBeat, "marker-sha", "", "", run)

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
		LeaseOptions{PlanID: 7, Remote: "origin"}, markerBeat, "marker-sha", "", "", run)

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
		LeaseOptions{PlanID: 7, Remote: "origin"}, markerBeat, "marker-sha", "", "", run)

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
		LeaseOptions{PlanID: 7, Remote: "origin"}, markerBeat, "marker-sha", "", "", run)

	require.Error(t, err)
	assert.ErrorIs(t, err, pushErr)
	assert.False(t, lost)
	assert.Empty(t, tip)
}

// TestSyncLocalRefPassesTheSeenValueAndAReason: the move carries the
// value the caller read before minting as update-ref's expected old
// value — never a fresh read, which would reset past a commit made
// while the push was in flight — and the transition as its reflog
// message, so `git reflog` alone recovers the tip it replaced (#189).
func TestSyncLocalRefPassesTheSeenValueAndAReason(t *testing.T) {
	var got []string
	run := func(dir string, args ...string) ([]byte, error) {
		if args[0] == "update-ref" {
			got = args
			return nil, nil
		}
		t.Fatalf("unexpected git call: %v", args)

		return nil, nil
	}

	syncLocalRef("/repo", "refs/heads/plan/7", "new-sha", "frit: plan 7: beat", "old-sha", run)

	assert.Equal(t, []string{"update-ref", "--create-reflog", "-m", "frit: plan 7: beat",
		"refs/heads/plan/7", "new-sha", "old-sha"}, got,
		"--create-reflog: a bare repository logs no branch moves by default")
}

// TestSyncLocalRefCreatesAnAbsentRefOnlyIfStillAbsent: a ref seen
// absent passes the empty old value, update-ref's "must not exist", so
// a branch created since the read is left alone.
func TestSyncLocalRefCreatesAnAbsentRefOnlyIfStillAbsent(t *testing.T) {
	var got []string
	run := func(dir string, args ...string) ([]byte, error) {
		if args[0] == "update-ref" {
			got = args
			return nil, nil
		}
		t.Fatalf("unexpected git call: %v", args)

		return nil, nil
	}

	syncLocalRef("/repo", "refs/heads/plan/7", "new-sha", "frit: plan 7: claim", "", run)

	assert.Equal(t, []string{"update-ref", "--create-reflog", "-m", "frit: plan 7: claim",
		"refs/heads/plan/7", "new-sha", ""}, got)
}

// TestLocalTipReadsTheRefOrEmpty: a readable ref answers its trimmed
// sha; a missing one answers "".
func TestLocalTipReadsTheRefOrEmpty(t *testing.T) {
	assert.Equal(t, "local-sha", localTip("/repo", "refs/heads/plan/7",
		relayRunner(t, "local-sha", nil)))
	assert.Empty(t, localTip("/repo", "refs/heads/plan/7",
		relayRunner(t, "", nil)))
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
			got, seen, err := relayBase("/repo", LeaseOptions{PlanID: 7},
				"refs/heads/plan/7", "from-sha", relayRunner(t, tc.local, tc.ancestors))
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.local, seen, "the sync's expected old value")
		})
	}
}

// TestRelayBaseRefusesADivergedLocalTip: a local tip on neither side of
// the handed tip is refused, naming the branch and both tips.
func TestRelayBaseRefusesADivergedLocalTip(t *testing.T) {
	_, _, err := relayBase("/repo", LeaseOptions{PlanID: 7},
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
