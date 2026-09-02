package claim

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParkSurfacesThePushsOwnErrorWhenTheRescueRefIsAbsent: the push
// fails and the confirmation read finds nothing at the rescue ref at
// all — a rejected or slow pre-push hook, a timeout, a genuine
// rejection. This is not a hand-moved ref, so the refusal must not
// claim one; it must carry the push's own fault.
func TestParkSurfacesThePushsOwnErrorWhenTheRescueRefIsAbsent(t *testing.T) {
	pushErr := errors.New("hook declined: pre-push rejected")
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

	err := park("/repo",
		LeaseOptions{PlanID: 7, Remote: "origin"}, "refs/frit/rescue/7/x", "tip-sha", run)

	require.Error(t, err)
	assert.ErrorIs(t, err, pushErr)
	assert.Contains(t, err.Error(), pushErr.Error())
	assert.NotContains(t, err.Error(), "moved by hand")
}

// TestParkReportsAnUnconfirmedParkWhenTheConfirmationReadAlsoFails:
// the push fails and the confirmation read itself fails too — the
// same stalled or dropped connection took out both calls. The park
// cannot be classified as absent, landed or conflicting, so the error
// must say so honestly and carry both faults.
func TestParkReportsAnUnconfirmedParkWhenTheConfirmationReadAlsoFails(t *testing.T) {
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

	err := park("/repo",
		LeaseOptions{PlanID: 7, Remote: "origin"}, "refs/frit/rescue/7/x", "tip-sha", run)

	require.Error(t, err)
	var unconfirmed *UnconfirmedPushError
	require.ErrorAs(t, err, &unconfirmed,
		"an unconfirmable park is a typed UnconfirmedPushError")
	assert.ErrorIs(t, err, pushErr, "the push's own fault is reachable via unwrap")
	assert.ErrorIs(t, err, readErr, "the read's own fault is reachable via unwrap")
	assert.NotContains(t, err.Error(), "moved by hand")
}

// TestParkNamesBothCommitsOnAGenuineConflict: the push fails and the
// confirmation read finds a different object at the rescue ref's
// exact content-addressed name — the one case "moved by hand" is
// actually true of. The refusal now names both the sha it found and
// the tip it was trying to park, so the mismatch is self-diagnosing.
func TestParkNamesBothCommitsOnAGenuineConflict(t *testing.T) {
	pushErr := errors.New("stale info")
	run := func(dir string, args ...string) ([]byte, error) {
		switch args[0] {
		case "push":
			return nil, pushErr
		case "ls-remote":
			return []byte("found-sha\trefs/frit/rescue/7/x\n"), nil
		}
		t.Fatalf("unexpected git call: %v", args)

		return nil, nil
	}

	err := park("/repo",
		LeaseOptions{PlanID: 7, Remote: "origin"}, "refs/frit/rescue/7/x", "tip-sha", run)

	require.Error(t, err)
	var conflict *RescueConflictError
	require.ErrorAs(t, err, &conflict)
	assert.Equal(t, int64(7), conflict.PlanID)
	assert.Equal(t, "refs/frit/rescue/7/x", conflict.Rescue)
	assert.Contains(t, err.Error(), "found-sha", "names the sha found")
	assert.Contains(t, err.Error(), "tip-sha", "names the tip being parked")
}

// TestParkIsANoOpWhenTheReadConfirmsTheTipAlreadyLanded: the push
// fails, but the confirmation read shows the rescue ref already
// holds exactly this tip — an earlier half-done run parked it first.
// This is the same outcome TestScavengeIsIdempotent pins end to end;
// this unit test pins it at the park boundary too.
func TestParkIsANoOpWhenTheReadConfirmsTheTipAlreadyLanded(t *testing.T) {
	pushErr := errors.New("already exists")
	run := func(dir string, args ...string) ([]byte, error) {
		switch args[0] {
		case "push":
			return nil, pushErr
		case "ls-remote":
			return []byte("tip-sha\trefs/frit/rescue/7/x\n"), nil
		}
		t.Fatalf("unexpected git call: %v", args)

		return nil, nil
	}

	err := park("/repo",
		LeaseOptions{PlanID: 7, Remote: "origin"}, "refs/frit/rescue/7/x", "tip-sha", run)

	require.NoError(t, err)
}
